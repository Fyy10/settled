package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/clock"
)

var csrfTestNow = time.Date(2026, time.July, 29, 20, 0, 0, 0, time.UTC)

func TestGetCSRFAnonymousIssueAndReuse(t *testing.T) {
	t.Parallel()

	csrfManager := newTestCSRFManager(t, csrfTestNow, &repeatReader{value: 0x33})
	options := defaultTestOptions()
	options.CSRF = csrfManager
	options.CSRFCookies = auth.NewCSRFCookies(
		"api.example.com",
		true,
		http.SameSiteNoneMode,
	)
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)

	firstResponse := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		firstResponse,
		httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil),
	)

	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first status = %d: %s", firstResponse.Code, firstResponse.Body.String())
	}
	firstBody := decodeCSRFResponse(t, firstResponse.Result())
	firstCookie := responseCookieNamed(t, firstResponse.Result(), auth.CSRFCookieName)
	if err := csrfManager.ValidateAnonymous(firstCookie.Value, firstBody.Token); err != nil {
		t.Fatalf("ValidateAnonymous first binding: %v", err)
	}
	assertCSRFCookieAttributes(t, firstCookie)
	if firstResponse.Header().Get("Set-Cookie") == "" {
		t.Fatal("first response did not set CSRF cookie")
	}
	if strings.Contains(firstResponse.Header().Get("Set-Cookie"), auth.SessionCookieName) {
		t.Error("CSRF endpoint created a session cookie")
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	secondRequest.AddCookie(firstCookie)
	secondResponse := httptest.NewRecorder()
	api.Handler().ServeHTTP(secondResponse, secondRequest)

	if secondResponse.Code != http.StatusOK {
		t.Fatalf("second status = %d: %s", secondResponse.Code, secondResponse.Body.String())
	}
	secondBody := decodeCSRFResponse(t, secondResponse.Result())
	if secondBody.Token != firstBody.Token {
		t.Errorf("reused token = %q, want %q", secondBody.Token, firstBody.Token)
	}
	if secondResponse.Header().Get("Set-Cookie") != "" {
		t.Errorf("reused binding unexpectedly reset cookie: %s", secondResponse.Header().Get("Set-Cookie"))
	}
}

func TestGetCSRFUsesValidatedSessionBinding(t *testing.T) {
	t.Parallel()

	sessionManager, issued := issueTestSession(t, csrfTestNow)
	csrfManager := newTestCSRFManager(t, csrfTestNow, &repeatReader{value: 0x44})
	options := defaultTestOptions()
	options.Sessions = sessionManager
	options.CSRF = csrfManager
	options.CSRFCookies = auth.NewCSRFCookies("", false, http.SameSiteLaxMode)
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	request.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: issued.Token,
	})
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	body := decodeCSRFResponse(t, response.Result())
	cookie := responseCookieNamed(t, response.Result(), auth.CSRFCookieName)
	if err := csrfManager.ValidateAuthenticated(
		cookie.Value,
		body.Token,
		issued.Session,
	); err != nil {
		t.Fatalf("ValidateAuthenticated: %v", err)
	}
	if err := csrfManager.ValidateAnonymous(cookie.Value, body.Token); !errors.Is(err, auth.ErrCSRFInvalid) {
		t.Errorf("anonymous validation error = %v, want ErrCSRFInvalid", err)
	}
	if !cookie.Expires.Equal(issued.ExpiresAt) {
		t.Errorf("cookie expiry = %v, want session expiry %v", cookie.Expires, issued.ExpiresAt)
	}
}

func TestGetCSRFInvalidSessionIsAnonymous(t *testing.T) {
	t.Parallel()

	csrfManager := newTestCSRFManager(t, csrfTestNow, &repeatReader{value: 0x55})
	options := defaultTestOptions()
	options.CSRF = csrfManager
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "invalid"})
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	body := decodeCSRFResponse(t, response.Result())
	cookie := responseCookieNamed(t, response.Result(), auth.CSRFCookieName)
	if err := csrfManager.ValidateAnonymous(cookie.Value, body.Token); err != nil {
		t.Fatalf("ValidateAnonymous: %v", err)
	}
	if strings.Contains(logs.String(), `"user_id"`) {
		t.Errorf("invalid session was logged as authenticated: %s", logs.String())
	}
}

func TestGetCSRFTreatsJWTExpiredWithinValidationSkewAsAnonymous(t *testing.T) {
	t.Parallel()

	issuer, issued := issueTestSession(t, csrfTestNow)
	_ = issuer
	requestTime := issued.ExpiresAt.Add(10 * time.Second)
	sessionValidator, err := auth.NewSessionManagerFrom(
		testSecret(0x11),
		clock.Fixed{Time: requestTime},
		&repeatReader{value: 0x66},
	)
	if err != nil {
		t.Fatalf("NewSessionManagerFrom: %v", err)
	}
	if _, err := sessionValidator.Validate(issued.Token); err != nil {
		t.Fatalf("test token should remain valid within JWT skew: %v", err)
	}
	csrfManager := newTestCSRFManager(t, requestTime, &repeatReader{value: 0x77})
	options := defaultTestOptions()
	options.Sessions = sessionValidator
	options.CSRF = csrfManager
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: issued.Token})
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	body := decodeCSRFResponse(t, response.Result())
	cookie := responseCookieNamed(t, response.Result(), auth.CSRFCookieName)
	if err := csrfManager.ValidateAnonymous(cookie.Value, body.Token); err != nil {
		t.Fatalf("fallback anonymous binding: %v", err)
	}
	if strings.Contains(logs.String(), `"user_id"`) {
		t.Errorf("expired session was logged as authenticated: %s", logs.String())
	}
}

func TestGetCSRFInternalFailureUsesCommonError(t *testing.T) {
	t.Parallel()

	csrfManager := newTestCSRFManager(
		t,
		csrfTestNow,
		errorReader{err: errors.New("entropy unavailable")},
	)
	options := defaultTestOptions()
	options.CSRF = csrfManager
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil),
	)

	assertAPIError(
		t,
		response.Result(),
		http.StatusInternalServerError,
		"internal_error",
	)
	if strings.Contains(response.Body.String(), "entropy unavailable") {
		t.Errorf("response exposes entropy failure: %s", response.Body.String())
	}
}

func TestCSRFMiddlewareMapsMaterialsAndRejectsCrossSession(t *testing.T) {
	t.Parallel()

	sessionOne := csrfTestSession(0x01)
	sessionTwo := csrfTestSession(0x02)
	csrfManager := newTestCSRFManager(t, csrfTestNow, &repeatReader{value: 0x22})
	binding, err := csrfManager.RotateAuthenticated(sessionOne)
	if err != nil {
		t.Fatalf("RotateAuthenticated: %v", err)
	}
	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	options.Sessions = sessionValidatorFunc(func(token string) (auth.Session, error) {
		switch token {
		case "session-one":
			return sessionOne, nil
		case "session-two":
			return sessionTwo, nil
		default:
			return auth.Session{}, auth.ErrUnauthenticated
		}
	})
	options.CSRF = csrfManager
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	api.register(
		"POST /api/test/authenticated",
		routeAuthenticatedUnsafe,
		func(w http.ResponseWriter, _ *http.Request) {
			writeNoContent(w)
		},
	)

	tests := []struct {
		name       string
		session    string
		origin     string
		cookie     string
		token      string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing session before origin",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "missing origin before CSRF",
			session:    "session-one",
			cookie:     binding.CookieValue,
			token:      binding.Token,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "missing CSRF",
			session:    "session-one",
			origin:     "https://web.example",
			wantStatus: http.StatusForbidden,
			wantCode:   "csrf_required",
		},
		{
			name:       "cross session",
			session:    "session-two",
			origin:     "https://web.example",
			cookie:     binding.CookieValue,
			token:      binding.Token,
			wantStatus: http.StatusForbidden,
			wantCode:   "csrf_invalid",
		},
		{
			name:       "valid",
			session:    "session-one",
			origin:     "https://web.example",
			cookie:     binding.CookieValue,
			token:      binding.Token,
			wantStatus: http.StatusNoContent,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/test/authenticated", nil)
			if test.session != "" {
				request.AddCookie(&http.Cookie{
					Name:  auth.SessionCookieName,
					Value: test.session,
				})
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.cookie != "" {
				request.AddCookie(&http.Cookie{
					Name:  auth.CSRFCookieName,
					Value: test.cookie,
				})
			}
			if test.token != "" {
				request.Header.Set(csrfTokenHeader, test.token)
			}
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			if test.wantCode == "" {
				if response.Code != test.wantStatus {
					t.Errorf("status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
				}
				if response.Body.Len() != 0 {
					t.Errorf("body = %q, want empty", response.Body.String())
				}
				return
			}
			assertAPIError(t, response.Result(), test.wantStatus, test.wantCode)
		})
	}
}

func TestCSRFMiddlewareRejectsDuplicateMaterials(t *testing.T) {
	t.Parallel()

	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	api.register(
		"POST /api/test/anonymous",
		routeAnonymousUnsafe,
		func(w http.ResponseWriter, _ *http.Request) {
			writeNoContent(w)
		},
	)
	request := httptest.NewRequest(http.MethodPost, "/api/test/anonymous", nil)
	request.Header.Set("Origin", "https://web.example")
	request.Header.Add(csrfTokenHeader, "first")
	request.Header.Add(csrfTokenHeader, "second")
	request.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "first"})
	request.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "second"})
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	assertAPIError(t, response.Result(), http.StatusForbidden, "csrf_invalid")
}

func TestCSRFMaterialsDuplicateClassificationIsOrderIndependent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cookies []string
		tokens  []string
	}{
		{
			name:    "cookie empty first",
			cookies: []string{"", "cookie"},
			tokens:  []string{"token"},
		},
		{
			name:    "cookie empty second",
			cookies: []string{"cookie", ""},
			tokens:  []string{"token"},
		},
		{
			name:    "header empty first",
			cookies: []string{"cookie"},
			tokens:  []string{"", "token"},
		},
		{
			name:    "header empty second",
			cookies: []string{"cookie"},
			tokens:  []string{"token", ""},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/test", nil)
			for _, cookie := range test.cookies {
				request.AddCookie(&http.Cookie{
					Name:  auth.CSRFCookieName,
					Value: cookie,
				})
			}
			for _, token := range test.tokens {
				request.Header.Add(csrfTokenHeader, token)
			}

			_, _, err := csrfMaterials(request)

			if !errors.Is(err, auth.ErrCSRFInvalid) {
				t.Errorf("error = %v, want ErrCSRFInvalid", err)
			}
		})
	}
}

func TestCSRFMiddlewareLogsUnexpectedProtectorFailure(t *testing.T) {
	t.Parallel()

	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	options.CSRF = fakeCSRFProtector{
		validateAnonymous: func(string, string) error {
			return errors.New("unexpected protector failure")
		},
	}
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	api.register(
		"POST /api/test/anonymous-failure",
		routeAnonymousUnsafe,
		func(w http.ResponseWriter, _ *http.Request) {
			writeNoContent(w)
		},
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/test/anonymous-failure",
		nil,
	)
	request.Header.Set("Origin", "https://web.example")
	request.Header.Set(csrfTokenHeader, "token")
	request.AddCookie(&http.Cookie{
		Name:  auth.CSRFCookieName,
		Value: "cookie",
	})
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	assertAPIError(
		t,
		response.Result(),
		http.StatusInternalServerError,
		"internal_error",
	)
	if !strings.Contains(logs.String(), `"msg":"HTTP request failed"`) {
		t.Errorf("unexpected protector failure was not logged: %s", logs.String())
	}
}

func newTestCSRFManager(
	t *testing.T,
	now time.Time,
	random interface{ Read([]byte) (int, error) },
) *auth.CSRFManager {
	t.Helper()
	manager, err := auth.NewCSRFManagerFrom(
		testSecret(0x22),
		clock.Fixed{Time: now},
		random,
	)
	if err != nil {
		t.Fatalf("NewCSRFManagerFrom: %v", err)
	}
	return manager
}

func issueTestSession(
	t *testing.T,
	now time.Time,
) (*auth.SessionManager, auth.IssuedSession) {
	t.Helper()
	manager, err := auth.NewSessionManagerFrom(
		testSecret(0x11),
		clock.Fixed{Time: now},
		&repeatReader{value: 0x44},
	)
	if err != nil {
		t.Fatalf("NewSessionManagerFrom: %v", err)
	}
	issued, err := manager.Issue("00112233-4455-4677-8899-aabbccddeeff")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return manager, issued
}

func csrfTestSession(jtiByte byte) auth.Session {
	return auth.Session{
		UserID: "00112233-4455-4677-8899-aabbccddeeff",
		JWTID: base64.RawURLEncoding.EncodeToString(
			bytes.Repeat([]byte{jtiByte}, 32),
		),
		IssuedAt:  csrfTestNow,
		ExpiresAt: csrfTestNow.Add(auth.SessionLifetime),
	}
}

func testSecret(value byte) []byte {
	return bytes.Repeat([]byte{value}, 32)
}

func decodeCSRFResponse(t *testing.T, response *http.Response) csrfResponse {
	t.Helper()
	defer response.Body.Close()
	var body csrfResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode CSRF response: %v", err)
	}
	if body.Token == "" {
		t.Error("CSRF token is empty")
	}
	return body
}

func responseCookieNamed(
	t *testing.T,
	response *http.Response,
	name string,
) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response has no %s cookie", name)
	return nil
}

func assertCSRFCookieAttributes(t *testing.T, cookie *http.Cookie) {
	t.Helper()
	if cookie.Name != auth.CSRFCookieName {
		t.Errorf("Name = %q", cookie.Name)
	}
	if cookie.Path != "/api" {
		t.Errorf("Path = %q", cookie.Path)
	}
	if cookie.Domain != "api.example.com" {
		t.Errorf("Domain = %q", cookie.Domain)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteNoneMode {
		t.Errorf(
			"cookie security = HttpOnly %v Secure %v SameSite %v",
			cookie.HttpOnly,
			cookie.Secure,
			cookie.SameSite,
		)
	}
	if cookie.MaxAge <= 0 || !cookie.Expires.After(csrfTestNow) {
		t.Errorf("cookie lifetime = MaxAge %d Expires %v", cookie.MaxAge, cookie.Expires)
	}
}
