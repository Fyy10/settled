package httpapi

import (
	"bytes"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/Fyy10/settled/server/internal/auth"
)

type sessionValidatorFunc func(string) (auth.Session, error)

func (validate sessionValidatorFunc) Validate(token string) (auth.Session, error) {
	return validate(token)
}

type fakeCSRFProtector struct {
	issueOrReuse          func(string, *auth.Session) (auth.CSRFBinding, error)
	validateAnonymous     func(string, string) error
	validateAuthenticated func(string, string, auth.Session) error
}

func (protector fakeCSRFProtector) IssueOrReuse(
	cookie string,
	session *auth.Session,
) (auth.CSRFBinding, error) {
	if protector.issueOrReuse == nil {
		return auth.CSRFBinding{}, errors.New("unexpected IssueOrReuse call")
	}
	return protector.issueOrReuse(cookie, session)
}

func (protector fakeCSRFProtector) ValidateAnonymous(cookie, token string) error {
	if protector.validateAnonymous == nil {
		return errors.New("unexpected ValidateAnonymous call")
	}
	return protector.validateAnonymous(cookie, token)
}

func (protector fakeCSRFProtector) ValidateAuthenticated(
	cookie string,
	token string,
	session auth.Session,
) error {
	if protector.validateAuthenticated == nil {
		return errors.New("unexpected ValidateAuthenticated call")
	}
	return protector.validateAuthenticated(cookie, token, session)
}

type repeatReader struct {
	mu    sync.Mutex
	value byte
}

func (reader *repeatReader) Read(destination []byte) (int, error) {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	for index := range destination {
		destination[index] = reader.value
	}
	return len(destination), nil
}

func defaultTestOptions() Options {
	return Options{
		Sessions: sessionValidatorFunc(func(string) (auth.Session, error) {
			return auth.Session{}, auth.ErrUnauthenticated
		}),
		CSRF: fakeCSRFProtector{
			issueOrReuse: func(string, *auth.Session) (auth.CSRFBinding, error) {
				return auth.CSRFBinding{
					CookieValue:   "test-cookie",
					Token:         "v1.test-token",
					MaxAgeSeconds: 3600,
				}, nil
			},
			validateAnonymous: func(string, string) error { return nil },
			validateAuthenticated: func(
				string,
				string,
				auth.Session,
			) error {
				return nil
			},
		},
		RequestIDBytes: &repeatReader{value: 0x11},
	}
}

func newTestAPI(
	t *testing.T,
	pinger Pinger,
	options Options,
) (*API, *bytes.Buffer) {
	t.Helper()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	api, err := New(pinger, logger, options)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return api, &logs
}
