//go:build integration

package store

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/groups"
)

func TestGroupOwnerStoreOwnerWorkflows(t *testing.T) {
	db := openIntegrationDatabase(t)
	createdAt := groupOwnerTestFixture(t, db)
	store := New(db)
	ctx := context.Background()

	joinCode, err := store.GetJoinCode(ctx, aliceID, groupOneID)
	if err != nil {
		t.Fatalf("GetJoinCode: %v", err)
	}
	if joinCode != "OWNERGROUP23" {
		t.Errorf("join code = %q, want %q", joinCode, "OWNERGROUP23")
	}

	renamedAt := createdAt.Add(time.Hour)
	renamed, err := store.RenameGroup(ctx, groups.RenameGroupInput{
		ActorID:   aliceID,
		GroupID:   groupOneID,
		Name:      "Renamed Group",
		UpdatedAt: renamedAt,
	})
	if err != nil {
		t.Fatalf("RenameGroup: %v", err)
	}
	wantRenamed := groups.Group{
		ID:              groupOneID,
		Name:            "Renamed Group",
		OwnerUserID:     aliceID,
		MemberCount:     4,
		CurrentUserRole: groups.RoleOwner,
		CreatedAt:       createdAt,
		UpdatedAt:       renamedAt,
	}
	if !reflect.DeepEqual(renamed, wantRenamed) {
		t.Errorf("renamed group = %#v, want %#v", renamed, wantRenamed)
	}

	dissolvedAt := renamedAt.Add(time.Hour)
	if err := store.DissolveGroup(ctx, groups.DissolveGroupInput{
		ActorID:     aliceID,
		GroupID:     groupOneID,
		DissolvedAt: dissolvedAt,
	}); err != nil {
		t.Fatalf("DissolveGroup: %v", err)
	}
	var gotDissolvedAt time.Time
	var gotUpdatedAt time.Time
	if err := db.QueryRow(`
		SELECT dissolved_at, updated_at
		FROM groups
		WHERE id = $1
	`, groupOneID).Scan(&gotDissolvedAt, &gotUpdatedAt); err != nil {
		t.Fatalf("query dissolved group: %v", err)
	}
	if !gotDissolvedAt.Equal(dissolvedAt) ||
		!gotUpdatedAt.Equal(dissolvedAt) {
		t.Errorf(
			"dissolved timestamps = (%v, %v), want %v",
			gotDissolvedAt,
			gotUpdatedAt,
			dissolvedAt,
		)
	}

	hiddenOperations := []struct {
		name string
		call func() error
	}{
		{
			name: "rename",
			call: func() error {
				_, err := store.RenameGroup(ctx, groups.RenameGroupInput{
					ActorID:   aliceID,
					GroupID:   groupOneID,
					Name:      "Hidden Rename",
					UpdatedAt: dissolvedAt.Add(time.Hour),
				})
				return err
			},
		},
		{
			name: "repeated dissolve",
			call: func() error {
				return store.DissolveGroup(ctx, groups.DissolveGroupInput{
					ActorID:     aliceID,
					GroupID:     groupOneID,
					DissolvedAt: dissolvedAt.Add(time.Hour),
				})
			},
		},
		{
			name: "join code",
			call: func() error {
				_, err := store.GetJoinCode(ctx, aliceID, groupOneID)
				return err
			},
		},
		{
			name: "remove member",
			call: func() error {
				return store.RemoveMember(ctx, groups.RemoveMemberInput{
					ActorID:   aliceID,
					GroupID:   groupOneID,
					UserID:    bobID,
					RemovedAt: dissolvedAt.Add(time.Hour),
				})
			},
		},
	}
	for _, operation := range hiddenOperations {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.call(); !errors.Is(err, groups.ErrNotFound) {
				t.Errorf("error = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestGroupOwnerStoreAuthorizationAndVisibility(t *testing.T) {
	db := openIntegrationDatabase(t)
	store := New(db)
	ctx := context.Background()

	states := []struct {
		name      string
		actorID   string
		groupID   string
		dissolved bool
		want      error
	}{
		{
			name:    "active member is forbidden",
			actorID: bobID,
			groupID: groupOneID,
			want:    groups.ErrForbidden,
		},
		{
			name:    "nonmember is hidden",
			actorID: frankID,
			groupID: groupOneID,
			want:    groups.ErrNotFound,
		},
		{
			name:    "removed actor is hidden",
			actorID: erinID,
			groupID: groupOneID,
			want:    groups.ErrNotFound,
		},
		{
			name:      "dissolved group is hidden",
			actorID:   aliceID,
			groupID:   groupOneID,
			dissolved: true,
			want:      groups.ErrNotFound,
		},
		{
			name:    "missing group is hidden",
			actorID: aliceID,
			groupID: groupTwoID,
			want:    groups.ErrNotFound,
		},
	}
	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			createdAt := groupOwnerTestFixture(t, db)
			if state.dissolved {
				if _, err := db.Exec(`
					UPDATE groups
					SET dissolved_at = $2,
						updated_at = $2
					WHERE id = $1
				`, groupOneID, createdAt.Add(time.Hour)); err != nil {
					t.Fatalf("dissolve fixture group: %v", err)
				}
			}

			operations := []struct {
				name string
				call func() error
			}{
				{
					name: "rename",
					call: func() error {
						_, err := store.RenameGroup(
							ctx,
							groups.RenameGroupInput{
								ActorID:   state.actorID,
								GroupID:   state.groupID,
								Name:      "Unauthorized Rename",
								UpdatedAt: createdAt.Add(2 * time.Hour),
							},
						)
						return err
					},
				},
				{
					name: "dissolve",
					call: func() error {
						return store.DissolveGroup(
							ctx,
							groups.DissolveGroupInput{
								ActorID:     state.actorID,
								GroupID:     state.groupID,
								DissolvedAt: createdAt.Add(2 * time.Hour),
							},
						)
					},
				},
				{
					name: "join code",
					call: func() error {
						_, err := store.GetJoinCode(
							ctx,
							state.actorID,
							state.groupID,
						)
						return err
					},
				},
				{
					name: "remove member",
					call: func() error {
						return store.RemoveMember(
							ctx,
							groups.RemoveMemberInput{
								ActorID:   state.actorID,
								GroupID:   state.groupID,
								UserID:    carolID,
								RemovedAt: createdAt.Add(2 * time.Hour),
							},
						)
					},
				},
			}
			for _, operation := range operations {
				t.Run(operation.name, func(t *testing.T) {
					err := operation.call()
					if !errors.Is(err, state.want) {
						t.Errorf("error = %v, want %v", err, state.want)
					}
					if state.want == groups.ErrNotFound &&
						errors.Is(err, groups.ErrForbidden) {
						t.Errorf("hidden state leaked forbidden distinction: %v", err)
					}
				})
			}
		})
	}
}

func TestGroupOwnerStoreRemoveMemberLifecycle(t *testing.T) {
	db := openIntegrationDatabase(t)
	createdAt := groupOwnerTestFixture(t, db)
	store := New(db)
	ctx := context.Background()

	if err := store.RemoveMember(ctx, groups.RemoveMemberInput{
		ActorID:   aliceID,
		GroupID:   groupOneID,
		UserID:    aliceID,
		RemovedAt: createdAt.Add(time.Hour),
	}); !errors.Is(err, groups.ErrForbidden) {
		t.Errorf("owner removal error = %v, want ErrForbidden", err)
	}
	if err := store.RemoveMember(ctx, groups.RemoveMemberInput{
		ActorID:   aliceID,
		GroupID:   groupOneID,
		UserID:    frankID,
		RemovedAt: createdAt.Add(time.Hour),
	}); !errors.Is(err, groups.ErrNotFound) {
		t.Errorf("missing target error = %v, want ErrNotFound", err)
	}

	removedAt := createdAt.Add(2 * time.Hour)
	if err := store.RemoveMember(ctx, groups.RemoveMemberInput{
		ActorID:   aliceID,
		GroupID:   groupOneID,
		UserID:    bobID,
		RemovedAt: removedAt,
	}); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	groupTestAssertMembership(
		t,
		db,
		groupOneID,
		bobID,
		createdAt.Add(time.Minute),
		&removedAt,
	)
	if _, err := store.GetGroup(
		ctx,
		bobID,
		groupOneID,
	); !errors.Is(err, groups.ErrNotFound) {
		t.Errorf("removed member visibility error = %v, want ErrNotFound", err)
	}
	detail, err := store.GetGroup(ctx, aliceID, groupOneID)
	if err != nil {
		t.Fatalf("owner GetGroup after removal: %v", err)
	}
	if detail.Group.MemberCount != 3 {
		t.Errorf("member count = %d, want 3", detail.Group.MemberCount)
	}
	for _, member := range detail.Members {
		if member.UserID == bobID {
			t.Errorf("removed member remains in detail: %+v", detail.Members)
		}
	}
	if err := store.RemoveMember(ctx, groups.RemoveMemberInput{
		ActorID:   aliceID,
		GroupID:   groupOneID,
		UserID:    bobID,
		RemovedAt: removedAt.Add(time.Hour),
	}); !errors.Is(err, groups.ErrNotFound) {
		t.Errorf("repeated removal error = %v, want ErrNotFound", err)
	}
}

func TestGroupOwnerStoreRemovalLocksMembershipsInUserIDOrder(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	groupTestInsertUsers(t, db)
	createdAt := time.Date(2026, time.July, 31, 10, 0, 0, 0, time.UTC)
	groupTestInsertGroup(
		t,
		db,
		groupOneID,
		frankID,
		"Reverse ID Group",
		"REVERSEID234",
		createdAt,
		nil,
	)
	groupTestInsertMembership(t, db, groupOneID, frankID, createdAt, nil)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		aliceID,
		createdAt.Add(time.Minute),
		nil,
	)

	removedAt := createdAt.Add(time.Hour)
	if err := New(db).RemoveMember(
		context.Background(),
		groups.RemoveMemberInput{
			ActorID:   frankID,
			GroupID:   groupOneID,
			UserID:    aliceID,
			RemovedAt: removedAt,
		},
	); err != nil {
		t.Fatalf("RemoveMember with owner ID after target: %v", err)
	}
	groupTestAssertMembership(
		t,
		db,
		groupOneID,
		aliceID,
		createdAt.Add(time.Minute),
		&removedAt,
	)
}

func TestGroupOwnerStoreRemoveMemberAccountingConflicts(t *testing.T) {
	db := openIntegrationDatabase(t)
	store := New(db)
	ctx := context.Background()

	tests := []struct {
		name string
		seed func(*testing.T, *sql.DB, time.Time)
	}{
		{
			name: "expense payer",
			seed: func(t *testing.T, db *sql.DB, createdAt time.Time) {
				if _, err := insertExpense(
					db,
					expenseOneID,
					groupOneID,
					bobID,
					aliceID,
					100,
					"USD",
					&recordTimes{createdAt: createdAt},
				); err != nil {
					t.Fatalf("insert payer expense: %v", err)
				}
				insertSplit(t, db, expenseOneID, groupOneID, aliceID, 100)
			},
		},
		{
			name: "expense split participant",
			seed: func(t *testing.T, db *sql.DB, createdAt time.Time) {
				if _, err := insertExpense(
					db,
					expenseOneID,
					groupOneID,
					aliceID,
					aliceID,
					100,
					"USD",
					&recordTimes{createdAt: createdAt},
				); err != nil {
					t.Fatalf("insert split expense: %v", err)
				}
				insertSplit(t, db, expenseOneID, groupOneID, bobID, 100)
			},
		},
		{
			name: "repayment sender",
			seed: func(t *testing.T, db *sql.DB, createdAt time.Time) {
				if _, err := insertRepayment(
					db,
					repaymentOneID,
					groupOneID,
					bobID,
					aliceID,
					aliceID,
					100,
					"USD",
					&recordTimes{createdAt: createdAt},
				); err != nil {
					t.Fatalf("insert sender repayment: %v", err)
				}
			},
		},
		{
			name: "repayment recipient",
			seed: func(t *testing.T, db *sql.DB, createdAt time.Time) {
				if _, err := insertRepayment(
					db,
					repaymentOneID,
					groupOneID,
					aliceID,
					bobID,
					aliceID,
					100,
					"USD",
					&recordTimes{createdAt: createdAt},
				); err != nil {
					t.Fatalf("insert recipient repayment: %v", err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			createdAt := groupOwnerTestFixture(t, db)
			test.seed(t, db, createdAt)

			err := store.RemoveMember(ctx, groups.RemoveMemberInput{
				ActorID:   aliceID,
				GroupID:   groupOneID,
				UserID:    bobID,
				RemovedAt: createdAt.Add(time.Hour),
			})
			if !errors.Is(err, groups.ErrMemberInUse) {
				t.Errorf("error = %v, want ErrMemberInUse", err)
			}
			groupOwnerTestAssertActiveMembership(t, db, groupOneID, bobID)
		})
	}
}

func TestGroupOwnerStoreRemoveMemberIgnoresNonCurrentReferences(t *testing.T) {
	db := openIntegrationDatabase(t)
	store := New(db)
	ctx := context.Background()

	t.Run("creator-only references", func(t *testing.T) {
		createdAt := groupOwnerTestFixture(t, db)
		if _, err := insertExpense(
			db,
			expenseOneID,
			groupOneID,
			aliceID,
			bobID,
			100,
			"USD",
			&recordTimes{createdAt: createdAt},
		); err != nil {
			t.Fatalf("insert creator-only expense: %v", err)
		}
		insertSplit(t, db, expenseOneID, groupOneID, aliceID, 100)
		if _, err := insertRepayment(
			db,
			repaymentOneID,
			groupOneID,
			aliceID,
			carolID,
			bobID,
			100,
			"USD",
			&recordTimes{createdAt: createdAt},
		); err != nil {
			t.Fatalf("insert creator-only repayment: %v", err)
		}

		if err := store.RemoveMember(ctx, groups.RemoveMemberInput{
			ActorID:   aliceID,
			GroupID:   groupOneID,
			UserID:    bobID,
			RemovedAt: createdAt.Add(time.Hour),
		}); err != nil {
			t.Fatalf("RemoveMember with creator-only references: %v", err)
		}
	})

	t.Run("soft-deleted participation", func(t *testing.T) {
		createdAt := groupOwnerTestFixture(t, db)
		deletedAt := createdAt.Add(time.Hour)
		times := &recordTimes{
			createdAt: createdAt,
			deletedAt: &deletedAt,
		}
		if _, err := insertExpense(
			db,
			expenseOneID,
			groupOneID,
			bobID,
			aliceID,
			100,
			"USD",
			times,
		); err != nil {
			t.Fatalf("insert deleted expense: %v", err)
		}
		insertSplit(t, db, expenseOneID, groupOneID, bobID, 100)
		if _, err := insertRepayment(
			db,
			repaymentOneID,
			groupOneID,
			bobID,
			aliceID,
			aliceID,
			100,
			"USD",
			times,
		); err != nil {
			t.Fatalf("insert deleted repayment: %v", err)
		}

		if err := store.RemoveMember(ctx, groups.RemoveMemberInput{
			ActorID:   aliceID,
			GroupID:   groupOneID,
			UserID:    bobID,
			RemovedAt: deletedAt.Add(time.Hour),
		}); err != nil {
			t.Fatalf("RemoveMember with deleted references: %v", err)
		}
	})
}

func TestGroupOwnerStoreRenameRollsBackSummaryFailure(t *testing.T) {
	db := openIntegrationDatabase(t)
	createdAt := groupOwnerTestFixture(t, db)

	if _, err := db.Exec(`
		CREATE FUNCTION group_owner_test_drop_membership()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
			DELETE FROM group_memberships
			WHERE group_id = NEW.id
				AND user_id = NEW.owner_user_id;
			RETURN NEW;
		END;
		$$
	`); err != nil {
		t.Fatalf("create rollback test function: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER group_owner_test_drop_membership
		AFTER UPDATE OF name ON groups
		FOR EACH ROW
		EXECUTE FUNCTION group_owner_test_drop_membership()
	`); err != nil {
		t.Fatalf("create rollback test trigger: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`
			DROP TRIGGER IF EXISTS group_owner_test_drop_membership ON groups
		`); err != nil {
			t.Errorf("drop rollback test trigger: %v", err)
		}
		if _, err := db.Exec(`
			DROP FUNCTION IF EXISTS group_owner_test_drop_membership()
		`); err != nil {
			t.Errorf("drop rollback test function: %v", err)
		}
	})

	_, err := New(db).RenameGroup(
		context.Background(),
		groups.RenameGroupInput{
			ActorID:   aliceID,
			GroupID:   groupOneID,
			Name:      "Must Roll Back",
			UpdatedAt: createdAt.Add(time.Hour),
		},
	)
	if err == nil {
		t.Fatal("RenameGroup succeeded, want summary failure")
	}
	if errors.Is(err, groups.ErrNotFound) ||
		errors.Is(err, groups.ErrForbidden) {
		t.Errorf("error = %v, want internal summary failure", err)
	}

	var name string
	var updatedAt time.Time
	var ownerActive bool
	if err := db.QueryRow(`
		SELECT
			g.name,
			g.updated_at,
			EXISTS (
				SELECT 1
				FROM group_memberships membership
				WHERE membership.group_id = g.id
					AND membership.user_id = g.owner_user_id
					AND membership.removed_at IS NULL
			)
		FROM groups g
		WHERE g.id = $1
	`, groupOneID).Scan(&name, &updatedAt, &ownerActive); err != nil {
		t.Fatalf("query rolled-back rename: %v", err)
	}
	if name != "Owner Group" || !updatedAt.Equal(createdAt) || !ownerActive {
		t.Errorf(
			"rolled-back state = name %q updated %v owner active %v",
			name,
			updatedAt,
			ownerActive,
		)
	}
}

func TestGroupOwnerStoreConcurrentDissolveHidesWaitingRename(t *testing.T) {
	db := openIntegrationDatabase(t)
	createdAt := groupOwnerTestFixture(t, db)

	blocker, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		t.Fatalf("begin blocker transaction: %v", err)
	}
	defer func() {
		_ = blocker.Rollback()
	}()
	var lockedID string
	if err := blocker.QueryRow(`
		SELECT id::text
		FROM groups
		WHERE id = $1
		FOR UPDATE
	`, groupOneID).Scan(&lockedID); err != nil {
		t.Fatalf("lock group in blocker: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := New(db).RenameGroup(ctx, groups.RenameGroupInput{
			ActorID:   aliceID,
			GroupID:   groupOneID,
			Name:      "Concurrent Rename",
			UpdatedAt: createdAt.Add(2 * time.Hour),
		})
		result <- err
	}()
	groupOwnerTestWaitForBlockedTransaction(t, db)

	dissolvedAt := createdAt.Add(time.Hour)
	if _, err := blocker.Exec(`
		UPDATE groups
		SET dissolved_at = $2,
			updated_at = $2
		WHERE id = $1
	`, groupOneID, dissolvedAt); err != nil {
		t.Fatalf("dissolve blocked group: %v", err)
	}
	if err := blocker.Commit(); err != nil {
		t.Fatalf("commit concurrent dissolution: %v", err)
	}

	if err := <-result; !errors.Is(err, groups.ErrNotFound) {
		t.Errorf("waiting rename error = %v, want ErrNotFound", err)
	}
	var name string
	if err := db.QueryRow(`
		SELECT name
		FROM groups
		WHERE id = $1
	`, groupOneID).Scan(&name); err != nil {
		t.Fatalf("query concurrently dissolved group: %v", err)
	}
	if name != "Owner Group" {
		t.Errorf("group name = %q, want unchanged", name)
	}
}

func TestGroupOwnerStoreConcurrentReferenceBlocksRemoval(t *testing.T) {
	db := openIntegrationDatabase(t)
	createdAt := groupOwnerTestFixture(t, db)

	blocker, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		t.Fatalf("begin blocker transaction: %v", err)
	}
	defer func() {
		_ = blocker.Rollback()
	}()
	var lockedID string
	if err := blocker.QueryRow(`
		SELECT id::text
		FROM groups
		WHERE id = $1
		FOR UPDATE
	`, groupOneID).Scan(&lockedID); err != nil {
		t.Fatalf("lock group in blocker: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- New(db).RemoveMember(ctx, groups.RemoveMemberInput{
			ActorID:   aliceID,
			GroupID:   groupOneID,
			UserID:    bobID,
			RemovedAt: createdAt.Add(2 * time.Hour),
		})
	}()
	groupOwnerTestWaitForBlockedTransaction(t, db)

	if _, err := blocker.Exec(`
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
		VALUES ($1, $2, $3, 'Concurrent expense', 100, 'USD', '2026-07-31', $4, $5, $5)
	`, expenseOneID, groupOneID, bobID, aliceID, createdAt.Add(time.Hour)); err != nil {
		t.Fatalf("insert concurrent expense: %v", err)
	}
	if _, err := blocker.Exec(`
		INSERT INTO expense_splits (
			expense_id,
			group_id,
			user_id,
			amount_cents
		)
		VALUES ($1, $2, $3, 100)
	`, expenseOneID, groupOneID, aliceID); err != nil {
		t.Fatalf("insert concurrent split: %v", err)
	}
	if err := blocker.Commit(); err != nil {
		t.Fatalf("commit concurrent expense: %v", err)
	}

	if err := <-result; !errors.Is(err, groups.ErrMemberInUse) {
		t.Errorf("waiting removal error = %v, want ErrMemberInUse", err)
	}
	groupOwnerTestAssertActiveMembership(t, db, groupOneID, bobID)
}

func groupOwnerTestFixture(t *testing.T, db *sql.DB) time.Time {
	t.Helper()

	resetIntegrationDatabase(t, db)
	groupTestInsertUsers(t, db)
	createdAt := time.Date(2026, time.July, 31, 10, 0, 0, 0, time.UTC)
	groupTestInsertGroup(
		t,
		db,
		groupOneID,
		aliceID,
		"Owner Group",
		"OWNERGROUP23",
		createdAt,
		nil,
	)
	groupTestInsertMembership(t, db, groupOneID, aliceID, createdAt, nil)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		bobID,
		createdAt.Add(time.Minute),
		nil,
	)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		carolID,
		createdAt.Add(2*time.Minute),
		nil,
	)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		daveID,
		createdAt.Add(3*time.Minute),
		nil,
	)
	removedAt := createdAt.Add(5 * time.Minute)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		erinID,
		createdAt.Add(4*time.Minute),
		&removedAt,
	)
	return createdAt
}

func groupOwnerTestAssertActiveMembership(
	t *testing.T,
	db *sql.DB,
	groupID string,
	userID string,
) {
	t.Helper()

	var removedAt *time.Time
	if err := db.QueryRow(`
		SELECT removed_at
		FROM group_memberships
		WHERE group_id = $1
			AND user_id = $2
	`, groupID, userID).Scan(&removedAt); err != nil {
		t.Fatalf("query active membership: %v", err)
	}
	if removedAt != nil {
		t.Errorf("membership removed_at = %v, want NULL", removedAt)
	}
}

func groupOwnerTestWaitForBlockedTransaction(t *testing.T, db *sql.DB) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := db.QueryRow(`
			SELECT EXISTS (
				SELECT 1
				FROM pg_catalog.pg_locks
				WHERE granted = false
			)
		`).Scan(&waiting); err != nil {
			t.Fatalf("query waiting locks: %v", err)
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("transaction did not block on the expected database lock")
}
