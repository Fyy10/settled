package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/clock"
	"github.com/Fyy10/settled/server/internal/config"
	"github.com/Fyy10/settled/server/internal/expenses"
	"github.com/Fyy10/settled/server/internal/groups"
	"github.com/Fyy10/settled/server/internal/httpapi"
	"github.com/Fyy10/settled/server/internal/repayments"
	"github.com/Fyy10/settled/server/internal/settlements"
	"github.com/Fyy10/settled/server/internal/store"
)

const (
	startupPingTimeout = 5 * time.Second
	shutdownTimeout    = 10 * time.Second
)

type serverLifecycle interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx, os.Stdout); err != nil {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		logger.Error("settled API stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, output io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return errors.New("open database: invalid database configuration")
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Warn("close database failed")
		}
	}()

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)

	pingCtx, cancelPing := context.WithTimeout(ctx, startupPingTimeout)
	err = db.PingContext(pingCtx)
	cancelPing()
	if err != nil {
		logger.Error("database startup check failed")
		return errors.New("database startup check failed")
	}

	systemClock := clock.System{}
	sessionManager, err := auth.NewSessionManager(cfg.JWTSecret, systemClock)
	if err != nil {
		return fmt.Errorf("configure session authentication: %w", err)
	}
	csrfManager, err := auth.NewCSRFManager(cfg.CSRFSecret, systemClock)
	if err != nil {
		return fmt.Errorf("configure CSRF protection: %w", err)
	}
	passwordHasher, err := auth.NewPasswordHasher()
	if err != nil {
		return fmt.Errorf("configure password authentication: %w", err)
	}
	applicationStore := store.New(db)
	authService, err := auth.NewService(
		applicationStore,
		passwordHasher,
		sessionManager,
		systemClock,
	)
	if err != nil {
		return fmt.Errorf("configure authentication service: %w", err)
	}
	groupService, err := groups.NewService(applicationStore, systemClock)
	if err != nil {
		return fmt.Errorf("configure group service: %w", err)
	}
	expenseService, err := expenses.NewService(applicationStore, systemClock)
	if err != nil {
		return fmt.Errorf("configure expense service: %w", err)
	}
	repaymentService, err := repayments.NewService(applicationStore, systemClock)
	if err != nil {
		return fmt.Errorf("configure repayment service: %w", err)
	}
	settlementService, err := settlements.NewService(
		applicationStore,
		settlements.PairwiseCalculator{},
	)
	if err != nil {
		return fmt.Errorf("configure settlement service: %w", err)
	}
	api, err := httpapi.New(db, logger, httpapi.Options{
		AllowedOrigins: cfg.AllowedOrigins,
		Auth:           authService,
		Groups:         groupService,
		Expenses:       expenseService,
		Repayments:     repaymentService,
		Settlements:    settlementService,
		Sessions:       sessionManager,
		CSRF:           csrfManager,
		SessionCookies: auth.NewSessionCookies(
			cfg.CookieDomain,
			cfg.CookieSecure,
			cfg.CookieSameSite,
		),
		CSRFCookies: auth.NewCSRFCookies(
			cfg.CookieDomain,
			cfg.CookieSecure,
			cfg.CookieSameSite,
		),
	})
	if err != nil {
		return fmt.Errorf("configure HTTP API: %w", err)
	}
	server := newHTTPServer(cfg, api.Handler())

	logger.Info(
		"HTTP server starting",
		slog.String("address", cfg.HTTPAddr),
		slog.String("environment", string(cfg.AppEnv)),
	)
	return serve(ctx, server)
}

func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func serve(ctx context.Context, server serverLifecycle) error {
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErrors:
		if ctx.Err() != nil {
			shutdownErr := shutdown(server)
			if shutdownErr != nil {
				return fmt.Errorf("shutdown HTTP server: %w", shutdownErr)
			}
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("serve HTTP during shutdown: %w", err)
			}
			return nil
		}
		if err == nil {
			err = errors.New("HTTP server stopped unexpectedly")
		}
		shutdownErr := shutdown(server)
		if shutdownErr != nil {
			return errors.Join(
				fmt.Errorf("serve HTTP: %w", err),
				fmt.Errorf("shutdown HTTP server: %w", shutdownErr),
			)
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		shutdownErr := shutdown(server)
		serveErr := <-serveErrors
		if shutdownErr != nil {
			return fmt.Errorf("shutdown HTTP server: %w", shutdownErr)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP during shutdown: %w", serveErr)
		}
		return nil
	}
}

func shutdown(server serverLifecycle) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return server.Shutdown(ctx)
}
