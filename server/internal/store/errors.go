package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type databaseErrorKind uint8

const (
	databaseErrorUnknown databaseErrorKind = iota
	databaseErrorDuplicateEmail
	databaseErrorJoinCodeCollision
	databaseErrorCheckViolation
	databaseErrorForeignKeyViolation
)

var knownCheckConstraints = map[string]struct{}{
	"users_email_not_blank":                  {},
	"users_display_name_not_blank":           {},
	"users_display_name_length":              {},
	"groups_name_not_blank":                  {},
	"groups_name_length":                     {},
	"groups_join_code_not_blank":             {},
	"groups_join_code_length":                {},
	"group_memberships_removed_after_joined": {},
	"expenses_amount_positive":               {},
	"expenses_currency_usd":                  {},
	"expenses_description_not_blank":         {},
	"expenses_description_length":            {},
	"expenses_deleted_after_created":         {},
	"expense_splits_amount_positive":         {},
	"repayments_amount_positive":             {},
	"repayments_currency_usd":                {},
	"repayments_users_different":             {},
	"repayments_note_length":                 {},
	"repayments_deleted_after_created":       {},
}

var knownForeignKeyConstraints = map[string]struct{}{
	"groups_owner_user_id_fkey":           {},
	"group_memberships_group_id_fkey":     {},
	"group_memberships_user_id_fkey":      {},
	"expenses_group_id_fkey":              {},
	"expenses_paid_by_user_id_fkey":       {},
	"expenses_created_by_user_id_fkey":    {},
	"expenses_paid_by_membership_fk":      {},
	"expenses_created_by_membership_fk":   {},
	"expense_splits_expense_id_fkey":      {},
	"expense_splits_user_id_fkey":         {},
	"expense_splits_user_membership_fk":   {},
	"expense_splits_expense_group_fk":     {},
	"repayments_group_id_fkey":            {},
	"repayments_from_user_id_fkey":        {},
	"repayments_to_user_id_fkey":          {},
	"repayments_created_by_user_id_fkey":  {},
	"repayments_from_membership_fk":       {},
	"repayments_to_membership_fk":         {},
	"repayments_created_by_membership_fk": {},
}

type databaseError struct {
	kind  databaseErrorKind
	cause error
}

func (e *databaseError) Error() string {
	switch e.kind {
	case databaseErrorDuplicateEmail:
		return "database duplicate email"
	case databaseErrorJoinCodeCollision:
		return "database join code collision"
	case databaseErrorCheckViolation:
		return "database check constraint violation"
	case databaseErrorForeignKeyViolation:
		return "database foreign key violation"
	default:
		return "database operation failed"
	}
}

func (e *databaseError) Unwrap() error {
	return e.cause
}

func translateDatabaseError(err error) error {
	if err == nil {
		return nil
	}

	kind := databaseErrorUnknown
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch {
		case postgresError.Code == "23505" &&
			postgresError.ConstraintName == "users_email_lower_unique_idx":
			kind = databaseErrorDuplicateEmail
		case postgresError.Code == "23505" &&
			postgresError.ConstraintName == "groups_join_code_unique_idx":
			kind = databaseErrorJoinCodeCollision
		case postgresError.Code == "23514" &&
			isKnownConstraint(postgresError.ConstraintName, knownCheckConstraints):
			kind = databaseErrorCheckViolation
		case postgresError.Code == "23503" &&
			isKnownConstraint(postgresError.ConstraintName, knownForeignKeyConstraints):
			kind = databaseErrorForeignKeyViolation
		}
	}

	return &databaseError{
		kind:  kind,
		cause: err,
	}
}

func databaseErrorKindOf(err error) databaseErrorKind {
	var translated *databaseError
	if errors.As(err, &translated) {
		return translated.kind
	}
	return databaseErrorUnknown
}

func isKnownConstraint(name string, known map[string]struct{}) bool {
	_, ok := known[name]
	return ok
}
