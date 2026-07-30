package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessionCookiesSetExactAttributes(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, time.August, 5, 20, 15, 30, 0, time.UTC)
	cookies := NewSessionCookies("api.example.com", true, http.SameSiteNoneMode)
	response := httptest.NewRecorder()

	cookies.Set(response, "signed.jwt.token", expiresAt)

	cookie := singleResponseCookie(t, response)
	assertCommonSessionCookieAttributes(
		t,
		cookie,
		"api.example.com",
		true,
		http.SameSiteNoneMode,
	)
	if cookie.Value != "signed.jwt.token" {
		t.Errorf("Value = %q", cookie.Value)
	}
	if cookie.MaxAge != int(SessionLifetime/time.Second) {
		t.Errorf("MaxAge = %d, want %d", cookie.MaxAge, int(SessionLifetime/time.Second))
	}
	if !cookie.Expires.Equal(expiresAt) {
		t.Errorf("Expires = %v, want %v", cookie.Expires, expiresAt)
	}
	header := response.Header().Get("Set-Cookie")
	for _, fragment := range []string{
		"Max-Age=604800",
		"HttpOnly",
		"Secure",
		"SameSite=None",
	} {
		if !strings.Contains(header, fragment) {
			t.Errorf("Set-Cookie missing %q: %s", fragment, header)
		}
	}
}

func TestSessionCookiesClearUsesMatchingAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		domain   string
		secure   bool
		sameSite http.SameSite
	}{
		{
			name:     "production cross-site",
			domain:   "api.example.com",
			secure:   true,
			sameSite: http.SameSiteNoneMode,
		},
		{
			name:     "local host-only",
			secure:   false,
			sameSite: http.SameSiteLaxMode,
		},
		{
			name:     "strict",
			domain:   "localhost",
			secure:   true,
			sameSite: http.SameSiteStrictMode,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cookies := NewSessionCookies(test.domain, test.secure, test.sameSite)
			response := httptest.NewRecorder()
			cookies.Clear(response)

			cookie := singleResponseCookie(t, response)
			assertCommonSessionCookieAttributes(
				t,
				cookie,
				test.domain,
				test.secure,
				test.sameSite,
			)
			if cookie.Value != "" {
				t.Errorf("Value = %q, want empty", cookie.Value)
			}
			if cookie.MaxAge >= 0 {
				t.Errorf("MaxAge = %d, want negative", cookie.MaxAge)
			}
			if !cookie.Expires.Equal(clearedSessionExpiration) {
				t.Errorf(
					"Expires = %v, want %v",
					cookie.Expires,
					clearedSessionExpiration,
				)
			}
			if !cookie.Expires.Before(time.Now()) {
				t.Errorf("Expires = %v, want a past instant", cookie.Expires)
			}

			header := response.Header().Get("Set-Cookie")
			if !strings.Contains(header, "Max-Age=0") {
				t.Errorf("clear Set-Cookie missing Max-Age=0: %s", header)
			}
			if test.domain == "" && strings.Contains(header, "Domain=") {
				t.Errorf("host-only cookie unexpectedly has Domain: %s", header)
			}
		})
	}
}

func assertCommonSessionCookieAttributes(
	t *testing.T,
	cookie *http.Cookie,
	domain string,
	secure bool,
	sameSite http.SameSite,
) {
	t.Helper()

	if cookie.Name != SessionCookieName {
		t.Errorf("Name = %q, want %q", cookie.Name, SessionCookieName)
	}
	if cookie.Path != sessionCookiePath {
		t.Errorf("Path = %q, want %q", cookie.Path, sessionCookiePath)
	}
	if cookie.Domain != domain {
		t.Errorf("Domain = %q, want %q", cookie.Domain, domain)
	}
	if !cookie.HttpOnly {
		t.Error("HttpOnly = false, want true")
	}
	if cookie.Secure != secure {
		t.Errorf("Secure = %v, want %v", cookie.Secure, secure)
	}
	if cookie.SameSite != sameSite {
		t.Errorf("SameSite = %v, want %v", cookie.SameSite, sameSite)
	}
}

func singleResponseCookie(
	t *testing.T,
	response *httptest.ResponseRecorder,
) *http.Cookie {
	t.Helper()

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("response cookie count = %d, want 1", len(cookies))
	}
	return cookies[0]
}
