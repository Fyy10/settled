package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"github.com/Fyy10/settled/server/internal/clock"
	"github.com/Fyy10/settled/server/internal/identifier"
	"github.com/Fyy10/settled/server/internal/input"
)

var (
	ErrInvalidCredentials          = errors.New("invalid credentials")
	ErrInvalidServiceConfiguration = errors.New("invalid auth service configuration")
)

const invalidPasswordDummy = "invalid-password"

type PasswordManager interface {
	Hash(string) (string, error)
	Verify(string, string) (bool, error)
	VerifyDummy(string) error
}

type SessionIssuer interface {
	Issue(string) (IssuedSession, error)
}

type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
}

type AuthResult struct {
	User    User
	Session IssuedSession
}

type ValidationError struct {
	Fields map[string]string
}

func (validation *ValidationError) Error() string {
	return "validation failed"
}

type Service struct {
	users     UserStore
	passwords PasswordManager
	sessions  SessionIssuer
	clock     clock.Clock
	random    io.Reader
}

func NewService(
	users UserStore,
	passwords PasswordManager,
	sessions SessionIssuer,
	serviceClock clock.Clock,
) (*Service, error) {
	return NewServiceFrom(users, passwords, sessions, serviceClock, rand.Reader)
}

func NewServiceFrom(
	users UserStore,
	passwords PasswordManager,
	sessions SessionIssuer,
	serviceClock clock.Clock,
	random io.Reader,
) (*Service, error) {
	if users == nil ||
		passwords == nil ||
		sessions == nil ||
		serviceClock == nil ||
		random == nil {
		return nil, ErrInvalidServiceConfiguration
	}
	return &Service{
		users:     users,
		passwords: passwords,
		sessions:  sessions,
		clock:     serviceClock,
		random:    random,
	}, nil
}

func (service *Service) Register(
	ctx context.Context,
	registration RegisterInput,
) (AuthResult, error) {
	email, displayName, err := validateRegistration(registration)
	if err != nil {
		return AuthResult{}, err
	}

	userID, err := identifier.NewUUIDFrom(service.random)
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate user ID: %w", err)
	}
	passwordHash, err := service.passwords.Hash(registration.Password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash registration password: %w", err)
	}

	now := service.clock.Now().UTC()
	user, err := service.users.CreateUser(ctx, NewUser{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
		DisplayName:  displayName,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return AuthResult{}, err
	}

	session, err := service.sessions.Issue(user.ID)
	if err != nil {
		return AuthResult{}, fmt.Errorf("issue registration session: %w", err)
	}
	return AuthResult{User: user, Session: session}, nil
}

func (service *Service) Login(
	ctx context.Context,
	email string,
	password string,
) (AuthResult, error) {
	normalizedEmail, emailErr := input.NormalizeEmail(email)
	passwordErr := input.ValidatePassword(password)
	if emailErr != nil || passwordErr != nil {
		dummyPassword := password
		if passwordErr != nil {
			dummyPassword = invalidPasswordDummy
		}
		if dummyErr := service.passwords.VerifyDummy(dummyPassword); dummyErr != nil {
			return AuthResult{}, fmt.Errorf("verify dummy password: %w", dummyErr)
		}
		return AuthResult{}, ErrInvalidCredentials
	}

	credentials, err := service.users.FindCredentialsByEmail(ctx, normalizedEmail)
	if errors.Is(err, ErrUserNotFound) {
		if dummyErr := service.passwords.VerifyDummy(password); dummyErr != nil {
			return AuthResult{}, fmt.Errorf("verify dummy password: %w", dummyErr)
		}
		return AuthResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return AuthResult{}, err
	}

	matched, err := service.passwords.Verify(password, credentials.PasswordHash)
	if err != nil {
		return AuthResult{}, fmt.Errorf("verify password: %w", err)
	}
	if !matched {
		return AuthResult{}, ErrInvalidCredentials
	}

	session, err := service.sessions.Issue(credentials.User.ID)
	if err != nil {
		return AuthResult{}, fmt.Errorf("issue login session: %w", err)
	}
	return AuthResult{User: credentials.User, Session: session}, nil
}

func (service *Service) FindUser(
	ctx context.Context,
	userID string,
) (User, error) {
	return service.users.FindUserByID(ctx, userID)
}

func validateRegistration(
	registration RegisterInput,
) (string, string, error) {
	fields := make(map[string]string)

	email, emailErr := input.NormalizeEmail(registration.Email)
	addValidationField(fields, emailErr)
	displayName, displayNameErr := input.NormalizeDisplayName(
		registration.DisplayName,
	)
	addValidationField(fields, displayNameErr)
	passwordErr := input.ValidatePassword(registration.Password)
	addValidationField(fields, passwordErr)

	if len(fields) > 0 {
		return "", "", &ValidationError{Fields: fields}
	}
	return email, displayName, nil
}

func addValidationField(fields map[string]string, err error) {
	if err == nil {
		return
	}
	var fieldError *input.FieldError
	if errors.As(err, &fieldError) {
		fields[fieldError.Field] = fieldError.Message
	}
}
