package store

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestValidateRepaymentInvariant(t *testing.T) {
	t.Parallel()

	validDate := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		fromUserID    string
		toUserID      string
		amountCents   int64
		currency      string
		note          *string
		repaymentDate time.Time
		wantNote      *string
		wantError     bool
	}{
		{
			name:          "valid without note",
			fromUserID:    "a",
			toUserID:      "b",
			amountCents:   1,
			currency:      "USD",
			repaymentDate: validDate,
		},
		{
			name:          "normalizes note",
			fromUserID:    "a",
			toUserID:      "b",
			amountCents:   1,
			currency:      "USD",
			note:          repaymentStringPointer("  paid in cash  "),
			repaymentDate: validDate,
			wantNote:      repaymentStringPointer("paid in cash"),
		},
		{
			name:          "blank note becomes null",
			fromUserID:    "a",
			toUserID:      "b",
			amountCents:   1,
			currency:      "USD",
			note:          repaymentStringPointer(" \t\n"),
			repaymentDate: validDate,
		},
		{
			name:          "same participant",
			fromUserID:    "a",
			toUserID:      "a",
			amountCents:   1,
			currency:      "USD",
			repaymentDate: validDate,
			wantError:     true,
		},
		{
			name:          "zero amount",
			fromUserID:    "a",
			toUserID:      "b",
			currency:      "USD",
			repaymentDate: validDate,
			wantError:     true,
		},
		{
			name:          "negative amount",
			fromUserID:    "a",
			toUserID:      "b",
			amountCents:   -1,
			currency:      "USD",
			repaymentDate: validDate,
			wantError:     true,
		},
		{
			name:          "non USD currency",
			fromUserID:    "a",
			toUserID:      "b",
			amountCents:   1,
			currency:      "EUR",
			repaymentDate: validDate,
			wantError:     true,
		},
		{
			name:          "nonmidnight date",
			fromUserID:    "a",
			toUserID:      "b",
			amountCents:   1,
			currency:      "USD",
			repaymentDate: validDate.Add(time.Nanosecond),
			wantError:     true,
		},
		{
			name:        "non UTC date",
			fromUserID:  "a",
			toUserID:    "b",
			amountCents: 1,
			currency:    "USD",
			repaymentDate: time.Date(
				2026,
				time.August,
				1,
				0,
				0,
				0,
				0,
				time.FixedZone("workflow", -7*60*60),
			),
			wantError: true,
		},
		{
			name:          "note too long",
			fromUserID:    "a",
			toUserID:      "b",
			amountCents:   1,
			currency:      "USD",
			note:          repaymentStringPointer(strings.Repeat("界", 241)),
			repaymentDate: validDate,
			wantError:     true,
		},
		{
			name:          "invalid UTF-8 note",
			fromUserID:    "a",
			toUserID:      "b",
			amountCents:   1,
			currency:      "USD",
			note:          repaymentStringPointer(string([]byte{0xff})),
			repaymentDate: validDate,
			wantError:     true,
		},
		{
			name:          "embedded control note",
			fromUserID:    "a",
			toUserID:      "b",
			amountCents:   1,
			currency:      "USD",
			note:          repaymentStringPointer("paid\ncash"),
			repaymentDate: validDate,
			wantError:     true,
		},
		{
			name:          "bidirectional control note",
			fromUserID:    "a",
			toUserID:      "b",
			amountCents:   1,
			currency:      "USD",
			note:          repaymentStringPointer("paid\u202ecash"),
			repaymentDate: validDate,
			wantError:     true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			gotNote, err := validateRepaymentInvariant(
				test.fromUserID,
				test.toUserID,
				test.amountCents,
				test.currency,
				test.note,
				test.repaymentDate,
			)
			if test.wantError {
				if !errors.Is(err, errRepaymentInvariant) {
					t.Errorf(
						"error = %v, want errRepaymentInvariant",
						err,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateRepaymentInvariant: %v", err)
			}
			if !reflect.DeepEqual(gotNote, test.wantNote) {
				t.Errorf("note = %#v, want %#v", gotNote, test.wantNote)
			}
			if gotNote != nil && test.note != nil && gotNote == test.note {
				t.Error("normalized note aliases the input pointer")
			}
		})
	}
}

func TestRepaymentMembershipLockQueryUsesSortedUniqueIDs(t *testing.T) {
	t.Parallel()

	memberIDs := uniqueSortedRepaymentMemberIDs([]string{
		"c",
		"a",
		"b",
		"a",
	})
	if !reflect.DeepEqual(memberIDs, []string{"a", "b", "c"}) {
		t.Fatalf("member IDs = %v", memberIDs)
	}

	query, arguments := repaymentMembershipLockQuery("group", memberIDs)
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

func repaymentStringPointer(value string) *string {
	return &value
}
