package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Fyy10/settled/server/internal/expenses"
)

var errExpenseSplitInvariant = errors.New("invalid expense split invariant")

var _ expenses.Store = (*Store)(nil)

type expenseRowScanner interface {
	Scan(...any) error
}

func (s *Store) CreateExpense(
	ctx context.Context,
	input expenses.CreateInput,
) (expenses.Expense, error) {
	var expense expenses.Expense
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		if err := lockExpenseGroup(ctx, tx, input.GroupID); err != nil {
			return err
		}
		if err := lockActiveExpenseMemberships(
			ctx,
			tx,
			input.GroupID,
			expenseMemberIDs(input.ActorID, input.PaidByUserID, input.Splits),
		); err != nil {
			return err
		}
		if err := validateExpenseSplitInvariant(
			input.AmountCents,
			input.Splits,
		); err != nil {
			return err
		}

		row := tx.QueryRowContext(ctx, `
			INSERT INTO expenses (
				id,
				group_id,
				paid_by_user_id,
				description,
				amount_cents,
				currency,
				expense_date,
				created_by_user_id,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING
				id::text,
				group_id::text,
				paid_by_user_id::text,
				description,
				amount_cents,
				currency::text,
				expense_date::text,
				created_by_user_id::text,
				created_at,
				updated_at
		`,
			input.ID,
			input.GroupID,
			input.PaidByUserID,
			input.Description,
			input.AmountCents,
			input.Currency,
			input.ExpenseDate,
			input.ActorID,
			input.CreatedAt,
			input.UpdatedAt,
		)
		var err error
		expense, err = scanExpense(row)
		if err != nil {
			return fmt.Errorf(
				"insert expense: %w",
				translateDatabaseError(err),
			)
		}
		if err := insertExpenseSplits(
			ctx,
			tx,
			input.ID,
			input.GroupID,
			input.Splits,
		); err != nil {
			return err
		}
		expense.Splits = cloneExpenseSplits(input.Splits)
		return nil
	})
	if err != nil {
		return expenses.Expense{}, err
	}
	return expense, nil
}

func (s *Store) ReplaceExpense(
	ctx context.Context,
	input expenses.ReplaceInput,
) (expenses.Expense, error) {
	var expense expenses.Expense
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		if err := lockExpenseGroup(ctx, tx, input.GroupID); err != nil {
			return err
		}
		if err := lockActiveExpenseMemberships(
			ctx,
			tx,
			input.GroupID,
			expenseMemberIDs(input.ActorID, input.PaidByUserID, input.Splits),
		); err != nil {
			return err
		}
		if err := lockVisibleExpense(
			ctx,
			tx,
			input.GroupID,
			input.ExpenseID,
		); err != nil {
			return err
		}
		if err := lockExpenseSplits(
			ctx,
			tx,
			input.GroupID,
			input.ExpenseID,
		); err != nil {
			return err
		}
		if err := validateExpenseSplitInvariant(
			input.AmountCents,
			input.Splits,
		); err != nil {
			return err
		}

		row := tx.QueryRowContext(ctx, `
			UPDATE expenses
			SET paid_by_user_id = $3,
				description = $4,
				amount_cents = $5,
				currency = $6,
				expense_date = $7,
				updated_at = $8
			WHERE id = $1
				AND group_id = $2
				AND deleted_at IS NULL
			RETURNING
				id::text,
				group_id::text,
				paid_by_user_id::text,
				description,
				amount_cents,
				currency::text,
				expense_date::text,
				created_by_user_id::text,
				created_at,
				updated_at
		`,
			input.ExpenseID,
			input.GroupID,
			input.PaidByUserID,
			input.Description,
			input.AmountCents,
			input.Currency,
			input.ExpenseDate,
			input.UpdatedAt,
		)
		var err error
		expense, err = scanExpense(row)
		if errors.Is(err, sql.ErrNoRows) {
			return expenseNotFound("replace expense")
		}
		if err != nil {
			return fmt.Errorf(
				"replace expense: %w",
				translateDatabaseError(err),
			)
		}

		if _, err := tx.ExecContext(ctx, `
			DELETE FROM expense_splits
			WHERE expense_id = $1
				AND group_id = $2
		`, input.ExpenseID, input.GroupID); err != nil {
			return fmt.Errorf(
				"delete replaced expense splits: %w",
				translateDatabaseError(err),
			)
		}
		if err := insertExpenseSplits(
			ctx,
			tx,
			input.ExpenseID,
			input.GroupID,
			input.Splits,
		); err != nil {
			return err
		}
		expense.Splits = cloneExpenseSplits(input.Splits)
		return nil
	})
	if err != nil {
		return expenses.Expense{}, err
	}
	return expense, nil
}

func (s *Store) DeleteExpense(
	ctx context.Context,
	input expenses.DeleteInput,
) error {
	return s.withinTx(ctx, func(tx *sql.Tx) error {
		if err := lockExpenseGroup(ctx, tx, input.GroupID); err != nil {
			return err
		}
		if err := lockActiveExpenseMemberships(
			ctx,
			tx,
			input.GroupID,
			[]string{input.ActorID},
		); err != nil {
			return err
		}
		if err := lockVisibleExpense(
			ctx,
			tx,
			input.GroupID,
			input.ExpenseID,
		); err != nil {
			return err
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE expenses
			SET deleted_at = $3,
				updated_at = $3
			WHERE id = $1
				AND group_id = $2
				AND deleted_at IS NULL
		`, input.ExpenseID, input.GroupID, input.DeletedAt)
		if err != nil {
			return fmt.Errorf(
				"soft-delete expense: %w",
				translateDatabaseError(err),
			)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf(
				"soft-delete expense rows affected: %w",
				translateDatabaseError(err),
			)
		}
		if affected != 1 {
			if affected == 0 {
				return expenseNotFound("soft-delete expense")
			}
			return fmt.Errorf(
				"soft-delete expense affected %d rows",
				affected,
			)
		}
		return nil
	})
}

func (s *Store) GetExpense(
	ctx context.Context,
	actorID string,
	groupID string,
	expenseID string,
) (expenses.Expense, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			expense.id::text,
			expense.group_id::text,
			expense.paid_by_user_id::text,
			expense.description,
			expense.amount_cents,
			expense.currency::text,
			expense.expense_date::text,
			expense.created_by_user_id::text,
			expense.created_at,
			expense.updated_at,
			split.user_id::text,
			split.amount_cents
		FROM expenses expense
		JOIN groups active_group
			ON active_group.id = expense.group_id
			AND active_group.dissolved_at IS NULL
		JOIN group_memberships actor_membership
			ON actor_membership.group_id = expense.group_id
			AND actor_membership.user_id = $3
			AND actor_membership.removed_at IS NULL
		JOIN expense_splits split
			ON split.expense_id = expense.id
			AND split.group_id = expense.group_id
		WHERE expense.id = $1
			AND expense.group_id = $2
			AND expense.deleted_at IS NULL
		ORDER BY split.user_id
	`, expenseID, groupID, actorID)
	if err != nil {
		return expenses.Expense{}, fmt.Errorf(
			"query visible expense: %w",
			translateDatabaseError(err),
		)
	}

	expense, err := scanExpenseWithSplits(rows)
	if err != nil {
		return expenses.Expense{}, err
	}
	return expense, nil
}

func (s *Store) ListExpenses(
	ctx context.Context,
	actorID string,
	groupID string,
) (expenses.ListResult, error) {
	result := expenses.ListResult{
		Expenses: make([]expenses.Expense, 0),
		Members:  make([]expenses.MemberSummary, 0),
	}
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		if err := lockReadableExpenseGroup(
			ctx,
			tx,
			actorID,
			groupID,
		); err != nil {
			return err
		}

		expenseRows, err := tx.QueryContext(ctx, `
			SELECT
				id::text,
				group_id::text,
				paid_by_user_id::text,
				description,
				amount_cents,
				currency::text,
				expense_date::text,
				created_by_user_id::text,
				created_at,
				updated_at
			FROM expenses
			WHERE group_id = $1
				AND deleted_at IS NULL
			ORDER BY expense_date DESC, created_at DESC, id DESC
		`, groupID)
		if err != nil {
			return fmt.Errorf(
				"list visible expenses: %w",
				translateDatabaseError(err),
			)
		}
		expenseIndex := make(map[string]int)
		for expenseRows.Next() {
			expense, err := scanExpense(expenseRows)
			if err != nil {
				expenseRows.Close()
				return fmt.Errorf(
					"scan visible expense: %w",
					translateDatabaseError(err),
				)
			}
			expenseIndex[expense.ID] = len(result.Expenses)
			result.Expenses = append(result.Expenses, expense)
		}
		if err := expenseRows.Err(); err != nil {
			expenseRows.Close()
			return fmt.Errorf(
				"iterate visible expenses: %w",
				translateDatabaseError(err),
			)
		}
		if err := expenseRows.Close(); err != nil {
			return fmt.Errorf(
				"close visible expenses: %w",
				translateDatabaseError(err),
			)
		}

		if len(result.Expenses) > 0 {
			splitRows, err := tx.QueryContext(ctx, `
				SELECT
					split.expense_id::text,
					split.user_id::text,
					split.amount_cents
				FROM expense_splits split
				JOIN expenses expense
					ON expense.id = split.expense_id
					AND expense.group_id = split.group_id
				WHERE split.group_id = $1
					AND expense.deleted_at IS NULL
				ORDER BY split.expense_id, split.user_id
			`, groupID)
			if err != nil {
				return fmt.Errorf(
					"list visible expense splits: %w",
					translateDatabaseError(err),
				)
			}
			for splitRows.Next() {
				var expenseID string
				var split expenses.Split
				if err := splitRows.Scan(
					&expenseID,
					&split.UserID,
					&split.AmountCents,
				); err != nil {
					splitRows.Close()
					return fmt.Errorf(
						"scan visible expense split: %w",
						translateDatabaseError(err),
					)
				}
				index, found := expenseIndex[expenseID]
				if !found {
					splitRows.Close()
					return fmt.Errorf(
						"assemble expense list: split references absent expense %s",
						expenseID,
					)
				}
				result.Expenses[index].Splits = append(
					result.Expenses[index].Splits,
					split,
				)
			}
			if err := splitRows.Err(); err != nil {
				splitRows.Close()
				return fmt.Errorf(
					"iterate visible expense splits: %w",
					translateDatabaseError(err),
				)
			}
			if err := splitRows.Close(); err != nil {
				return fmt.Errorf(
					"close visible expense splits: %w",
					translateDatabaseError(err),
				)
			}
		}

		memberRows, err := tx.QueryContext(ctx, `
			SELECT
				membership.user_id::text,
				member_user.display_name
			FROM group_memberships membership
			JOIN groups active_group
				ON active_group.id = membership.group_id
				AND active_group.dissolved_at IS NULL
			JOIN users member_user ON member_user.id = membership.user_id
			WHERE membership.group_id = $1
				AND membership.removed_at IS NULL
			ORDER BY
				CASE
					WHEN membership.user_id = active_group.owner_user_id THEN 0
					ELSE 1
				END,
				lower(member_user.display_name),
				membership.user_id
		`, groupID)
		if err != nil {
			return fmt.Errorf(
				"list expense member summaries: %w",
				translateDatabaseError(err),
			)
		}
		for memberRows.Next() {
			var member expenses.MemberSummary
			if err := memberRows.Scan(
				&member.UserID,
				&member.DisplayName,
			); err != nil {
				memberRows.Close()
				return fmt.Errorf(
					"scan expense member summary: %w",
					translateDatabaseError(err),
				)
			}
			result.Members = append(result.Members, member)
		}
		if err := memberRows.Err(); err != nil {
			memberRows.Close()
			return fmt.Errorf(
				"iterate expense member summaries: %w",
				translateDatabaseError(err),
			)
		}
		if err := memberRows.Close(); err != nil {
			return fmt.Errorf(
				"close expense member summaries: %w",
				translateDatabaseError(err),
			)
		}
		return nil
	})
	if err != nil {
		return expenses.ListResult{}, err
	}
	return result, nil
}

func lockExpenseGroup(
	ctx context.Context,
	tx *sql.Tx,
	groupID string,
) error {
	var lockedID string
	err := tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM groups
		WHERE id = $1
			AND dissolved_at IS NULL
		FOR UPDATE
	`, groupID).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return expenseNotFound("lock active expense group")
	}
	if err != nil {
		return fmt.Errorf(
			"lock active expense group: %w",
			translateDatabaseError(err),
		)
	}
	return nil
}

func lockReadableExpenseGroup(
	ctx context.Context,
	tx *sql.Tx,
	actorID string,
	groupID string,
) error {
	var lockedGroupID string
	err := tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM groups
		WHERE id = $1
			AND dissolved_at IS NULL
		FOR SHARE
	`, groupID).Scan(&lockedGroupID)
	if errors.Is(err, sql.ErrNoRows) {
		return expenseNotFound("lock readable expense group")
	}
	if err != nil {
		return fmt.Errorf(
			"lock readable expense group: %w",
			translateDatabaseError(err),
		)
	}

	var lockedActorID string
	err = tx.QueryRowContext(ctx, `
		SELECT user_id::text
		FROM group_memberships
		WHERE group_id = $1
			AND user_id = $2
			AND removed_at IS NULL
		FOR SHARE
	`, groupID, actorID).Scan(&lockedActorID)
	if errors.Is(err, sql.ErrNoRows) {
		return expenseNotFound("authorize expense list")
	}
	if err != nil {
		return fmt.Errorf(
			"lock expense list actor membership: %w",
			translateDatabaseError(err),
		)
	}
	return nil
}

func lockActiveExpenseMemberships(
	ctx context.Context,
	tx *sql.Tx,
	groupID string,
	memberIDs []string,
) error {
	memberIDs = uniqueSortedExpenseMemberIDs(memberIDs)
	if len(memberIDs) == 0 {
		return expenseNotFound("authorize expense memberships")
	}

	query, arguments := expenseMembershipLockQuery(groupID, memberIDs)
	rows, err := tx.QueryContext(ctx, query, arguments...)
	if err != nil {
		return fmt.Errorf(
			"lock expense memberships: %w",
			translateDatabaseError(err),
		)
	}

	active := make(map[string]bool, len(memberIDs))
	for rows.Next() {
		var userID string
		var removedAt *time.Time
		if err := rows.Scan(&userID, &removedAt); err != nil {
			rows.Close()
			return fmt.Errorf(
				"scan locked expense membership: %w",
				translateDatabaseError(err),
			)
		}
		active[userID] = removedAt == nil
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf(
			"iterate locked expense memberships: %w",
			translateDatabaseError(err),
		)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf(
			"close locked expense memberships: %w",
			translateDatabaseError(err),
		)
	}
	for _, userID := range memberIDs {
		if !active[userID] {
			return expenseNotFound("authorize expense memberships")
		}
	}
	return nil
}

func expenseMembershipLockQuery(
	groupID string,
	memberIDs []string,
) (string, []any) {
	var query strings.Builder
	query.WriteString(`
		SELECT user_id::text, removed_at
		FROM group_memberships
		WHERE group_id = $1
			AND user_id IN (`)
	arguments := make([]any, 0, len(memberIDs)+1)
	arguments = append(arguments, groupID)
	for index, userID := range memberIDs {
		if index > 0 {
			query.WriteString(", ")
		}
		fmt.Fprintf(&query, "$%d", index+2)
		arguments = append(arguments, userID)
	}
	query.WriteString(`)
		ORDER BY user_id
		FOR UPDATE`)
	return query.String(), arguments
}

func lockVisibleExpense(
	ctx context.Context,
	tx *sql.Tx,
	groupID string,
	expenseID string,
) error {
	var lockedID string
	err := tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM expenses
		WHERE id = $1
			AND group_id = $2
			AND deleted_at IS NULL
		FOR UPDATE
	`, expenseID, groupID).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return expenseNotFound("lock visible expense")
	}
	if err != nil {
		return fmt.Errorf(
			"lock visible expense: %w",
			translateDatabaseError(err),
		)
	}
	return nil
}

func lockExpenseSplits(
	ctx context.Context,
	tx *sql.Tx,
	groupID string,
	expenseID string,
) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT user_id::text
		FROM expense_splits
		WHERE expense_id = $1
			AND group_id = $2
		ORDER BY user_id
		FOR UPDATE
	`, expenseID, groupID)
	if err != nil {
		return fmt.Errorf(
			"lock expense splits: %w",
			translateDatabaseError(err),
		)
	}
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			rows.Close()
			return fmt.Errorf(
				"scan locked expense split: %w",
				translateDatabaseError(err),
			)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf(
			"iterate locked expense splits: %w",
			translateDatabaseError(err),
		)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf(
			"close locked expense splits: %w",
			translateDatabaseError(err),
		)
	}
	return nil
}

func insertExpenseSplits(
	ctx context.Context,
	tx *sql.Tx,
	expenseID string,
	groupID string,
	splits []expenses.Split,
) error {
	for _, split := range splits {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO expense_splits (
				expense_id,
				group_id,
				user_id,
				amount_cents
			)
			VALUES ($1, $2, $3, $4)
		`,
			expenseID,
			groupID,
			split.UserID,
			split.AmountCents,
		); err != nil {
			return fmt.Errorf(
				"insert expense split: %w",
				translateDatabaseError(err),
			)
		}
	}
	return nil
}

func scanExpense(scanner expenseRowScanner) (expenses.Expense, error) {
	var expense expenses.Expense
	var expenseDate string
	err := scanner.Scan(
		&expense.ID,
		&expense.GroupID,
		&expense.PaidByUserID,
		&expense.Description,
		&expense.AmountCents,
		&expense.Currency,
		&expenseDate,
		&expense.CreatedByUserID,
		&expense.CreatedAt,
		&expense.UpdatedAt,
	)
	if err != nil {
		return expenses.Expense{}, err
	}
	parsedDate, err := time.Parse(time.DateOnly, expenseDate)
	if err != nil {
		return expenses.Expense{}, fmt.Errorf(
			"parse persisted expense date: %w",
			err,
		)
	}
	expense.ExpenseDate = parsedDate
	expense.CreatedAt = expense.CreatedAt.UTC()
	expense.UpdatedAt = expense.UpdatedAt.UTC()
	expense.Splits = make([]expenses.Split, 0)
	return expense, nil
}

func scanExpenseWithSplits(rows *sql.Rows) (expenses.Expense, error) {
	defer rows.Close()

	var expense expenses.Expense
	found := false
	for rows.Next() {
		var current expenses.Expense
		var expenseDate string
		var split expenses.Split
		if err := rows.Scan(
			&current.ID,
			&current.GroupID,
			&current.PaidByUserID,
			&current.Description,
			&current.AmountCents,
			&current.Currency,
			&expenseDate,
			&current.CreatedByUserID,
			&current.CreatedAt,
			&current.UpdatedAt,
			&split.UserID,
			&split.AmountCents,
		); err != nil {
			return expenses.Expense{}, fmt.Errorf(
				"scan visible expense: %w",
				translateDatabaseError(err),
			)
		}
		if !found {
			parsedDate, err := time.Parse(time.DateOnly, expenseDate)
			if err != nil {
				return expenses.Expense{}, fmt.Errorf(
					"parse visible expense date: %w",
					err,
				)
			}
			current.ExpenseDate = parsedDate
			current.CreatedAt = current.CreatedAt.UTC()
			current.UpdatedAt = current.UpdatedAt.UTC()
			current.Splits = make([]expenses.Split, 0)
			expense = current
			found = true
		}
		expense.Splits = append(expense.Splits, split)
	}
	if err := rows.Err(); err != nil {
		return expenses.Expense{}, fmt.Errorf(
			"iterate visible expense: %w",
			translateDatabaseError(err),
		)
	}
	if err := rows.Close(); err != nil {
		return expenses.Expense{}, fmt.Errorf(
			"close visible expense: %w",
			translateDatabaseError(err),
		)
	}
	if !found {
		return expenses.Expense{}, expenseNotFound("get visible expense")
	}
	return expense, nil
}

func expenseMemberIDs(
	actorID string,
	payerID string,
	splits []expenses.Split,
) []string {
	result := make([]string, 0, len(splits)+2)
	result = append(result, actorID, payerID)
	for _, split := range splits {
		result = append(result, split.UserID)
	}
	return result
}

func uniqueSortedExpenseMemberIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func validateExpenseSplitInvariant(
	amountCents int64,
	splits []expenses.Split,
) error {
	if len(splits) == 0 {
		return fmt.Errorf("%w: splits are required", errExpenseSplitInvariant)
	}
	seen := make(map[string]struct{}, len(splits))
	var total int64
	for index, split := range splits {
		if split.AmountCents <= 0 {
			return fmt.Errorf(
				"%w: split %d is not positive",
				errExpenseSplitInvariant,
				index,
			)
		}
		if _, duplicate := seen[split.UserID]; duplicate {
			return fmt.Errorf(
				"%w: split %d duplicates a participant",
				errExpenseSplitInvariant,
				index,
			)
		}
		seen[split.UserID] = struct{}{}
		if total > math.MaxInt64-split.AmountCents {
			return fmt.Errorf(
				"%w: split total overflows",
				errExpenseSplitInvariant,
			)
		}
		total += split.AmountCents
	}
	if total != amountCents {
		return fmt.Errorf(
			"%w: split total does not match expense amount",
			errExpenseSplitInvariant,
		)
	}
	return nil
}

func cloneExpenseSplits(splits []expenses.Split) []expenses.Split {
	return append([]expenses.Split(nil), splits...)
}

func expenseNotFound(operation string) error {
	return fmt.Errorf("%s: %w", operation, expenses.ErrNotFound)
}
