//go:build integration

package store

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/expenses"
	"github.com/Fyy10/settled/server/internal/groups"
	"github.com/Fyy10/settled/server/internal/repayments"
	"github.com/Fyy10/settled/server/internal/settlements"
)

func TestStoreUnavailablePostgreSQLReturnsInternalErrors(t *testing.T) {
	address := unavailablePostgreSQLAddress(t)
	connectionURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword("settled", "unavailable-test-password"),
		Host:   address,
		Path:   "/settled_test",
		RawQuery: url.Values{
			"connect_timeout": {"1"},
			"sslmode":         {"disable"},
		}.Encode(),
	}
	db, err := sql.Open("pgx", connectionURL.String())
	if err != nil {
		t.Fatalf("open unavailable PostgreSQL pool: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close unavailable PostgreSQL pool: %v", err)
		}
	})
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	store := New(db)
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		call func(context.Context) error
	}{
		{
			name: "direct user write",
			call: func(ctx context.Context) error {
				_, err := store.CreateUser(ctx, auth.NewUser{
					ID:           aliceID,
					Email:        "alice@example.com",
					PasswordHash: "hash",
					DisplayName:  "Alice",
					CreatedAt:    now,
					UpdatedAt:    now,
				})
				return err
			},
		},
		{
			name: "direct user read",
			call: func(ctx context.Context) error {
				_, err := store.FindUserByID(ctx, aliceID)
				return err
			},
		},
		{
			name: "direct group list",
			call: func(ctx context.Context) error {
				_, err := store.ListGroups(ctx, aliceID)
				return err
			},
		},
		{
			name: "transactional group write",
			call: func(ctx context.Context) error {
				_, err := store.CreateGroup(ctx, groups.NewGroup{
					ID:          groupOneID,
					Name:        "Unavailable",
					JoinCode:    "UNAVAIL23456",
					OwnerUserID: aliceID,
					CreatedAt:   now,
					UpdatedAt:   now,
				})
				return err
			},
		},
		{
			name: "direct expense item read",
			call: func(ctx context.Context) error {
				_, err := store.GetExpense(
					ctx,
					aliceID,
					groupOneID,
					expenseOneID,
				)
				return err
			},
		},
		{
			name: "transactional expense list",
			call: func(ctx context.Context) error {
				_, err := store.ListExpenses(ctx, aliceID, groupOneID)
				return err
			},
		},
		{
			name: "direct repayment item read",
			call: func(ctx context.Context) error {
				_, err := store.GetRepayment(
					ctx,
					aliceID,
					groupOneID,
					repaymentOneID,
				)
				return err
			},
		},
		{
			name: "transactional repayment list",
			call: func(ctx context.Context) error {
				_, err := store.ListRepayments(ctx, aliceID, groupOneID)
				return err
			},
		},
		{
			name: "transactional settlement list",
			call: func(ctx context.Context) error {
				_, _, err := store.ListDebtEntries(
					ctx,
					aliceID,
					groupOneID,
				)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				2*time.Second,
			)
			defer cancel()

			err := test.call(ctx)
			if err == nil {
				t.Fatal("Store call succeeded while PostgreSQL was unavailable")
			}
			var internal *databaseError
			if !errors.As(err, &internal) {
				t.Errorf("error = %v, want wrapped databaseError", err)
			}
			if databaseErrorKindOf(err) != databaseErrorUnknown {
				t.Errorf(
					"database error kind = %v, want unknown",
					databaseErrorKindOf(err),
				)
			}
			for _, domainError := range []error{
				auth.ErrDuplicateEmail,
				auth.ErrUserNotFound,
				groups.ErrNotFound,
				groups.ErrForbidden,
				groups.ErrMemberInUse,
				expenses.ErrNotFound,
				repayments.ErrNotFound,
				settlements.ErrNotFound,
			} {
				if errors.Is(err, domainError) {
					t.Errorf(
						"unavailable PostgreSQL error was misclassified as %v: %v",
						domainError,
						err,
					)
				}
			}
		})
	}
}

func unavailablePostgreSQLAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve unavailable PostgreSQL address: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			_ = connection.Close()
		}
	}()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil &&
			!errors.Is(err, net.ErrClosed) {
			t.Errorf("close unavailable PostgreSQL listener: %v", err)
		}
		<-done
	})
	return listener.Addr().String()
}
