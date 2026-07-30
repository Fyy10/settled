package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const readinessTimeout = 2 * time.Second

type healthResponse struct {
	Status string `json:"status"`
}

func (a *API) live(w http.ResponseWriter, _ *http.Request) {
	a.writeHealth(w, http.StatusOK, "ok")
}

func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
	defer cancel()

	if err := a.db.PingContext(ctx); err != nil {
		a.logger.Warn(
			"database readiness check failed",
			slog.String("error", safeDatabaseError(err)),
		)
		a.writeHealth(w, http.StatusServiceUnavailable, "unavailable")
		return
	}

	a.writeHealth(w, http.StatusOK, "ok")
}

func (a *API) writeHealth(w http.ResponseWriter, status int, healthStatus string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(healthResponse{Status: healthStatus}); err != nil {
		a.logger.Error("encode health response", slog.Any("error", err))
	}
}

func safeDatabaseError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline exceeded"
	case errors.Is(err, context.Canceled):
		return "request canceled"
	default:
		return "database ping failed"
	}
}
