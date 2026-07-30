package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fyy10/settled/server/internal/auth"
)

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("entropy panic")
}

func TestRequestIDSelection(t *testing.T) {
	t.Parallel()

	generated := strings.Repeat("ab", requestIDByteLength)
	tests := []struct {
		name     string
		values   []string
		want     string
		wantUsed bool
	}{
		{name: "missing", want: generated},
		{name: "valid", values: []string{"client-request-id_123"}, want: "client-request-id_123", wantUsed: true},
		{name: "one visible character", values: []string{"!"}, want: "!", wantUsed: true},
		{name: "maximum visible ASCII", values: []string{strings.Repeat("x", maxRequestIDLength)}, want: strings.Repeat("x", maxRequestIDLength), wantUsed: true},
		{name: "empty", values: []string{""}, want: generated},
		{name: "space", values: []string{"request id"}, want: generated},
		{name: "control", values: []string{"request\nid"}, want: generated},
		{name: "too long", values: []string{strings.Repeat("x", maxRequestIDLength+1)}, want: generated},
		{name: "multiple", values: []string{"first", "second"}, want: generated},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := defaultTestOptions()
			options.RequestIDBytes = &repeatReader{value: 0xab}
			api, logs := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := httptest.NewRequest(http.MethodGet, "/api/health/live", nil)
			for _, value := range test.values {
				request.Header.Add(requestIDHeader, value)
			}
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			if got := response.Header().Get(requestIDHeader); got != test.want {
				t.Errorf("X-Request-ID = %q, want %q", got, test.want)
			}
			if !strings.Contains(logs.String(), `"request_id":"`+test.want+`"`) {
				t.Errorf("access log lacks selected request ID: %s", logs.String())
			}
		})
	}
}

func TestRequestIDEntropyFailureReturnsSecuredInternalError(t *testing.T) {
	t.Parallel()

	options := defaultTestOptions()
	options.RequestIDBytes = errorReader{err: errors.New("entropy unavailable")}
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := httptest.NewRequest(http.MethodGet, "/api/health/live", nil)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	assertSecurityHeaders(t, response.Header())
	assertAPIError(
		t,
		response.Result(),
		http.StatusInternalServerError,
		"internal_error",
	)
	if !strings.Contains(logs.String(), "generate request ID") {
		t.Errorf("log lacks entropy failure event: %s", logs.String())
	}
	if !strings.Contains(logs.String(), `"status":500`) {
		t.Errorf("access log lacks status 500: %s", logs.String())
	}
}

func TestPanicBeforeSecurityMiddlewareStillSetsSecurityHeaders(t *testing.T) {
	t.Parallel()

	options := defaultTestOptions()
	options.RequestIDBytes = panicReader{}
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/health/live", nil),
	)

	assertSecurityHeaders(t, response.Header())
	assertAPIError(
		t,
		response.Result(),
		http.StatusInternalServerError,
		"internal_error",
	)
	if !strings.Contains(logs.String(), "HTTP handler panic") {
		t.Errorf("panic was not logged: %s", logs.String())
	}
}

func TestAccessLogUsesSafeRouteFieldsOnly(t *testing.T) {
	t.Parallel()

	options := defaultTestOptions()
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	api.register(
		"GET /api/test/{value}",
		routePublic,
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, "ok")
		},
	)
	privateValues := []string{
		"private-path-value",
		"private-query-value",
		"private-cookie-value",
		"private-authorization-value",
		"private-csrf-value",
		"private-body-value",
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/test/"+privateValues[0]+"?password="+privateValues[1],
		strings.NewReader(privateValues[5]),
	)
	request.AddCookie(&http.Cookie{Name: "private", Value: privateValues[2]})
	request.Header.Set("Authorization", "Bearer "+privateValues[3])
	request.Header.Set(csrfTokenHeader, privateValues[4])
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	logOutput := logs.String()
	for _, privateValue := range privateValues {
		if strings.Contains(logOutput, privateValue) {
			t.Errorf("access log exposes %q: %s", privateValue, logOutput)
		}
	}
	for _, field := range []string{
		`"method":"GET"`,
		`"route_pattern":"GET /api/test/{value}"`,
		`"status":201`,
		`"response_bytes":2`,
		`"duration":`,
	} {
		if !strings.Contains(logOutput, field) {
			t.Errorf("access log lacks %s: %s", field, logOutput)
		}
	}
}

func TestAccessLogIncludesValidatedSessionUserID(t *testing.T) {
	t.Parallel()

	session := auth.Session{
		UserID: "00112233-4455-4677-8899-aabbccddeeff",
		JWTID:  "test-jwt-id",
	}
	options := defaultTestOptions()
	options.Sessions = sessionValidatorFunc(func(token string) (auth.Session, error) {
		if token != "valid-session-token" {
			return auth.Session{}, auth.ErrUnauthenticated
		}
		return session, nil
	})
	options.CSRF = fakeCSRFProtector{
		issueOrReuse: func(_ string, got *auth.Session) (auth.CSRFBinding, error) {
			if got == nil || *got != session {
				t.Errorf("session = %v, want %v", got, session)
			}
			return auth.CSRFBinding{
				Token:         "v1.token",
				CookieValue:   "cookie",
				MaxAgeSeconds: 60,
			}, nil
		},
	}
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	request.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: "valid-session-token",
	})
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Errorf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(logs.String(), `"user_id":"`+session.UserID+`"`) {
		t.Errorf("access log lacks user ID: %s", logs.String())
	}
}

func TestPanicRecoveryBeforeAndAfterCommit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantBody   string
	}{
		{
			name: "before commit",
			handler: func(http.ResponseWriter, *http.Request) {
				panic("test panic")
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "after commit",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = io.WriteString(w, "partial")
				panic("test panic")
			},
			wantStatus: http.StatusAccepted,
			wantBody:   "partial",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api, logs := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				defaultTestOptions(),
			)
			api.register("GET /api/test/panic", routePublic, test.handler)
			request := httptest.NewRequest(http.MethodGet, "/api/test/panic", nil)
			request.Header.Set(requestIDHeader, "panic-request")
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Errorf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantBody != "" && response.Body.String() != test.wantBody {
				t.Errorf("body = %q, want %q", response.Body.String(), test.wantBody)
			}
			if test.wantStatus == http.StatusInternalServerError &&
				!strings.Contains(response.Body.String(), `"code":"internal_error"`) {
				t.Errorf("body = %s", response.Body.String())
			}
			assertSecurityHeaders(t, response.Header())
			logOutput := logs.String()
			for _, value := range []string{
				"HTTP handler panic",
				`"panic_class":"string"`,
				`"stack":`,
				`"request_id":"panic-request"`,
				`"status":` + httpStatusText(test.wantStatus),
			} {
				if !strings.Contains(logOutput, value) {
					t.Errorf("log lacks %q: %s", value, logOutput)
				}
			}
			if strings.Contains(logOutput, "test panic") {
				t.Errorf("log exposes panic value: %s", logOutput)
			}
			if strings.Count(logOutput, `"msg":"HTTP request"`) != 1 {
				t.Errorf("access log count is not one: %s", logOutput)
			}
		})
	}
}

func TestPanicLogDoesNotExposeRequestOrPanicValues(t *testing.T) {
	t.Parallel()

	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		defaultTestOptions(),
	)
	privateValues := []string{
		"panic-private-cookie",
		"panic-private-authorization",
		"panic-private-csrf",
		"panic-private-body-password-join-code",
	}
	api.register(
		"GET /api/test/panic-redaction",
		routePublic,
		func(_ http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			cookie, _ := request.Cookie("private")
			panic(strings.Join([]string{
				cookie.Value,
				request.Header.Get("Authorization"),
				request.Header.Get(csrfTokenHeader),
				string(body),
			}, "|"))
		},
	)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/test/panic-redaction",
		strings.NewReader(privateValues[3]),
	)
	request.AddCookie(&http.Cookie{Name: "private", Value: privateValues[0]})
	request.Header.Set("Authorization", privateValues[1])
	request.Header.Set(csrfTokenHeader, privateValues[2])
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", response.Code, response.Body.String())
	}
	assertSecurityHeaders(t, response.Header())
	logOutput := logs.String()
	for _, privateValue := range privateValues {
		if strings.Contains(logOutput, privateValue) {
			t.Errorf("panic log exposes %q: %s", privateValue, logOutput)
		}
	}
	for _, required := range []string{
		`"msg":"HTTP handler panic"`,
		`"panic_class":"string"`,
		`"request_id":`,
		`"stack":`,
	} {
		if !strings.Contains(logOutput, required) {
			t.Errorf("panic log lacks %q: %s", required, logOutput)
		}
	}
}

func httpStatusText(status int) string {
	switch status {
	case http.StatusAccepted:
		return "202"
	case http.StatusInternalServerError:
		return "500"
	default:
		return ""
	}
}
