package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fyy10/settled/server/internal/groups"
)

func TestRequestLogsExcludeSensitiveValues(t *testing.T) {
	t.Parallel()

	const (
		databaseURL   = "postgres://private-db-user:private-db-password@private-db-host/settled"
		sessionToken  = "valid-session"
		cookieValue   = "private-cookie-value"
		authorization = "Bearer private-authorization-token"
		csrfToken     = "private-csrf-token"
		password      = "private-login-password"
		joinCode      = "PRIVATE8"
		passwordHash  = "$argon2id$v=19$m=65536,t=3,p=4$private-salt$private-hash"
		requestBody   = "private-request-body"
	)
	privateValues := []string{
		databaseURL,
		sessionToken,
		cookieValue,
		authorization,
		csrfToken,
		password,
		joinCode,
		passwordHash,
		requestBody,
	}
	privateError := errors.New(strings.Join(privateValues, " "))
	options := groupHandlerTestOptions(fakeGroupService{
		list: func(context.Context, string) ([]groups.Group, error) {
			return nil, privateError
		},
	})
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := groupHandlerRequest(
		http.MethodGet,
		"/api/groups?password="+password+"&joinCode="+joinCode,
		strings.NewReader(requestBody),
		false,
	)
	request.Header.Set("Authorization", authorization)
	request.Header.Set(csrfTokenHeader, csrfToken)
	request.AddCookie(&http.Cookie{Name: "private", Value: cookieValue})
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	clientBody := response.Body.String()
	assertAPIError(
		t,
		response.Result(),
		http.StatusInternalServerError,
		"internal_error",
	)
	logOutput := logs.String()
	for _, privateValue := range privateValues {
		if strings.Contains(clientBody, privateValue) {
			t.Errorf("client response contains private value %q: %s", privateValue, clientBody)
		}
		if strings.Contains(logOutput, privateValue) {
			t.Errorf("log contains private value %q: %s", privateValue, logOutput)
		}
	}
	if !strings.Contains(logOutput, `"error_class":"internal"`) {
		t.Errorf("log lacks safe internal error class: %s", logOutput)
	}
}

func TestAPIContinuesAndRecoversAfterTemporaryDatabaseUnavailability(t *testing.T) {
	t.Parallel()

	databaseAvailable := false
	pingCalls := 0
	listCalls := 0
	privateDatabaseError := errors.New(
		"connect postgres://private-user:private-password@private-host/settled",
	)
	options := groupHandlerTestOptions(fakeGroupService{
		list: func(context.Context, string) ([]groups.Group, error) {
			listCalls++
			if !databaseAvailable {
				return nil, privateDatabaseError
			}
			return []groups.Group{}, nil
		},
	})
	api, logs := newTestAPI(t, pingerFunc(func(context.Context) error {
		pingCalls++
		if !databaseAvailable {
			return privateDatabaseError
		}
		return nil
	}), options)

	liveResponse := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		liveResponse,
		httptest.NewRequest(http.MethodGet, "/api/health/live", nil),
	)
	assertHealthResponse(t, liveResponse.Result(), http.StatusOK, "ok")
	if pingCalls != 0 {
		t.Errorf("liveness database ping calls = %d, want zero", pingCalls)
	}

	unavailableReady := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		unavailableReady,
		httptest.NewRequest(http.MethodGet, "/api/health/ready", nil),
	)
	assertHealthResponse(
		t,
		unavailableReady.Result(),
		http.StatusServiceUnavailable,
		"unavailable",
	)

	unavailableGroups := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		unavailableGroups,
		groupHandlerRequest(http.MethodGet, "/api/groups", nil, false),
	)
	assertAPIError(
		t,
		unavailableGroups.Result(),
		http.StatusInternalServerError,
		"internal_error",
	)

	databaseAvailable = true

	recoveredReady := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		recoveredReady,
		httptest.NewRequest(http.MethodGet, "/api/health/ready", nil),
	)
	assertHealthResponse(t, recoveredReady.Result(), http.StatusOK, "ok")

	recoveredGroups := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		recoveredGroups,
		groupHandlerRequest(http.MethodGet, "/api/groups", nil, false),
	)
	if recoveredGroups.Code != http.StatusOK {
		t.Errorf("recovered group list status = %d, want %d", recoveredGroups.Code, http.StatusOK)
	}
	if got := recoveredGroups.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("recovered group list Content-Type = %q", got)
	}
	if got := recoveredGroups.Body.String(); got != "{\"groups\":[]}\n" {
		t.Errorf("recovered group list body = %q, want empty groups", got)
	}
	if pingCalls != 2 {
		t.Errorf("database ping calls = %d, want two", pingCalls)
	}
	if listCalls != 2 {
		t.Errorf("group list calls = %d, want two", listCalls)
	}
	logOutput := logs.String()
	for _, privateValue := range []string{
		privateDatabaseError.Error(),
		"private-password",
		"private-host",
	} {
		if strings.Contains(logOutput, privateValue) {
			t.Errorf("log contains private database value %q: %s", privateValue, logOutput)
		}
	}
}
