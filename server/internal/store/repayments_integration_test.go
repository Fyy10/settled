//go:build integration

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/groups"
	"github.com/Fyy10/settled/server/internal/repayments"
)

const (
	repaymentFourID         = "30000000-0000-4000-8000-000000000004"
	repaymentMissingGroupID = "10000000-0000-4000-8000-000000000099"
)

func TestRepaymentStoreCreateGetAndNullableNotes(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := repaymentStoreTestFixture(t, db)
	store := New(db)

	input := repaymentStoreTestCreateInput(baseTime)
	input.CreatedAt = time.Date(
		2026,
		time.August,
		1,
		12,
		34,
		56,
		123456000,
		time.FixedZone("workflow", -7*60*60),
	)
	input.UpdatedAt = input.CreatedAt
	created, err := store.CreateRepayment(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateRepayment: %v", err)
	}
	wantCreated := repaymentStoreTestRepayment(input)
	if !reflect.DeepEqual(created, wantCreated) {
		t.Errorf("created repayment = %#v, want %#v", created, wantCreated)
	}
	if created.Note == input.Note {
		t.Error("created repayment note aliases the input pointer")
	}

	got, err := store.GetRepayment(
		context.Background(),
		carolID,
		groupOneID,
		repaymentOneID,
	)
	if err != nil {
		t.Fatalf("GetRepayment: %v", err)
	}
	if !reflect.DeepEqual(got, wantCreated) {
		t.Errorf("fetched repayment = %#v, want %#v", got, wantCreated)
	}

	var creatorID string
	var persistedNote *string
	if err := db.QueryRow(`
		SELECT created_by_user_id::text, note
		FROM repayments
		WHERE id = $1
			AND group_id = $2
	`, repaymentOneID, groupOneID).Scan(
		&creatorID,
		&persistedNote,
	); err != nil {
		t.Fatalf("query persisted repayment: %v", err)
	}
	if creatorID != input.ActorID {
		t.Errorf("creator ID = %q, want actor %q", creatorID, input.ActorID)
	}
	if creatorID == input.FromUserID {
		t.Errorf("creator ID unexpectedly derived from sender: %q", creatorID)
	}
	if !reflect.DeepEqual(persistedNote, input.Note) {
		t.Errorf("persisted note = %#v, want %#v", persistedNote, input.Note)
	}

	blankNote := " \t\n "
	blankInput := repaymentStoreTestCreateInput(baseTime)
	blankInput.ID = repaymentTwoID
	blankInput.Note = &blankNote
	blankInput.CreatedAt = input.CreatedAt.Add(time.Minute)
	blankInput.UpdatedAt = blankInput.CreatedAt
	blank, err := store.CreateRepayment(context.Background(), blankInput)
	if err != nil {
		t.Fatalf("CreateRepayment with blank note: %v", err)
	}
	if blank.Note != nil {
		t.Errorf("blank note = %#v, want nil", blank.Note)
	}
	var noteIsNull bool
	if err := db.QueryRow(`
		SELECT note IS NULL
		FROM repayments
		WHERE id = $1
	`, blankInput.ID).Scan(&noteIsNull); err != nil {
		t.Fatalf("query blank repayment note: %v", err)
	}
	if !noteIsNull {
		t.Error("blank repayment note was not persisted as SQL NULL")
	}
}

func TestRepaymentStoreCreateRequiresCurrentVisibleMemberships(t *testing.T) {
	db := openIntegrationDatabase(t)
	store := New(db)

	tests := []struct {
		name      string
		mutate    func(*repayments.CreateInput)
		dissolved bool
	}{
		{
			name: "nonmember actor",
			mutate: func(input *repayments.CreateInput) {
				input.ActorID = frankID
			},
		},
		{
			name: "removed actor",
			mutate: func(input *repayments.CreateInput) {
				input.ActorID = erinID
			},
		},
		{
			name: "removed sender",
			mutate: func(input *repayments.CreateInput) {
				input.FromUserID = erinID
			},
		},
		{
			name: "missing sender",
			mutate: func(input *repayments.CreateInput) {
				input.FromUserID = frankID
			},
		},
		{
			name: "removed recipient",
			mutate: func(input *repayments.CreateInput) {
				input.ToUserID = erinID
			},
		},
		{
			name: "missing recipient",
			mutate: func(input *repayments.CreateInput) {
				input.ToUserID = frankID
			},
		},
		{
			name:      "dissolved group",
			dissolved: true,
			mutate:    func(*repayments.CreateInput) {},
		},
		{
			name: "missing group",
			mutate: func(input *repayments.CreateInput) {
				input.GroupID = repaymentMissingGroupID
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseTime := repaymentStoreTestFixture(t, db)
			input := repaymentStoreTestCreateInput(baseTime)
			test.mutate(&input)
			if test.dissolved {
				if _, err := db.Exec(`
					UPDATE groups
					SET dissolved_at = $2,
						updated_at = $2
					WHERE id = $1
				`, groupOneID, baseTime.Add(time.Hour)); err != nil {
					t.Fatalf("dissolve fixture group: %v", err)
				}
			}

			_, err := store.CreateRepayment(context.Background(), input)
			if !errors.Is(err, repayments.ErrNotFound) {
				t.Errorf("error = %v, want ErrNotFound", err)
			}
			repaymentStoreTestAssertRepaymentExists(
				t,
				db,
				input.ID,
				false,
			)
		})
	}
}

func TestRepaymentStoreRejectsInvalidPrevalidatedWrites(t *testing.T) {
	db := openIntegrationDatabase(t)
	store := New(db)

	tests := []struct {
		name   string
		mutate func(*repayments.CreateInput)
	}{
		{
			name: "same sender and recipient",
			mutate: func(input *repayments.CreateInput) {
				input.ToUserID = input.FromUserID
			},
		},
		{
			name: "zero amount",
			mutate: func(input *repayments.CreateInput) {
				input.AmountCents = 0
			},
		},
		{
			name: "negative amount",
			mutate: func(input *repayments.CreateInput) {
				input.AmountCents = -1
			},
		},
		{
			name: "non USD currency",
			mutate: func(input *repayments.CreateInput) {
				input.Currency = "EUR"
			},
		},
		{
			name: "nonmidnight date",
			mutate: func(input *repayments.CreateInput) {
				input.RepaymentDate = input.RepaymentDate.Add(time.Nanosecond)
			},
		},
		{
			name: "non UTC date",
			mutate: func(input *repayments.CreateInput) {
				input.RepaymentDate = time.Date(
					2026,
					time.August,
					1,
					0,
					0,
					0,
					0,
					time.FixedZone("workflow", -7*60*60),
				)
			},
		},
		{
			name: "note too long",
			mutate: func(input *repayments.CreateInput) {
				note := strings.Repeat("界", 241)
				input.Note = &note
			},
		},
		{
			name: "note with embedded control",
			mutate: func(input *repayments.CreateInput) {
				note := "paid\ncash"
				input.Note = &note
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseTime := repaymentStoreTestFixture(t, db)
			input := repaymentStoreTestCreateInput(baseTime)
			test.mutate(&input)

			_, err := store.CreateRepayment(context.Background(), input)
			if !errors.Is(err, errRepaymentInvariant) {
				t.Errorf(
					"error = %v, want errRepaymentInvariant",
					err,
				)
			}
			if errors.Is(err, repayments.ErrNotFound) {
				t.Errorf("invariant error is miscategorized as not found: %v", err)
			}
			repaymentStoreTestAssertRepaymentExists(
				t,
				db,
				input.ID,
				false,
			)
		})
	}
}

func TestRepaymentStoreCreateRollsBackDatabaseFailure(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := repaymentStoreTestFixture(t, db)
	repaymentStoreTestInstallRejectInsertTrigger(t, db)

	input := repaymentStoreTestCreateInput(baseTime)
	if _, err := New(db).CreateRepayment(
		context.Background(),
		input,
	); err == nil {
		t.Fatal("CreateRepayment succeeded, want forced insert failure")
	}
	repaymentStoreTestAssertRepaymentExists(t, db, input.ID, false)
}

func TestRepaymentStoreCollaborativeReplacementPreservesMetadata(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := repaymentStoreTestFixture(t, db)
	store := New(db)

	originalInput := repaymentStoreTestCreateInput(baseTime)
	original, err := store.CreateRepayment(
		context.Background(),
		originalInput,
	)
	if err != nil {
		t.Fatalf("CreateRepayment: %v", err)
	}

	replacedAt := originalInput.CreatedAt.Add(2 * time.Hour)
	replacement := repayments.ReplaceInput{
		ActorID:       carolID,
		GroupID:       groupOneID,
		RepaymentID:   repaymentOneID,
		FromUserID:    bobID,
		ToUserID:      aliceID,
		AmountCents:   1250,
		Currency:      "USD",
		Note:          nil,
		RepaymentDate: originalInput.RepaymentDate.AddDate(0, 0, 1),
		UpdatedAt:     replacedAt,
	}
	replaced, err := store.ReplaceRepayment(
		context.Background(),
		replacement,
	)
	if err != nil {
		t.Fatalf("ReplaceRepayment: %v", err)
	}
	wantReplaced := repayments.Repayment{
		ID:              original.ID,
		GroupID:         original.GroupID,
		FromUserID:      replacement.FromUserID,
		ToUserID:        replacement.ToUserID,
		AmountCents:     replacement.AmountCents,
		Currency:        replacement.Currency,
		Note:            nil,
		RepaymentDate:   replacement.RepaymentDate,
		CreatedByUserID: original.CreatedByUserID,
		CreatedAt:       original.CreatedAt,
		UpdatedAt:       replacedAt.UTC(),
	}
	if !reflect.DeepEqual(replaced, wantReplaced) {
		t.Errorf("replaced repayment = %#v, want %#v", replaced, wantReplaced)
	}

	got, err := store.GetRepayment(
		context.Background(),
		aliceID,
		groupOneID,
		repaymentOneID,
	)
	if err != nil {
		t.Fatalf("GetRepayment after replacement: %v", err)
	}
	if !reflect.DeepEqual(got, wantReplaced) {
		t.Errorf("persisted replacement = %#v, want %#v", got, wantReplaced)
	}
}

func TestRepaymentStoreReplacementFailuresRollBackOriginal(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := repaymentStoreTestFixture(t, db)
	store := New(db)

	originalInput := repaymentStoreTestCreateInput(baseTime)
	if _, err := store.CreateRepayment(
		context.Background(),
		originalInput,
	); err != nil {
		t.Fatalf("CreateRepayment: %v", err)
	}
	wantOriginal, err := store.GetRepayment(
		context.Background(),
		aliceID,
		groupOneID,
		repaymentOneID,
	)
	if err != nil {
		t.Fatalf("GetRepayment original: %v", err)
	}

	replacementNote := "Replacement note"
	baseReplacement := repayments.ReplaceInput{
		ActorID:       carolID,
		GroupID:       groupOneID,
		RepaymentID:   repaymentOneID,
		FromUserID:    bobID,
		ToUserID:      aliceID,
		AmountCents:   1250,
		Currency:      "USD",
		Note:          &replacementNote,
		RepaymentDate: originalInput.RepaymentDate.AddDate(0, 0, 1),
		UpdatedAt:     originalInput.UpdatedAt.Add(time.Hour),
	}

	inactive := baseReplacement
	inactive.ToUserID = erinID
	if _, err := store.ReplaceRepayment(
		context.Background(),
		inactive,
	); !errors.Is(err, repayments.ErrNotFound) {
		t.Errorf("inactive replacement error = %v, want ErrNotFound", err)
	}
	repaymentStoreTestAssertRepayment(t, store, wantOriginal)

	invalid := baseReplacement
	invalid.AmountCents = 0
	if _, err := store.ReplaceRepayment(
		context.Background(),
		invalid,
	); !errors.Is(err, errRepaymentInvariant) {
		t.Errorf(
			"invalid replacement error = %v, want repayment invariant",
			err,
		)
	}
	repaymentStoreTestAssertRepayment(t, store, wantOriginal)

	repaymentStoreTestInstallRejectUpdateTrigger(t, db)
	if _, err := store.ReplaceRepayment(
		context.Background(),
		baseReplacement,
	); err == nil {
		t.Fatal("replacement succeeded, want forced update failure")
	}
	repaymentStoreTestAssertRepayment(t, store, wantOriginal)
}

func TestRepaymentStoreSoftDeleteRetainsHistoryAndHidesRecord(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := repaymentStoreTestFixture(t, db)
	store := New(db)
	input := repaymentStoreTestCreateInput(baseTime)
	if _, err := store.CreateRepayment(context.Background(), input); err != nil {
		t.Fatalf("CreateRepayment: %v", err)
	}

	deletedAt := input.CreatedAt.Add(2 * time.Hour)
	if err := store.DeleteRepayment(
		context.Background(),
		repayments.DeleteInput{
			ActorID:     carolID,
			GroupID:     groupOneID,
			RepaymentID: repaymentOneID,
			DeletedAt:   deletedAt,
		},
	); err != nil {
		t.Fatalf("DeleteRepayment by collaborator: %v", err)
	}

	var gotDeletedAt time.Time
	var gotUpdatedAt time.Time
	var retainedNote *string
	if err := db.QueryRow(`
		SELECT deleted_at, updated_at, note
		FROM repayments
		WHERE id = $1
	`, repaymentOneID).Scan(
		&gotDeletedAt,
		&gotUpdatedAt,
		&retainedNote,
	); err != nil {
		t.Fatalf("query soft-deleted repayment: %v", err)
	}
	if !gotDeletedAt.Equal(deletedAt) ||
		!gotUpdatedAt.Equal(deletedAt) {
		t.Errorf(
			"deleted timestamps = (%v, %v), want %v",
			gotDeletedAt,
			gotUpdatedAt,
			deletedAt,
		)
	}
	if !reflect.DeepEqual(retainedNote, input.Note) {
		t.Errorf("retained note = %#v, want %#v", retainedNote, input.Note)
	}

	if _, err := store.GetRepayment(
		context.Background(),
		aliceID,
		groupOneID,
		repaymentOneID,
	); !errors.Is(err, repayments.ErrNotFound) {
		t.Errorf("deleted GetRepayment error = %v, want ErrNotFound", err)
	}
	list, err := store.ListRepayments(
		context.Background(),
		aliceID,
		groupOneID,
	)
	if err != nil {
		t.Fatalf("ListRepayments after deletion: %v", err)
	}
	if len(list.Repayments) != 0 {
		t.Errorf("deleted repayment remains in list: %+v", list.Repayments)
	}
	var activeCount int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM active_repayments
		WHERE id = $1
	`, repaymentOneID).Scan(&activeCount); err != nil {
		t.Fatalf("count active repayment: %v", err)
	}
	if activeCount != 0 {
		t.Errorf("active repayment count = %d, want zero", activeCount)
	}

	if err := store.DeleteRepayment(
		context.Background(),
		repayments.DeleteInput{
			ActorID:     aliceID,
			GroupID:     groupOneID,
			RepaymentID: repaymentOneID,
			DeletedAt:   deletedAt.Add(time.Hour),
		},
	); !errors.Is(err, repayments.ErrNotFound) {
		t.Errorf("repeated delete error = %v, want ErrNotFound", err)
	}
	replacement := repayments.ReplaceInput{
		ActorID:       aliceID,
		GroupID:       groupOneID,
		RepaymentID:   repaymentOneID,
		FromUserID:    aliceID,
		ToUserID:      bobID,
		AmountCents:   1,
		Currency:      "USD",
		RepaymentDate: input.RepaymentDate,
		UpdatedAt:     deletedAt.Add(time.Hour),
	}
	if _, err := store.ReplaceRepayment(
		context.Background(),
		replacement,
	); !errors.Is(err, repayments.ErrNotFound) {
		t.Errorf("replace deleted repayment error = %v, want ErrNotFound", err)
	}
}

func TestRepaymentStoreHiddenAndCrossGroupStates(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := repaymentStoreTestFixture(t, db)
	store := New(db)
	input := repaymentStoreTestCreateInput(baseTime)
	if _, err := store.CreateRepayment(context.Background(), input); err != nil {
		t.Fatalf("CreateRepayment: %v", err)
	}

	crossGroupReplacement := repayments.ReplaceInput{
		ActorID:       aliceID,
		GroupID:       groupTwoID,
		RepaymentID:   repaymentOneID,
		FromUserID:    daveID,
		ToUserID:      aliceID,
		AmountCents:   100,
		Currency:      "USD",
		RepaymentDate: input.RepaymentDate,
		UpdatedAt:     input.UpdatedAt.Add(time.Hour),
	}
	crossGroupOperations := []struct {
		name string
		call func() error
	}{
		{
			name: "get",
			call: func() error {
				_, err := store.GetRepayment(
					context.Background(),
					aliceID,
					groupTwoID,
					repaymentOneID,
				)
				return err
			},
		},
		{
			name: "replace",
			call: func() error {
				_, err := store.ReplaceRepayment(
					context.Background(),
					crossGroupReplacement,
				)
				return err
			},
		},
		{
			name: "delete",
			call: func() error {
				return store.DeleteRepayment(
					context.Background(),
					repayments.DeleteInput{
						ActorID:     aliceID,
						GroupID:     groupTwoID,
						RepaymentID: repaymentOneID,
						DeletedAt:   input.UpdatedAt.Add(time.Hour),
					},
				)
			},
		},
	}
	for _, operation := range crossGroupOperations {
		t.Run("cross-group "+operation.name, func(t *testing.T) {
			if err := operation.call(); !errors.Is(err, repayments.ErrNotFound) {
				t.Errorf("error = %v, want ErrNotFound", err)
			}
		})
	}

	for name, actorID := range map[string]string{
		"nonmember": frankID,
		"removed":   erinID,
	} {
		t.Run(name+" visibility", func(t *testing.T) {
			if _, err := store.GetRepayment(
				context.Background(),
				actorID,
				groupOneID,
				repaymentOneID,
			); !errors.Is(err, repayments.ErrNotFound) {
				t.Errorf("GetRepayment error = %v, want ErrNotFound", err)
			}
			if _, err := store.ListRepayments(
				context.Background(),
				actorID,
				groupOneID,
			); !errors.Is(err, repayments.ErrNotFound) {
				t.Errorf("ListRepayments error = %v, want ErrNotFound", err)
			}
			replacement := crossGroupReplacement
			replacement.ActorID = actorID
			replacement.GroupID = groupOneID
			replacement.FromUserID = aliceID
			replacement.ToUserID = carolID
			if _, err := store.ReplaceRepayment(
				context.Background(),
				replacement,
			); !errors.Is(err, repayments.ErrNotFound) {
				t.Errorf("ReplaceRepayment error = %v, want ErrNotFound", err)
			}
			if err := store.DeleteRepayment(
				context.Background(),
				repayments.DeleteInput{
					ActorID:     actorID,
					GroupID:     groupOneID,
					RepaymentID: repaymentOneID,
					DeletedAt:   input.UpdatedAt.Add(time.Hour),
				},
			); !errors.Is(err, repayments.ErrNotFound) {
				t.Errorf("DeleteRepayment error = %v, want ErrNotFound", err)
			}
		})
	}

	dissolvedAt := input.UpdatedAt.Add(2 * time.Hour)
	if _, err := db.Exec(`
		UPDATE groups
		SET dissolved_at = $2,
			updated_at = $2
		WHERE id = $1
	`, groupOneID, dissolvedAt); err != nil {
		t.Fatalf("dissolve group: %v", err)
	}
	dissolvedOperations := []func() error{
		func() error {
			_, err := store.GetRepayment(
				context.Background(),
				aliceID,
				groupOneID,
				repaymentOneID,
			)
			return err
		},
		func() error {
			_, err := store.ListRepayments(
				context.Background(),
				aliceID,
				groupOneID,
			)
			return err
		},
		func() error {
			replacement := crossGroupReplacement
			replacement.GroupID = groupOneID
			replacement.FromUserID = aliceID
			replacement.ToUserID = carolID
			_, err := store.ReplaceRepayment(
				context.Background(),
				replacement,
			)
			return err
		},
		func() error {
			return store.DeleteRepayment(
				context.Background(),
				repayments.DeleteInput{
					ActorID:     aliceID,
					GroupID:     groupOneID,
					RepaymentID: repaymentOneID,
					DeletedAt:   dissolvedAt.Add(time.Hour),
				},
			)
		},
	}
	for index, operation := range dissolvedOperations {
		if err := operation(); !errors.Is(err, repayments.ErrNotFound) {
			t.Errorf(
				"dissolved operation %d error = %v, want ErrNotFound",
				index,
				err,
			)
		}
	}
}

func TestRepaymentStoreListStableOrderingAndMemberSummaries(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := repaymentStoreTestFixture(t, db)
	store := New(db)

	if _, err := db.Exec(`
		UPDATE users
		SET display_name = CASE id
			WHEN $1 THEN 'Zulu Owner'
			WHEN $2 THEN 'alice'
			WHEN $3 THEN 'Alice'
			ELSE display_name
		END
		WHERE id IN ($1, $2, $3)
	`, aliceID, bobID, carolID); err != nil {
		t.Fatalf("update fixture display names: %v", err)
	}

	inputs := []repayments.CreateInput{
		repaymentStoreTestCreateInput(baseTime),
		repaymentStoreTestCreateInput(baseTime),
		repaymentStoreTestCreateInput(baseTime),
	}
	inputs[0].ID = repaymentOneID
	inputs[0].RepaymentDate = time.Date(
		2026,
		time.July,
		31,
		0,
		0,
		0,
		0,
		time.UTC,
	)
	inputs[0].CreatedAt = baseTime.Add(3 * time.Hour)
	inputs[0].UpdatedAt = inputs[0].CreatedAt

	inputs[1].ID = repaymentTwoID
	inputs[1].RepaymentDate = time.Date(
		2026,
		time.August,
		1,
		0,
		0,
		0,
		0,
		time.UTC,
	)
	inputs[1].CreatedAt = baseTime.Add(2 * time.Hour)
	inputs[1].UpdatedAt = inputs[1].CreatedAt
	inputs[1].Note = nil

	inputs[2].ID = repaymentThreeID
	inputs[2].RepaymentDate = inputs[1].RepaymentDate
	inputs[2].CreatedAt = inputs[1].CreatedAt
	inputs[2].UpdatedAt = inputs[2].CreatedAt
	tieNote := "Higher ID"
	inputs[2].Note = &tieNote

	for _, input := range inputs {
		if _, err := store.CreateRepayment(
			context.Background(),
			input,
		); err != nil {
			t.Fatalf("CreateRepayment %s: %v", input.ID, err)
		}
	}

	deletedInput := repaymentStoreTestCreateInput(baseTime)
	deletedInput.ID = repaymentFourID
	deletedInput.RepaymentDate = inputs[1].RepaymentDate.AddDate(0, 0, 1)
	deletedInput.CreatedAt = baseTime.Add(4 * time.Hour)
	deletedInput.UpdatedAt = deletedInput.CreatedAt
	if _, err := store.CreateRepayment(
		context.Background(),
		deletedInput,
	); err != nil {
		t.Fatalf("create deleted fixture repayment: %v", err)
	}
	if err := store.DeleteRepayment(
		context.Background(),
		repayments.DeleteInput{
			ActorID:     aliceID,
			GroupID:     groupOneID,
			RepaymentID: repaymentFourID,
			DeletedAt:   deletedInput.CreatedAt.Add(time.Hour),
		},
	); err != nil {
		t.Fatalf("delete fixture repayment: %v", err)
	}

	result, err := store.ListRepayments(
		context.Background(),
		bobID,
		groupOneID,
	)
	if err != nil {
		t.Fatalf("ListRepayments: %v", err)
	}
	gotIDs := make([]string, len(result.Repayments))
	for index, repayment := range result.Repayments {
		gotIDs[index] = repayment.ID
	}
	wantIDs := []string{repaymentThreeID, repaymentTwoID, repaymentOneID}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("repayment IDs = %v, want %v", gotIDs, wantIDs)
	}
	if result.Repayments[0].Note == nil ||
		*result.Repayments[0].Note != tieNote {
		t.Errorf("first repayment note = %#v, want %q", result.Repayments[0].Note, tieNote)
	}
	if result.Repayments[1].Note != nil {
		t.Errorf("second repayment note = %#v, want nil", result.Repayments[1].Note)
	}
	wantMembers := []repayments.MemberSummary{
		{UserID: aliceID, DisplayName: "Zulu Owner"},
		{UserID: bobID, DisplayName: "alice"},
		{UserID: carolID, DisplayName: "Alice"},
	}
	if !reflect.DeepEqual(result.Members, wantMembers) {
		t.Errorf("members = %#v, want %#v", result.Members, wantMembers)
	}

	empty, err := store.ListRepayments(
		context.Background(),
		aliceID,
		groupTwoID,
	)
	if err != nil {
		t.Fatalf("ListRepayments empty visible group: %v", err)
	}
	if empty.Repayments == nil || len(empty.Repayments) != 0 {
		t.Errorf("empty repayments = %#v, want non-nil empty", empty.Repayments)
	}
	wantEmptyMembers := []repayments.MemberSummary{
		{UserID: daveID, DisplayName: "Dave"},
		{UserID: aliceID, DisplayName: "Zulu Owner"},
	}
	if !reflect.DeepEqual(empty.Members, wantEmptyMembers) {
		t.Errorf(
			"empty-list members = %#v, want %#v",
			empty.Members,
			wantEmptyMembers,
		)
	}
}

func TestRepaymentStoreCreateWinsRaceWithMemberRemoval(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := repaymentStoreTestFixture(t, db)
	const advisoryKey int64 = 8_120_001
	repaymentStoreTestInstallPauseInsertTrigger(t, db, advisoryKey)

	blocker := repaymentStoreTestAdvisoryBlocker(t, db, advisoryKey)
	defer func() {
		_ = blocker.Rollback()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	input := repaymentStoreTestCreateInput(baseTime)
	input.ActorID = aliceID
	input.FromUserID = bobID
	input.ToUserID = carolID
	createDone := make(chan error, 1)
	go func() {
		_, err := New(db).CreateRepayment(ctx, input)
		createDone <- err
	}()
	repaymentStoreTestWaitForWaitingLocks(t, db, 1)

	removeDone := make(chan error, 1)
	go func() {
		removeDone <- New(db).RemoveMember(
			ctx,
			groups.RemoveMemberInput{
				ActorID:   aliceID,
				GroupID:   groupOneID,
				UserID:    bobID,
				RemovedAt: baseTime.Add(3 * time.Hour),
			},
		)
	}()
	repaymentStoreTestWaitForWaitingLocks(t, db, 2)

	if err := blocker.Commit(); err != nil {
		t.Fatalf("release advisory blocker: %v", err)
	}
	if err := repaymentStoreTestReceiveError(
		t,
		createDone,
		"CreateRepayment",
	); err != nil {
		t.Errorf("CreateRepayment error = %v, want nil", err)
	}
	if err := repaymentStoreTestReceiveError(
		t,
		removeDone,
		"RemoveMember",
	); !errors.Is(err, groups.ErrMemberInUse) {
		t.Errorf("RemoveMember error = %v, want ErrMemberInUse", err)
	}
	repaymentStoreTestAssertRepaymentExists(t, db, input.ID, true)
	groupOwnerTestAssertActiveMembership(t, db, groupOneID, bobID)
}

func TestRepaymentStoreMemberRemovalWinsRaceWithCreate(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := repaymentStoreTestFixture(t, db)
	const advisoryKey int64 = 8_120_002
	repaymentStoreTestInstallPauseMembershipRemovalTrigger(t, db, advisoryKey)

	blocker := repaymentStoreTestAdvisoryBlocker(t, db, advisoryKey)
	defer func() {
		_ = blocker.Rollback()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	removeDone := make(chan error, 1)
	go func() {
		removeDone <- New(db).RemoveMember(
			ctx,
			groups.RemoveMemberInput{
				ActorID:   aliceID,
				GroupID:   groupOneID,
				UserID:    bobID,
				RemovedAt: baseTime.Add(2 * time.Hour),
			},
		)
	}()
	repaymentStoreTestWaitForWaitingLocks(t, db, 1)

	input := repaymentStoreTestCreateInput(baseTime)
	input.ActorID = aliceID
	input.FromUserID = bobID
	input.ToUserID = carolID
	createDone := make(chan error, 1)
	go func() {
		_, err := New(db).CreateRepayment(ctx, input)
		createDone <- err
	}()
	repaymentStoreTestWaitForWaitingLocks(t, db, 2)

	if err := blocker.Commit(); err != nil {
		t.Fatalf("release advisory blocker: %v", err)
	}
	if err := repaymentStoreTestReceiveError(
		t,
		removeDone,
		"RemoveMember",
	); err != nil {
		t.Errorf("RemoveMember error = %v, want nil", err)
	}
	if err := repaymentStoreTestReceiveError(
		t,
		createDone,
		"CreateRepayment",
	); !errors.Is(err, repayments.ErrNotFound) {
		t.Errorf("CreateRepayment error = %v, want ErrNotFound", err)
	}
	repaymentStoreTestAssertRepaymentExists(t, db, input.ID, false)
	repaymentStoreTestAssertRemovedMembership(t, db, groupOneID, bobID)
}

func TestRepaymentStoreMemberRemovalWinsRaceWithReplacement(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := repaymentStoreTestFixture(t, db)
	store := New(db)

	originalInput := repaymentStoreTestCreateInput(baseTime)
	originalInput.ActorID = bobID
	originalInput.FromUserID = aliceID
	originalInput.ToUserID = carolID
	if _, err := store.CreateRepayment(
		context.Background(),
		originalInput,
	); err != nil {
		t.Fatalf("create creator-only fixture repayment: %v", err)
	}
	wantOriginal, err := store.GetRepayment(
		context.Background(),
		aliceID,
		groupOneID,
		repaymentOneID,
	)
	if err != nil {
		t.Fatalf("GetRepayment original: %v", err)
	}

	const advisoryKey int64 = 8_120_003
	repaymentStoreTestInstallPauseMembershipRemovalTrigger(t, db, advisoryKey)
	blocker := repaymentStoreTestAdvisoryBlocker(t, db, advisoryKey)
	defer func() {
		_ = blocker.Rollback()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	removeDone := make(chan error, 1)
	go func() {
		removeDone <- store.RemoveMember(
			ctx,
			groups.RemoveMemberInput{
				ActorID:   aliceID,
				GroupID:   groupOneID,
				UserID:    bobID,
				RemovedAt: baseTime.Add(2 * time.Hour),
			},
		)
	}()
	repaymentStoreTestWaitForWaitingLocks(t, db, 1)

	replacement := repayments.ReplaceInput{
		ActorID:       aliceID,
		GroupID:       groupOneID,
		RepaymentID:   repaymentOneID,
		FromUserID:    bobID,
		ToUserID:      carolID,
		AmountCents:   1000,
		Currency:      "USD",
		RepaymentDate: originalInput.RepaymentDate.AddDate(0, 0, 1),
		UpdatedAt:     originalInput.UpdatedAt.Add(3 * time.Hour),
	}
	replaceDone := make(chan error, 1)
	go func() {
		_, err := store.ReplaceRepayment(ctx, replacement)
		replaceDone <- err
	}()
	repaymentStoreTestWaitForWaitingLocks(t, db, 2)

	if err := blocker.Commit(); err != nil {
		t.Fatalf("release advisory blocker: %v", err)
	}
	if err := repaymentStoreTestReceiveError(
		t,
		removeDone,
		"RemoveMember",
	); err != nil {
		t.Errorf("RemoveMember error = %v, want nil", err)
	}
	if err := repaymentStoreTestReceiveError(
		t,
		replaceDone,
		"ReplaceRepayment",
	); !errors.Is(err, repayments.ErrNotFound) {
		t.Errorf("ReplaceRepayment error = %v, want ErrNotFound", err)
	}
	repaymentStoreTestAssertRemovedMembership(t, db, groupOneID, bobID)
	repaymentStoreTestAssertRepayment(t, store, wantOriginal)
}

func repaymentStoreTestFixture(t *testing.T, db *sql.DB) time.Time {
	t.Helper()

	resetIntegrationDatabase(t, db)
	groupTestInsertUsers(t, db)
	baseTime := time.Date(2026, time.August, 1, 10, 0, 0, 0, time.UTC)
	groupTestInsertGroup(
		t,
		db,
		groupOneID,
		aliceID,
		"Repayment Group",
		"REPAYMENT234",
		baseTime,
		nil,
	)
	groupTestInsertMembership(t, db, groupOneID, aliceID, baseTime, nil)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		bobID,
		baseTime.Add(time.Minute),
		nil,
	)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		carolID,
		baseTime.Add(2*time.Minute),
		nil,
	)
	removedAt := baseTime.Add(4 * time.Minute)
	groupTestInsertMembership(
		t,
		db,
		groupOneID,
		erinID,
		baseTime.Add(3*time.Minute),
		&removedAt,
	)

	groupTestInsertGroup(
		t,
		db,
		groupTwoID,
		daveID,
		"Other Repayment Group",
		"REPAYMENT235",
		baseTime,
		nil,
	)
	groupTestInsertMembership(t, db, groupTwoID, daveID, baseTime, nil)
	groupTestInsertMembership(
		t,
		db,
		groupTwoID,
		aliceID,
		baseTime.Add(time.Minute),
		nil,
	)
	return baseTime
}

func repaymentStoreTestCreateInput(baseTime time.Time) repayments.CreateInput {
	workflowTime := baseTime.Add(time.Hour)
	note := "Paid in cash"
	return repayments.CreateInput{
		ID:          repaymentOneID,
		ActorID:     bobID,
		GroupID:     groupOneID,
		FromUserID:  carolID,
		ToUserID:    aliceID,
		AmountCents: 900,
		Currency:    "USD",
		Note:        &note,
		RepaymentDate: time.Date(
			2026,
			time.August,
			1,
			0,
			0,
			0,
			0,
			time.UTC,
		),
		CreatedAt: workflowTime,
		UpdatedAt: workflowTime,
	}
}

func repaymentStoreTestRepayment(
	input repayments.CreateInput,
) repayments.Repayment {
	var note *string
	if input.Note != nil {
		copied := strings.TrimSpace(*input.Note)
		if copied != "" {
			note = &copied
		}
	}
	return repayments.Repayment{
		ID:              input.ID,
		GroupID:         input.GroupID,
		FromUserID:      input.FromUserID,
		ToUserID:        input.ToUserID,
		AmountCents:     input.AmountCents,
		Currency:        input.Currency,
		Note:            note,
		RepaymentDate:   input.RepaymentDate,
		CreatedByUserID: input.ActorID,
		CreatedAt:       input.CreatedAt.UTC(),
		UpdatedAt:       input.UpdatedAt.UTC(),
	}
}

func repaymentStoreTestAssertRepaymentExists(
	t *testing.T,
	db *sql.DB,
	repaymentID string,
	want bool,
) {
	t.Helper()

	var exists bool
	if err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM repayments
			WHERE id = $1
		)
	`, repaymentID).Scan(&exists); err != nil {
		t.Fatalf("query repayment existence: %v", err)
	}
	if exists != want {
		t.Errorf("repayment exists = %v, want %v", exists, want)
	}
}

func repaymentStoreTestAssertRepayment(
	t *testing.T,
	store *Store,
	want repayments.Repayment,
) {
	t.Helper()

	got, err := store.GetRepayment(
		context.Background(),
		aliceID,
		want.GroupID,
		want.ID,
	)
	if err != nil {
		t.Fatalf("GetRepayment after failed replacement: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("repayment after rollback = %#v, want %#v", got, want)
	}
}

func repaymentStoreTestInstallRejectInsertTrigger(
	t *testing.T,
	db *sql.DB,
) {
	t.Helper()

	if _, err := db.Exec(`
		CREATE FUNCTION repayment_store_test_reject_insert()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
			RAISE EXCEPTION 'forced repayment insert failure';
		END;
		$$
	`); err != nil {
		t.Fatalf("create repayment insert failure function: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER repayment_store_test_reject_insert
		AFTER INSERT ON repayments
		FOR EACH ROW
		EXECUTE FUNCTION repayment_store_test_reject_insert()
	`); err != nil {
		t.Fatalf("create repayment insert failure trigger: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`
			DROP TRIGGER IF EXISTS repayment_store_test_reject_insert
			ON repayments
		`); err != nil {
			t.Errorf("drop repayment insert failure trigger: %v", err)
		}
		if _, err := db.Exec(`
			DROP FUNCTION IF EXISTS repayment_store_test_reject_insert()
		`); err != nil {
			t.Errorf("drop repayment insert failure function: %v", err)
		}
	})
}

func repaymentStoreTestInstallRejectUpdateTrigger(
	t *testing.T,
	db *sql.DB,
) {
	t.Helper()

	if _, err := db.Exec(`
		CREATE FUNCTION repayment_store_test_reject_update()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
			RAISE EXCEPTION 'forced repayment update failure';
		END;
		$$
	`); err != nil {
		t.Fatalf("create repayment update failure function: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER repayment_store_test_reject_update
		AFTER UPDATE ON repayments
		FOR EACH ROW
		EXECUTE FUNCTION repayment_store_test_reject_update()
	`); err != nil {
		t.Fatalf("create repayment update failure trigger: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`
			DROP TRIGGER IF EXISTS repayment_store_test_reject_update
			ON repayments
		`); err != nil {
			t.Errorf("drop repayment update failure trigger: %v", err)
		}
		if _, err := db.Exec(`
			DROP FUNCTION IF EXISTS repayment_store_test_reject_update()
		`); err != nil {
			t.Errorf("drop repayment update failure function: %v", err)
		}
	})
}

func repaymentStoreTestInstallPauseInsertTrigger(
	t *testing.T,
	db *sql.DB,
	advisoryKey int64,
) {
	t.Helper()

	if _, err := db.Exec(`
		CREATE FUNCTION repayment_store_test_pause_insert()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(TG_ARGV[0]::bigint);
			RETURN NEW;
		END;
		$$
	`); err != nil {
		t.Fatalf("create pause-repayment-insert function: %v", err)
	}
	triggerStatement := fmt.Sprintf(`
		CREATE TRIGGER repayment_store_test_pause_insert
		BEFORE INSERT ON repayments
		FOR EACH ROW
		EXECUTE FUNCTION repayment_store_test_pause_insert('%d')
	`, advisoryKey)
	if _, err := db.Exec(triggerStatement); err != nil {
		t.Fatalf("create pause-repayment-insert trigger: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`
			DROP TRIGGER IF EXISTS repayment_store_test_pause_insert
			ON repayments
		`); err != nil {
			t.Errorf("drop pause-repayment-insert trigger: %v", err)
		}
		if _, err := db.Exec(`
			DROP FUNCTION IF EXISTS repayment_store_test_pause_insert()
		`); err != nil {
			t.Errorf("drop pause-repayment-insert function: %v", err)
		}
	})
}

func repaymentStoreTestInstallPauseMembershipRemovalTrigger(
	t *testing.T,
	db *sql.DB,
	advisoryKey int64,
) {
	t.Helper()

	if _, err := db.Exec(`
		CREATE FUNCTION repayment_store_test_pause_membership_removal()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(TG_ARGV[0]::bigint);
			RETURN NEW;
		END;
		$$
	`); err != nil {
		t.Fatalf("create pause-membership-removal function: %v", err)
	}
	triggerStatement := fmt.Sprintf(`
		CREATE TRIGGER repayment_store_test_pause_membership_removal
		BEFORE UPDATE OF removed_at ON group_memberships
		FOR EACH ROW
		WHEN (OLD.removed_at IS NULL AND NEW.removed_at IS NOT NULL)
		EXECUTE FUNCTION repayment_store_test_pause_membership_removal('%d')
	`, advisoryKey)
	if _, err := db.Exec(triggerStatement); err != nil {
		t.Fatalf("create pause-membership-removal trigger: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`
			DROP TRIGGER IF EXISTS repayment_store_test_pause_membership_removal
			ON group_memberships
		`); err != nil {
			t.Errorf("drop pause-membership-removal trigger: %v", err)
		}
		if _, err := db.Exec(`
			DROP FUNCTION IF EXISTS repayment_store_test_pause_membership_removal()
		`); err != nil {
			t.Errorf("drop pause-membership-removal function: %v", err)
		}
	})
}

func repaymentStoreTestAdvisoryBlocker(
	t *testing.T,
	db *sql.DB,
	advisoryKey int64,
) *sql.Tx {
	t.Helper()

	blocker, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		t.Fatalf("begin advisory blocker: %v", err)
	}
	if _, err := blocker.Exec(`
		SELECT pg_advisory_xact_lock($1)
	`, advisoryKey); err != nil {
		_ = blocker.Rollback()
		t.Fatalf("acquire advisory blocker: %v", err)
	}
	return blocker
}

func repaymentStoreTestWaitForWaitingLocks(
	t *testing.T,
	db *sql.DB,
	want int,
) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := db.QueryRow(`
			SELECT count(*)
			FROM pg_catalog.pg_locks
			WHERE granted = false
		`).Scan(&waiting); err != nil {
			t.Fatalf("query waiting locks: %v", err)
		}
		if waiting >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("waiting lock count did not reach %d", want)
}

func repaymentStoreTestReceiveError(
	t *testing.T,
	result <-chan error,
	operation string,
) error {
	t.Helper()

	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not complete", operation)
		return nil
	}
}

func repaymentStoreTestAssertRemovedMembership(
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
		t.Fatalf("query removed membership: %v", err)
	}
	if removedAt == nil {
		t.Error("membership removed_at = NULL, want removal timestamp")
	}
}
