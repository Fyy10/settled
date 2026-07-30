package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrDuplicateEmail = errors.New("duplicate email")
)

type User struct {
	ID          string
	Email       string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type NewUser struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Credentials struct {
	User         User
	PasswordHash string
}

type UserStore interface {
	CreateUser(context.Context, NewUser) (User, error)
	FindCredentialsByEmail(context.Context, string) (Credentials, error)
	FindUserByID(context.Context, string) (User, error)
}
