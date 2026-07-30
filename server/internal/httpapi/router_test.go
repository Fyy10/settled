package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouterReturnsCommonJSONForUnknownPathAndMethod(t *testing.T) {
	t.Parallel()

	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		defaultTestOptions(),
	)

	t.Run("unknown path", func(t *testing.T) {
		response := httptest.NewRecorder()
		api.Handler().ServeHTTP(
			response,
			httptest.NewRequest(http.MethodGet, "/api/unknown", nil),
		)

		assertSecurityHeaders(t, response.Header())
		assertAPIError(t, response.Result(), http.StatusNotFound, "not_found")
	})

	t.Run("unsupported method", func(t *testing.T) {
		response := httptest.NewRecorder()
		api.Handler().ServeHTTP(
			response,
			httptest.NewRequest(http.MethodPost, "/api/health/live", nil),
		)

		assertSecurityHeaders(t, response.Header())
		assertAPIError(
			t,
			response.Result(),
			http.StatusMethodNotAllowed,
			"method_not_allowed",
		)
		allow := response.Header().Get("Allow")
		if !strings.Contains(allow, http.MethodGet) {
			t.Errorf("Allow = %q, want GET", allow)
		}
	})

	for _, test := range []struct {
		name   string
		origin string
	}{
		{name: "ordinary OPTIONS without origin"},
		{name: "ordinary OPTIONS with allowed origin", origin: "https://web.example"},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := defaultTestOptions()
			options.AllowedOrigins = []string{"https://web.example"}
			optionsAPI, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := httptest.NewRequest(http.MethodOptions, "/api/health/live", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()

			optionsAPI.Handler().ServeHTTP(response, request)

			assertAPIError(
				t,
				response.Result(),
				http.StatusMethodNotAllowed,
				"method_not_allowed",
			)
			if allow := response.Header().Get("Allow"); !strings.Contains(allow, http.MethodGet) {
				t.Errorf("Allow = %q, want GET", allow)
			}
			if test.origin != "" {
				assertCORSHeaders(t, response.Header(), test.origin)
			}
			if got := response.Header().Get("Access-Control-Allow-Methods"); got != "" {
				t.Errorf("ordinary OPTIONS got preflight methods %q", got)
			}
		})
	}
}

func TestRouterDoesNotRedirectPathVariants(t *testing.T) {
	t.Parallel()

	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		defaultTestOptions(),
	)
	for _, requestPath := range []string{
		"/api/health/live/",
		"/api//health/live",
		"/api/health/../health/live",
	} {
		t.Run(requestPath, func(t *testing.T) {
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(
				response,
				httptest.NewRequest(http.MethodGet, requestPath, nil),
			)

			assertAPIError(t, response.Result(), http.StatusNotFound, "not_found")
			if location := response.Header().Get("Location"); location != "" {
				t.Errorf("Location = %q, want no redirect", location)
			}
		})
	}
}
