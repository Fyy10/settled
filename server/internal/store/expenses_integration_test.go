//go:build integration

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/expenses"
	"github.com/Fyy10/settled/server/internal/groups"
)

const expenseMissingGroupID = "10000000-0000-4000-8000-000000000099"

func TestExpenseStoreCreateAndGet(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := expenseStoreTestFixture(t, db)
	store := New(db)

	input := expenseStoreTestCreateInput(baseTime)
	input.CreatedAt = time.Date(
		2026,
		time.July,
		31,
		12,
		34,
		56,
		123456000,
		time.FixedZone("workflow", -7*60*60),
	)
	input.UpdatedAt = input.CreatedAt
	created, err := store.CreateExpense(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateExpense: %v", err)
	}
	wantCreated := expenseStoreTestExpense(input)
	if !reflect.DeepEqual(created, wantCreated) {
		t.Errorf("created expense = %#v, want %#v", created, wantCreated)
	}

	got, err := store.GetExpense(
		context.Background(),
		carolID,
		groupOneID,
		expenseOneID,
	)
	if err != nil {
		t.Fatalf("GetExpense: %v", err)
	}
	wantGet := wantCreated
	wantGet.Splits = []expenses.Split{
		{UserID: aliceID, AmountCents: 300},
		{UserID: bobID, AmountCents: 300},
		{UserID: carolID, AmountCents: 300},
	}
	if !reflect.DeepEqual(got, wantGet) {
		t.Errorf("fetched expense = %#v, want %#v", got, wantGet)
	}

	var splitCount int
	var splitTotal int64
	var creatorID string
	if err := db.QueryRow(`
		SELECT
			count(split.*),
			coalesce(sum(split.amount_cents), 0),
			expense.created_by_user_id::text
		FROM expenses expense
		JOIN expense_splits split
			ON split.expense_id = expense.id
			AND split.group_id = expense.group_id
		WHERE expense.id = $1
			AND expense.group_id = $2
		GROUP BY expense.created_by_user_id
	`, expenseOneID, groupOneID).Scan(
		&splitCount,
		&splitTotal,
		&creatorID,
	); err != nil {
		t.Fatalf("query persisted expense: %v", err)
	}
	if splitCount != 3 || splitTotal != input.AmountCents {
		t.Errorf(
			"persisted splits = count %d total %d, want 3 and %d",
			splitCount,
			splitTotal,
			input.AmountCents,
		)
	}
	if creatorID != input.ActorID {
		t.Errorf("creator ID = %q, want %q", creatorID, input.ActorID)
	}
}

func TestExpenseStoreCreateRequiresCurrentVisibleMemberships(t *testing.T) {
	db := openIntegrationDatabase(t)
	store := New(db)

	tests := []struct {
		name      string
		mutate    func(*expenses.CreateInput)
		dissolved bool
	}{
		{
			name: "nonmember actor",
			mutate: func(input *expenses.CreateInput) {
				input.ActorID = frankID
			},
		},
		{
			name: "removed actor",
			mutate: func(input *expenses.CreateInput) {
				input.ActorID = erinID
			},
		},
		{
			name: "removed payer",
			mutate: func(input *expenses.CreateInput) {
				input.PaidByUserID = erinID
			},
		},
		{
			name: "missing payer",
			mutate: func(input *expenses.CreateInput) {
				input.PaidByUserID = frankID
			},
		},
		{
			name: "removed participant",
			mutate: func(input *expenses.CreateInput) {
				input.Splits[0].UserID = erinID
			},
		},
		{
			name: "missing participant",
			mutate: func(input *expenses.CreateInput) {
				input.Splits[0].UserID = frankID
			},
		},
		{
			name:      "dissolved group",
			dissolved: true,
			mutate:    func(*expenses.CreateInput) {},
		},
		{
			name: "missing group",
			mutate: func(input *expenses.CreateInput) {
				input.GroupID = expenseMissingGroupID
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseTime := expenseStoreTestFixture(t, db)
			input := expenseStoreTestCreateInput(baseTime)
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

			_, err := store.CreateExpense(context.Background(), input)
			if !errors.Is(err, expenses.ErrNotFound) {
				t.Errorf("error = %v, want ErrNotFound", err)
			}
			expenseStoreTestAssertExpenseExists(
				t,
				db,
				input.ID,
				false,
			)
		})
	}
}

func TestExpenseStoreRejectsInvalidPrecomputedSplitsWithoutWrites(t *testing.T) {
	db := openIntegrationDatabase(t)
	store := New(db)

	tests := []struct {
		name   string
		amount int64
		splits []expenses.Split
	}{
		{
			name:   "missing splits",
			amount: 900,
		},
		{
			name:   "nonpositive split",
			amount: 900,
			splits: []expenses.Split{
				{UserID: aliceID, AmountCents: 900},
				{UserID: bobID, AmountCents: 0},
			},
		},
		{
			name:   "duplicate participant",
			amount: 900,
			splits: []expenses.Split{
				{UserID: aliceID, AmountCents: 500},
				{UserID: aliceID, AmountCents: 400},
			},
		},
		{
			name:   "total mismatch",
			amount: 900,
			splits: []expenses.Split{
				{UserID: aliceID, AmountCents: 500},
				{UserID: bobID, AmountCents: 399},
			},
		},
		{
			name:   "overflow",
			amount: math.MaxInt64,
			splits: []expenses.Split{
				{UserID: aliceID, AmountCents: math.MaxInt64},
				{UserID: bobID, AmountCents: 1},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseTime := expenseStoreTestFixture(t, db)
			input := expenseStoreTestCreateInput(baseTime)
			input.AmountCents = test.amount
			input.Splits = test.splits

			_, err := store.CreateExpense(context.Background(), input)
			if !errors.Is(err, errExpenseSplitInvariant) {
				t.Errorf(
					"error = %v, want errExpenseSplitInvariant",
					err,
				)
			}
			if errors.Is(err, expenses.ErrNotFound) {
				t.Errorf("invariant error is miscategorized as not found: %v", err)
			}
			expenseStoreTestAssertExpenseExists(
				t,
				db,
				input.ID,
				false,
			)
		})
	}
}

func TestExpenseStoreCreateRollsBackSplitFailure(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := expenseStoreTestFixture(t, db)
	expenseStoreTestInstallRejectBobSplitTrigger(t, db)

	input := expenseStoreTestCreateInput(baseTime)
	input.Splits = []expenses.Split{
		{UserID: aliceID, AmountCents: 600},
		{UserID: bobID, AmountCents: 300},
	}
	_, err := New(db).CreateExpense(context.Background(), input)
	if err == nil {
		t.Fatal("CreateExpense succeeded, want split insert failure")
	}
	if errors.Is(err, expenses.ErrNotFound) ||
		errors.Is(err, errExpenseSplitInvariant) {
		t.Errorf("error = %v, want internal database error", err)
	}
	expenseStoreTestAssertExpenseExists(t, db, input.ID, false)

	var splitCount int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM expense_splits
		WHERE expense_id = $1
	`, input.ID).Scan(&splitCount); err != nil {
		t.Fatalf("count rolled-back splits: %v", err)
	}
	if splitCount != 0 {
		t.Errorf("split count = %d, want zero", splitCount)
	}
}

func TestExpenseStoreCollaborativeReplacementPreservesMetadata(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := expenseStoreTestFixture(t, db)
	store := New(db)

	originalInput := expenseStoreTestCreateInput(baseTime)
	originalInput.ActorID = aliceID
	originalInput.Splits = []expenses.Split{
		{UserID: aliceID, AmountCents: 500},
		{UserID: carolID, AmountCents: 400},
	}
	original, err := store.CreateExpense(context.Background(), originalInput)
	if err != nil {
		t.Fatalf("CreateExpense: %v", err)
	}

	replacedAt := originalInput.CreatedAt.Add(2 * time.Hour)
	replacement := expenses.ReplaceInput{
		ActorID:      bobID,
		GroupID:      groupOneID,
		ExpenseID:    expenseOneID,
		PaidByUserID: bobID,
		Description:  "Replacement dinner",
		AmountCents:  1000,
		Currency:     "USD",
		ExpenseDate:  originalInput.ExpenseDate.AddDate(0, 0, 1),
		Splits: []expenses.Split{
			{UserID: carolID, AmountCents: 400},
			{UserID: bobID, AmountCents: 600},
		},
		UpdatedAt: replacedAt,
	}
	replaced, err := store.ReplaceExpense(context.Background(), replacement)
	if err != nil {
		t.Fatalf("ReplaceExpense: %v", err)
	}
	wantReplaced := expenses.Expense{
		ID:              original.ID,
		GroupID:         original.GroupID,
		PaidByUserID:    replacement.PaidByUserID,
		Description:     replacement.Description,
		AmountCents:     replacement.AmountCents,
		Currency:        replacement.Currency,
		ExpenseDate:     replacement.ExpenseDate,
		CreatedByUserID: original.CreatedByUserID,
		Splits:          cloneExpenseSplits(replacement.Splits),
		CreatedAt:       original.CreatedAt,
		UpdatedAt:       replacedAt.UTC(),
	}
	if !reflect.DeepEqual(replaced, wantReplaced) {
		t.Errorf("replaced expense = %#v, want %#v", replaced, wantReplaced)
	}

	got, err := store.GetExpense(
		context.Background(),
		aliceID,
		groupOneID,
		expenseOneID,
	)
	if err != nil {
		t.Fatalf("GetExpense after replacement: %v", err)
	}
	wantGet := wantReplaced
	wantGet.Splits = []expenses.Split{
		{UserID: bobID, AmountCents: 600},
		{UserID: carolID, AmountCents: 400},
	}
	if !reflect.DeepEqual(got, wantGet) {
		t.Errorf("persisted replacement = %#v, want %#v", got, wantGet)
	}

	var oldSplitCount int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM expense_splits
		WHERE expense_id = $1
			AND user_id = $2
	`, expenseOneID, aliceID).Scan(&oldSplitCount); err != nil {
		t.Fatalf("count replaced old split: %v", err)
	}
	if oldSplitCount != 0 {
		t.Errorf("old split count = %d, want zero", oldSplitCount)
	}
}

func TestExpenseStoreReplacementFailuresRollBackOriginal(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := expenseStoreTestFixture(t, db)
	store := New(db)

	originalInput := expenseStoreTestCreateInput(baseTime)
	originalInput.ActorID = aliceID
	originalInput.Splits = []expenses.Split{
		{UserID: aliceID, AmountCents: 500},
		{UserID: carolID, AmountCents: 400},
	}
	if _, err := store.CreateExpense(
		context.Background(),
		originalInput,
	); err != nil {
		t.Fatalf("CreateExpense: %v", err)
	}
	wantOriginal, err := store.GetExpense(
		context.Background(),
		aliceID,
		groupOneID,
		expenseOneID,
	)
	if err != nil {
		t.Fatalf("GetExpense original: %v", err)
	}

	baseReplacement := expenses.ReplaceInput{
		ActorID:      bobID,
		GroupID:      groupOneID,
		ExpenseID:    expenseOneID,
		PaidByUserID: bobID,
		Description:  "Must roll back",
		AmountCents:  1000,
		Currency:     "USD",
		ExpenseDate:  originalInput.ExpenseDate.AddDate(0, 0, 1),
		Splits: []expenses.Split{
			{UserID: aliceID, AmountCents: 400},
			{UserID: bobID, AmountCents: 600},
		},
		UpdatedAt: originalInput.UpdatedAt.Add(time.Hour),
	}

	inactive := baseReplacement
	inactive.PaidByUserID = erinID
	if _, err := store.ReplaceExpense(
		context.Background(),
		inactive,
	); !errors.Is(err, expenses.ErrNotFound) {
		t.Errorf("inactive replacement error = %v, want ErrNotFound", err)
	}
	expenseStoreTestAssertExpense(
		t,
		store,
		wantOriginal,
	)

	invalid := baseReplacement
	invalid.AmountCents = 1001
	if _, err := store.ReplaceExpense(
		context.Background(),
		invalid,
	); !errors.Is(err, errExpenseSplitInvariant) {
		t.Errorf(
			"invalid replacement error = %v, want split invariant",
			err,
		)
	}
	expenseStoreTestAssertExpense(
		t,
		store,
		wantOriginal,
	)

	expenseStoreTestInstallRejectBobSplitTrigger(t, db)
	if _, err := store.ReplaceExpense(
		context.Background(),
		baseReplacement,
	); err == nil {
		t.Fatal("replacement succeeded, want split insert failure")
	}
	expenseStoreTestAssertExpense(
		t,
		store,
		wantOriginal,
	)
}

func TestExpenseStoreSoftDeleteRetainsHistoryAndHidesRecord(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := expenseStoreTestFixture(t, db)
	store := New(db)
	input := expenseStoreTestCreateInput(baseTime)
	if _, err := store.CreateExpense(context.Background(), input); err != nil {
		t.Fatalf("CreateExpense: %v", err)
	}

	deletedAt := input.CreatedAt.Add(2 * time.Hour)
	if err := store.DeleteExpense(
		context.Background(),
		expenses.DeleteInput{
			ActorID:   carolID,
			GroupID:   groupOneID,
			ExpenseID: expenseOneID,
			DeletedAt: deletedAt,
		},
	); err != nil {
		t.Fatalf("DeleteExpense by collaborator: %v", err)
	}

	var gotDeletedAt time.Time
	var gotUpdatedAt time.Time
	var splitCount int
	if err := db.QueryRow(`
		SELECT
			expense.deleted_at,
			expense.updated_at,
			count(split.*)
		FROM expenses expense
		LEFT JOIN expense_splits split
			ON split.expense_id = expense.id
			AND split.group_id = expense.group_id
		WHERE expense.id = $1
		GROUP BY expense.id
	`, expenseOneID).Scan(
		&gotDeletedAt,
		&gotUpdatedAt,
		&splitCount,
	); err != nil {
		t.Fatalf("query soft-deleted expense: %v", err)
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
	if splitCount != len(input.Splits) {
		t.Errorf("retained split count = %d, want %d", splitCount, len(input.Splits))
	}

	if _, err := store.GetExpense(
		context.Background(),
		aliceID,
		groupOneID,
		expenseOneID,
	); !errors.Is(err, expenses.ErrNotFound) {
		t.Errorf("deleted GetExpense error = %v, want ErrNotFound", err)
	}
	list, err := store.ListExpenses(
		context.Background(),
		aliceID,
		groupOneID,
	)
	if err != nil {
		t.Fatalf("ListExpenses after deletion: %v", err)
	}
	if len(list.Expenses) != 0 {
		t.Errorf("deleted expense remains in list: %+v", list.Expenses)
	}
	var activeCount int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM active_expenses
		WHERE id = $1
	`, expenseOneID).Scan(&activeCount); err != nil {
		t.Fatalf("count active expense: %v", err)
	}
	if activeCount != 0 {
		t.Errorf("active expense count = %d, want zero", activeCount)
	}

	if err := store.DeleteExpense(
		context.Background(),
		expenses.DeleteInput{
			ActorID:   aliceID,
			GroupID:   groupOneID,
			ExpenseID: expenseOneID,
			DeletedAt: deletedAt.Add(time.Hour),
		},
	); !errors.Is(err, expenses.ErrNotFound) {
		t.Errorf("repeated delete error = %v, want ErrNotFound", err)
	}
}

func TestExpenseStoreHiddenAndCrossGroupStates(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := expenseStoreTestFixture(t, db)
	store := New(db)
	input := expenseStoreTestCreateInput(baseTime)
	if _, err := store.CreateExpense(context.Background(), input); err != nil {
		t.Fatalf("CreateExpense: %v", err)
	}

	crossGroupReplacement := expenses.ReplaceInput{
		ActorID:      aliceID,
		GroupID:      groupTwoID,
		ExpenseID:    expenseOneID,
		PaidByUserID: daveID,
		Description:  "Cross-group replacement",
		AmountCents:  100,
		Currency:     "USD",
		ExpenseDate:  input.ExpenseDate,
		Splits: []expenses.Split{
			{UserID: daveID, AmountCents: 100},
		},
		UpdatedAt: input.UpdatedAt.Add(time.Hour),
	}
	crossGroupOperations := []struct {
		name string
		call func() error
	}{
		{
			name: "get",
			call: func() error {
				_, err := store.GetExpense(
					context.Background(),
					aliceID,
					groupTwoID,
					expenseOneID,
				)
				return err
			},
		},
		{
			name: "replace",
			call: func() error {
				_, err := store.ReplaceExpense(
					context.Background(),
					crossGroupReplacement,
				)
				return err
			},
		},
		{
			name: "delete",
			call: func() error {
				return store.DeleteExpense(
					context.Background(),
					expenses.DeleteInput{
						ActorID:   aliceID,
						GroupID:   groupTwoID,
						ExpenseID: expenseOneID,
						DeletedAt: input.UpdatedAt.Add(time.Hour),
					},
				)
			},
		},
	}
	for _, operation := range crossGroupOperations {
		t.Run("cross-group "+operation.name, func(t *testing.T) {
			if err := operation.call(); !errors.Is(err, expenses.ErrNotFound) {
				t.Errorf("error = %v, want ErrNotFound", err)
			}
		})
	}

	for name, actorID := range map[string]string{
		"nonmember": frankID,
		"removed":   erinID,
	} {
		t.Run(name+" read visibility", func(t *testing.T) {
			if _, err := store.GetExpense(
				context.Background(),
				actorID,
				groupOneID,
				expenseOneID,
			); !errors.Is(err, expenses.ErrNotFound) {
				t.Errorf("GetExpense error = %v, want ErrNotFound", err)
			}
			if _, err := store.ListExpenses(
				context.Background(),
				actorID,
				groupOneID,
			); !errors.Is(err, expenses.ErrNotFound) {
				t.Errorf("ListExpenses error = %v, want ErrNotFound", err)
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
			_, err := store.GetExpense(
				context.Background(),
				aliceID,
				groupOneID,
				expenseOneID,
			)
			return err
		},
		func() error {
			_, err := store.ListExpenses(
				context.Background(),
				aliceID,
				groupOneID,
			)
			return err
		},
		func() error {
			replacement := crossGroupReplacement
			replacement.GroupID = groupOneID
			replacement.PaidByUserID = aliceID
			replacement.Splits = []expenses.Split{{
				UserID:      aliceID,
				AmountCents: 100,
			}}
			_, err := store.ReplaceExpense(
				context.Background(),
				replacement,
			)
			return err
		},
		func() error {
			return store.DeleteExpense(
				context.Background(),
				expenses.DeleteInput{
					ActorID:   aliceID,
					GroupID:   groupOneID,
					ExpenseID: expenseOneID,
					DeletedAt: dissolvedAt.Add(time.Hour),
				},
			)
		},
	}
	for index, operation := range dissolvedOperations {
		if err := operation(); !errors.Is(err, expenses.ErrNotFound) {
			t.Errorf(
				"dissolved operation %d error = %v, want ErrNotFound",
				index,
				err,
			)
		}
	}
}

func TestExpenseStoreListBulkAssemblyAndStableOrdering(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := expenseStoreTestFixture(t, db)
	store := New(db)

	inputs := []expenses.CreateInput{
		expenseStoreTestCreateInput(baseTime),
		expenseStoreTestCreateInput(baseTime),
		expenseStoreTestCreateInput(baseTime),
	}
	inputs[0].ID = expenseOneID
	inputs[0].Description = "Older date"
	inputs[0].ExpenseDate = time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	inputs[0].CreatedAt = baseTime.Add(3 * time.Hour)
	inputs[0].UpdatedAt = inputs[0].CreatedAt

	inputs[1].ID = expenseTwoID
	inputs[1].Description = "Tie lower ID"
	inputs[1].ExpenseDate = time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	inputs[1].CreatedAt = baseTime.Add(2 * time.Hour)
	inputs[1].UpdatedAt = inputs[1].CreatedAt
	inputs[1].Splits = []expenses.Split{
		{UserID: carolID, AmountCents: 450},
		{UserID: aliceID, AmountCents: 450},
	}

	inputs[2].ID = expenseThreeID
	inputs[2].Description = "Tie higher ID"
	inputs[2].ExpenseDate = inputs[1].ExpenseDate
	inputs[2].CreatedAt = inputs[1].CreatedAt
	inputs[2].UpdatedAt = inputs[2].CreatedAt
	inputs[2].Splits = []expenses.Split{
		{UserID: bobID, AmountCents: 300},
		{UserID: aliceID, AmountCents: 600},
	}

	for _, input := range inputs {
		if _, err := store.CreateExpense(
			context.Background(),
			input,
		); err != nil {
			t.Fatalf("CreateExpense %s: %v", input.ID, err)
		}
	}

	deletedInput := expenseStoreTestCreateInput(baseTime)
	deletedInput.ID = expenseFourID
	deletedInput.Description = "Deleted newest"
	deletedInput.ExpenseDate = inputs[1].ExpenseDate.AddDate(0, 0, 1)
	deletedInput.CreatedAt = baseTime.Add(4 * time.Hour)
	deletedInput.UpdatedAt = deletedInput.CreatedAt
	if _, err := store.CreateExpense(
		context.Background(),
		deletedInput,
	); err != nil {
		t.Fatalf("create deleted fixture expense: %v", err)
	}
	if err := store.DeleteExpense(
		context.Background(),
		expenses.DeleteInput{
			ActorID:   aliceID,
			GroupID:   groupOneID,
			ExpenseID: expenseFourID,
			DeletedAt: deletedInput.CreatedAt.Add(time.Hour),
		},
	); err != nil {
		t.Fatalf("delete fixture expense: %v", err)
	}

	result, err := store.ListExpenses(
		context.Background(),
		bobID,
		groupOneID,
	)
	if err != nil {
		t.Fatalf("ListExpenses: %v", err)
	}
	gotIDs := make([]string, len(result.Expenses))
	for index, expense := range result.Expenses {
		gotIDs[index] = expense.ID
		if expense.Splits == nil {
			t.Errorf("expense %s has nil splits", expense.ID)
		}
	}
	wantIDs := []string{expenseThreeID, expenseTwoID, expenseOneID}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("expense IDs = %v, want %v", gotIDs, wantIDs)
	}
	if !reflect.DeepEqual(
		result.Expenses[0].Splits,
		[]expenses.Split{
			{UserID: aliceID, AmountCents: 600},
			{UserID: bobID, AmountCents: 300},
		},
	) {
		t.Errorf("first expense splits = %+v", result.Expenses[0].Splits)
	}
	if !reflect.DeepEqual(
		result.Expenses[1].Splits,
		[]expenses.Split{
			{UserID: aliceID, AmountCents: 450},
			{UserID: carolID, AmountCents: 450},
		},
	) {
		t.Errorf("second expense splits = %+v", result.Expenses[1].Splits)
	}
	wantMembers := []expenses.MemberSummary{
		{UserID: aliceID, DisplayName: "Alice"},
		{UserID: bobID, DisplayName: "Bob"},
		{UserID: carolID, DisplayName: "Carol"},
	}
	if !reflect.DeepEqual(result.Members, wantMembers) {
		t.Errorf("members = %#v, want %#v", result.Members, wantMembers)
	}

	empty, err := store.ListExpenses(
		context.Background(),
		aliceID,
		groupTwoID,
	)
	if err != nil {
		t.Fatalf("ListExpenses empty visible group: %v", err)
	}
	if empty.Expenses == nil || len(empty.Expenses) != 0 {
		t.Errorf("empty expenses = %#v, want non-nil empty", empty.Expenses)
	}
	wantEmptyMembers := []expenses.MemberSummary{
		{UserID: daveID, DisplayName: "Dave"},
		{UserID: aliceID, DisplayName: "Alice"},
	}
	if !reflect.DeepEqual(empty.Members, wantEmptyMembers) {
		t.Errorf(
			"empty-list members = %#v, want %#v",
			empty.Members,
			wantEmptyMembers,
		)
	}
}

func TestExpenseStoreCreateWinsRaceWithMemberRemoval(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := expenseStoreTestFixture(t, db)
	const advisoryKey int64 = 8_110_001
	expenseStoreTestInstallPauseExpenseInsertTrigger(t, db, advisoryKey)

	blocker, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		t.Fatalf("begin advisory blocker: %v", err)
	}
	defer func() {
		_ = blocker.Rollback()
	}()
	if _, err := blocker.Exec(`
		SELECT pg_advisory_xact_lock($1)
	`, advisoryKey); err != nil {
		t.Fatalf("acquire advisory blocker: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	input := expenseStoreTestCreateInput(baseTime)
	input.ActorID = aliceID
	input.PaidByUserID = bobID
	input.Splits = []expenses.Split{
		{UserID: bobID, AmountCents: 450},
		{UserID: carolID, AmountCents: 450},
	}
	createDone := make(chan error, 1)
	go func() {
		_, err := New(db).CreateExpense(ctx, input)
		createDone <- err
	}()
	expenseStoreTestWaitForWaitingLocks(t, db, 1)

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
	expenseStoreTestWaitForWaitingLocks(t, db, 2)

	if err := blocker.Commit(); err != nil {
		t.Fatalf("release advisory blocker: %v", err)
	}
	if err := expenseStoreTestReceiveError(
		t,
		createDone,
		"CreateExpense",
	); err != nil {
		t.Errorf("CreateExpense error = %v, want nil", err)
	}
	if err := expenseStoreTestReceiveError(
		t,
		removeDone,
		"RemoveMember",
	); !errors.Is(err, groups.ErrMemberInUse) {
		t.Errorf("RemoveMember error = %v, want ErrMemberInUse", err)
	}
	expenseStoreTestAssertExpenseExists(t, db, input.ID, true)
	groupOwnerTestAssertActiveMembership(t, db, groupOneID, bobID)
}

func TestExpenseStoreMemberRemovalWinsRaceWithCreate(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := expenseStoreTestFixture(t, db)
	const advisoryKey int64 = 8_110_002
	expenseStoreTestInstallPauseMembershipRemovalTrigger(t, db, advisoryKey)

	blocker, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		t.Fatalf("begin advisory blocker: %v", err)
	}
	defer func() {
		_ = blocker.Rollback()
	}()
	if _, err := blocker.Exec(`
		SELECT pg_advisory_xact_lock($1)
	`, advisoryKey); err != nil {
		t.Fatalf("acquire advisory blocker: %v", err)
	}

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
	expenseStoreTestWaitForWaitingLocks(t, db, 1)

	input := expenseStoreTestCreateInput(baseTime)
	input.ActorID = aliceID
	input.PaidByUserID = bobID
	input.Splits = []expenses.Split{
		{UserID: bobID, AmountCents: 450},
		{UserID: carolID, AmountCents: 450},
	}
	createDone := make(chan error, 1)
	go func() {
		_, err := New(db).CreateExpense(ctx, input)
		createDone <- err
	}()
	expenseStoreTestWaitForWaitingLocks(t, db, 2)

	if err := blocker.Commit(); err != nil {
		t.Fatalf("release advisory blocker: %v", err)
	}
	if err := expenseStoreTestReceiveError(
		t,
		removeDone,
		"RemoveMember",
	); err != nil {
		t.Errorf("RemoveMember error = %v, want nil", err)
	}
	if err := expenseStoreTestReceiveError(
		t,
		createDone,
		"CreateExpense",
	); !errors.Is(err, expenses.ErrNotFound) {
		t.Errorf("CreateExpense error = %v, want ErrNotFound", err)
	}
	expenseStoreTestAssertExpenseExists(t, db, input.ID, false)
	expenseStoreTestAssertRemovedMembership(t, db, groupOneID, bobID)
}

func TestExpenseStoreMemberRemovalWinsRaceWithReplacement(t *testing.T) {
	db := openIntegrationDatabase(t)
	baseTime := expenseStoreTestFixture(t, db)
	store := New(db)

	originalInput := expenseStoreTestCreateInput(baseTime)
	originalInput.ActorID = bobID
	originalInput.PaidByUserID = aliceID
	originalInput.Splits = []expenses.Split{
		{UserID: aliceID, AmountCents: 500},
		{UserID: carolID, AmountCents: 400},
	}
	if _, err := store.CreateExpense(
		context.Background(),
		originalInput,
	); err != nil {
		t.Fatalf("create creator-only fixture expense: %v", err)
	}
	wantOriginal, err := store.GetExpense(
		context.Background(),
		aliceID,
		groupOneID,
		expenseOneID,
	)
	if err != nil {
		t.Fatalf("GetExpense original: %v", err)
	}

	const advisoryKey int64 = 8_110_003
	expenseStoreTestInstallPauseMembershipRemovalTrigger(t, db, advisoryKey)
	blocker, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		t.Fatalf("begin advisory blocker: %v", err)
	}
	defer func() {
		_ = blocker.Rollback()
	}()
	if _, err := blocker.Exec(`
		SELECT pg_advisory_xact_lock($1)
	`, advisoryKey); err != nil {
		t.Fatalf("acquire advisory blocker: %v", err)
	}

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
	expenseStoreTestWaitForWaitingLocks(t, db, 1)

	replacement := expenses.ReplaceInput{
		ActorID:      aliceID,
		GroupID:      groupOneID,
		ExpenseID:    expenseOneID,
		PaidByUserID: bobID,
		Description:  "Concurrent replacement",
		AmountCents:  900,
		Currency:     "USD",
		ExpenseDate:  originalInput.ExpenseDate.AddDate(0, 0, 1),
		Splits: []expenses.Split{
			{UserID: bobID, AmountCents: 450},
			{UserID: carolID, AmountCents: 450},
		},
		UpdatedAt: originalInput.UpdatedAt.Add(3 * time.Hour),
	}
	replaceDone := make(chan error, 1)
	go func() {
		_, err := store.ReplaceExpense(ctx, replacement)
		replaceDone <- err
	}()
	expenseStoreTestWaitForWaitingLocks(t, db, 2)

	if err := blocker.Commit(); err != nil {
		t.Fatalf("release advisory blocker: %v", err)
	}
	if err := expenseStoreTestReceiveError(
		t,
		removeDone,
		"RemoveMember",
	); err != nil {
		t.Errorf("RemoveMember error = %v, want nil", err)
	}
	if err := expenseStoreTestReceiveError(
		t,
		replaceDone,
		"ReplaceExpense",
	); !errors.Is(err, expenses.ErrNotFound) {
		t.Errorf("ReplaceExpense error = %v, want ErrNotFound", err)
	}
	expenseStoreTestAssertRemovedMembership(t, db, groupOneID, bobID)
	expenseStoreTestAssertExpense(t, store, wantOriginal)
}

func expenseStoreTestFixture(t *testing.T, db *sql.DB) time.Time {
	t.Helper()

	resetIntegrationDatabase(t, db)
	groupTestInsertUsers(t, db)
	baseTime := time.Date(2026, time.July, 31, 10, 0, 0, 0, time.UTC)
	groupTestInsertGroup(
		t,
		db,
		groupOneID,
		aliceID,
		"Expense Group",
		"EXPENSEGRP23",
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
		"Other Expense Group",
		"EXPENSEGRP24",
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

func expenseStoreTestCreateInput(baseTime time.Time) expenses.CreateInput {
	workflowTime := baseTime.Add(time.Hour)
	return expenses.CreateInput{
		ID:           expenseOneID,
		ActorID:      bobID,
		GroupID:      groupOneID,
		PaidByUserID: aliceID,
		Description:  "Shared dinner",
		AmountCents:  900,
		Currency:     "USD",
		ExpenseDate: time.Date(
			2026,
			time.July,
			31,
			0,
			0,
			0,
			0,
			time.UTC,
		),
		Splits: []expenses.Split{
			{UserID: carolID, AmountCents: 300},
			{UserID: aliceID, AmountCents: 300},
			{UserID: bobID, AmountCents: 300},
		},
		CreatedAt: workflowTime,
		UpdatedAt: workflowTime,
	}
}

func expenseStoreTestExpense(input expenses.CreateInput) expenses.Expense {
	return expenses.Expense{
		ID:              input.ID,
		GroupID:         input.GroupID,
		PaidByUserID:    input.PaidByUserID,
		Description:     input.Description,
		AmountCents:     input.AmountCents,
		Currency:        input.Currency,
		ExpenseDate:     input.ExpenseDate,
		CreatedByUserID: input.ActorID,
		Splits:          cloneExpenseSplits(input.Splits),
		CreatedAt:       input.CreatedAt.UTC(),
		UpdatedAt:       input.UpdatedAt.UTC(),
	}
}

func expenseStoreTestAssertExpenseExists(
	t *testing.T,
	db *sql.DB,
	expenseID string,
	want bool,
) {
	t.Helper()

	var exists bool
	if err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM expenses
			WHERE id = $1
		)
	`, expenseID).Scan(&exists); err != nil {
		t.Fatalf("query expense existence: %v", err)
	}
	if exists != want {
		t.Errorf("expense exists = %v, want %v", exists, want)
	}
}

func expenseStoreTestAssertExpense(
	t *testing.T,
	store *Store,
	want expenses.Expense,
) {
	t.Helper()

	got, err := store.GetExpense(
		context.Background(),
		aliceID,
		want.GroupID,
		want.ID,
	)
	if err != nil {
		t.Fatalf("GetExpense after failed replacement: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expense after rollback = %#v, want %#v", got, want)
	}
}

func expenseStoreTestInstallRejectBobSplitTrigger(
	t *testing.T,
	db *sql.DB,
) {
	t.Helper()

	if _, err := db.Exec(`
		CREATE FUNCTION expense_store_test_reject_bob_split()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
			IF NEW.user_id = '00000000-0000-4000-8000-000000000002'::uuid THEN
				RAISE EXCEPTION 'forced expense split failure';
			END IF;
			RETURN NEW;
		END;
		$$
	`); err != nil {
		t.Fatalf("create expense split failure function: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER expense_store_test_reject_bob_split
		BEFORE INSERT ON expense_splits
		FOR EACH ROW
		EXECUTE FUNCTION expense_store_test_reject_bob_split()
	`); err != nil {
		t.Fatalf("create expense split failure trigger: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`
			DROP TRIGGER IF EXISTS expense_store_test_reject_bob_split
			ON expense_splits
		`); err != nil {
			t.Errorf("drop expense split failure trigger: %v", err)
		}
		if _, err := db.Exec(`
			DROP FUNCTION IF EXISTS expense_store_test_reject_bob_split()
		`); err != nil {
			t.Errorf("drop expense split failure function: %v", err)
		}
	})
}

func expenseStoreTestInstallPauseExpenseInsertTrigger(
	t *testing.T,
	db *sql.DB,
	advisoryKey int64,
) {
	t.Helper()

	if _, err := db.Exec(`
		CREATE FUNCTION expense_store_test_pause_expense_insert()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(TG_ARGV[0]::bigint);
			RETURN NEW;
		END;
		$$
	`); err != nil {
		t.Fatalf("create pause-expense-insert function: %v", err)
	}
	triggerStatement := fmt.Sprintf(`
		CREATE TRIGGER expense_store_test_pause_expense_insert
		BEFORE INSERT ON expenses
		FOR EACH ROW
		EXECUTE FUNCTION expense_store_test_pause_expense_insert('%d')
	`, advisoryKey)
	if _, err := db.Exec(triggerStatement); err != nil {
		t.Fatalf("create pause-expense-insert trigger: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`
			DROP TRIGGER IF EXISTS expense_store_test_pause_expense_insert
			ON expenses
		`); err != nil {
			t.Errorf("drop pause-expense-insert trigger: %v", err)
		}
		if _, err := db.Exec(`
			DROP FUNCTION IF EXISTS expense_store_test_pause_expense_insert()
		`); err != nil {
			t.Errorf("drop pause-expense-insert function: %v", err)
		}
	})
}

func expenseStoreTestInstallPauseMembershipRemovalTrigger(
	t *testing.T,
	db *sql.DB,
	advisoryKey int64,
) {
	t.Helper()

	if _, err := db.Exec(`
		CREATE FUNCTION expense_store_test_pause_membership_removal()
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
		CREATE TRIGGER expense_store_test_pause_membership_removal
		BEFORE UPDATE OF removed_at ON group_memberships
		FOR EACH ROW
		WHEN (OLD.removed_at IS NULL AND NEW.removed_at IS NOT NULL)
		EXECUTE FUNCTION expense_store_test_pause_membership_removal('%d')
	`, advisoryKey)
	if _, err := db.Exec(triggerStatement); err != nil {
		t.Fatalf("create pause-membership-removal trigger: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`
			DROP TRIGGER IF EXISTS expense_store_test_pause_membership_removal
			ON group_memberships
		`); err != nil {
			t.Errorf("drop pause-membership-removal trigger: %v", err)
		}
		if _, err := db.Exec(`
			DROP FUNCTION IF EXISTS expense_store_test_pause_membership_removal()
		`); err != nil {
			t.Errorf("drop pause-membership-removal function: %v", err)
		}
	})
}

func expenseStoreTestWaitForWaitingLocks(
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

func expenseStoreTestReceiveError(
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

func expenseStoreTestAssertRemovedMembership(
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
