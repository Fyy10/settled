package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Fyy10/settled/server/internal/repayments"
)

var errRepaymentInvariant = errors.New("invalid repayment invariant")

var _ repayments.Store = (*Store)(nil)

type repaymentRowScanner interface {
	Scan(...any) error
}

func (s *Store) CreateRepayment(
	ctx context.Context,
	input repayments.CreateInput,
) (repayments.Repayment, error) {
	var repayment repayments.Repayment
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		if err := lockRepaymentGroup(ctx, tx, input.GroupID); err != nil {
			return err
		}
		if err := lockActiveRepaymentMemberships(
			ctx,
			tx,
			input.GroupID,
			[]string{input.ActorID, input.FromUserID, input.ToUserID},
		); err != nil {
			return err
		}
		note, err := validateRepaymentInvariant(
			input.FromUserID,
			input.ToUserID,
			input.AmountCents,
			input.Currency,
			input.Note,
			input.RepaymentDate,
		)
		if err != nil {
			return err
		}

		row := tx.QueryRowContext(ctx, `
			INSERT INTO repayments (
				id,
				group_id,
				from_user_id,
				to_user_id,
				amount_cents,
				currency,
				note,
				repayment_date,
				created_by_user_id,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING
				id::text,
				group_id::text,
				from_user_id::text,
				to_user_id::text,
				amount_cents,
				currency::text,
				note,
				repayment_date::text,
				created_by_user_id::text,
				created_at,
				updated_at
		`,
			input.ID,
			input.GroupID,
			input.FromUserID,
			input.ToUserID,
			input.AmountCents,
			input.Currency,
			note,
			input.RepaymentDate,
			input.ActorID,
			input.CreatedAt,
			input.UpdatedAt,
		)
		repayment, err = scanRepayment(row)
		if err != nil {
			return fmt.Errorf(
				"insert repayment: %w",
				translateDatabaseError(err),
			)
		}
		return nil
	})
	if err != nil {
		return repayments.Repayment{}, err
	}
	return repayment, nil
}

func (s *Store) ReplaceRepayment(
	ctx context.Context,
	input repayments.ReplaceInput,
) (repayments.Repayment, error) {
	var repayment repayments.Repayment
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		if err := lockRepaymentGroup(ctx, tx, input.GroupID); err != nil {
			return err
		}
		if err := lockActiveRepaymentMemberships(
			ctx,
			tx,
			input.GroupID,
			[]string{input.ActorID, input.FromUserID, input.ToUserID},
		); err != nil {
			return err
		}
		if err := lockVisibleRepayment(
			ctx,
			tx,
			input.GroupID,
			input.RepaymentID,
		); err != nil {
			return err
		}
		note, err := validateRepaymentInvariant(
			input.FromUserID,
			input.ToUserID,
			input.AmountCents,
			input.Currency,
			input.Note,
			input.RepaymentDate,
		)
		if err != nil {
			return err
		}

		row := tx.QueryRowContext(ctx, `
			UPDATE repayments
			SET from_user_id = $3,
				to_user_id = $4,
				amount_cents = $5,
				currency = $6,
				note = $7,
				repayment_date = $8,
				updated_at = $9
			WHERE id = $1
				AND group_id = $2
				AND deleted_at IS NULL
			RETURNING
				id::text,
				group_id::text,
				from_user_id::text,
				to_user_id::text,
				amount_cents,
				currency::text,
				note,
				repayment_date::text,
				created_by_user_id::text,
				created_at,
				updated_at
		`,
			input.RepaymentID,
			input.GroupID,
			input.FromUserID,
			input.ToUserID,
			input.AmountCents,
			input.Currency,
			note,
			input.RepaymentDate,
			input.UpdatedAt,
		)
		repayment, err = scanRepayment(row)
		if errors.Is(err, sql.ErrNoRows) {
			return repaymentNotFound("replace repayment")
		}
		if err != nil {
			return fmt.Errorf(
				"replace repayment: %w",
				translateDatabaseError(err),
			)
		}
		return nil
	})
	if err != nil {
		return repayments.Repayment{}, err
	}
	return repayment, nil
}

func (s *Store) DeleteRepayment(
	ctx context.Context,
	input repayments.DeleteInput,
) error {
	return s.withinTx(ctx, func(tx *sql.Tx) error {
		if err := lockRepaymentGroup(ctx, tx, input.GroupID); err != nil {
			return err
		}
		if err := lockActiveRepaymentMemberships(
			ctx,
			tx,
			input.GroupID,
			[]string{input.ActorID},
		); err != nil {
			return err
		}
		if err := lockVisibleRepayment(
			ctx,
			tx,
			input.GroupID,
			input.RepaymentID,
		); err != nil {
			return err
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE repayments
			SET deleted_at = $3,
				updated_at = $3
			WHERE id = $1
				AND group_id = $2
				AND deleted_at IS NULL
		`, input.RepaymentID, input.GroupID, input.DeletedAt)
		if err != nil {
			return fmt.Errorf(
				"soft-delete repayment: %w",
				translateDatabaseError(err),
			)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf(
				"soft-delete repayment rows affected: %w",
				translateDatabaseError(err),
			)
		}
		if affected != 1 {
			if affected == 0 {
				return repaymentNotFound("soft-delete repayment")
			}
			return fmt.Errorf(
				"soft-delete repayment affected %d rows",
				affected,
			)
		}
		return nil
	})
}

func (s *Store) GetRepayment(
	ctx context.Context,
	actorID string,
	groupID string,
	repaymentID string,
) (repayments.Repayment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			repayment.id::text,
			repayment.group_id::text,
			repayment.from_user_id::text,
			repayment.to_user_id::text,
			repayment.amount_cents,
			repayment.currency::text,
			repayment.note,
			repayment.repayment_date::text,
			repayment.created_by_user_id::text,
			repayment.created_at,
			repayment.updated_at
		FROM repayments repayment
		JOIN groups active_group
			ON active_group.id = repayment.group_id
			AND active_group.dissolved_at IS NULL
		JOIN group_memberships actor_membership
			ON actor_membership.group_id = repayment.group_id
			AND actor_membership.user_id = $3
			AND actor_membership.removed_at IS NULL
		WHERE repayment.id = $1
			AND repayment.group_id = $2
			AND repayment.deleted_at IS NULL
	`, repaymentID, groupID, actorID)
	repayment, err := scanRepayment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return repayments.Repayment{}, repaymentNotFound("get visible repayment")
	}
	if err != nil {
		return repayments.Repayment{}, fmt.Errorf(
			"get visible repayment: %w",
			translateDatabaseError(err),
		)
	}
	return repayment, nil
}

func (s *Store) ListRepayments(
	ctx context.Context,
	actorID string,
	groupID string,
) (repayments.ListResult, error) {
	result := repayments.ListResult{
		Repayments: make([]repayments.Repayment, 0),
		Members:    make([]repayments.MemberSummary, 0),
	}
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		if err := lockReadableRepaymentGroup(
			ctx,
			tx,
			actorID,
			groupID,
		); err != nil {
			return err
		}

		repaymentRows, err := tx.QueryContext(ctx, `
			SELECT
				id::text,
				group_id::text,
				from_user_id::text,
				to_user_id::text,
				amount_cents,
				currency::text,
				note,
				repayment_date::text,
				created_by_user_id::text,
				created_at,
				updated_at
			FROM repayments
			WHERE group_id = $1
				AND deleted_at IS NULL
			ORDER BY repayment_date DESC, created_at DESC, id DESC
		`, groupID)
		if err != nil {
			return fmt.Errorf(
				"list visible repayments: %w",
				translateDatabaseError(err),
			)
		}
		for repaymentRows.Next() {
			repayment, err := scanRepayment(repaymentRows)
			if err != nil {
				repaymentRows.Close()
				return fmt.Errorf(
					"scan visible repayment: %w",
					translateDatabaseError(err),
				)
			}
			result.Repayments = append(result.Repayments, repayment)
		}
		if err := repaymentRows.Err(); err != nil {
			repaymentRows.Close()
			return fmt.Errorf(
				"iterate visible repayments: %w",
				translateDatabaseError(err),
			)
		}
		if err := repaymentRows.Close(); err != nil {
			return fmt.Errorf(
				"close visible repayments: %w",
				translateDatabaseError(err),
			)
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
				"list repayment member summaries: %w",
				translateDatabaseError(err),
			)
		}
		for memberRows.Next() {
			var member repayments.MemberSummary
			if err := memberRows.Scan(
				&member.UserID,
				&member.DisplayName,
			); err != nil {
				memberRows.Close()
				return fmt.Errorf(
					"scan repayment member summary: %w",
					translateDatabaseError(err),
				)
			}
			result.Members = append(result.Members, member)
		}
		if err := memberRows.Err(); err != nil {
			memberRows.Close()
			return fmt.Errorf(
				"iterate repayment member summaries: %w",
				translateDatabaseError(err),
			)
		}
		if err := memberRows.Close(); err != nil {
			return fmt.Errorf(
				"close repayment member summaries: %w",
				translateDatabaseError(err),
			)
		}
		return nil
	})
	if err != nil {
		return repayments.ListResult{}, err
	}
	return result, nil
}

func lockRepaymentGroup(
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
		return repaymentNotFound("lock active repayment group")
	}
	if err != nil {
		return fmt.Errorf(
			"lock active repayment group: %w",
			translateDatabaseError(err),
		)
	}
	return nil
}

func lockReadableRepaymentGroup(
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
		return repaymentNotFound("lock readable repayment group")
	}
	if err != nil {
		return fmt.Errorf(
			"lock readable repayment group: %w",
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
		return repaymentNotFound("authorize repayment list")
	}
	if err != nil {
		return fmt.Errorf(
			"lock repayment list actor membership: %w",
			translateDatabaseError(err),
		)
	}
	return nil
}

func lockActiveRepaymentMemberships(
	ctx context.Context,
	tx *sql.Tx,
	groupID string,
	memberIDs []string,
) error {
	memberIDs = uniqueSortedRepaymentMemberIDs(memberIDs)
	if len(memberIDs) == 0 {
		return repaymentNotFound("authorize repayment memberships")
	}

	query, arguments := repaymentMembershipLockQuery(groupID, memberIDs)
	rows, err := tx.QueryContext(ctx, query, arguments...)
	if err != nil {
		return fmt.Errorf(
			"lock repayment memberships: %w",
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
				"scan locked repayment membership: %w",
				translateDatabaseError(err),
			)
		}
		active[userID] = removedAt == nil
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf(
			"iterate locked repayment memberships: %w",
			translateDatabaseError(err),
		)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf(
			"close locked repayment memberships: %w",
			translateDatabaseError(err),
		)
	}
	for _, userID := range memberIDs {
		if !active[userID] {
			return repaymentNotFound("authorize repayment memberships")
		}
	}
	return nil
}

func repaymentMembershipLockQuery(
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

func lockVisibleRepayment(
	ctx context.Context,
	tx *sql.Tx,
	groupID string,
	repaymentID string,
) error {
	var lockedID string
	err := tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM repayments
		WHERE id = $1
			AND group_id = $2
			AND deleted_at IS NULL
		FOR UPDATE
	`, repaymentID, groupID).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return repaymentNotFound("lock visible repayment")
	}
	if err != nil {
		return fmt.Errorf(
			"lock visible repayment: %w",
			translateDatabaseError(err),
		)
	}
	return nil
}

func scanRepayment(scanner repaymentRowScanner) (repayments.Repayment, error) {
	var repayment repayments.Repayment
	var note sql.NullString
	var repaymentDate string
	err := scanner.Scan(
		&repayment.ID,
		&repayment.GroupID,
		&repayment.FromUserID,
		&repayment.ToUserID,
		&repayment.AmountCents,
		&repayment.Currency,
		&note,
		&repaymentDate,
		&repayment.CreatedByUserID,
		&repayment.CreatedAt,
		&repayment.UpdatedAt,
	)
	if err != nil {
		return repayments.Repayment{}, err
	}
	parsedDate, err := time.Parse(time.DateOnly, repaymentDate)
	if err != nil {
		return repayments.Repayment{}, fmt.Errorf(
			"parse persisted repayment date: %w",
			err,
		)
	}
	repayment.Note = nullableStringPointer(note)
	repayment.RepaymentDate = parsedDate
	repayment.CreatedAt = repayment.CreatedAt.UTC()
	repayment.UpdatedAt = repayment.UpdatedAt.UTC()
	return repayment, nil
}

func uniqueSortedRepaymentMemberIDs(values []string) []string {
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

func validateRepaymentInvariant(
	fromUserID string,
	toUserID string,
	amountCents int64,
	currency string,
	note *string,
	repaymentDate time.Time,
) (*string, error) {
	if fromUserID == toUserID {
		return nil, fmt.Errorf(
			"%w: sender and recipient must differ",
			errRepaymentInvariant,
		)
	}
	if amountCents <= 0 {
		return nil, fmt.Errorf(
			"%w: amount must be positive",
			errRepaymentInvariant,
		)
	}
	if currency != "USD" {
		return nil, fmt.Errorf(
			"%w: currency must be USD",
			errRepaymentInvariant,
		)
	}
	hour, minute, second := repaymentDate.Clock()
	_, offset := repaymentDate.Zone()
	if hour != 0 ||
		minute != 0 ||
		second != 0 ||
		repaymentDate.Nanosecond() != 0 ||
		offset != 0 {
		return nil, fmt.Errorf(
			"%w: repayment date must be a UTC calendar date",
			errRepaymentInvariant,
		)
	}
	if note == nil {
		return nil, nil
	}
	if !utf8.ValidString(*note) {
		return nil, fmt.Errorf(
			"%w: note must be valid UTF-8",
			errRepaymentInvariant,
		)
	}
	normalized := strings.TrimSpace(*note)
	if normalized == "" {
		return nil, nil
	}
	for _, character := range normalized {
		if unicode.IsControl(character) ||
			unicode.Is(unicode.Bidi_Control, character) {
			return nil, fmt.Errorf(
				"%w: note contains a disallowed control character",
				errRepaymentInvariant,
			)
		}
	}
	if utf8.RuneCountInString(normalized) > 240 {
		return nil, fmt.Errorf(
			"%w: note exceeds 240 code points",
			errRepaymentInvariant,
		)
	}
	return &normalized, nil
}

func nullableStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func repaymentNotFound(operation string) error {
	return fmt.Errorf("%s: %w", operation, repayments.ErrNotFound)
}
