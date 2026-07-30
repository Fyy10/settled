//go:build integration

package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
)

func TestUserStoreCreateAndLookups(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	userStore := New(db)
	ctx := context.Background()

	createdAt := time.Date(2026, 7, 29, 12, 34, 56, 123456000, time.UTC)
	input := auth.NewUser{
		ID:           aliceID,
		Email:        "alice@example.com",
		PasswordHash: "private-password-hash",
		DisplayName:  "Alice",
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}

	created, err := userStore.CreateUser(ctx, input)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	wantUser := auth.User{
		ID:          input.ID,
		Email:       input.Email,
		DisplayName: input.DisplayName,
		CreatedAt:   input.CreatedAt,
		UpdatedAt:   input.UpdatedAt,
	}
	if !reflect.DeepEqual(created, wantUser) {
		t.Errorf("created user = %#v, want %#v", created, wantUser)
	}

	credentials, err := userStore.FindCredentialsByEmail(ctx, input.Email)
	if err != nil {
		t.Fatalf("FindCredentialsByEmail: %v", err)
	}
	if !reflect.DeepEqual(credentials.User, wantUser) {
		t.Errorf("credential user = %#v, want %#v", credentials.User, wantUser)
	}
	if credentials.PasswordHash != input.PasswordHash {
		t.Errorf(
			"credential password hash = %q, want %q",
			credentials.PasswordHash,
			input.PasswordHash,
		)
	}

	found, err := userStore.FindUserByID(ctx, input.ID)
	if err != nil {
		t.Fatalf("FindUserByID: %v", err)
	}
	if !reflect.DeepEqual(found, wantUser) {
		t.Errorf("found user = %#v, want %#v", found, wantUser)
	}
}

func TestUserStoreDuplicateEmail(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	userStore := New(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	first := auth.NewUser{
		ID:           aliceID,
		Email:        "alice@example.com",
		PasswordHash: "first-hash",
		DisplayName:  "Alice",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := userStore.CreateUser(ctx, first); err != nil {
		t.Fatalf("create first user: %v", err)
	}

	duplicate := auth.NewUser{
		ID:           bobID,
		Email:        "ALICE@EXAMPLE.COM",
		PasswordHash: "second-hash",
		DisplayName:  "Other Alice",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_, err := userStore.CreateUser(ctx, duplicate)
	if !errors.Is(err, auth.ErrDuplicateEmail) {
		t.Fatalf("duplicate error = %v, want ErrDuplicateEmail", err)
	}
	if errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("duplicate error = %v, must not be ErrUserNotFound", err)
	}
}

func TestUserStoreMissingLookups(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	userStore := New(db)
	ctx := context.Background()

	if _, err := userStore.FindCredentialsByEmail(
		ctx,
		"missing@example.com",
	); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("missing credentials error = %v, want ErrUserNotFound", err)
	}
	if _, err := userStore.FindUserByID(
		ctx,
		aliceID,
	); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("missing user error = %v, want ErrUserNotFound", err)
	}
}
