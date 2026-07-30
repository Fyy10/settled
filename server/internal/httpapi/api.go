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

type CSRFProtector interface {
	IssueOrReuse(string, *auth.Session) (auth.CSRFBinding, error)
	ValidateAnonymous(string, string) error
	ValidateAuthenticated(string, string, auth.Session) error
}

type Options struct {
	AllowedOrigins []string
	Sessions       SessionValidator
	CSRF           CSRFProtector
	CSRFCookies    auth.CSRFCookies
	RequestIDBytes io.Reader
}

type API struct {
	db             Pinger
	logger         *slog.Logger
	sessions       SessionValidator
	csrf           CSRFProtector
	csrfCookies    auth.CSRFCookies
	origins        OriginPolicy
	requestIDBytes io.Reader
	mux            *http.ServeMux
	handler        http.Handler
}

func New(db Pinger, logger *slog.Logger, options Options) (*API, error) {
	if db == nil || logger == nil || options.Sessions == nil || options.CSRF == nil {
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
		sessions:       options.Sessions,
		csrf:           options.CSRF,
		csrfCookies:    options.CSRFCookies,
		origins:        origins,
		requestIDBytes: requestIDBytes,
		mux:            http.NewServeMux(),
	}
	api.register("GET /api/health/live", routePublic, api.live)
	api.register("GET /api/health/ready", routePublic, api.ready)
	api.register("GET /api/auth/csrf", routeOptionalSession, api.getCSRF)
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
