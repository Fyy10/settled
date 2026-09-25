package store

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/Fyy10/settled/server/internal/expenses"
)

func TestValidateExpenseSplitInvariant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		amountCents int64
		splits      []expenses.Split
		wantError   bool
	}{
		{
			name:        "valid",
			amountCents: 10,
			splits: []expenses.Split{
				{UserID: "a", AmountCents: 6},
				{UserID: "b", AmountCents: 4},
			},
		},
		{
			name:        "missing splits",
			amountCents: 10,
			wantError:   true,
		},
		{
			name:        "nonpositive split",
			amountCents: 10,
			splits: []expenses.Split{
				{UserID: "a", AmountCents: 10},
				{UserID: "b", AmountCents: 0},
			},
			wantError: true,
		},
		{
			name:        "duplicate participant",
			amountCents: 10,
			splits: []expenses.Split{
				{UserID: "a", AmountCents: 6},
				{UserID: "a", AmountCents: 4},
			},
			wantError: true,
		},
		{
			name:        "overflow",
			amountCents: math.MaxInt64,
			splits: []expenses.Split{
				{UserID: "a", AmountCents: math.MaxInt64},
				{UserID: "b", AmountCents: 1},
			},
			wantError: true,
		},
		{
			name:        "total mismatch",
			amountCents: 11,
			splits: []expenses.Split{
				{UserID: "a", AmountCents: 6},
				{UserID: "b", AmountCents: 4},
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateExpenseSplitInvariant(
				test.amountCents,
				test.splits,
			)
			if test.wantError {
				if !errors.Is(err, errExpenseSplitInvariant) {
					t.Errorf(
						"error = %v, want errExpenseSplitInvariant",
						err,
					)
				}
				if errors.Is(err, expenses.ErrNotFound) {
					t.Errorf("invariant error is miscategorized as not found: %v", err)
				}
				return
			}
			if err != nil {
				t.Errorf("error = %v, want nil", err)
			}
		})
	}
}

func TestExpenseMembershipLockQueryUsesSortedUniqueIDs(t *testing.T) {
	t.Parallel()

	memberIDs := uniqueSortedExpenseMemberIDs([]string{
		"c",
		"a",
		"b",
		"a",
	})
	if !reflect.DeepEqual(memberIDs, []string{"a", "b", "c"}) {
		t.Fatalf("member IDs = %v", memberIDs)
	}

	query, arguments := expenseMembershipLockQuery("group", memberIDs)
	if !strings.Contains(query, "IN ($2, $3, $4)") ||
		!strings.Contains(query, "ORDER BY user_id") ||
		!strings.Contains(query, "FOR UPDATE") {
		t.Errorf("membership lock query does not preserve lock order: %s", query)
	}
	wantArguments := []any{"group", "a", "b", "c"}
	if !reflect.DeepEqual(arguments, wantArguments) {
		t.Errorf("arguments = %#v, want %#v", arguments, wantArguments)
	}
}

func TestCloneExpenseSplitsDoesNotAliasInput(t *testing.T) {
	t.Parallel()

	input := []expenses.Split{{UserID: "a", AmountCents: 10}}
	cloned := cloneExpenseSplits(input)
	cloned[0].AmountCents = 20
	if input[0].AmountCents != 10 {
		t.Errorf("input was mutated through cloned slice: %+v", input)
	}
}
