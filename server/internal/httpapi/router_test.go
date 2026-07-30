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

func TestDocumentedRouteMethodMatrix(t *testing.T) {
	t.Parallel()

	const (
		groupID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
		userID  = "11112222-3333-4444-8555-666677778888"
		itemID  = "22222222-3333-4444-8555-666666666666"
	)
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		defaultTestOptions(),
	)
	tests := []struct {
		path    string
		methods []string
	}{
		{path: "/api/health/live", methods: []string{http.MethodGet, http.MethodHead}},
		{path: "/api/health/ready", methods: []string{http.MethodGet, http.MethodHead}},
		{path: "/api/auth/csrf", methods: []string{http.MethodGet, http.MethodHead}},
		{path: "/api/auth/register", methods: []string{http.MethodPost}},
		{path: "/api/auth/login", methods: []string{http.MethodPost}},
		{path: "/api/auth/logout", methods: []string{http.MethodPost}},
		{path: "/api/me", methods: []string{http.MethodGet, http.MethodHead}},
		{
			path:    "/api/groups",
			methods: []string{http.MethodGet, http.MethodHead, http.MethodPost},
		},
		{path: "/api/groups/join", methods: []string{http.MethodPost}},
		{
			path: "/api/groups/" + groupID,
			methods: []string{
				http.MethodGet,
				http.MethodHead,
				http.MethodPatch,
				http.MethodDelete,
			},
		},
		{
			path:    "/api/groups/" + groupID + "/join-code",
			methods: []string{http.MethodGet, http.MethodHead},
		},
		{
			path:    "/api/groups/" + groupID + "/members/" + userID,
			methods: []string{http.MethodDelete},
		},
		{
			path: "/api/groups/" + groupID + "/expenses",
			methods: []string{
				http.MethodGet,
				http.MethodHead,
				http.MethodPost,
			},
		},
		{
			path: "/api/groups/" + groupID + "/expenses/" + itemID,
			methods: []string{
				http.MethodGet,
				http.MethodHead,
				http.MethodPut,
				http.MethodDelete,
			},
		},
		{
			path: "/api/groups/" + groupID + "/repayments",
			methods: []string{
				http.MethodGet,
				http.MethodHead,
				http.MethodPost,
			},
		},
		{
			path: "/api/groups/" + groupID + "/repayments/" + itemID,
			methods: []string{
				http.MethodGet,
				http.MethodHead,
				http.MethodPut,
				http.MethodDelete,
			},
		},
		{
			path:    "/api/groups/" + groupID + "/settlements",
			methods: []string{http.MethodGet, http.MethodHead},
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			api.Handler().ServeHTTP(
				response,
				httptest.NewRequest(http.MethodTrace, test.path, nil),
			)
			result := response.Result()

			assertAllowedMethods(t, result.Header.Get("Allow"), test.methods)
			assertAPIError(
				t,
				result,
				http.StatusMethodNotAllowed,
				"method_not_allowed",
			)
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

func assertAllowedMethods(t *testing.T, allow string, want []string) {
	t.Helper()

	got := make(map[string]struct{})
	for _, method := range strings.Split(allow, ",") {
		method = strings.TrimSpace(method)
		if method != "" {
			got[method] = struct{}{}
		}
	}
	if len(got) != len(want) {
		t.Errorf("Allow = %q, want methods %v", allow, want)
		return
	}
	for _, method := range want {
		if _, ok := got[method]; !ok {
			t.Errorf("Allow = %q, want methods %v", allow, want)
			return
		}
	}
}
