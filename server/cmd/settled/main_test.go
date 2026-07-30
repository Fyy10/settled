package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/config"
)

type fakeServer struct {
	listenAndServe func() error
	shutdown       func(context.Context) error
}

func (s fakeServer) ListenAndServe() error {
	return s.listenAndServe()
}

func (s fakeServer) Shutdown(ctx context.Context) error {
	return s.shutdown(ctx)
}

func TestNewHTTPServer(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := newHTTPServer(config.Config{HTTPAddr: "127.0.0.1:8081"}, handler)

	if server.Addr != "127.0.0.1:8081" {
		t.Errorf("Addr = %q", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("Handler = nil")
	}
	if server.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %v", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout = %v", server.ReadTimeout)
	}
	if server.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v", server.WriteTimeout)
	}
	if server.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v", server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 1<<20 {
		t.Errorf("MaxHeaderBytes = %d", server.MaxHeaderBytes)
	}
}

func TestServeShutsDownAfterContextCancellation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	stopped := make(chan struct{})
	shutdownContexts := make(chan context.Context, 1)
	server := fakeServer{
		listenAndServe: func() error {
			close(started)
			<-stopped
			return http.ErrServerClosed
		},
		shutdown: func(ctx context.Context) error {
			shutdownContexts <- ctx
			close(stopped)
			return nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- serve(ctx, server)
	}()

	<-started
	cancel()

	if err := <-result; err != nil {
		t.Fatalf("serve: %v", err)
	}
	shutdownCtx := <-shutdownContexts
	deadline, ok := shutdownCtx.Deadline()
	if !ok {
		t.Fatal("shutdown context has no deadline")
	}
	remaining := time.Until(deadline)
	if remaining < 9*time.Second || remaining > shutdownTimeout {
		t.Errorf("shutdown deadline remaining = %v, want approximately 10s", remaining)
	}
	if err := shutdownCtx.Err(); err != context.Canceled {
		t.Errorf("shutdown context error after shutdown = %v, want canceled", err)
	}
}

func TestServeReturnsUnexpectedErrorAfterShutdown(t *testing.T) {
	t.Parallel()

	serveError := errors.New("listen failed")
	shutdownCalled := false
	server := fakeServer{
		listenAndServe: func() error {
			return serveError
		},
		shutdown: func(context.Context) error {
			shutdownCalled = true
			return nil
		},
	}

	err := serve(context.Background(), server)

	if !errors.Is(err, serveError) {
		t.Errorf("serve error = %v, want wrapped %v", err, serveError)
	}
	if !shutdownCalled {
		t.Error("Shutdown was not called")
	}
}

func TestServeReturnsUnexpectedNilResult(t *testing.T) {
	t.Parallel()

	shutdownCalled := false
	server := fakeServer{
		listenAndServe: func() error {
			return nil
		},
		shutdown: func(context.Context) error {
			shutdownCalled = true
			return nil
		},
	}

	err := serve(context.Background(), server)

	if err == nil || !strings.Contains(err.Error(), "stopped unexpectedly") {
		t.Errorf("serve error = %v, want unexpected-stop error", err)
	}
	if !shutdownCalled {
		t.Error("Shutdown was not called")
	}
}

func TestServeTreatsUnsolicitedServerClosedAsError(t *testing.T) {
	t.Parallel()

	shutdownCalled := false
	server := fakeServer{
		listenAndServe: func() error {
			return http.ErrServerClosed
		},
		shutdown: func(context.Context) error {
			shutdownCalled = true
			return nil
		},
	}

	err := serve(context.Background(), server)

	if !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("serve error = %v, want wrapped http.ErrServerClosed", err)
	}
	if !shutdownCalled {
		t.Error("Shutdown was not called")
	}
}

func TestServeHandlesCancellationAndServerCloseRace(t *testing.T) {
	t.Parallel()

	const iterations = 100
	for range iterations {
		started := make(chan struct{})
		serverClosed := make(chan struct{})
		shutdownCalled := false
		server := fakeServer{
			listenAndServe: func() error {
				close(started)
				<-serverClosed
				return http.ErrServerClosed
			},
			shutdown: func(context.Context) error {
				shutdownCalled = true
				return nil
			},
		}
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() {
			result <- serve(ctx, server)
		}()

		<-started
		cancel()
		close(serverClosed)

		if err := <-result; err != nil {
			t.Fatalf("serve: %v", err)
		}
		if !shutdownCalled {
			t.Fatal("Shutdown was not called")
		}
	}
}

func TestServeReturnsShutdownError(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	stopped := make(chan struct{})
	shutdownError := errors.New("shutdown failed")
	server := fakeServer{
		listenAndServe: func() error {
			close(started)
			<-stopped
			return http.ErrServerClosed
		},
		shutdown: func(context.Context) error {
			close(stopped)
			return shutdownError
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- serve(ctx, server)
	}()

	<-started
	cancel()
	err := <-result

	if !errors.Is(err, shutdownError) {
		t.Errorf("serve error = %v, want wrapped %v", err, shutdownError)
	}
}

func TestServeJoinsServeAndShutdownErrors(t *testing.T) {
	t.Parallel()

	serveError := errors.New("listen failed")
	shutdownError := errors.New("shutdown failed")
	server := fakeServer{
		listenAndServe: func() error {
			return serveError
		},
		shutdown: func(context.Context) error {
			return shutdownError
		},
	}

	err := serve(context.Background(), server)

	if !errors.Is(err, serveError) {
		t.Errorf("serve error = %v, want wrapped %v", err, serveError)
	}
	if !errors.Is(err, shutdownError) {
		t.Errorf("serve error = %v, want wrapped %v", err, shutdownError)
	}
}
