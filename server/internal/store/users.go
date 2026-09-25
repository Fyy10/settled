package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fyy10/settled/server/internal/auth"
)

var _ auth.UserStore = (*Store)(nil)

func (s *Store) CreateUser(
	ctx context.Context,
	input auth.NewUser,
) (auth.User, error) {
	var user auth.User
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO users (
			id,
			email,
			password_hash,
			display_name,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING
			id::text,
			email,
			display_name,
			created_at,
			updated_at
	`,
		input.ID,
		input.Email,
		input.PasswordHash,
		input.DisplayName,
		input.CreatedAt,
		input.UpdatedAt,
	).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		translated := translateDatabaseError(err)
		if databaseErrorKindOf(translated) == databaseErrorDuplicateEmail {
			return auth.User{}, fmt.Errorf("create user: %w", auth.ErrDuplicateEmail)
		}
		return auth.User{}, fmt.Errorf("create user: %w", translated)
	}

	user.CreatedAt = user.CreatedAt.UTC()
	user.UpdatedAt = user.UpdatedAt.UTC()
	return user, nil
}

func (s *Store) FindCredentialsByEmail(
	ctx context.Context,
	email string,
) (auth.Credentials, error) {
	var credentials auth.Credentials
	err := s.db.QueryRowContext(ctx, `
		SELECT
			id::text,
			email,
			password_hash,
			display_name,
			created_at,
			updated_at
		FROM users
		WHERE lower(email) = $1
	`, email).Scan(
		&credentials.User.ID,
		&credentials.User.Email,
		&credentials.PasswordHash,
		&credentials.User.DisplayName,
		&credentials.User.CreatedAt,
		&credentials.User.UpdatedAt,
	)
	if err != nil {
		return auth.Credentials{}, translateUserLookupError(
			"find credentials by email",
			err,
		)
	}

	credentials.User.CreatedAt = credentials.User.CreatedAt.UTC()
	credentials.User.UpdatedAt = credentials.User.UpdatedAt.UTC()
	return credentials, nil
}

func (s *Store) FindUserByID(
	ctx context.Context,
	userID string,
) (auth.User, error) {
	var user auth.User
	err := s.db.QueryRowContext(ctx, `
		SELECT
			id::text,
			email,
			display_name,
			created_at,
			updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return auth.User{}, translateUserLookupError("find user by ID", err)
	}

	user.CreatedAt = user.CreatedAt.UTC()
	user.UpdatedAt = user.UpdatedAt.UTC()
	return user, nil
}

func translateUserLookupError(operation string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", operation, auth.ErrUserNotFound)
	}
	return fmt.Errorf("%s: %w", operation, translateDatabaseError(err))
}
