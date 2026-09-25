package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestTranslateDatabaseError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantKind   databaseErrorKind
		wantPublic string
	}{
		{
			name: "duplicate email",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "users_email_lower_unique_idx",
				Message:        "private duplicate details",
			},
			wantKind:   databaseErrorDuplicateEmail,
			wantPublic: "database duplicate email",
		},
		{
			name: "join code collision",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "groups_join_code_unique_idx",
				Message:        "private duplicate details",
			},
			wantKind:   databaseErrorJoinCodeCollision,
			wantPublic: "database join code collision",
		},
		{
			name: "check violation",
			err: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "expenses_amount_positive",
				Message:        "private check details",
			},
			wantKind:   databaseErrorCheckViolation,
			wantPublic: "database check constraint violation",
		},
		{
			name: "foreign key violation",
			err: &pgconn.PgError{
				Code:           "23503",
				ConstraintName: "expenses_paid_by_membership_fk",
				Message:        "private foreign key details",
			},
			wantKind:   databaseErrorForeignKeyViolation,
			wantPublic: "database foreign key violation",
		},
		{
			name: "unknown unique constraint",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "future_unique_constraint",
				Message:        "private unknown details",
			},
			wantKind:   databaseErrorUnknown,
			wantPublic: "database operation failed",
		},
		{
			name: "unknown check constraint",
			err: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "future_check_constraint",
				Message:        "private unknown details",
			},
			wantKind:   databaseErrorUnknown,
			wantPublic: "database operation failed",
		},
		{
			name: "unknown foreign key constraint",
			err: &pgconn.PgError{
				Code:           "23503",
				ConstraintName: "future_foreign_key_constraint",
				Message:        "private unknown details",
			},
			wantKind:   databaseErrorUnknown,
			wantPublic: "database operation failed",
		},
		{
			name:       "non PostgreSQL error",
			err:        errors.New("postgres://private:secret@database/settled"),
			wantKind:   databaseErrorUnknown,
			wantPublic: "database operation failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := translateDatabaseError(test.err)
			if gotKind := databaseErrorKindOf(got); gotKind != test.wantKind {
				t.Errorf("database error kind = %d, want %d", gotKind, test.wantKind)
			}
			if got.Error() != test.wantPublic {
				t.Errorf("error = %q, want %q", got, test.wantPublic)
			}
			if !errors.Is(got, test.err) {
				t.Error("translated error does not preserve its cause")
			}
			for _, privateValue := range []string{
				"private",
				"secret",
				"future_unique_constraint",
			} {
				if strings.Contains(got.Error(), privateValue) {
					t.Errorf("error contains private value %q: %v", privateValue, got)
				}
			}
		})
	}
}

func TestTranslateDatabaseErrorNil(t *testing.T) {
	t.Parallel()

	if got := translateDatabaseError(nil); got != nil {
		t.Errorf("translateDatabaseError(nil) = %v, want nil", got)
	}
}
