package httpapi

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/Fyy10/settled/server/internal/auth"
)

type Pinger interface {
	PingContext(context.Context) error
}

type SessionValidator interface {
	Validate(string) (auth.Session, error)
}

type AuthService interface {
	Register(context.Context, auth.RegisterInput) (auth.AuthResult, error)
	Login(context.Context, string, string) (auth.AuthResult, error)
	FindUser(context.Context, string) (auth.User, error)
}

type CSRFProtector interface {
	IssueOrReuse(string, *auth.Session) (auth.CSRFBinding, error)
	RotateAnonymous() (auth.CSRFBinding, error)
	RotateAuthenticated(auth.Session) (auth.CSRFBinding, error)
	ValidateAnonymous(string, string) error
	ValidateAuthenticated(string, string, auth.Session) error
}

type Options struct {
	AllowedOrigins []string
	Auth           AuthService
	Sessions       SessionValidator
	CSRF           CSRFProtector
	SessionCookies auth.SessionCookies
	CSRFCookies    auth.CSRFCookies
	RequestIDBytes io.Reader
}

type API struct {
	db             Pinger
	logger         *slog.Logger
	authService    AuthService
	sessions       SessionValidator
	csrf           CSRFProtector
	sessionCookies auth.SessionCookies
	csrfCookies    auth.CSRFCookies
	origins        OriginPolicy
	requestIDBytes io.Reader
	mux            *http.ServeMux
	handler        http.Handler
}

func New(db Pinger, logger *slog.Logger, options Options) (*API, error) {
	if db == nil ||
		logger == nil ||
		options.Auth == nil ||
		options.Sessions == nil ||
		options.CSRF == nil {
		return nil, errors.New("invalid HTTP API configuration")
	}

	origins, err := NewOriginPolicy(options.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	requestIDBytes := options.RequestIDBytes
	if requestIDBytes == nil {
		requestIDBytes = rand.Reader
	}

	api := &API{
		db:             db,
		logger:         logger,
		authService:    options.Auth,
		sessions:       options.Sessions,
		csrf:           options.CSRF,
		sessionCookies: options.SessionCookies,
		csrfCookies:    options.CSRFCookies,
		origins:        origins,
		requestIDBytes: requestIDBytes,
		mux:            http.NewServeMux(),
	}
	api.register("GET /api/health/live", routePublic, api.live)
	api.register("GET /api/health/ready", routePublic, api.ready)
	api.register("GET /api/auth/csrf", routeOptionalSession, api.getCSRF)
	api.register("POST /api/auth/register", routeAnonymousUnsafe, api.registerUser)
	api.register("POST /api/auth/login", routeAnonymousUnsafe, api.login)
	api.register("POST /api/auth/logout", routeAuthenticatedUnsafe, api.logout)
	api.register("GET /api/me", routeAuthenticated, api.me)
	api.handler = api.withPanicRecovery(
		api.withRequestIDAndAccessLog(
			api.withSecurityHeaders(
				api.withCORS(http.HandlerFunc(api.serveRoutes)),
			),
		),
	)
	return api, nil
}

func (a *API) Handler() http.Handler {
	return a.handler
}
