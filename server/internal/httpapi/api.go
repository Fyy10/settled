package httpapi

import (
	"context"
	"log/slog"
	"net/http"
)

type Pinger interface {
	PingContext(context.Context) error
}

type API struct {
	db     Pinger
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(db Pinger, logger *slog.Logger) *API {
	api := &API{
		db:     db,
		logger: logger,
		mux:    http.NewServeMux(),
	}
	api.mux.HandleFunc("GET /api/health/live", api.live)
	api.mux.HandleFunc("GET /api/health/ready", api.ready)
	return api
}

func (a *API) Handler() http.Handler {
	return a.mux
}
