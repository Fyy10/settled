package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/groups"
)

type sessionValidatorFunc func(string) (auth.Session, error)

func (validate sessionValidatorFunc) Validate(token string) (auth.Session, error) {
	return validate(token)
}

type fakeAuthService struct {
	register func(context.Context, auth.RegisterInput) (auth.AuthResult, error)
	login    func(context.Context, string, string) (auth.AuthResult, error)
	findUser func(context.Context, string) (auth.User, error)
}

type fakeGroupService struct {
	create       func(context.Context, string, string) (groups.Group, error)
	list         func(context.Context, string) ([]groups.Group, error)
	join         func(context.Context, string, string) (groups.Group, error)
	get          func(context.Context, string, string) (groups.Detail, error)
	rename       func(context.Context, string, string, string) (groups.Group, error)
	dissolve     func(context.Context, string, string) error
	getJoinCode  func(context.Context, string, string) (string, error)
	removeMember func(context.Context, string, string, string) error
}

func (service fakeGroupService) Create(
	ctx context.Context,
	actorID string,
	name string,
) (groups.Group, error) {
	if service.create == nil {
		return groups.Group{}, errors.New("unexpected group Create call")
	}
	return service.create(ctx, actorID, name)
}

func (service fakeGroupService) List(
	ctx context.Context,
	actorID string,
) ([]groups.Group, error) {
	if service.list == nil {
		return nil, errors.New("unexpected group List call")
	}
	return service.list(ctx, actorID)
}

func (service fakeGroupService) Join(
	ctx context.Context,
	actorID string,
	joinCode string,
) (groups.Group, error) {
	if service.join == nil {
		return groups.Group{}, errors.New("unexpected group Join call")
	}
	return service.join(ctx, actorID, joinCode)
}

func (service fakeGroupService) Get(
	ctx context.Context,
	actorID string,
	groupID string,
) (groups.Detail, error) {
	if service.get == nil {
		return groups.Detail{}, errors.New("unexpected group Get call")
	}
	return service.get(ctx, actorID, groupID)
}

func (service fakeGroupService) Rename(
	ctx context.Context,
	actorID string,
	groupID string,
	name string,
) (groups.Group, error) {
	if service.rename == nil {
		return groups.Group{}, errors.New("unexpected group Rename call")
	}
	return service.rename(ctx, actorID, groupID, name)
}

func (service fakeGroupService) Dissolve(
	ctx context.Context,
	actorID string,
	groupID string,
) error {
	if service.dissolve == nil {
		return errors.New("unexpected group Dissolve call")
	}
	return service.dissolve(ctx, actorID, groupID)
}

func (service fakeGroupService) GetJoinCode(
	ctx context.Context,
	actorID string,
	groupID string,
) (string, error) {
	if service.getJoinCode == nil {
		return "", errors.New("unexpected group GetJoinCode call")
	}
	return service.getJoinCode(ctx, actorID, groupID)
}

func (service fakeGroupService) RemoveMember(
	ctx context.Context,
	actorID string,
	groupID string,
	userID string,
) error {
	if service.removeMember == nil {
		return errors.New("unexpected group RemoveMember call")
	}
	return service.removeMember(ctx, actorID, groupID, userID)
}

func (service fakeAuthService) Register(
	ctx context.Context,
	input auth.RegisterInput,
) (auth.AuthResult, error) {
	if service.register == nil {
		return auth.AuthResult{}, errors.New("unexpected Register call")
	}
	return service.register(ctx, input)
}

func (service fakeAuthService) Login(
	ctx context.Context,
	email string,
	password string,
) (auth.AuthResult, error) {
	if service.login == nil {
		return auth.AuthResult{}, errors.New("unexpected Login call")
	}
	return service.login(ctx, email, password)
}

func (service fakeAuthService) FindUser(
	ctx context.Context,
	userID string,
) (auth.User, error) {
	if service.findUser == nil {
		return auth.User{}, errors.New("unexpected FindUser call")
	}
	return service.findUser(ctx, userID)
}

type fakeCSRFProtector struct {
	issueOrReuse          func(string, *auth.Session) (auth.CSRFBinding, error)
	rotateAnonymous       func() (auth.CSRFBinding, error)
	rotateAuthenticated   func(auth.Session) (auth.CSRFBinding, error)
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

func (protector fakeCSRFProtector) RotateAnonymous() (auth.CSRFBinding, error) {
	if protector.rotateAnonymous == nil {
		return auth.CSRFBinding{}, errors.New("unexpected RotateAnonymous call")
	}
	return protector.rotateAnonymous()
}

func (protector fakeCSRFProtector) RotateAuthenticated(
	session auth.Session,
) (auth.CSRFBinding, error) {
	if protector.rotateAuthenticated == nil {
		return auth.CSRFBinding{}, errors.New("unexpected RotateAuthenticated call")
	}
	return protector.rotateAuthenticated(session)
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
		Auth: fakeAuthService{
			register: func(
				context.Context,
				auth.RegisterInput,
			) (auth.AuthResult, error) {
				return auth.AuthResult{}, errors.New("unexpected Register call")
			},
			login: func(
				context.Context,
				string,
				string,
			) (auth.AuthResult, error) {
				return auth.AuthResult{}, errors.New("unexpected Login call")
			},
			findUser: func(
				_ context.Context,
				userID string,
			) (auth.User, error) {
				return auth.User{ID: userID}, nil
			},
		},
		Groups: fakeGroupService{
			create: func(
				context.Context,
				string,
				string,
			) (groups.Group, error) {
				return groups.Group{}, errors.New("unexpected group Create call")
			},
			list: func(
				context.Context,
				string,
			) ([]groups.Group, error) {
				return nil, errors.New("unexpected group List call")
			},
			join: func(
				context.Context,
				string,
				string,
			) (groups.Group, error) {
				return groups.Group{}, errors.New("unexpected group Join call")
			},
			get: func(
				context.Context,
				string,
				string,
			) (groups.Detail, error) {
				return groups.Detail{}, errors.New("unexpected group Get call")
			},
			rename: func(
				context.Context,
				string,
				string,
				string,
			) (groups.Group, error) {
				return groups.Group{}, errors.New("unexpected group Rename call")
			},
			dissolve: func(context.Context, string, string) error {
				return errors.New("unexpected group Dissolve call")
			},
			getJoinCode: func(
				context.Context,
				string,
				string,
			) (string, error) {
				return "", errors.New("unexpected group GetJoinCode call")
			},
			removeMember: func(
				context.Context,
				string,
				string,
				string,
			) error {
				return errors.New("unexpected group RemoveMember call")
			},
		},
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
			rotateAnonymous: func() (auth.CSRFBinding, error) {
				return auth.CSRFBinding{
					CookieValue:   "rotated-anonymous-cookie",
					Token:         "v1.rotated-anonymous-token",
					MaxAgeSeconds: 3600,
				}, nil
			},
			rotateAuthenticated: func(
				auth.Session,
			) (auth.CSRFBinding, error) {
				return auth.CSRFBinding{
					CookieValue:   "rotated-session-cookie",
					Token:         "v1.rotated-session-token",
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
