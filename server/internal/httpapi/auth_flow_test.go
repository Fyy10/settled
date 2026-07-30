package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/clock"
)

func TestAuthenticationCookieJarFlow(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	fixedClock := clock.Fixed{Time: now}
	sessions, err := auth.NewSessionManagerFrom(
		testSecret(0x61),
		fixedClock,
		&repeatReader{value: 0x62},
	)
	if err != nil {
		t.Fatalf("NewSessionManagerFrom: %v", err)
	}
	csrf, err := auth.NewCSRFManagerFrom(
		testSecret(0x63),
		fixedClock,
		&repeatReader{value: 0x64},
	)
	if err != nil {
		t.Fatalf("NewCSRFManagerFrom: %v", err)
	}
	users := &flowUserStore{}
	passwords := &flowPasswordManager{}
	authService, err := auth.NewServiceFrom(
		users,
		passwords,
		sessions,
		fixedClock,
		&repeatReader{value: 0x65},
	)
	if err != nil {
		t.Fatalf("NewServiceFrom: %v", err)
	}
	api, err := New(
		pingerFunc(func(context.Context) error { return nil }),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Options{
			AllowedOrigins: []string{"https://web.example"},
			Auth:           authService,
			Sessions:       sessions,
			CSRF:           csrf,
			SessionCookies: auth.NewSessionCookies(
				"",
				false,
				http.SameSiteLaxMode,
			),
			CSRFCookies: auth.NewCSRFCookies(
				"",
				false,
				http.SameSiteLaxMode,
			),
			RequestIDBytes: &repeatReader{value: 0x66},
		},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	client := &http.Client{
		Transport: handlerRoundTripper{handler: api.Handler()},
		Jar:       jar,
	}
	const serverURL = "http://settled.test"

	anonymousToken := flowGetCSRF(t, client, serverURL)
	registerRequest, err := http.NewRequest(
		http.MethodPost,
		serverURL+"/api/auth/register",
		strings.NewReader(
			`{"email":"Alice@EXAMPLE.COM","password":"private password",`+
				`"displayName":" Alice "}`,
		),
	)
	if err != nil {
		t.Fatalf("NewRequest register: %v", err)
	}
	registerRequest.Header.Set("Origin", "https://web.example")
	registerRequest.Header.Set("Content-Type", "application/json")
	registerRequest.Header.Set(csrfTokenHeader, anonymousToken)
	registerStatus, registerBody := flowDo(t, client, registerRequest)
	if registerStatus != http.StatusCreated {
		t.Fatalf("register status = %d: %s", registerStatus, registerBody)
	}
	registerResult := decodeFlowAuthResponse(t, registerBody)
	if registerResult.User.Email != "alice@example.com" ||
		registerResult.User.DisplayName != "Alice" ||
		registerResult.CSRFToken == "" {
		t.Errorf("register response = %+v", registerResult)
	}
	registerSessionToken := flowCookieValue(
		t,
		jar,
		serverURL,
		auth.SessionCookieName,
	)
	if strings.Contains(string(registerBody), "private password") ||
		strings.Contains(string(registerBody), "password_hash") ||
		strings.Contains(string(registerBody), flowPasswordHash) ||
		strings.Contains(string(registerBody), registerSessionToken) {
		t.Errorf("register response exposes private authentication material: %s", registerBody)
	}
	assertFlowCookiePresence(t, jar, serverURL, true)

	flowRequireMe(t, client, serverURL, "Alice")
	flowLogout(t, client, serverURL, registerResult.CSRFToken)
	assertFlowCookiePresence(t, jar, serverURL, false)
	flowRequireUnauthorizedMe(t, client, serverURL)

	loginAnonymousToken := flowGetCSRF(t, client, serverURL)
	loginRequest, err := http.NewRequest(
		http.MethodPost,
		serverURL+"/api/auth/login",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest login: %v", err)
	}
	loginRequest.Header.Set("Origin", "https://web.example")
	loginRequest.Header.Set(csrfTokenHeader, loginAnonymousToken)
	loginRequest.SetBasicAuth("ALICE@example.com", "private password")
	loginStatus, loginBody := flowDo(t, client, loginRequest)
	if loginStatus != http.StatusOK {
		t.Fatalf("login status = %d: %s", loginStatus, loginBody)
	}
	loginResult := decodeFlowAuthResponse(t, loginBody)
	if loginResult.User != registerResult.User || loginResult.CSRFToken == "" {
		t.Errorf("login response = %+v, register response = %+v", loginResult, registerResult)
	}
	loginSessionToken := flowCookieValue(
		t,
		jar,
		serverURL,
		auth.SessionCookieName,
	)
	if strings.Contains(string(loginBody), "private password") ||
		strings.Contains(string(loginBody), "password_hash") ||
		strings.Contains(string(loginBody), flowPasswordHash) ||
		strings.Contains(string(loginBody), loginSessionToken) {
		t.Errorf("login response exposes private authentication material: %s", loginBody)
	}
	assertFlowCookiePresence(t, jar, serverURL, true)
	flowRequireMe(t, client, serverURL, "Alice")
	flowLogout(t, client, serverURL, loginResult.CSRFToken)
	assertFlowCookiePresence(t, jar, serverURL, false)
}

func flowGetCSRF(t *testing.T, client *http.Client, serverURL string) string {
	t.Helper()
	request, err := http.NewRequest(
		http.MethodGet,
		serverURL+"/api/auth/csrf",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest CSRF: %v", err)
	}
	status, body := flowDo(t, client, request)
	if status != http.StatusOK {
		t.Fatalf("CSRF status = %d: %s", status, body)
	}
	var response csrfResponse
	if err := decodeFlowJSON(body, &response); err != nil {
		t.Fatalf("decode CSRF response: %v", err)
	}
	if response.Token == "" {
		t.Fatal("CSRF token is empty")
	}
	return response.Token
}

func flowRequireMe(
	t *testing.T,
	client *http.Client,
	serverURL string,
	displayName string,
) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, serverURL+"/api/me", nil)
	if err != nil {
		t.Fatalf("NewRequest me: %v", err)
	}
	status, body := flowDo(t, client, request)
	if status != http.StatusOK {
		t.Fatalf("me status = %d: %s", status, body)
	}
	var response meResponse
	if err := decodeFlowJSON(body, &response); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if response.User.DisplayName != displayName {
		t.Errorf("me user = %+v", response.User)
	}
}

func flowRequireUnauthorizedMe(
	t *testing.T,
	client *http.Client,
	serverURL string,
) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, serverURL+"/api/me", nil)
	if err != nil {
		t.Fatalf("NewRequest me: %v", err)
	}
	status, body := flowDo(t, client, request)
	if status != http.StatusUnauthorized {
		t.Fatalf("me status after logout = %d: %s", status, body)
	}
}

func flowLogout(
	t *testing.T,
	client *http.Client,
	serverURL string,
	csrfToken string,
) {
	t.Helper()
	request, err := http.NewRequest(
		http.MethodPost,
		serverURL+"/api/auth/logout",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest logout: %v", err)
	}
	request.Header.Set("Origin", "https://web.example")
	request.Header.Set(csrfTokenHeader, csrfToken)
	status, body := flowDo(t, client, request)
	if status != http.StatusNoContent || len(body) != 0 {
		t.Fatalf("logout result = %d %q", status, body)
	}
}

func flowDo(
	t *testing.T,
	client *http.Client,
	request *http.Request,
) (int, []byte) {
	t.Helper()
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", request.Method, request.URL, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s %s: %v", request.Method, request.URL, err)
	}
	return response.StatusCode, body
}

func decodeFlowAuthResponse(t *testing.T, body []byte) authResponse {
	t.Helper()
	var response authResponse
	if err := decodeFlowJSON(body, &response); err != nil {
		t.Fatalf("decode auth response: %v", err)
	}
	return response
}

func decodeFlowJSON(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}

func assertFlowCookiePresence(
	t *testing.T,
	jar http.CookieJar,
	serverURL string,
	wantSession bool,
) {
	t.Helper()
	endpoint, err := url.Parse(serverURL + "/api/me")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	hasCSRF := false
	hasSession := false
	for _, cookie := range jar.Cookies(endpoint) {
		switch cookie.Name {
		case auth.CSRFCookieName:
			hasCSRF = true
		case auth.SessionCookieName:
			hasSession = true
		}
	}
	if !hasCSRF || hasSession != wantSession {
		t.Errorf(
			"cookie presence = CSRF %v session %v, want CSRF true session %v",
			hasCSRF,
			hasSession,
			wantSession,
		)
	}
}

func flowCookieValue(
	t *testing.T,
	jar http.CookieJar,
	serverURL string,
	name string,
) string {
	t.Helper()
	endpoint, err := url.Parse(serverURL + "/api/me")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	for _, cookie := range jar.Cookies(endpoint) {
		if cookie.Name == name {
			if cookie.Value == "" {
				t.Fatalf("%s cookie is empty", name)
			}
			return cookie.Value
		}
	}
	t.Fatalf("%s cookie is missing", name)
	return ""
}

type flowUserStore struct {
	mu           sync.Mutex
	user         auth.User
	passwordHash string
	created      bool
}

func (store *flowUserStore) CreateUser(
	_ context.Context,
	input auth.NewUser,
) (auth.User, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.created {
		return auth.User{}, auth.ErrDuplicateEmail
	}
	store.user = auth.User{
		ID:          input.ID,
		Email:       input.Email,
		DisplayName: input.DisplayName,
		CreatedAt:   input.CreatedAt,
		UpdatedAt:   input.UpdatedAt,
	}
	store.passwordHash = input.PasswordHash
	store.created = true
	return store.user, nil
}

func (store *flowUserStore) FindCredentialsByEmail(
	_ context.Context,
	email string,
) (auth.Credentials, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.created || email != store.user.Email {
		return auth.Credentials{}, auth.ErrUserNotFound
	}
	return auth.Credentials{
		User:         store.user,
		PasswordHash: store.passwordHash,
	}, nil
}

func (store *flowUserStore) FindUserByID(
	_ context.Context,
	userID string,
) (auth.User, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.created || userID != store.user.ID {
		return auth.User{}, auth.ErrUserNotFound
	}
	return store.user, nil
}

type flowPasswordManager struct {
	mu       sync.Mutex
	password string
}

const flowPasswordHash = "test-password-hash"

func (passwords *flowPasswordManager) Hash(password string) (string, error) {
	passwords.mu.Lock()
	defer passwords.mu.Unlock()
	passwords.password = password
	return flowPasswordHash, nil
}

func (passwords *flowPasswordManager) Verify(
	password string,
	encodedHash string,
) (bool, error) {
	passwords.mu.Lock()
	defer passwords.mu.Unlock()
	return password == passwords.password &&
		encodedHash == flowPasswordHash, nil
}

func (*flowPasswordManager) VerifyDummy(string) error {
	return nil
}

type handlerRoundTripper struct {
	handler http.Handler
}

func (transport handlerRoundTripper) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	response.Request = request
	return response, nil
}
