package auth

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/clock"
)

var serviceTestNow = time.Date(
	2026,
	time.July,
	30,
	8,
	15,
	30,
	123456789,
	time.FixedZone("test", -7*60*60),
)

type userStoreStub struct {
	createUser             func(context.Context, NewUser) (User, error)
	findCredentialsByEmail func(context.Context, string) (Credentials, error)
	findUserByID           func(context.Context, string) (User, error)
}

func (store userStoreStub) CreateUser(
	ctx context.Context,
	input NewUser,
) (User, error) {
	return store.createUser(ctx, input)
}

func (store userStoreStub) FindCredentialsByEmail(
	ctx context.Context,
	email string,
) (Credentials, error) {
	return store.findCredentialsByEmail(ctx, email)
}

func (store userStoreStub) FindUserByID(
	ctx context.Context,
	userID string,
) (User, error) {
	return store.findUserByID(ctx, userID)
}

type passwordManagerStub struct {
	hash        func(string) (string, error)
	verify      func(string, string) (bool, error)
	verifyDummy func(string) error
}

func (passwords passwordManagerStub) Hash(password string) (string, error) {
	return passwords.hash(password)
}

func (passwords passwordManagerStub) Verify(
	password string,
	encodedHash string,
) (bool, error) {
	return passwords.verify(password, encodedHash)
}

func (passwords passwordManagerStub) VerifyDummy(password string) error {
	return passwords.verifyDummy(password)
}

type sessionIssuerFunc func(string) (IssuedSession, error)

func (issue sessionIssuerFunc) Issue(userID string) (IssuedSession, error) {
	return issue(userID)
}

func TestServiceRegisterNormalizesHashesPersistsAndIssues(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), serviceContextKey{}, "value")
	events := make([]string, 0, 3)
	wantUser := User{
		ID:          "00000000-0000-4000-8000-000000000000",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		CreatedAt:   serviceTestNow.UTC(),
		UpdatedAt:   serviceTestNow.UTC(),
	}
	wantSession := IssuedSession{
		Token: "signed-token",
		Session: Session{
			UserID:    wantUser.ID,
			JWTID:     "jwt-id",
			IssuedAt:  serviceTestNow.UTC(),
			ExpiresAt: serviceTestNow.UTC().Add(SessionLifetime),
		},
	}
	service := newServiceForTest(
		t,
		userStoreStub{
			createUser: func(gotContext context.Context, input NewUser) (User, error) {
				events = append(events, "create")
				if gotContext != ctx {
					t.Error("CreateUser did not receive caller context")
				}
				wantInput := NewUser{
					ID:           wantUser.ID,
					Email:        wantUser.Email,
					PasswordHash: "$argon2id$test",
					DisplayName:  wantUser.DisplayName,
					CreatedAt:    serviceTestNow.UTC(),
					UpdatedAt:    serviceTestNow.UTC(),
				}
				if input != wantInput {
					t.Errorf("CreateUser input = %+v, want %+v", input, wantInput)
				}
				return wantUser, nil
			},
		},
		passwordManagerStub{
			hash: func(password string) (string, error) {
				events = append(events, "hash")
				if password != " raw password " {
					t.Errorf("password = %q, want unnormalized raw value", password)
				}
				return "$argon2id$test", nil
			},
		},
		sessionIssuerFunc(func(userID string) (IssuedSession, error) {
			events = append(events, "session")
			if userID != wantUser.ID {
				t.Errorf("session user ID = %q, want %q", userID, wantUser.ID)
			}
			return wantSession, nil
		}),
		bytes.NewReader(make([]byte, 16)),
	)

	result, err := service.Register(ctx, RegisterInput{
		Email:       "\u2003Alice@EXAMPLE.COM\u2003",
		Password:    " raw password ",
		DisplayName: "\u2003Alice\u2003",
	})

	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if result.User != wantUser || result.Session != wantSession {
		t.Errorf("result = %+v, want user %+v session %+v", result, wantUser, wantSession)
	}
	if !reflect.DeepEqual(events, []string{"hash", "create", "session"}) {
		t.Errorf("events = %v", events)
	}
}

func TestServiceRegisterValidationCollectsFieldsBeforeWork(t *testing.T) {
	t.Parallel()

	const privatePassword = "private"
	hashCalled := false
	createCalled := false
	service := newServiceForTest(
		t,
		userStoreStub{
			createUser: func(context.Context, NewUser) (User, error) {
				createCalled = true
				return User{}, nil
			},
		},
		passwordManagerStub{
			hash: func(string) (string, error) {
				hashCalled = true
				return "", nil
			},
		},
		sessionIssuerFunc(func(string) (IssuedSession, error) {
			return IssuedSession{}, nil
		}),
		&fillReader{},
	)

	_, err := service.Register(context.Background(), RegisterInput{
		Email:       "invalid",
		Password:    privatePassword,
		DisplayName: "",
	})

	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
	for _, field := range []string{"email", "password", "displayName"} {
		if validation.Fields[field] == "" {
			t.Errorf("validation fields lack %q: %v", field, validation.Fields)
		}
	}
	if len(validation.Fields) != 3 {
		t.Errorf("field count = %d, want 3", len(validation.Fields))
	}
	if hashCalled || createCalled {
		t.Errorf("invalid registration performed work: hash %v create %v", hashCalled, createCalled)
	}
	if strings.Contains(err.Error(), privatePassword) {
		t.Errorf("validation error exposes password: %v", err)
	}
}

func TestServiceRegisterHashesBeforeDuplicateEmailResult(t *testing.T) {
	t.Parallel()

	events := make([]string, 0, 2)
	sessionCalled := false
	service := newServiceForTest(
		t,
		userStoreStub{
			createUser: func(context.Context, NewUser) (User, error) {
				events = append(events, "create")
				return User{}, ErrDuplicateEmail
			},
		},
		passwordManagerStub{
			hash: func(string) (string, error) {
				events = append(events, "hash")
				return "hash", nil
			},
		},
		sessionIssuerFunc(func(string) (IssuedSession, error) {
			sessionCalled = true
			return IssuedSession{}, nil
		}),
		&fillReader{},
	)

	_, err := service.Register(context.Background(), RegisterInput{
		Email:       "alice@example.com",
		Password:    "password",
		DisplayName: "Alice",
	})

	if !errors.Is(err, ErrDuplicateEmail) {
		t.Errorf("error = %v, want ErrDuplicateEmail", err)
	}
	if !reflect.DeepEqual(events, []string{"hash", "create"}) {
		t.Errorf("events = %v, want hash then create", events)
	}
	if sessionCalled {
		t.Error("duplicate registration issued a session")
	}
}

func TestServiceRegisterDependencyFailuresStopWorkflow(t *testing.T) {
	t.Parallel()

	sourceError := errors.New("dependency failed")
	tests := []struct {
		name         string
		random       interface{ Read([]byte) (int, error) }
		hashError    error
		createError  error
		sessionError error
		wantEvents   []string
	}{
		{
			name:       "ID generation",
			random:     failingReader{err: sourceError},
			wantEvents: []string{},
		},
		{
			name:       "password hashing",
			random:     &fillReader{},
			hashError:  sourceError,
			wantEvents: []string{"hash"},
		},
		{
			name:        "store create",
			random:      &fillReader{},
			createError: sourceError,
			wantEvents:  []string{"hash", "create"},
		},
		{
			name:         "session issue",
			random:       &fillReader{},
			sessionError: sourceError,
			wantEvents:   []string{"hash", "create", "session"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events := make([]string, 0, 3)
			service := newServiceForTest(
				t,
				userStoreStub{
					createUser: func(context.Context, NewUser) (User, error) {
						events = append(events, "create")
						return User{
							ID: "00000000-0000-4000-8000-000000000000",
						}, test.createError
					},
				},
				passwordManagerStub{
					hash: func(string) (string, error) {
						events = append(events, "hash")
						return "hash", test.hashError
					},
				},
				sessionIssuerFunc(func(string) (IssuedSession, error) {
					events = append(events, "session")
					return IssuedSession{}, test.sessionError
				}),
				test.random,
			)

			_, err := service.Register(context.Background(), RegisterInput{
				Email:       "alice@example.com",
				Password:    "password",
				DisplayName: "Alice",
			})

			if !errors.Is(err, sourceError) {
				t.Errorf("error = %v, want wrapped dependency error", err)
			}
			if !reflect.DeepEqual(events, test.wantEvents) {
				t.Errorf("events = %v, want %v", events, test.wantEvents)
			}
		})
	}
}

func TestServiceLoginNormalizesVerifiesAndIssues(t *testing.T) {
	t.Parallel()

	events := make([]string, 0, 3)
	user := serviceTestUser()
	session := IssuedSession{
		Token: "session-token",
		Session: Session{
			UserID: user.ID,
			JWTID:  "jwt-id",
		},
	}
	service := newServiceForTest(
		t,
		userStoreStub{
			findCredentialsByEmail: func(
				_ context.Context,
				email string,
			) (Credentials, error) {
				events = append(events, "lookup")
				if email != "alice@example.com" {
					t.Errorf("lookup email = %q", email)
				}
				return Credentials{User: user, PasswordHash: "stored hash"}, nil
			},
		},
		passwordManagerStub{
			verify: func(password, encodedHash string) (bool, error) {
				events = append(events, "verify")
				if password != "password" || encodedHash != "stored hash" {
					t.Errorf("Verify inputs = (%q, %q)", password, encodedHash)
				}
				return true, nil
			},
		},
		sessionIssuerFunc(func(userID string) (IssuedSession, error) {
			events = append(events, "session")
			if userID != user.ID {
				t.Errorf("session user ID = %q", userID)
			}
			return session, nil
		}),
		&fillReader{},
	)

	result, err := service.Login(
		context.Background(),
		"\u2003Alice@EXAMPLE.COM\u2003",
		"password",
	)

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.User != user || result.Session != session {
		t.Errorf("result = %+v", result)
	}
	if !reflect.DeepEqual(events, []string{"lookup", "verify", "session"}) {
		t.Errorf("events = %v", events)
	}
}

func TestServiceLoginInvalidCredentialsAreIndistinguishable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		email         string
		lookupError   error
		verifyMatched bool
		wantLookup    bool
		wantDummy     bool
	}{
		{
			name:      "invalid email",
			email:     "invalid",
			wantDummy: true,
		},
		{
			name:        "unknown email",
			email:       "unknown@example.com",
			lookupError: ErrUserNotFound,
			wantLookup:  true,
			wantDummy:   true,
		},
		{
			name:       "wrong password",
			email:      "alice@example.com",
			wantLookup: true,
		},
		{
			name:      "invalid password candidate",
			email:     "alice@example.com",
			wantDummy: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lookupCalled := false
			verifyCalled := false
			dummyCalled := false
			sessionCalled := false
			service := newServiceForTest(
				t,
				userStoreStub{
					findCredentialsByEmail: func(
						context.Context,
						string,
					) (Credentials, error) {
						lookupCalled = true
						return Credentials{
							User:         serviceTestUser(),
							PasswordHash: "stored hash",
						}, test.lookupError
					},
				},
				passwordManagerStub{
					verify: func(string, string) (bool, error) {
						verifyCalled = true
						return test.verifyMatched, nil
					},
					verifyDummy: func(password string) error {
						dummyCalled = true
						wantPassword := "private password"
						if test.name == "invalid password candidate" {
							wantPassword = invalidPasswordDummy
						}
						if password != wantPassword {
							t.Errorf("dummy password = %q", password)
						}
						return nil
					},
				},
				sessionIssuerFunc(func(string) (IssuedSession, error) {
					sessionCalled = true
					return IssuedSession{}, nil
				}),
				&fillReader{},
			)

			password := "private password"
			if test.name == "invalid password candidate" {
				password = strings.Repeat("p", 129)
			}
			_, err := service.Login(
				context.Background(),
				test.email,
				password,
			)

			if err != ErrInvalidCredentials {
				t.Errorf("error = %v, want exact ErrInvalidCredentials", err)
			}
			if lookupCalled != test.wantLookup {
				t.Errorf("lookup called = %v, want %v", lookupCalled, test.wantLookup)
			}
			if dummyCalled != test.wantDummy {
				t.Errorf("dummy called = %v, want %v", dummyCalled, test.wantDummy)
			}
			if verifyCalled != (test.wantLookup && test.lookupError == nil) {
				t.Errorf("verify called = %v", verifyCalled)
			}
			if sessionCalled {
				t.Error("invalid credentials issued a session")
			}
			if strings.Contains(err.Error(), password) {
				t.Errorf("error exposes password: %v", err)
			}
		})
	}
}

func TestServiceLoginDependencyFailures(t *testing.T) {
	t.Parallel()

	sourceError := errors.New("dependency failed")
	tests := []struct {
		name         string
		lookupError  error
		verifyError  error
		dummyError   error
		sessionError error
		email        string
	}{
		{name: "lookup", lookupError: sourceError, email: "alice@example.com"},
		{name: "verification", verifyError: sourceError, email: "alice@example.com"},
		{name: "dummy verification", dummyError: sourceError, email: "invalid"},
		{name: "session issue", sessionError: sourceError, email: "alice@example.com"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newServiceForTest(
				t,
				userStoreStub{
					findCredentialsByEmail: func(
						context.Context,
						string,
					) (Credentials, error) {
						return Credentials{
							User:         serviceTestUser(),
							PasswordHash: "stored hash",
						}, test.lookupError
					},
				},
				passwordManagerStub{
					verify: func(string, string) (bool, error) {
						return true, test.verifyError
					},
					verifyDummy: func(string) error {
						return test.dummyError
					},
				},
				sessionIssuerFunc(func(string) (IssuedSession, error) {
					return IssuedSession{}, test.sessionError
				}),
				&fillReader{},
			)

			_, err := service.Login(context.Background(), test.email, "password")

			if !errors.Is(err, sourceError) {
				t.Errorf("error = %v, want wrapped dependency error", err)
			}
		})
	}
}

func TestServiceFindUser(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), serviceContextKey{}, "value")
	want := serviceTestUser()
	service := newServiceForTest(
		t,
		userStoreStub{
			findUserByID: func(gotContext context.Context, userID string) (User, error) {
				if gotContext != ctx || userID != want.ID {
					t.Errorf("FindUserByID inputs = (%v, %q)", gotContext, userID)
				}
				return want, nil
			},
		},
		passwordManagerStub{},
		sessionIssuerFunc(func(string) (IssuedSession, error) {
			return IssuedSession{}, nil
		}),
		&fillReader{},
	)

	got, err := service.FindUser(ctx, want.ID)

	if err != nil {
		t.Fatalf("FindUser: %v", err)
	}
	if got != want {
		t.Errorf("FindUser = %+v, want %+v", got, want)
	}
}

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	validStore := userStoreStub{}
	validPasswords := passwordManagerStub{}
	validSessions := sessionIssuerFunc(func(string) (IssuedSession, error) {
		return IssuedSession{}, nil
	})
	validClock := clock.Fixed{Time: serviceTestNow}
	tests := []struct {
		name      string
		store     UserStore
		passwords PasswordManager
		sessions  SessionIssuer
		clock     clock.Clock
		random    interface{ Read([]byte) (int, error) }
	}{
		{
			name:      "store",
			passwords: validPasswords,
			sessions:  validSessions,
			clock:     validClock,
			random:    &fillReader{},
		},
		{
			name:     "passwords",
			store:    validStore,
			sessions: validSessions,
			clock:    validClock,
			random:   &fillReader{},
		},
		{
			name:      "sessions",
			store:     validStore,
			passwords: validPasswords,
			clock:     validClock,
			random:    &fillReader{},
		},
		{
			name:      "clock",
			store:     validStore,
			passwords: validPasswords,
			sessions:  validSessions,
			random:    &fillReader{},
		},
		{
			name:      "random",
			store:     validStore,
			passwords: validPasswords,
			sessions:  validSessions,
			clock:     validClock,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewServiceFrom(
				test.store,
				test.passwords,
				test.sessions,
				test.clock,
				test.random,
			)
			if err != ErrInvalidServiceConfiguration || service != nil {
				t.Errorf("result = (%v, %v)", service, err)
			}
		})
	}
}

type serviceContextKey struct{}

func newServiceForTest(
	t *testing.T,
	store UserStore,
	passwords PasswordManager,
	sessions SessionIssuer,
	random interface{ Read([]byte) (int, error) },
) *Service {
	t.Helper()
	service, err := NewServiceFrom(
		store,
		passwords,
		sessions,
		clock.Fixed{Time: serviceTestNow},
		random,
	)
	if err != nil {
		t.Fatalf("NewServiceFrom: %v", err)
	}
	return service
}

func serviceTestUser() User {
	return User{
		ID:          "00112233-4455-4677-8899-aabbccddeeff",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		CreatedAt:   serviceTestNow.UTC(),
		UpdatedAt:   serviceTestNow.UTC(),
	}
}
