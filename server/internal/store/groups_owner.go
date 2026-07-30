package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Fyy10/settled/server/internal/groups"
)

type lockedOwnerGroup struct {
	ownerID  string
	joinCode string
}

func (s *Store) RenameGroup(
	ctx context.Context,
	input groups.RenameGroupInput,
) (groups.Group, error) {
	var group groups.Group
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		if _, err := lockOwnerGroup(
			ctx,
			tx,
			input.ActorID,
			input.GroupID,
		); err != nil {
			return err
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE groups
			SET name = $2,
				updated_at = $3
			WHERE id = $1
				AND dissolved_at IS NULL
		`, input.GroupID, input.Name, input.UpdatedAt)
		if err != nil {
			return translateGroupWriteError("rename group", err)
		}
		if err := requireOneGroupRow("rename group", result); err != nil {
			return err
		}

		group, err = queryVisibleGroupSummary(
			ctx,
			tx,
			input.ActorID,
			input.GroupID,
		)
		if err != nil {
			return fmt.Errorf(
				"query renamed group summary: %w",
				translateDatabaseError(err),
			)
		}
		return nil
	})
	if err != nil {
		return groups.Group{}, err
	}
	return group, nil
}

func (s *Store) DissolveGroup(
	ctx context.Context,
	input groups.DissolveGroupInput,
) error {
	return s.withinTx(ctx, func(tx *sql.Tx) error {
		if _, err := lockOwnerGroup(
			ctx,
			tx,
			input.ActorID,
			input.GroupID,
		); err != nil {
			return err
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE groups
			SET dissolved_at = $2,
				updated_at = $2
			WHERE id = $1
				AND dissolved_at IS NULL
		`, input.GroupID, input.DissolvedAt)
		if err != nil {
			return translateGroupWriteError("dissolve group", err)
		}
		return requireOneGroupRow("dissolve group", result)
	})
}

func (s *Store) GetJoinCode(
	ctx context.Context,
	actorID string,
	groupID string,
) (string, error) {
	var joinCode string
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		locked, err := lockOwnerGroup(ctx, tx, actorID, groupID)
		if err != nil {
			return err
		}
		joinCode = locked.joinCode
		return nil
	})
	if err != nil {
		return "", err
	}
	return joinCode, nil
}

func (s *Store) RemoveMember(
	ctx context.Context,
	input groups.RemoveMemberInput,
) error {
	return s.withinTx(ctx, func(tx *sql.Tx) error {
		locked, err := lockActiveGroup(ctx, tx, input.GroupID)
		if err != nil {
			return err
		}

		activeMemberships, err := lockGroupMemberships(
			ctx,
			tx,
			input.GroupID,
			input.ActorID,
			input.UserID,
		)
		if err != nil {
			return err
		}
		if !activeMemberships[input.ActorID] {
			return fmt.Errorf("authorize member removal: %w", groups.ErrNotFound)
		}
		if locked.ownerID != input.ActorID {
			return fmt.Errorf("authorize member removal: %w", groups.ErrForbidden)
		}
		if input.UserID == locked.ownerID {
			return fmt.Errorf("remove group owner: %w", groups.ErrForbidden)
		}
		if !activeMemberships[input.UserID] {
			return fmt.Errorf("find active target member: %w", groups.ErrNotFound)
		}

		inUse, err := groupMemberInUse(ctx, tx, input.GroupID, input.UserID)
		if err != nil {
			return err
		}
		if inUse {
			return fmt.Errorf("remove active group member: %w", groups.ErrMemberInUse)
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE group_memberships
			SET removed_at = $3
			WHERE group_id = $1
				AND user_id = $2
				AND removed_at IS NULL
		`, input.GroupID, input.UserID, input.RemovedAt)
		if err != nil {
			return translateGroupWriteError("remove group member", err)
		}
		return requireOneGroupRow("remove group member", result)
	})
}

func lockOwnerGroup(
	ctx context.Context,
	tx *sql.Tx,
	actorID string,
	groupID string,
) (lockedOwnerGroup, error) {
	locked, err := lockActiveGroup(ctx, tx, groupID)
	if err != nil {
		return lockedOwnerGroup{}, err
	}

	var removedAt *time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT removed_at
		FROM group_memberships
		WHERE group_id = $1
			AND user_id = $2
		FOR UPDATE
	`, groupID, actorID).Scan(&removedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return lockedOwnerGroup{}, fmt.Errorf(
			"authorize group owner: %w",
			groups.ErrNotFound,
		)
	}
	if err != nil {
		return lockedOwnerGroup{}, fmt.Errorf(
			"lock owner membership: %w",
			translateDatabaseError(err),
		)
	}
	if removedAt != nil {
		return lockedOwnerGroup{}, fmt.Errorf(
			"authorize group owner: %w",
			groups.ErrNotFound,
		)
	}
	if locked.ownerID != actorID {
		return lockedOwnerGroup{}, fmt.Errorf(
			"authorize group owner: %w",
			groups.ErrForbidden,
		)
	}
	return locked, nil
}

func lockActiveGroup(
	ctx context.Context,
	tx *sql.Tx,
	groupID string,
) (lockedOwnerGroup, error) {
	var locked lockedOwnerGroup
	err := tx.QueryRowContext(ctx, `
		SELECT owner_user_id::text, join_code
		FROM groups
		WHERE id = $1
			AND dissolved_at IS NULL
		FOR UPDATE
	`, groupID).Scan(&locked.ownerID, &locked.joinCode)
	if errors.Is(err, sql.ErrNoRows) {
		return lockedOwnerGroup{}, fmt.Errorf(
			"lock active group: %w",
			groups.ErrNotFound,
		)
	}
	if err != nil {
		return lockedOwnerGroup{}, fmt.Errorf(
			"lock active group: %w",
			translateDatabaseError(err),
		)
	}
	return locked, nil
}

func lockGroupMemberships(
	ctx context.Context,
	tx *sql.Tx,
	groupID string,
	actorID string,
	targetID string,
) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT user_id::text, removed_at
		FROM group_memberships
		WHERE group_id = $1
			AND user_id IN ($2, $3)
		ORDER BY user_id
		FOR UPDATE
	`, groupID, actorID, targetID)
	if err != nil {
		return nil, fmt.Errorf(
			"lock group memberships: %w",
			translateDatabaseError(err),
		)
	}

	active := make(map[string]bool, 2)
	for rows.Next() {
		var userID string
		var removedAt *time.Time
		if err := rows.Scan(&userID, &removedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf(
				"scan locked group membership: %w",
				translateDatabaseError(err),
			)
		}
		active[userID] = removedAt == nil
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf(
			"iterate locked group memberships: %w",
			translateDatabaseError(err),
		)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf(
			"close locked group memberships: %w",
			translateDatabaseError(err),
		)
	}
	return active, nil
}

func groupMemberInUse(
	ctx context.Context,
	tx *sql.Tx,
	groupID string,
	userID string,
) (bool, error) {
	var inUse bool
	err := tx.QueryRowContext(ctx, `
		SELECT
			EXISTS (
				SELECT 1
				FROM expenses expense
				WHERE expense.group_id = $1
					AND expense.deleted_at IS NULL
					AND expense.paid_by_user_id = $2
			)
			OR EXISTS (
				SELECT 1
				FROM expense_splits split
				JOIN expenses expense
					ON expense.id = split.expense_id
					AND expense.group_id = split.group_id
				WHERE split.group_id = $1
					AND split.user_id = $2
					AND expense.deleted_at IS NULL
			)
			OR EXISTS (
				SELECT 1
				FROM repayments repayment
				WHERE repayment.group_id = $1
					AND repayment.deleted_at IS NULL
					AND (
						repayment.from_user_id = $2
						OR repayment.to_user_id = $2
					)
			)
	`, groupID, userID).Scan(&inUse)
	if err != nil {
		return false, fmt.Errorf(
			"check group member accounting references: %w",
			translateDatabaseError(err),
		)
	}
	return inUse, nil
}

func requireOneGroupRow(operation string, result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"%s rows affected: %w",
			operation,
			translateDatabaseError(err),
		)
	}
	switch affected {
	case 0:
		return fmt.Errorf("%s: %w", operation, groups.ErrNotFound)
	case 1:
		return nil
	default:
		return fmt.Errorf("%s affected %d rows", operation, affected)
	}
}
