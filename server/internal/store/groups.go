package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Fyy10/settled/server/internal/groups"
)

var _ groups.Store = (*Store)(nil)

type groupRowScanner interface {
	Scan(...any) error
}

type groupQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) CreateGroup(
	ctx context.Context,
	input groups.NewGroup,
) (groups.Group, error) {
	var group groups.Group
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO groups (
				id,
				name,
				join_code,
				owner_user_id,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6)
		`,
			input.ID,
			input.Name,
			input.JoinCode,
			input.OwnerUserID,
			input.CreatedAt,
			input.UpdatedAt,
		); err != nil {
			return translateGroupWriteError("insert group", err)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO group_memberships (
				group_id,
				user_id,
				joined_at
			)
			VALUES ($1, $2, $3)
		`,
			input.ID,
			input.OwnerUserID,
			input.CreatedAt,
		); err != nil {
			return translateGroupWriteError("insert owner membership", err)
		}

		var err error
		group, err = queryVisibleGroupSummary(
			ctx,
			tx,
			input.OwnerUserID,
			input.ID,
		)
		if err != nil {
			return fmt.Errorf(
				"query created group summary: %w",
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

func (s *Store) ListGroups(
	ctx context.Context,
	userID string,
) ([]groups.Group, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			g.id::text,
			g.name,
			g.owner_user_id::text,
			(
				SELECT count(*)
				FROM group_memberships member_count
				WHERE member_count.group_id = g.id
					AND member_count.removed_at IS NULL
			),
			CASE
				WHEN g.owner_user_id = $1 THEN 'owner'
				ELSE 'member'
			END,
			g.created_at,
			g.updated_at
		FROM group_memberships actor_membership
		JOIN groups g ON g.id = actor_membership.group_id
		WHERE actor_membership.user_id = $1
			AND actor_membership.removed_at IS NULL
			AND g.dissolved_at IS NULL
		ORDER BY g.updated_at DESC, g.id DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf(
			"list visible groups: %w",
			translateDatabaseError(err),
		)
	}
	defer rows.Close()

	result := make([]groups.Group, 0)
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"scan visible group: %w",
				translateDatabaseError(err),
			)
		}
		result = append(result, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate visible groups: %w",
			translateDatabaseError(err),
		)
	}
	return result, nil
}

func (s *Store) JoinGroup(
	ctx context.Context,
	input groups.JoinGroupInput,
) (groups.Group, error) {
	var group groups.Group
	err := s.withinTx(ctx, func(tx *sql.Tx) error {
		var groupID string
		err := tx.QueryRowContext(ctx, `
			SELECT id::text
			FROM groups
			WHERE join_code = $1
				AND dissolved_at IS NULL
			FOR UPDATE
		`, input.JoinCode).Scan(&groupID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("lock joinable group: %w", groups.ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf(
				"lock joinable group: %w",
				translateDatabaseError(err),
			)
		}

		var removedAt *time.Time
		err = tx.QueryRowContext(ctx, `
			SELECT removed_at
			FROM group_memberships
			WHERE group_id = $1
				AND user_id = $2
			FOR UPDATE
		`, groupID, input.UserID).Scan(&removedAt)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO group_memberships (
					group_id,
					user_id,
					joined_at
				)
				VALUES ($1, $2, $3)
			`, groupID, input.UserID, input.JoinedAt); err != nil {
				return translateGroupWriteError("insert group membership", err)
			}
		case err != nil:
			return fmt.Errorf(
				"lock group membership: %w",
				translateDatabaseError(err),
			)
		case removedAt != nil:
			if _, err := tx.ExecContext(ctx, `
				UPDATE group_memberships
				SET joined_at = $3,
					removed_at = NULL
				WHERE group_id = $1
					AND user_id = $2
			`, groupID, input.UserID, input.JoinedAt); err != nil {
				return translateGroupWriteError(
					"reactivate group membership",
					err,
				)
			}
		}

		group, err = queryVisibleGroupSummary(ctx, tx, input.UserID, groupID)
		if err != nil {
			return fmt.Errorf(
				"query joined group summary: %w",
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

func (s *Store) GetGroup(
	ctx context.Context,
	actorID string,
	groupID string,
) (groups.Detail, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			g.id::text,
			g.name,
			g.owner_user_id::text,
			count(*) OVER (PARTITION BY g.id),
			CASE
				WHEN g.owner_user_id = $2 THEN 'owner'
				ELSE 'member'
			END,
			g.created_at,
			g.updated_at,
			member_membership.user_id::text,
			member_user.email,
			member_user.display_name,
			CASE
				WHEN member_membership.user_id = g.owner_user_id THEN 'owner'
				ELSE 'member'
			END,
			member_membership.joined_at
		FROM groups g
		JOIN group_memberships actor_membership
			ON actor_membership.group_id = g.id
			AND actor_membership.user_id = $2
			AND actor_membership.removed_at IS NULL
		JOIN group_memberships member_membership
			ON member_membership.group_id = g.id
			AND member_membership.removed_at IS NULL
		JOIN users member_user
			ON member_user.id = member_membership.user_id
		WHERE g.id = $1
			AND g.dissolved_at IS NULL
		ORDER BY
			CASE
				WHEN member_membership.user_id = g.owner_user_id THEN 0
				ELSE 1
			END,
			lower(member_user.display_name),
			member_membership.user_id
	`, groupID, actorID)
	if err != nil {
		return groups.Detail{}, fmt.Errorf(
			"get visible group: %w",
			translateDatabaseError(err),
		)
	}
	defer rows.Close()

	detail := groups.Detail{Members: make([]groups.Member, 0)}
	found := false
	for rows.Next() {
		var group groups.Group
		var member groups.Member
		if err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.OwnerUserID,
			&group.MemberCount,
			&group.CurrentUserRole,
			&group.CreatedAt,
			&group.UpdatedAt,
			&member.UserID,
			&member.Email,
			&member.DisplayName,
			&member.Role,
			&member.JoinedAt,
		); err != nil {
			return groups.Detail{}, fmt.Errorf(
				"scan visible group detail: %w",
				translateDatabaseError(err),
			)
		}
		normalizeGroupTimes(&group)
		member.JoinedAt = member.JoinedAt.UTC()
		if !found {
			detail.Group = group
			found = true
		}
		detail.Members = append(detail.Members, member)
	}
	if err := rows.Err(); err != nil {
		return groups.Detail{}, fmt.Errorf(
			"iterate visible group detail: %w",
			translateDatabaseError(err),
		)
	}
	if !found {
		return groups.Detail{}, fmt.Errorf("get visible group: %w", groups.ErrNotFound)
	}
	return detail, nil
}

func queryVisibleGroupSummary(
	ctx context.Context,
	db groupQueryRower,
	actorID string,
	groupID string,
) (groups.Group, error) {
	return scanGroup(db.QueryRowContext(ctx, `
		SELECT
			g.id::text,
			g.name,
			g.owner_user_id::text,
			(
				SELECT count(*)
				FROM group_memberships member_count
				WHERE member_count.group_id = g.id
					AND member_count.removed_at IS NULL
			),
			CASE
				WHEN g.owner_user_id = $2 THEN 'owner'
				ELSE 'member'
			END,
			g.created_at,
			g.updated_at
		FROM groups g
		JOIN group_memberships actor_membership
			ON actor_membership.group_id = g.id
			AND actor_membership.user_id = $2
			AND actor_membership.removed_at IS NULL
		WHERE g.id = $1
			AND g.dissolved_at IS NULL
	`, groupID, actorID))
}

func scanGroup(row groupRowScanner) (groups.Group, error) {
	var group groups.Group
	if err := row.Scan(
		&group.ID,
		&group.Name,
		&group.OwnerUserID,
		&group.MemberCount,
		&group.CurrentUserRole,
		&group.CreatedAt,
		&group.UpdatedAt,
	); err != nil {
		return groups.Group{}, err
	}
	normalizeGroupTimes(&group)
	return group, nil
}

func normalizeGroupTimes(group *groups.Group) {
	group.CreatedAt = group.CreatedAt.UTC()
	group.UpdatedAt = group.UpdatedAt.UTC()
}

func translateGroupWriteError(operation string, err error) error {
	translated := translateDatabaseError(err)
	if databaseErrorKindOf(translated) == databaseErrorJoinCodeCollision {
		return fmt.Errorf("%s: %w", operation, groups.ErrJoinCodeCollision)
	}
	return fmt.Errorf("%s: %w", operation, translated)
}
