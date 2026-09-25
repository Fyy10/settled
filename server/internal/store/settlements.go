package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fyy10/settled/server/internal/settlements"
)

var _ settlements.Store = (*Store)(nil)

const settlementDebtEntriesQuery = `
	SELECT
		from_user_id::text,
		to_user_id::text,
		amount_cents,
		currency::text
	FROM settlement_debt_entries
	WHERE group_id = $1
	ORDER BY
		occurred_on,
		created_at,
		from_user_id,
		to_user_id,
		amount_cents,
		currency
`

const settlementMemberSummariesQuery = `
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
`

func (s *Store) ListDebtEntries(
	ctx context.Context,
	actorID string,
	groupID string,
) ([]settlements.DebtEntry, []settlements.MemberSummary, error) {
	entries := make([]settlements.DebtEntry, 0)
	members := make([]settlements.MemberSummary, 0)
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		if err := lockReadableSettlementGroup(
			ctx,
			tx,
			actorID,
			groupID,
		); err != nil {
			return err
		}

		rows, err := tx.QueryContext(
			ctx,
			settlementDebtEntriesQuery,
			groupID,
		)
		if err != nil {
			return fmt.Errorf(
				"list settlement debt entries: %w",
				translateDatabaseError(err),
			)
		}
		for rows.Next() {
			var entry settlements.DebtEntry
			if err := rows.Scan(
				&entry.FromUserID,
				&entry.ToUserID,
				&entry.AmountCents,
				&entry.Currency,
			); err != nil {
				rows.Close()
				return fmt.Errorf(
					"scan settlement debt entry: %w",
					translateDatabaseError(err),
				)
			}
			entries = append(entries, entry)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf(
				"iterate settlement debt entries: %w",
				translateDatabaseError(err),
			)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf(
				"close settlement debt entries: %w",
				translateDatabaseError(err),
			)
		}

		rows, err = tx.QueryContext(
			ctx,
			settlementMemberSummariesQuery,
			groupID,
		)
		if err != nil {
			return fmt.Errorf(
				"list settlement member summaries: %w",
				translateDatabaseError(err),
			)
		}
		for rows.Next() {
			var member settlements.MemberSummary
			if err := rows.Scan(
				&member.UserID,
				&member.DisplayName,
			); err != nil {
				rows.Close()
				return fmt.Errorf(
					"scan settlement member summary: %w",
					translateDatabaseError(err),
				)
			}
			members = append(members, member)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf(
				"iterate settlement member summaries: %w",
				translateDatabaseError(err),
			)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf(
				"close settlement member summaries: %w",
				translateDatabaseError(err),
			)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return entries, members, nil
}

func lockReadableSettlementGroup(
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
		return settlementNotFound("lock readable settlement group")
	}
	if err != nil {
		return fmt.Errorf(
			"lock readable settlement group: %w",
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
		return settlementNotFound("authorize settlement list")
	}
	if err != nil {
		return fmt.Errorf(
			"lock settlement actor membership: %w",
			translateDatabaseError(err),
		)
	}
	return nil
}

func settlementNotFound(operation string) error {
	return fmt.Errorf("%s: %w", operation, settlements.ErrNotFound)
}
