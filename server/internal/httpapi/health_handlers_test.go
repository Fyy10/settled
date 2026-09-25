package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type pingerFunc func(context.Context) error

func (f pingerFunc) PingContext(ctx context.Context) error {
	return f(ctx)
}

func TestLive(t *testing.T) {
	t.Parallel()

	pingCalls := 0
	api, _ := testAPI(t, pingerFunc(func(context.Context) error {
		pingCalls++
		return errors.New("liveness must not ping the database")
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/health/live", nil)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	assertHealthResponse(t, response.Result(), http.StatusOK, "ok")
	if pingCalls != 0 {
		t.Errorf("database ping calls = %d, want zero", pingCalls)
	}
}

func TestReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pingError  error
		wantStatus int
		wantHealth string
	}{
		{
			name:       "available",
			wantStatus: http.StatusOK,
			wantHealth: "ok",
		},
		{
			name:       "unavailable",
			pingError:  errors.New("connection refused"),
			wantStatus: http.StatusServiceUnavailable,
			wantHealth: "unavailable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pingCalls := 0
			api, _ := testAPI(t, pingerFunc(func(context.Context) error {
				pingCalls++
				return test.pingError
			}))
			request := httptest.NewRequest(http.MethodGet, "/api/health/ready", nil)
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			assertHealthResponse(t, response.Result(), test.wantStatus, test.wantHealth)
			if pingCalls != 1 {
				t.Errorf("database ping calls = %d, want one", pingCalls)
			}
		})
	}
}

func TestReadyUsesTwoSecondChildDeadline(t *testing.T) {
	t.Parallel()

	requestStarted := time.Now()
	var receivedDeadline time.Time
	api, _ := testAPI(t, pingerFunc(func(ctx context.Context) error {
		var ok bool
		receivedDeadline, ok = ctx.Deadline()
		if !ok {
			return errors.New("ping context has no deadline")
		}
		return nil
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/health/ready", nil)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	assertHealthResponse(t, response.Result(), http.StatusOK, "ok")
	deadlineAfterStart := receivedDeadline.Sub(requestStarted)
	if deadlineAfterStart < 1900*time.Millisecond ||
		deadlineAfterStart > readinessTimeout+100*time.Millisecond {
		t.Errorf("ping deadline after request start = %v, want approximately 2s", deadlineAfterStart)
	}
}

func TestReadyLogsSanitizedDatabaseError(t *testing.T) {
	t.Parallel()

	privateError := errors.New(
		"connect postgres://private-user:private-password@secret-host/database",
	)
	api, logs := testAPI(t, pingerFunc(func(context.Context) error {
		return privateError
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/health/ready", nil)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	assertHealthResponse(
		t,
		response.Result(),
		http.StatusServiceUnavailable,
		"unavailable",
	)
	logOutput := logs.String()
	for _, privateValue := range []string{
		privateError.Error(),
		"private-password",
		"secret-host",
	} {
		if strings.Contains(logOutput, privateValue) {
			t.Errorf("log contains private value %q: %s", privateValue, logOutput)
		}
	}
	if !strings.Contains(logOutput, `"level":"WARN"`) {
		t.Errorf("log does not contain warning level: %s", logOutput)
	}
	if !strings.Contains(logOutput, "database readiness check failed") {
		t.Errorf("log does not contain readiness message: %s", logOutput)
	}
}

func TestSafeDatabaseError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", want: ""},
		{
			name: "deadline",
			err:  errors.Join(errors.New("ping"), context.DeadlineExceeded),
			want: "deadline exceeded",
		},
		{
			name: "canceled",
			err:  errors.Join(errors.New("ping"), context.Canceled),
			want: "request canceled",
		},
		{name: "other", err: errors.New("private details"), want: "database ping failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := safeDatabaseError(test.err); got != test.want {
				t.Errorf("safeDatabaseError() = %q, want %q", got, test.want)
			}
		})
	}
}

func testAPI(t *testing.T, pinger Pinger) (*API, *bytes.Buffer) {
	t.Helper()
	return newTestAPI(t, pinger, defaultTestOptions())
}

func assertHealthResponse(
	t *testing.T,
	response *http.Response,
	wantStatus int,
	wantHealth string,
) {
	t.Helper()
	defer response.Body.Close()

	if response.StatusCode != wantStatus {
		t.Errorf("status = %d, want %d", response.StatusCode, wantStatus)
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", contentType)
	}

	var body healthResponse
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != wantHealth {
		t.Errorf("health status = %q, want %q", body.Status, wantHealth)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		t.Errorf("body contains another JSON value: %v", err)
	}
}
