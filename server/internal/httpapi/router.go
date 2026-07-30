package httpapi

import (
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/Fyy10/settled/server/internal/auth"
)

type routeSecurity uint8

const (
	routePublic routeSecurity = iota
	routeOptionalSession
	routeAuthenticated
	routeAnonymousUnsafe
	routeAuthenticatedUnsafe
)

func (a *API) register(
	pattern string,
	security routeSecurity,
	handler http.HandlerFunc,
) {
	method, _, found := strings.Cut(pattern, " ")
	if !found || method == "" {
		panic("HTTP route pattern must include an explicit method")
	}

	unsafe := isUnsafeMethod(method)
	if unsafe && security != routeAnonymousUnsafe && security != routeAuthenticatedUnsafe {
		panic("unsafe HTTP route requires explicit Origin and CSRF protection")
	}
	if !unsafe && (security == routeAnonymousUnsafe || security == routeAuthenticatedUnsafe) {
		panic("safe HTTP route must not use unsafe route protection")
	}

	var route http.Handler = handler
	switch security {
	case routePublic:
	case routeOptionalSession:
		route = a.withOptionalSession(route)
	case routeAuthenticated:
		route = a.withRequiredSession(route)
	case routeAnonymousUnsafe:
		route = a.requireAllowedOrigin(a.requireAnonymousCSRF(route))
	case routeAuthenticatedUnsafe:
		route = a.withRequiredSession(
			a.requireAllowedOrigin(a.requireAuthenticatedCSRF(route)),
		)
	default:
		panic("unknown HTTP route security policy")
	}
	a.mux.Handle(pattern, route)
}

func (a *API) serveRoutes(w http.ResponseWriter, request *http.Request) {
	if !canonicalRequestPath(request) {
		a.writeError(w, notFoundError)
		return
	}

	handler, pattern := a.mux.Handler(request)
	if pattern == "" {
		probe := &statusProbe{header: make(http.Header)}
		handler.ServeHTTP(probe, request)
		if probe.status == http.StatusMethodNotAllowed {
			w.Header().Set("Allow", probe.header.Get("Allow"))
			a.writeError(w, methodNotAllowedError)
			return
		}
		a.writeError(w, notFoundError)
		return
	}

	requestStateFromContext(request.Context()).routePattern = pattern
	a.mux.ServeHTTP(w, request)
}

func canonicalRequestPath(request *http.Request) bool {
	if request.URL.RawPath != "" || request.URL.Path == "" {
		return false
	}
	return path.Clean(request.URL.Path) == request.URL.Path
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

type statusProbe struct {
	header http.Header
	status int
}

func (probe *statusProbe) Header() http.Header {
	return probe.header
}

func (probe *statusProbe) WriteHeader(status int) {
	if probe.status == 0 {
		probe.status = status
	}
}

func (probe *statusProbe) Write(value []byte) (int, error) {
	if probe.status == 0 {
		probe.status = http.StatusOK
	}
	return len(value), nil
}

func (a *API) withOptionalSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		session, user, err := a.authenticationFromRequest(request)
		switch {
		case err == nil:
			request = withAuthentication(request, session, user)
		case errors.Is(err, auth.ErrUnauthenticated):
		default:
			a.handleError(w, request, err)
			return
		}
		next.ServeHTTP(w, request)
	})
}

func (a *API) withRequiredSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		session, user, err := a.authenticationFromRequest(request)
		if errors.Is(err, auth.ErrUnauthenticated) {
			a.writeError(w, unauthorizedError)
			return
		}
		if err != nil {
			a.handleError(w, request, err)
			return
		}
		next.ServeHTTP(w, withAuthentication(request, session, user))
	})
}

func (a *API) authenticationFromRequest(
	request *http.Request,
) (auth.Session, auth.User, error) {
	session, valid := a.sessionFromRequest(request)
	if !valid {
		return auth.Session{}, auth.User{}, auth.ErrUnauthenticated
	}
	user, err := a.authService.FindUser(request.Context(), session.UserID)
	if errors.Is(err, auth.ErrUserNotFound) {
		return auth.Session{}, auth.User{}, auth.ErrUnauthenticated
	}
	if err != nil {
		return auth.Session{}, auth.User{}, err
	}
	return session, user, nil
}

func (a *API) sessionFromRequest(request *http.Request) (auth.Session, bool) {
	cookies := request.CookiesNamed(auth.SessionCookieName)
	if len(cookies) != 1 || cookies[0].Value == "" {
		return auth.Session{}, false
	}
	session, err := a.sessions.Validate(cookies[0].Value)
	if err != nil {
		return auth.Session{}, false
	}
	return session, true
}

func withAuthentication(
	request *http.Request,
	session auth.Session,
	user auth.User,
) *http.Request {
	state := requestStateFromContext(request.Context())
	state.userID = user.ID
	ctx := contextWithSession(request.Context(), session)
	ctx = contextWithUser(ctx, user)
	return request.WithContext(ctx)
}

func sessionFromContext(request *http.Request) (auth.Session, bool) {
	session, ok := request.Context().Value(sessionContextKey{}).(auth.Session)
	return session, ok
}

func userFromContext(request *http.Request) (auth.User, bool) {
	user, ok := request.Context().Value(userContextKey{}).(auth.User)
	return user, ok
}
