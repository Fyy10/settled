package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/clock"
)

var authHandlerTestNow = time.Date(
	2026,
	time.July,
	30,
	18,
	45,
	12,
	345678901,
	time.UTC,
)

func TestRegisterReturnsUserAndRotatedCookieBindings(t *testing.T) {
	t.Parallel()

	result := authHandlerTestResult()
	binding := auth.CSRFBinding{
		CookieValue:   "authenticated-csrf-cookie",
		Token:         "v1.authenticated-csrf-token",
		ExpiresAt:     result.Session.ExpiresAt,
		MaxAgeSeconds: int(auth.SessionLifetime / time.Second),
	}
	events := make([]string, 0, 3)
	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	options.Auth = fakeAuthService{
		register: func(
			_ context.Context,
			input auth.RegisterInput,
		) (auth.AuthResult, error) {
			events = append(events, "register")
			want := auth.RegisterInput{
				Email:       " Alice@EXAMPLE.COM ",
				Password:    "private password",
				DisplayName: " Alice ",
			}
			if input != want {
				t.Errorf("registration input = %+v, want %+v", input, want)
			}
			return result, nil
		},
	}
	options.CSRF = fakeCSRFProtector{
		validateAnonymous: func(cookie, token string) error {
			events = append(events, "csrf")
			if cookie != "anonymous-cookie" || token != "anonymous-token" {
				t.Errorf("anonymous CSRF = (%q, %q)", cookie, token)
			}
			return nil
		},
		rotateAuthenticated: func(
			session auth.Session,
		) (auth.CSRFBinding, error) {
			events = append(events, "rotate")
			if session != result.Session.Session {
				t.Errorf("rotated session = %+v", session)
			}
			return binding, nil
		},
	}
	options.SessionCookies = auth.NewSessionCookies(
		"api.example.com",
		true,
		http.SameSiteNoneMode,
	)
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
	request := anonymousAuthRequest(
		http.MethodPost,
		"/api/auth/register",
		strings.NewReader(
			`{"email":" Alice@EXAMPLE.COM ","password":"private password",`+
				`"displayName":" Alice "}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", response.Code, response.Body.String())
	}
	if !reflect.DeepEqual(events, []string{"csrf", "register", "rotate"}) {
		t.Errorf("events = %v", events)
	}
	rawBody := response.Body.String()
	sessionCookie := responseCookieNamed(
		t,
		response.Result(),
		auth.SessionCookieName,
	)
	assertSessionCookie(t, sessionCookie, result.Session)
	csrfCookie := responseCookieNamed(t, response.Result(), auth.CSRFCookieName)
	assertRotatedCSRFCookie(t, csrfCookie, binding)

	body := decodeAuthResultResponse(t, response.Result())
	wantBody := authResponse{
		CSRFToken: binding.Token,
		User:      newUserResponse(result.User),
	}
	if body != wantBody {
		t.Errorf("response = %+v, want %+v", body, wantBody)
	}
	if strings.Contains(rawBody, result.Session.Token) ||
		strings.Contains(rawBody, "password_hash") {
		t.Error("response body exposes session or password hash material")
	}
	assertCORSHeaders(t, response.Header(), "https://web.example")
	assertSecurityHeaders(t, response.Header())
}

func TestRegisterStrictJSONAndDomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		contentType string
		serviceErr  error
		wantStatus  int
		wantCode    string
		wantCalls   int
	}{
		{
			name:        "unknown field",
			body:        `{"email":"a@b","password":"password","displayName":"A","extra":true}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:        "trailing value",
			body:        `{"email":"a@b","password":"password","displayName":"A"} {}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:       "missing content type",
			body:       `{"email":"a@b","password":"password","displayName":"A"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:        "validation",
			body:        `{}`,
			contentType: "application/json",
			serviceErr: &auth.ValidationError{Fields: map[string]string{
				"email":       "Email is required.",
				"password":    "Password must be between 8 and 128 bytes.",
				"displayName": "Display name is required.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantCalls:  1,
		},
		{
			name:        "duplicate email",
			body:        `{"email":"alice@example.com","password":"password","displayName":"Alice"}`,
			contentType: "application/json",
			serviceErr:  auth.ErrDuplicateEmail,
			wantStatus:  http.StatusConflict,
			wantCode:    "conflict",
			wantCalls:   1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registerCalls := 0
			csrfCalls := 0
			options := defaultTestOptions()
			options.AllowedOrigins = []string{"https://web.example"}
			options.Auth = fakeAuthService{
				register: func(
					context.Context,
					auth.RegisterInput,
				) (auth.AuthResult, error) {
					registerCalls++
					return auth.AuthResult{}, test.serviceErr
				},
			}
			options.CSRF = fakeCSRFProtector{
				validateAnonymous: func(string, string) error {
					csrfCalls++
					return nil
				},
			}
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := anonymousAuthRequest(
				http.MethodPost,
				"/api/auth/register",
				strings.NewReader(test.body),
			)
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			assertAPIError(
				t,
				response.Result(),
				test.wantStatus,
				test.wantCode,
			)
			if csrfCalls != 1 {
				t.Errorf("CSRF calls = %d, want 1", csrfCalls)
			}
			if registerCalls != test.wantCalls {
				t.Errorf("Register calls = %d, want %d", registerCalls, test.wantCalls)
			}
		})
	}
}

func TestRegisterChecksOriginAndCSRFFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		origin    string
		csrfError error
		wantCode  string
		wantCSRF  int
	}{
		{
			name:     "missing origin",
			wantCode: "forbidden",
		},
		{
			name:      "missing CSRF",
			origin:    "https://web.example",
			csrfError: auth.ErrCSRFRequired,
			wantCode:  "csrf_required",
			wantCSRF:  1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registerCalls := 0
			csrfCalls := 0
			options := defaultTestOptions()
			options.AllowedOrigins = []string{"https://web.example"}
			options.Auth = fakeAuthService{
				register: func(
					context.Context,
					auth.RegisterInput,
				) (auth.AuthResult, error) {
					registerCalls++
					return auth.AuthResult{}, nil
				},
			}
			options.CSRF = fakeCSRFProtector{
				validateAnonymous: func(string, string) error {
					csrfCalls++
					return test.csrfError
				},
			}
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := anonymousAuthRequest(
				http.MethodPost,
				"/api/auth/register",
				strings.NewReader(`{}`),
			)
			request.Header.Set("Content-Type", "application/json")
			if test.origin == "" {
				request.Header.Del("Origin")
			}
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			assertAPIError(
				t,
				response.Result(),
				http.StatusForbidden,
				test.wantCode,
			)
			if csrfCalls != test.wantCSRF {
				t.Errorf("CSRF calls = %d, want %d", csrfCalls, test.wantCSRF)
			}
			if registerCalls != 0 {
				t.Errorf("Register called %d times", registerCalls)
			}
		})
	}
}

func TestLoginRejectsMalformedBasicAuthAndRequestBodies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       io.Reader
		prepare    func(*http.Request)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing credentials",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name: "wrong scheme",
			prepare: func(request *http.Request) {
				request.Header.Set("Authorization", "Bearer token")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name: "malformed base64",
			prepare: func(request *http.Request) {
				request.Header.Set("Authorization", "Basic !!!")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name: "missing colon",
			prepare: func(request *http.Request) {
				request.Header.Set(
					"Authorization",
					"Basic "+base64.StdEncoding.EncodeToString([]byte("alice")),
				)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name: "duplicate headers",
			prepare: func(request *http.Request) {
				request.SetBasicAuth("alice@example.com", "password")
				request.Header.Add("Authorization", "Basic duplicate")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name: "invalid UTF-8 username",
			prepare: func(request *http.Request) {
				request.Header.Set(
					"Authorization",
					"Basic "+base64.StdEncoding.EncodeToString(
						[]byte{0xff, ':', 'p', 'a', 's', 's', 'w', 'o', 'r', 'd'},
					),
				)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name: "invalid UTF-8 password",
			prepare: func(request *http.Request) {
				request.Header.Set(
					"Authorization",
					"Basic "+base64.StdEncoding.EncodeToString(
						append([]byte("alice@example.com:"), 0xff),
					),
				)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name: "nonempty body",
			body: strings.NewReader("{}"),
			prepare: func(request *http.Request) {
				request.SetBasicAuth("alice@example.com", "password")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loginCalls := 0
			options := defaultTestOptions()
			options.AllowedOrigins = []string{"https://web.example"}
			options.Auth = fakeAuthService{
				login: func(
					context.Context,
					string,
					string,
				) (auth.AuthResult, error) {
					loginCalls++
					return auth.AuthResult{}, nil
				},
			}
			options.CSRF = fakeCSRFProtector{
				validateAnonymous: func(string, string) error { return nil },
			}
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := anonymousAuthRequest(
				http.MethodPost,
				"/api/auth/login",
				test.body,
			)
			if test.prepare != nil {
				test.prepare(request)
			}
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			assertAPIError(
				t,
				response.Result(),
				test.wantStatus,
				test.wantCode,
			)
			if loginCalls != 0 {
				t.Errorf("Login called %d times", loginCalls)
			}
		})
	}
}

func TestLoginSuccessUsesUTF8BasicAuthAndIndistinguishableFailures(t *testing.T) {
	t.Parallel()

	result := authHandlerTestResult()
	binding := auth.CSRFBinding{
		CookieValue:   "login-csrf-cookie",
		Token:         "v1.login-csrf-token",
		ExpiresAt:     result.Session.ExpiresAt,
		MaxAgeSeconds: int(auth.SessionLifetime / time.Second),
	}
	events := make([]string, 0, 3)
	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	options.Auth = fakeAuthService{
		login: func(
			_ context.Context,
			email string,
			password string,
		) (auth.AuthResult, error) {
			events = append(events, "login")
			if email != "álîçé@example.com" || password != "密码secure" {
				t.Errorf("credentials = (%q, %q)", email, password)
			}
			return result, nil
		},
	}
	options.CSRF = fakeCSRFProtector{
		validateAnonymous: func(string, string) error {
			events = append(events, "csrf")
			return nil
		},
		rotateAuthenticated: func(
			session auth.Session,
		) (auth.CSRFBinding, error) {
			events = append(events, "rotate")
			if session != result.Session.Session {
				t.Errorf("session = %+v", session)
			}
			return binding, nil
		},
	}
	options.SessionCookies = auth.NewSessionCookies("", true, http.SameSiteNoneMode)
	options.CSRFCookies = auth.NewCSRFCookies("", true, http.SameSiteNoneMode)
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := anonymousAuthRequest(http.MethodPost, "/api/auth/login", nil)
	request.SetBasicAuth("álîçé@example.com", "密码secure")
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if !reflect.DeepEqual(events, []string{"csrf", "login", "rotate"}) {
		t.Errorf("events = %v", events)
	}
	body := decodeAuthResultResponse(t, response.Result())
	if body != (authResponse{
		CSRFToken: binding.Token,
		User:      newUserResponse(result.User),
	}) {
		t.Errorf("body = %+v", body)
	}
	assertSessionCookie(
		t,
		responseCookieNamed(t, response.Result(), auth.SessionCookieName),
		result.Session,
	)

	var failureBodies []string
	for _, email := range []string{"unknown@example.com", "alice@example.com"} {
		failureOptions := defaultTestOptions()
		failureOptions.AllowedOrigins = []string{"https://web.example"}
		failureOptions.Auth = fakeAuthService{
			login: func(
				context.Context,
				string,
				string,
			) (auth.AuthResult, error) {
				return auth.AuthResult{}, auth.ErrInvalidCredentials
			},
		}
		failureOptions.CSRF = fakeCSRFProtector{
			validateAnonymous: func(string, string) error { return nil },
		}
		failureAPI, _ := newTestAPI(
			t,
			pingerFunc(func(context.Context) error { return nil }),
			failureOptions,
		)
		failureRequest := anonymousAuthRequest(
			http.MethodPost,
			"/api/auth/login",
			nil,
		)
		failureRequest.SetBasicAuth(email, "wrong password")
		failureResponse := httptest.NewRecorder()

		failureAPI.Handler().ServeHTTP(failureResponse, failureRequest)

		if failureResponse.Code != http.StatusUnauthorized {
			t.Errorf(
				"failure status = %d: %s",
				failureResponse.Code,
				failureResponse.Body.String(),
			)
		}
		failureBodies = append(failureBodies, failureResponse.Body.String())
	}
	if failureBodies[0] != failureBodies[1] {
		t.Errorf("credential failure bodies differ: %q != %q", failureBodies[0], failureBodies[1])
	}
}

func TestLoginOversizedPasswordUsesBoundedDummyWithoutLookup(t *testing.T) {
	t.Parallel()

	passwords := &boundedPasswordManager{}
	users := &countingUserStore{}
	service, err := auth.NewServiceFrom(
		users,
		passwords,
		sessionIssuerStub{},
		clock.Fixed{Time: authHandlerTestNow},
		bytes.NewReader(make([]byte, 16)),
	)
	if err != nil {
		t.Fatalf("NewServiceFrom: %v", err)
	}
	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	options.Auth = service
	options.CSRF = fakeCSRFProtector{
		validateAnonymous: func(string, string) error { return nil },
	}
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	privatePassword := strings.Repeat("p", 129)
	request := anonymousAuthRequest(http.MethodPost, "/api/auth/login", nil)
	request.SetBasicAuth("alice@example.com", privatePassword)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	assertAPIError(
		t,
		response.Result(),
		http.StatusUnauthorized,
		"unauthorized",
	)
	if users.credentialLookups != 0 {
		t.Errorf("credential lookups = %d, want 0", users.credentialLookups)
	}
	if passwords.verifyCalls != 0 {
		t.Errorf("real verification calls = %d, want 0", passwords.verifyCalls)
	}
	if passwords.dummyPassword != "invalid-password" {
		t.Errorf("dummy password = %q, want fixed bounded surrogate", passwords.dummyPassword)
	}
	if strings.Contains(response.Body.String(), privatePassword) {
		t.Error("response exposes oversized password")
	}
}

func TestMeReloadsCurrentUserForEveryRequest(t *testing.T) {
	t.Parallel()

	session := authHandlerTestResult().Session.Session
	first := authHandlerTestUser()
	second := first
	second.DisplayName = "Updated Alice"
	second.UpdatedAt = second.UpdatedAt.Add(time.Minute)
	findCalls := 0
	options := defaultTestOptions()
	options.Sessions = sessionValidatorFunc(func(token string) (auth.Session, error) {
		if token != "valid-session" {
			return auth.Session{}, auth.ErrUnauthenticated
		}
		return session, nil
	})
	options.Auth = fakeAuthService{
		findUser: func(
			_ context.Context,
			userID string,
		) (auth.User, error) {
			findCalls++
			if userID != session.UserID {
				t.Errorf("user ID = %q", userID)
			}
			if findCalls == 1 {
				return first, nil
			}
			return second, nil
		},
	}
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)

	for index, want := range []auth.User{first, second} {
		request := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		request.AddCookie(&http.Cookie{
			Name:  auth.SessionCookieName,
			Value: "valid-session",
		})
		response := httptest.NewRecorder()

		api.Handler().ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf(
				"request %d status = %d: %s",
				index,
				response.Code,
				response.Body.String(),
			)
		}
		var body meResponse
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatalf("decode request %d: %v", index, err)
		}
		if body.User != newUserResponse(want) {
			t.Errorf("request %d user = %+v, want %+v", index, body.User, want)
		}
	}
	if findCalls != 2 {
		t.Errorf("FindUser calls = %d, want 2", findCalls)
	}
}

func TestAuthenticationMiddlewareRejectsExpiredDeletedAndFailedLookups(t *testing.T) {
	t.Parallel()

	session := authHandlerTestResult().Session.Session
	sourceError := errors.New("database unavailable")
	tests := []struct {
		name       string
		sessionErr error
		findErr    error
		wantStatus int
		wantCode   string
		wantFind   int
	}{
		{
			name:       "expired session",
			sessionErr: auth.ErrUnauthenticated,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "deleted user",
			findErr:    auth.ErrUserNotFound,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
			wantFind:   1,
		},
		{
			name:       "lookup failure",
			findErr:    sourceError,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantFind:   1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			findCalls := 0
			options := defaultTestOptions()
			options.Sessions = sessionValidatorFunc(
				func(string) (auth.Session, error) {
					if test.sessionErr != nil {
						return auth.Session{}, test.sessionErr
					}
					return session, nil
				},
			)
			options.Auth = fakeAuthService{
				findUser: func(
					context.Context,
					string,
				) (auth.User, error) {
					findCalls++
					return auth.User{}, test.findErr
				},
			}
			api, logs := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := httptest.NewRequest(http.MethodGet, "/api/me", nil)
			request.AddCookie(&http.Cookie{
				Name:  auth.SessionCookieName,
				Value: "session",
			})
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			assertAPIError(
				t,
				response.Result(),
				test.wantStatus,
				test.wantCode,
			)
			if findCalls != test.wantFind {
				t.Errorf("FindUser calls = %d, want %d", findCalls, test.wantFind)
			}
			if test.findErr == sourceError &&
				!strings.Contains(logs.String(), "HTTP request failed") {
				t.Errorf("lookup error not logged: %s", logs.String())
			}
		})
	}
}

func TestLogoutRequiresAuthenticationAndRotatesToAnonymous(t *testing.T) {
	t.Parallel()

	result := authHandlerTestResult()
	binding := auth.CSRFBinding{
		CookieValue:   "fresh-anonymous-cookie",
		Token:         "v1.fresh-anonymous-token",
		ExpiresAt:     authHandlerTestNow.Truncate(time.Second).Add(time.Hour),
		MaxAgeSeconds: int(time.Hour / time.Second),
	}
	events := make([]string, 0, 3)
	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	options.Sessions = sessionValidatorFunc(func(token string) (auth.Session, error) {
		if token != result.Session.Token {
			return auth.Session{}, auth.ErrUnauthenticated
		}
		return result.Session.Session, nil
	})
	options.Auth = fakeAuthService{
		findUser: func(
			context.Context,
			string,
		) (auth.User, error) {
			events = append(events, "user")
			return result.User, nil
		},
	}
	options.CSRF = fakeCSRFProtector{
		validateAuthenticated: func(
			cookie string,
			token string,
			session auth.Session,
		) error {
			events = append(events, "csrf")
			if cookie != "session-csrf-cookie" ||
				token != "session-csrf-token" ||
				session != result.Session.Session {
				t.Errorf(
					"authenticated CSRF inputs = (%q, %q, %+v)",
					cookie,
					token,
					session,
				)
			}
			return nil
		},
		rotateAnonymous: func() (auth.CSRFBinding, error) {
			events = append(events, "rotate")
			return binding, nil
		},
	}
	options.SessionCookies = auth.NewSessionCookies(
		"api.example.com",
		true,
		http.SameSiteNoneMode,
	)
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
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.Header.Set("Origin", "https://web.example")
	request.Header.Set(csrfTokenHeader, "session-csrf-token")
	request.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: result.Session.Token,
	})
	request.AddCookie(&http.Cookie{
		Name:  auth.CSRFCookieName,
		Value: "session-csrf-cookie",
	})
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", response.Code, response.Body.String())
	}
	if response.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", response.Body.String())
	}
	if response.Header().Get("Content-Type") != "" {
		t.Errorf("Content-Type = %q, want empty", response.Header().Get("Content-Type"))
	}
	if !reflect.DeepEqual(events, []string{"user", "csrf", "rotate"}) {
		t.Errorf("events = %v", events)
	}
	cleared := responseCookieNamed(t, response.Result(), auth.SessionCookieName)
	if cleared.Value != "" || cleared.MaxAge != -1 || !cleared.HttpOnly ||
		cleared.Path != "/api" || cleared.Domain != "api.example.com" {
		t.Errorf("cleared session cookie = %+v", cleared)
	}
	assertRotatedCSRFCookie(
		t,
		responseCookieNamed(t, response.Result(), auth.CSRFCookieName),
		binding,
	)

	missingRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	missingResponse := httptest.NewRecorder()
	api.Handler().ServeHTTP(missingResponse, missingRequest)
	assertAPIError(
		t,
		missingResponse.Result(),
		http.StatusUnauthorized,
		"unauthorized",
	)
}

func anonymousAuthRequest(
	method string,
	path string,
	body io.Reader,
) *http.Request {
	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Origin", "https://web.example")
	request.Header.Set(csrfTokenHeader, "anonymous-token")
	request.AddCookie(&http.Cookie{
		Name:  auth.CSRFCookieName,
		Value: "anonymous-cookie",
	})
	return request
}

func decodeAuthResultResponse(
	t *testing.T,
	response *http.Response,
) authResponse {
	t.Helper()
	defer response.Body.Close()
	var body authResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode auth response: %v", err)
	}
	return body
}

func assertSessionCookie(
	t *testing.T,
	cookie *http.Cookie,
	issued auth.IssuedSession,
) {
	t.Helper()
	if cookie.Value != issued.Token ||
		cookie.Path != "/api" ||
		!cookie.HttpOnly ||
		!cookie.Secure ||
		cookie.SameSite != http.SameSiteNoneMode ||
		cookie.MaxAge != int(auth.SessionLifetime/time.Second) ||
		!cookie.Expires.Equal(issued.ExpiresAt) {
		t.Errorf("session cookie = %+v", cookie)
	}
}

func assertRotatedCSRFCookie(
	t *testing.T,
	cookie *http.Cookie,
	binding auth.CSRFBinding,
) {
	t.Helper()
	if cookie.Value != binding.CookieValue ||
		cookie.Path != "/api" ||
		!cookie.HttpOnly ||
		!cookie.Secure ||
		cookie.SameSite != http.SameSiteNoneMode ||
		cookie.MaxAge != binding.MaxAgeSeconds ||
		!cookie.Expires.Equal(binding.ExpiresAt) {
		t.Errorf("CSRF cookie = %+v", cookie)
	}
}

func authHandlerTestResult() auth.AuthResult {
	user := authHandlerTestUser()
	return auth.AuthResult{
		User: user,
		Session: auth.IssuedSession{
			Token: "signed-session-token",
			Session: auth.Session{
				UserID:    user.ID,
				JWTID:     "session-jwt-id",
				IssuedAt:  authHandlerTestNow.Truncate(time.Second),
				ExpiresAt: authHandlerTestNow.Truncate(time.Second).Add(auth.SessionLifetime),
			},
		},
	}
}

func authHandlerTestUser() auth.User {
	return auth.User{
		ID:          "00112233-4455-4677-8899-aabbccddeeff",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		CreatedAt:   authHandlerTestNow,
		UpdatedAt:   authHandlerTestNow,
	}
}

type boundedPasswordManager struct {
	dummyPassword string
	verifyCalls   int
}

func (*boundedPasswordManager) Hash(string) (string, error) {
	return "", errors.New("unexpected Hash call")
}

func (passwords *boundedPasswordManager) Verify(string, string) (bool, error) {
	passwords.verifyCalls++
	return false, nil
}

func (passwords *boundedPasswordManager) VerifyDummy(password string) error {
	passwords.dummyPassword = password
	return nil
}

type countingUserStore struct {
	credentialLookups int
}

func (*countingUserStore) CreateUser(
	context.Context,
	auth.NewUser,
) (auth.User, error) {
	return auth.User{}, errors.New("unexpected CreateUser call")
}

func (store *countingUserStore) FindCredentialsByEmail(
	context.Context,
	string,
) (auth.Credentials, error) {
	store.credentialLookups++
	return auth.Credentials{}, auth.ErrUserNotFound
}

func (*countingUserStore) FindUserByID(
	context.Context,
	string,
) (auth.User, error) {
	return auth.User{}, errors.New("unexpected FindUserByID call")
}

type sessionIssuerStub struct{}

func (sessionIssuerStub) Issue(string) (auth.IssuedSession, error) {
	return auth.IssuedSession{}, errors.New("unexpected Issue call")
}
