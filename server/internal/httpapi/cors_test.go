package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fyy10/settled/server/internal/auth"
)

func TestCredentialedCORSForAllowedOrigin(t *testing.T) {
	t.Parallel()

	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	api, _ := newTestAPI(t, pingerFunc(func(context.Context) error { return nil }), options)
	request := httptest.NewRequest(http.MethodGet, "/api/health/live", nil)
	request.Header.Set("Origin", "HTTPS://WEB.EXAMPLE:443")
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", response.Code)
	}
	assertCORSHeaders(t, response.Header(), "https://web.example")
	assertSecurityHeaders(t, response.Header())
}

func TestSafeRequestWithMissingOrDisallowedOriginProceedsWithoutCORS(t *testing.T) {
	t.Parallel()

	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	api, _ := newTestAPI(t, pingerFunc(func(context.Context) error { return nil }), options)
	for _, origin := range []string{"", "https://other.example", "not-an-origin"} {
		request := httptest.NewRequest(http.MethodGet, "/api/health/live", nil)
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response := httptest.NewRecorder()

		api.Handler().ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Errorf("Origin %q status = %d, want 200", origin, response.Code)
		}
		if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Origin %q got CORS allow origin %q", origin, got)
		}
		if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Errorf("Origin %q got CORS credentials %q", origin, got)
		}
	}
}

func TestAllowedPreflight(t *testing.T) {
	t.Parallel()

	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	api, logs := newTestAPI(t, pingerFunc(func(context.Context) error { return nil }), options)
	request := httptest.NewRequest(http.MethodOptions, "/api/future/route", nil)
	request.Header.Set("Origin", "https://WEB.example:443")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "Content-Type, X-CSRF-Token")
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", response.Code)
	}
	if response.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "" {
		t.Errorf("Content-Type = %q, want empty", contentType)
	}
	assertCORSHeaders(t, response.Header(), "https://web.example")
	assertSecurityHeaders(t, response.Header())
	if got := response.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methods = %q", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); got != "Accept, Content-Type, Authorization, X-CSRF-Token" {
		t.Errorf("Access-Control-Allow-Headers = %q", got)
	}
	if got := response.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Errorf("Access-Control-Max-Age = %q", got)
	}
	if !strings.Contains(logs.String(), `"route_pattern":"OPTIONS (preflight)"`) {
		t.Errorf("access log lacks preflight route: %s", logs.String())
	}
}

func TestPreflightRequiresAllowedOrigin(t *testing.T) {
	t.Parallel()

	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	api, _ := newTestAPI(t, pingerFunc(func(context.Context) error { return nil }), options)

	tests := []struct {
		name    string
		origins []string
	}{
		{name: "missing"},
		{name: "disallowed", origins: []string{"https://other.example"}},
		{name: "malformed", origins: []string{"not-an-origin"}},
		{name: "multiple", origins: []string{"https://web.example", "https://web.example"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodOptions, "/api/health/live", nil)
			for _, origin := range test.origins {
				request.Header.Add("Origin", origin)
			}
			request.Header.Set("Access-Control-Request-Method", http.MethodPost)
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			assertAPIError(t, response.Result(), http.StatusForbidden, "forbidden")
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Errorf("Access-Control-Allow-Origin = %q", got)
			}
		})
	}
}

func TestMalformedPreflightRequestMethod(t *testing.T) {
	t.Parallel()

	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	api, _ := newTestAPI(t, pingerFunc(func(context.Context) error { return nil }), options)

	tests := []struct {
		name    string
		methods []string
	}{
		{name: "blank", methods: []string{""}},
		{name: "padded", methods: []string{" POST "}},
		{name: "unsupported", methods: []string{http.MethodTrace}},
		{name: "multiple", methods: []string{http.MethodPost, http.MethodDelete}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodOptions, "/api/health/live", nil)
			request.Header.Set("Origin", "https://web.example")
			for _, method := range test.methods {
				request.Header.Add("Access-Control-Request-Method", method)
			}
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			assertAPIError(t, response.Result(), http.StatusBadRequest, "bad_request")
		})
	}
}

func TestUnsafeOriginRunsBeforeCSRF(t *testing.T) {
	t.Parallel()

	validationCalls := 0
	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	options.CSRF = fakeCSRFProtector{
		issueOrReuse: func(string, *auth.Session) (auth.CSRFBinding, error) {
			return auth.CSRFBinding{}, nil
		},
		validateAnonymous: func(cookie, token string) error {
			validationCalls++
			if cookie != "cookie" || token != "token" {
				t.Errorf("CSRF materials = (%q, %q)", cookie, token)
			}
			return nil
		},
	}
	api, _ := newTestAPI(t, pingerFunc(func(context.Context) error { return nil }), options)
	api.register(
		"POST /api/test/unsafe",
		routeAnonymousUnsafe,
		func(w http.ResponseWriter, _ *http.Request) {
			writeNoContent(w)
		},
	)

	for _, test := range []struct {
		name       string
		origin     string
		wantStatus int
		wantCalls  int
	}{
		{name: "missing", wantStatus: http.StatusForbidden},
		{name: "disallowed", origin: "https://other.example", wantStatus: http.StatusForbidden},
		{
			name:       "allowed",
			origin:     "HTTPS://WEB.EXAMPLE:443",
			wantStatus: http.StatusNoContent,
			wantCalls:  1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			validationCalls = 0
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/test/unsafe",
				strings.NewReader("{}"),
			)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			request.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "cookie"})
			request.Header.Set(csrfTokenHeader, "token")
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Errorf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if validationCalls != test.wantCalls {
				t.Errorf("CSRF validation calls = %d, want %d", validationCalls, test.wantCalls)
			}
		})
	}
}

func TestUnsafeRoutesRequireExplicitSecurityComposition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pattern  string
		security routeSecurity
	}{
		{name: "public unsafe", pattern: "POST /api/test", security: routePublic},
		{name: "optional unsafe", pattern: "DELETE /api/test", security: routeOptionalSession},
		{name: "authentication only unsafe", pattern: "PATCH /api/test", security: routeAuthenticated},
		{name: "unsafe policy on safe route", pattern: "GET /api/test", security: routeAnonymousUnsafe},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := defaultTestOptions()
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			defer func() {
				if recover() == nil {
					t.Error("register did not panic")
				}
			}()
			api.register(test.pattern, test.security, func(http.ResponseWriter, *http.Request) {})
		})
	}
}

func assertCORSHeaders(t *testing.T, header http.Header, origin string) {
	t.Helper()
	if got := header.Get("Access-Control-Allow-Origin"); got != origin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, origin)
	}
	if got := header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q", got)
	}
	vary := strings.Join(header.Values("Vary"), ",")
	if !strings.Contains(vary, "Origin") {
		t.Errorf("Vary = %q, want Origin", vary)
	}
	if strings.Contains(header.Get("Access-Control-Allow-Origin"), "*") {
		t.Error("wildcard CORS origin was returned")
	}
}

func assertSecurityHeaders(t *testing.T, header http.Header) {
	t.Helper()
	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	} {
		if got := header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func assertAPIError(
	t *testing.T,
	response *http.Response,
	status int,
	code string,
) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != status {
		t.Errorf("status = %d, want %d", response.StatusCode, status)
	}
	if got := response.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	var envelope errorEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if envelope.Error.Code != code {
		t.Errorf("error code = %q, want %q", envelope.Error.Code, code)
	}
	if envelope.Error.Message == "" {
		t.Error("error message is empty")
	}
	if extra, err := io.ReadAll(response.Body); err != nil || len(extra) != 0 {
		t.Errorf("trailing response body = %q, error %v", extra, err)
	}
}
