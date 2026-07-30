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

const (
	groupThreeID = "10000000-0000-4000-8000-000000000003"
	groupFourID  = "10000000-0000-4000-8000-000000000004"
	groupFiveID  = "10000000-0000-4000-8000-000000000005"
	groupSixID   = "10000000-0000-4000-8000-000000000006"
	frankID      = "00000000-0000-4000-8000-000000000006"
)

func TestGroupStoreCreateGroup(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	insertUser(t, db, aliceID, "alice@example.com", "Alice")

	store := New(db)
	workflowTime := time.Date(
		2026,
		time.July,
		30,
		12,
		34,
		56,
		123456000,
		time.FixedZone("test", -7*60*60),
	)
	input := groups.NewGroup{
		ID:          groupOneID,
		Name:        "Lake Trip",
		JoinCode:    "LAKETRIP2345",
		OwnerUserID: aliceID,
		CreatedAt:   workflowTime,
		UpdatedAt:   workflowTime,
	}

	got, err := store.CreateGroup(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	want := groups.Group{
		ID:              groupOneID,
		Name:            "Lake Trip",
		OwnerUserID:     aliceID,
		MemberCount:     1,
		CurrentUserRole: groups.RoleOwner,
		CreatedAt:       workflowTime.UTC(),
		UpdatedAt:       workflowTime.UTC(),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("group = %#v, want %#v", got, want)
	}

	var joinCode string
	var joinedAt time.Time
	if err := db.QueryRow(`
		SELECT g.join_code, membership.joined_at
		FROM groups g
		JOIN group_memberships membership
			ON membership.group_id = g.id
			AND membership.user_id = g.owner_user_id
		WHERE g.id = $1
	`, groupOneID).Scan(&joinCode, &joinedAt); err != nil {
		t.Fatalf("query persisted group: %v", err)
	}
	if joinCode != input.JoinCode {
		t.Errorf("join code = %q, want %q", joinCode, input.JoinCode)
	}
	if !joinedAt.Equal(input.CreatedAt) {
		t.Errorf("owner joined at = %v, want %v", joinedAt, input.CreatedAt)
	}
}

func TestGroupStoreCreateGroupRollsBackOwnerMembershipFailure(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	insertUser(t, db, aliceID, "alice@example.com", "Alice")

	const testConstraint = "group_memberships_test_reject_owner"
	if _, err := db.Exec(`
		ALTER TABLE group_memberships
		ADD CONSTRAINT group_memberships_test_reject_owner
		CHECK (user_id <> '00000000-0000-4000-8000-000000000001'::uuid)
	`); err != nil {
		t.Fatalf("add test constraint: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`
			ALTER TABLE group_memberships
			DROP CONSTRAINT IF EXISTS group_memberships_test_reject_owner
		`); err != nil {
			t.Errorf("drop test constraint %s: %v", testConstraint, err)
		}
	})

	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	_, err := New(db).CreateGroup(context.Background(), groups.NewGroup{
		ID:          groupOneID,
		Name:        "Rolled Back Group",
		JoinCode:    "ROLLBACK2345",
		OwnerUserID: aliceID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err == nil {
		t.Fatal("CreateGroup succeeded, want owner membership failure")
	}
	if errors.Is(err, groups.ErrJoinCodeCollision) ||
		errors.Is(err, groups.ErrNotFound) {
		t.Errorf("error = %v, want internal database error", err)
	}

	var exists bool
	if err := db.QueryRow(`
		SELECT EXISTS (SELECT 1 FROM groups WHERE id = $1)
	`, groupOneID).Scan(&exists); err != nil {
		t.Fatalf("query rolled-back group: %v", err)
	}
	if exists {
		t.Error("group insert remained after owner membership failure")
	}
}

func TestGroupStoreCreateGroupTranslatesJoinCodeCollision(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	insertUser(t, db, aliceID, "alice@example.com", "Alice")
	insertUser(t, db, bobID, "bob@example.com", "Bob")

	store := New(db)
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	first := groups.NewGroup{
		ID:          groupOneID,
		Name:        "First Group",
		JoinCode:    "DUPLICATE234",
		OwnerUserID: aliceID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if _, err := store.CreateGroup(context.Background(), first); err != nil {
		t.Fatalf("create first group: %v", err)
	}

	duplicate := groups.NewGroup{
		ID:          groupTwoID,
		Name:        "Second Group",
		JoinCode:    first.JoinCode,
		OwnerUserID: bobID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := store.CreateGroup(context.Background(), duplicate)
	if !errors.Is(err, groups.ErrJoinCodeCollision) {
		t.Fatalf("duplicate error = %v, want ErrJoinCodeCollision", err)
	}

	var groupExists bool
	var membershipExists bool
	if err := db.QueryRow(`
		SELECT
			EXISTS (SELECT 1 FROM groups WHERE id = $1),
			EXISTS (
				SELECT 1
				FROM group_memberships
				WHERE group_id = $1
			)
	`, groupTwoID).Scan(&groupExists, &membershipExists); err != nil {
		t.Fatalf("query duplicate attempt: %v", err)
	}
	if groupExists || membershipExists {
		t.Errorf(
			"duplicate attempt persisted group=%v membership=%v",
			groupExists,
			membershipExists,
		)
	}
}

func TestGroupStoreListGroupsIsolationAndOrdering(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	groupTestInsertUsers(t, db)

	baseTime := time.Date(2026, time.July, 30, 10, 0, 0, 0, time.UTC)
	groupTestInsertGroup(
		t, db, groupOneID, aliceID, "Old Owner Group", "LISTGROUP234", baseTime, nil,
	)
	groupTestInsertMembership(t, db, groupOneID, aliceID, baseTime, nil)
	groupTestInsertMembership(t, db, groupOneID, bobID, baseTime, nil)

	newerTime := baseTime.Add(2 * time.Hour)
	groupTestInsertGroup(
		t, db, groupTwoID, bobID, "New Member Group", "LISTGROUP235", newerTime, nil,
	)
	groupTestInsertMembership(t, db, groupTwoID, bobID, newerTime, nil)
	groupTestInsertMembership(t, db, groupTwoID, aliceID, newerTime, nil)

	groupTestInsertGroup(
		t, db, groupThreeID, aliceID, "New Owner Group", "LISTGROUP236", newerTime, nil,
	)
	groupTestInsertMembership(t, db, groupThreeID, aliceID, newerTime, nil)

	groupTestInsertGroup(
		t, db, groupFourID, bobID, "Private Group", "LISTGROUP237", newerTime, nil,
	)
	groupTestInsertMembership(t, db, groupFourID, bobID, newerTime, nil)

	removedAt := newerTime.Add(time.Hour)
	groupTestInsertGroup(
		t, db, groupFiveID, bobID, "Removed Group", "LISTGROUP238", newerTime, nil,
	)
	groupTestInsertMembership(t, db, groupFiveID, bobID, newerTime, nil)
	groupTestInsertMembership(t, db, groupFiveID, aliceID, newerTime, &removedAt)

	dissolvedAt := newerTime.Add(time.Hour)
	groupTestInsertGroup(
		t,
		db,
		groupSixID,
		aliceID,
		"Dissolved Group",
		"LISTGROUP239",
		newerTime,
		&dissolvedAt,
	)
	groupTestInsertMembership(t, db, groupSixID, aliceID, newerTime, nil)

	got, err := New(db).ListGroups(context.Background(), aliceID)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	want := []groups.Group{
		{
			ID:              groupThreeID,
			Name:            "New Owner Group",
			OwnerUserID:     aliceID,
			MemberCount:     1,
			CurrentUserRole: groups.RoleOwner,
			CreatedAt:       newerTime,
			UpdatedAt:       newerTime,
		},
		{
			ID:              groupTwoID,
			Name:            "New Member Group",
			OwnerUserID:     bobID,
			MemberCount:     2,
			CurrentUserRole: groups.RoleMember,
			CreatedAt:       newerTime,
			UpdatedAt:       newerTime,
		},
		{
			ID:              groupOneID,
			Name:            "Old Owner Group",
			OwnerUserID:     aliceID,
			MemberCount:     2,
			CurrentUserRole: groups.RoleOwner,
			CreatedAt:       baseTime,
			UpdatedAt:       baseTime,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("groups = %#v, want %#v", got, want)
	}

	empty, err := New(db).ListGroups(context.Background(), frankID)
	if err != nil {
		t.Fatalf("ListGroups without memberships: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("empty groups = %#v, want non-nil empty slice", empty)
	}
}

func TestGroupStoreJoinGroupLifecycle(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)
	groupTestInsertUsers(t, db)

	createdAt := time.Date(2026, time.July, 30, 10, 0, 0, 0, time.UTC)
	groupTestInsertGroup(
		t, db, groupOneID, aliceID, "Joinable Group", "JOINGROUP234", createdAt, nil,
	)
	groupTestInsertMembership(t, db, groupOneID, aliceID, createdAt, nil)
	store := New(db)

	firstJoin := createdAt.Add(time.Hour)
	got, err := store.JoinGroup(context.Background(), groups.JoinGroupInput{
		UserID:   bobID,
		JoinCode: "JOINGROUP234",
		JoinedAt: firstJoin,
	})
	if err != nil {
		t.Fatalf("first JoinGroup: %v", err)
	}
	groupTestAssertJoinSummary(t, got, 2)
	groupTestAssertMembership(t, db, groupOneID, bobID, firstJoin, nil)

	repeatedJoin := firstJoin.Add(time.Hour)
	got, err = store.JoinGroup(context.Background(), groups.JoinGroupInput{
		UserID:   bobID,
		JoinCode: "JOINGROUP234",
		JoinedAt: repeatedJoin,
	})
	if err != nil {
		t.Fatalf("repeated JoinGroup: %v", err)
	}
	groupTestAssertJoinSummary(t, got, 2)
	groupTestAssertMembership(t, db, groupOneID, bobID, firstJoin, nil)

	removedAt := repeatedJoin.Add(time.Hour)
	if _, err := db.Exec(`
		UPDATE group_memberships
		SET removed_at = $3
		WHERE group_id = $1
			AND user_id = $2
	`, groupOneID, bobID, removedAt); err != nil {
		t.Fatalf("remove membership: %v", err)
	}

	reactivatedAt := removedAt.Add(time.Hour)
	got, err = store.JoinGroup(context.Background(), groups.JoinGroupInput{
		UserID:   bobID,
		JoinCode: "JOINGROUP234",
		JoinedAt: reactivatedAt,
	})
	if err != nil {
		t.Fatalf("reactivate JoinGroup: %v", err)
	}
	groupTestAssertJoinSummary(t, got, 2)
	groupTestAssertMembership(t, db, groupOneID, bobID, reactivatedAt, nil)

	if _, err := store.JoinGroup(
		context.Background(),
		groups.JoinGroupInput{
			UserID:   carolID,
			JoinCode: "MISSINGCODE2",
			JoinedAt: reactivatedAt,
		},
	); !errors.Is(err, groups.ErrNotFound) {
		t.Errorf("missing-code error = %v, want ErrNotFound", err)
	}

	dissolvedAt := reactivatedAt.Add(time.Hour)
	if _, err := db.Exec(`
		UPDATE groups
		SET dissolved_at = $2,
			updated_at = $2
		WHERE id = $1
	`, groupOneID, dissolvedAt); err != nil {
		t.Fatalf("dissolve group: %v", err)
	}
	if _, err := store.JoinGroup(
		context.Background(),
		groups.JoinGroupInput{
			UserID:   carolID,
			JoinCode: "JOINGROUP234",
			JoinedAt: dissolvedAt.Add(time.Hour),
		},
	); !errors.Is(err, groups.ErrNotFound) {
		t.Errorf("dissolved-group error = %v, want ErrNotFound", err)
	}
}

func TestGroupStoreGetGroupVisibilityAndMemberOrdering(t *testing.T) {
	db := openIntegrationDatabase(t)
	resetIntegrationDatabase(t, db)

	insertUser(t, db, aliceID, "owner@example.com", "Zulu Owner")
	insertUser(t, db, bobID, "bob@example.com", "alice")
	insertUser(t, db, carolID, "carol@example.com", "Alice")
	insertUser(t, db, daveID, "dave@example.com", "beta")
	insertUser(t, db, erinID, "erin@example.com", "Removed")
	insertUser(t, db, frankID, "frank@example.com", "Nonmember")

	createdAt := time.Date(2026, time.July, 30, 10, 0, 0, 0, time.UTC)
	groupTestInsertGroup(
		t, db, groupOneID, aliceID, "Detailed Group", "DETAILGROUP2", createdAt, nil,
	)
	groupTestInsertMembership(t, db, groupOneID, aliceID, createdAt, nil)
	groupTestInsertMembership(
		t, db, groupOneID, bobID, createdAt.Add(time.Minute), nil,
	)
	groupTestInsertMembership(
		t, db, groupOneID, carolID, createdAt.Add(2*time.Minute), nil,
	)
	groupTestInsertMembership(
		t, db, groupOneID, daveID, createdAt.Add(3*time.Minute), nil,
	)
	removedAt := createdAt.Add(5 * time.Minute)
	groupTestInsertMembership(
		t, db, groupOneID, erinID, createdAt.Add(4*time.Minute), &removedAt,
	)

	store := New(db)
	got, err := store.GetGroup(context.Background(), bobID, groupOneID)
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	wantGroup := groups.Group{
		ID:              groupOneID,
		Name:            "Detailed Group",
		OwnerUserID:     aliceID,
		MemberCount:     4,
		CurrentUserRole: groups.RoleMember,
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	}
	if !reflect.DeepEqual(got.Group, wantGroup) {
		t.Errorf("group = %#v, want %#v", got.Group, wantGroup)
	}
	wantMembers := []groups.Member{
		{
			UserID:      aliceID,
			Email:       "owner@example.com",
			DisplayName: "Zulu Owner",
			Role:        groups.RoleOwner,
			JoinedAt:    createdAt,
		},
		{
			UserID:      bobID,
			Email:       "bob@example.com",
			DisplayName: "alice",
			Role:        groups.RoleMember,
			JoinedAt:    createdAt.Add(time.Minute),
		},
		{
			UserID:      carolID,
			Email:       "carol@example.com",
			DisplayName: "Alice",
			Role:        groups.RoleMember,
			JoinedAt:    createdAt.Add(2 * time.Minute),
		},
		{
			UserID:      daveID,
			Email:       "dave@example.com",
			DisplayName: "beta",
			Role:        groups.RoleMember,
			JoinedAt:    createdAt.Add(3 * time.Minute),
		},
	}
	if !reflect.DeepEqual(got.Members, wantMembers) {
		t.Errorf("members = %#v, want %#v", got.Members, wantMembers)
	}

	ownerDetail, err := store.GetGroup(context.Background(), aliceID, groupOneID)
	if err != nil {
		t.Fatalf("owner GetGroup: %v", err)
	}
	if ownerDetail.Group.CurrentUserRole != groups.RoleOwner {
		t.Errorf(
			"owner role = %q, want %q",
			ownerDetail.Group.CurrentUserRole,
			groups.RoleOwner,
		)
	}

	for name, actorID := range map[string]string{
		"nonmember": frankID,
		"removed":   erinID,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := store.GetGroup(
				context.Background(),
				actorID,
				groupOneID,
			); !errors.Is(err, groups.ErrNotFound) {
				t.Errorf("error = %v, want ErrNotFound", err)
			}
		})
	}

	if _, err := store.GetGroup(
		context.Background(),
		aliceID,
		groupTwoID,
	); !errors.Is(err, groups.ErrNotFound) {
		t.Errorf("missing-group error = %v, want ErrNotFound", err)
	}

	dissolvedAt := createdAt.Add(time.Hour)
	if _, err := db.Exec(`
		UPDATE groups
		SET dissolved_at = $2,
			updated_at = $2
		WHERE id = $1
	`, groupOneID, dissolvedAt); err != nil {
		t.Fatalf("dissolve group: %v", err)
	}
	if _, err := store.GetGroup(
		context.Background(),
		aliceID,
		groupOneID,
	); !errors.Is(err, groups.ErrNotFound) {
		t.Errorf("dissolved-group error = %v, want ErrNotFound", err)
	}
}

func groupTestInsertUsers(t *testing.T, db *sql.DB) {
	t.Helper()

	insertUser(t, db, aliceID, "alice@example.com", "Alice")
	insertUser(t, db, bobID, "bob@example.com", "Bob")
	insertUser(t, db, carolID, "carol@example.com", "Carol")
	insertUser(t, db, daveID, "dave@example.com", "Dave")
	insertUser(t, db, erinID, "erin@example.com", "Erin")
	insertUser(t, db, frankID, "frank@example.com", "Frank")
}

func groupTestInsertGroup(
	t *testing.T,
	db *sql.DB,
	id string,
	ownerID string,
	name string,
	joinCode string,
	timestamp time.Time,
	dissolvedAt *time.Time,
) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO groups (
			id,
			name,
			join_code,
			owner_user_id,
			created_at,
			updated_at,
			dissolved_at
		)
		VALUES ($1, $2, $3, $4, $5, $5, $6)
	`, id, name, joinCode, ownerID, timestamp, dissolvedAt); err != nil {
		t.Fatalf("insert group %s: %v", id, err)
	}
}

func groupTestInsertMembership(
	t *testing.T,
	db *sql.DB,
	groupID string,
	userID string,
	joinedAt time.Time,
	removedAt *time.Time,
) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO group_memberships (
			group_id,
			user_id,
			joined_at,
			removed_at
		)
		VALUES ($1, $2, $3, $4)
	`, groupID, userID, joinedAt, removedAt); err != nil {
		t.Fatalf("insert membership (%s, %s): %v", groupID, userID, err)
	}
}

func groupTestAssertJoinSummary(
	t *testing.T,
	group groups.Group,
	memberCount int64,
) {
	t.Helper()

	if group.ID != groupOneID ||
		group.Name != "Joinable Group" ||
		group.OwnerUserID != aliceID ||
		group.MemberCount != memberCount ||
		group.CurrentUserRole != groups.RoleMember {
		t.Errorf("joined group summary = %+v", group)
	}
}

func groupTestAssertMembership(
	t *testing.T,
	db *sql.DB,
	groupID string,
	userID string,
	wantJoinedAt time.Time,
	wantRemovedAt *time.Time,
) {
	t.Helper()

	var joinedAt time.Time
	var removedAt *time.Time
	if err := db.QueryRow(`
		SELECT joined_at, removed_at
		FROM group_memberships
		WHERE group_id = $1
			AND user_id = $2
	`, groupID, userID).Scan(&joinedAt, &removedAt); err != nil {
		t.Fatalf("query membership: %v", err)
	}
	if !joinedAt.Equal(wantJoinedAt) {
		t.Errorf("joined_at = %v, want %v", joinedAt, wantJoinedAt)
	}
	switch {
	case removedAt == nil && wantRemovedAt == nil:
	case removedAt == nil || wantRemovedAt == nil:
		t.Errorf("removed_at = %v, want %v", removedAt, wantRemovedAt)
	case !removedAt.Equal(*wantRemovedAt):
		t.Errorf("removed_at = %v, want %v", removedAt, wantRemovedAt)
	}
}
