package httpapi

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

const (
	requestIDHeader       = "X-Request-ID"
	requestIDByteLength   = 16
	maxRequestIDLength    = 128
	csrfTokenHeader       = "X-CSRF-Token"
	preflightRoutePattern = "OPTIONS (preflight)"
	unmatchedRoutePattern = "unmatched"
)

type requestStateContextKey struct{}
type requestOriginContextKey struct{}
type sessionContextKey struct{}

type requestState struct {
	requestID    string
	routePattern string
	userID       string
	startedAt    time.Time
	accessLogged bool
}

type requestOrigin struct {
	present   bool
	canonical string
	allowed   bool
}

type responseTracker struct {
	http.ResponseWriter
	status    int
	bytes     int
	committed bool
}

func (writer *responseTracker) WriteHeader(status int) {
	if writer.committed {
		return
	}
	writer.status = status
	writer.committed = true
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *responseTracker) Write(value []byte) (int, error) {
	if !writer.committed {
		writer.WriteHeader(http.StatusOK)
	}
	written, err := writer.ResponseWriter.Write(value)
	writer.bytes += written
	return written, err
}

func (writer *responseTracker) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (a *API) withPanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		tracker := &responseTracker{ResponseWriter: w}
		state := &requestState{
			startedAt:    time.Now(),
			routePattern: unmatchedRoutePattern,
		}
		request = request.WithContext(context.WithValue(
			request.Context(),
			requestStateContextKey{},
			state,
		))

		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			a.logger.Error(
				"HTTP handler panic",
				slog.String("request_id", state.requestID),
				slog.String("panic_class", classifyPanic(recovered)),
				slog.String("stack", string(debug.Stack())),
			)
			if !tracker.committed {
				setSecurityHeaders(tracker.Header())
				a.writeError(tracker, internalError)
			}
			if !state.accessLogged {
				a.logAccess(request, tracker, state)
			}
		}()

		next.ServeHTTP(tracker, request)
	})
}

func classifyPanic(recovered any) string {
	switch recovered.(type) {
	case runtime.Error:
		return "runtime_error"
	case error:
		return "error"
	case string:
		return "string"
	default:
		return "value"
	}
}

func (a *API) withRequestIDAndAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		state := requestStateFromContext(request.Context())
		state.startedAt = time.Now()

		requestID := incomingRequestID(request.Header.Values(requestIDHeader))
		if requestID == "" {
			generated, err := generateRequestID(a.requestIDBytes)
			if err != nil {
				a.logger.Error("generate request ID")
				setSecurityHeaders(w.Header())
				a.writeError(w, internalError)
				a.logAccess(request, responseTrackerFrom(w), state)
				state.accessLogged = true
				return
			}
			requestID = generated
		}
		state.requestID = requestID
		w.Header().Set(requestIDHeader, requestID)

		defer func() {
			if recovered := recover(); recovered != nil {
				panic(recovered)
			}
			a.logAccess(request, responseTrackerFrom(w), state)
			state.accessLogged = true
		}()

		next.ServeHTTP(w, request)
	})
}

func (a *API) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		setSecurityHeaders(w.Header())
		next.ServeHTTP(w, request)
	})
}

func setSecurityHeaders(header http.Header) {
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Cache-Control", "no-store")
}

func (a *API) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		origin := requestOrigin{}
		values := request.Header.Values("Origin")
		if len(values) > 0 {
			origin.present = true
		}
		if len(values) == 1 {
			origin.canonical, origin.allowed = a.origins.allows(values[0])
		}
		if origin.allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin.canonical)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			addVary(w.Header(), "Origin")
		}

		request = request.WithContext(context.WithValue(
			request.Context(),
			requestOriginContextKey{},
			origin,
		))
		if request.Method == http.MethodOptions {
			requestedMethods := request.Header.Values(
				"Access-Control-Request-Method",
			)
			if len(requestedMethods) == 0 {
				next.ServeHTTP(w, request)
				return
			}
			requestStateFromContext(request.Context()).routePattern = preflightRoutePattern
			if len(requestedMethods) != 1 ||
				!allowedPreflightMethod(requestedMethods[0]) {
				a.writeError(w, badRequestError)
				return
			}
			if !origin.allowed {
				a.writeError(w, forbiddenError)
				return
			}
			w.Header().Set(
				"Access-Control-Allow-Methods",
				"GET, POST, PUT, PATCH, DELETE, OPTIONS",
			)
			w.Header().Set(
				"Access-Control-Allow-Headers",
				"Accept, Content-Type, Authorization, X-CSRF-Token",
			)
			w.Header().Set("Access-Control-Max-Age", "600")
			writeNoContent(w)
			return
		}

		next.ServeHTTP(w, request)
	})
}

func allowedPreflightMethod(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions:
		return true
	default:
		return false
	}
}

func (a *API) requireAllowedOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if !requestOriginFromContext(request.Context()).allowed {
			a.writeError(w, forbiddenError)
			return
		}
		next.ServeHTTP(w, request)
	})
}

func incomingRequestID(values []string) string {
	if len(values) != 1 || len(values[0]) == 0 || len(values[0]) > maxRequestIDLength {
		return ""
	}
	for index := range len(values[0]) {
		if values[0][index] < 0x21 || values[0][index] > 0x7e {
			return ""
		}
	}
	return values[0]
}

func generateRequestID(reader io.Reader) (string, error) {
	random := make([]byte, requestIDByteLength)
	if _, err := io.ReadFull(reader, random); err != nil {
		return "", fmt.Errorf("read request ID bytes: %w", err)
	}
	return hex.EncodeToString(random), nil
}

func addVary(header http.Header, value string) {
	for _, existing := range header.Values("Vary") {
		for field := range strings.SplitSeq(existing, ",") {
			if strings.EqualFold(strings.TrimSpace(field), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

func (a *API) logAccess(
	request *http.Request,
	response *responseTracker,
	state *requestState,
) {
	status := response.status
	if status == 0 {
		status = http.StatusOK
	}
	attributes := []any{
		slog.String("request_id", state.requestID),
		slog.String("method", request.Method),
		slog.String("route_pattern", state.routePattern),
		slog.Int("status", status),
		slog.Int("response_bytes", response.bytes),
		slog.Duration("duration", time.Since(state.startedAt)),
	}
	if state.userID != "" {
		attributes = append(attributes, slog.String("user_id", state.userID))
	}
	a.logger.Info("HTTP request", attributes...)
}

func requestStateFromContext(ctx context.Context) *requestState {
	state, _ := ctx.Value(requestStateContextKey{}).(*requestState)
	if state == nil {
		return &requestState{routePattern: unmatchedRoutePattern, startedAt: time.Now()}
	}
	return state
}

func requestIDFromContext(ctx context.Context) string {
	return requestStateFromContext(ctx).requestID
}

func requestOriginFromContext(ctx context.Context) requestOrigin {
	origin, _ := ctx.Value(requestOriginContextKey{}).(requestOrigin)
	return origin
}

func responseTrackerFrom(w http.ResponseWriter) *responseTracker {
	if tracker, ok := w.(*responseTracker); ok {
		return tracker
	}
	return &responseTracker{ResponseWriter: w}
}
