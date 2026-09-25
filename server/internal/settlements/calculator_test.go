package settlements

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

const (
	settlementUserA = "00112233-4455-4677-8899-aabbccddeef1"
	settlementUserB = "00112233-4455-4677-8899-aabbccddeef2"
	settlementUserC = "00112233-4455-4677-8899-aabbccddeef3"
	settlementUserD = "00112233-4455-4677-8899-aabbccddeef4"
)

func TestPairwiseCalculatorAggregatesAndNetsEachPair(t *testing.T) {
	t.Parallel()

	entries := []DebtEntry{
		debtEntry(settlementUserB, settlementUserA, 2000),
		debtEntry(settlementUserB, settlementUserA, 1000),
		debtEntry(settlementUserA, settlementUserB, 1200),
		debtEntry(settlementUserA, settlementUserB, 1000),
		debtEntry(settlementUserC, settlementUserA, 3000),
	}
	got, err := (PairwiseCalculator{}).Calculate(entries)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	want := []Transfer{
		transfer(settlementUserC, settlementUserA, 3000),
		transfer(settlementUserB, settlementUserA, 800),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("transfers = %#v, want %#v", got, want)
	}
}

func TestPairwiseCalculatorRepaymentDirectionReversesDebt(t *testing.T) {
	t.Parallel()

	entries := []DebtEntry{
		debtEntry(settlementUserB, settlementUserA, 3000),
		debtEntry(settlementUserA, settlementUserB, 1000),
	}
	got, err := (PairwiseCalculator{}).Calculate(entries)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	want := []Transfer{
		transfer(settlementUserB, settlementUserA, 2000),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("transfers = %#v, want %#v", got, want)
	}
}

func TestPairwiseCalculatorOmitsZeroBalancesAndReturnsNonNilEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []DebtEntry
	}{
		{name: "nil input"},
		{name: "empty input", entries: []DebtEntry{}},
		{
			name: "equal opposing debt",
			entries: []DebtEntry{
				debtEntry(settlementUserA, settlementUserB, 50),
				debtEntry(settlementUserB, settlementUserA, 20),
				debtEntry(settlementUserB, settlementUserA, 30),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := (PairwiseCalculator{}).Calculate(test.entries)
			if err != nil {
				t.Fatalf("Calculate: %v", err)
			}
			if got == nil || len(got) != 0 {
				t.Errorf("transfers = %#v, want non-nil empty", got)
			}
		})
	}
}

func TestPairwiseCalculatorSortsByAmountThenDirection(t *testing.T) {
	t.Parallel()

	entries := []DebtEntry{
		debtEntry(settlementUserB, settlementUserC, 10),
		debtEntry(settlementUserA, settlementUserD, 10),
		debtEntry(settlementUserB, settlementUserA, 20),
		debtEntry(settlementUserA, settlementUserC, 10),
	}
	want := []Transfer{
		transfer(settlementUserB, settlementUserA, 20),
		transfer(settlementUserA, settlementUserC, 10),
		transfer(settlementUserA, settlementUserD, 10),
		transfer(settlementUserB, settlementUserC, 10),
	}
	calculator := PairwiseCalculator{}
	for run := 0; run < 100; run++ {
		got, err := calculator.Calculate(entries)
		if err != nil {
			t.Fatalf("Calculate run %d: %v", run, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf(
				"run %d transfers = %#v, want %#v",
				run,
				got,
				want,
			)
		}
	}
}

func TestPairwiseCalculatorRejectsInvalidEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*DebtEntry)
	}{
		{
			name: "invalid from-user ID",
			mutate: func(entry *DebtEntry) {
				entry.FromUserID = "not-a-uuid"
			},
		},
		{
			name: "noncanonical uppercase from-user ID",
			mutate: func(entry *DebtEntry) {
				entry.FromUserID = strings.ToUpper(entry.FromUserID)
			},
		},
		{
			name: "invalid to-user ID",
			mutate: func(entry *DebtEntry) {
				entry.ToUserID = "not-a-uuid"
			},
		},
		{
			name: "noncanonical uppercase to-user ID",
			mutate: func(entry *DebtEntry) {
				entry.ToUserID = strings.ToUpper(entry.ToUserID)
			},
		},
		{
			name: "self debt",
			mutate: func(entry *DebtEntry) {
				entry.ToUserID = entry.FromUserID
			},
		},
		{
			name: "zero amount",
			mutate: func(entry *DebtEntry) {
				entry.AmountCents = 0
			},
		},
		{
			name: "negative amount",
			mutate: func(entry *DebtEntry) {
				entry.AmountCents = -1
			},
		},
		{
			name: "non-USD currency",
			mutate: func(entry *DebtEntry) {
				entry.Currency = "EUR"
			},
		},
		{
			name: "empty currency",
			mutate: func(entry *DebtEntry) {
				entry.Currency = ""
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			entry := debtEntry(settlementUserA, settlementUserB, 1)
			test.mutate(&entry)
			got, err := (PairwiseCalculator{}).Calculate([]DebtEntry{entry})
			if !errors.Is(err, ErrInvalidDebtEntry) {
				t.Errorf("error = %v, want ErrInvalidDebtEntry", err)
			}
			if got != nil {
				t.Errorf("transfers = %#v, want nil", got)
			}
		})
	}
}

func TestPairwiseCalculatorChecksAggregationOverflow(t *testing.T) {
	t.Parallel()

	got, err := (PairwiseCalculator{}).Calculate([]DebtEntry{
		debtEntry(settlementUserA, settlementUserB, math.MaxInt64),
		debtEntry(settlementUserA, settlementUserB, 1),
	})
	if !errors.Is(err, ErrArithmeticOverflow) {
		t.Errorf("error = %v, want ErrArithmeticOverflow", err)
	}
	if got != nil {
		t.Errorf("transfers = %#v, want nil", got)
	}
}

func TestPairwiseCalculatorOffsetsInt64BoundariesWithoutOverflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []DebtEntry
		want    []Transfer
	}{
		{
			name: "equal maximums cancel",
			entries: []DebtEntry{
				debtEntry(settlementUserA, settlementUserB, math.MaxInt64),
				debtEntry(settlementUserB, settlementUserA, math.MaxInt64),
			},
			want: []Transfer{},
		},
		{
			name: "maximum forward less one reverse",
			entries: []DebtEntry{
				debtEntry(settlementUserA, settlementUserB, math.MaxInt64),
				debtEntry(settlementUserB, settlementUserA, 1),
			},
			want: []Transfer{
				transfer(
					settlementUserA,
					settlementUserB,
					math.MaxInt64-1,
				),
			},
		},
		{
			name: "maximum reverse less one forward",
			entries: []DebtEntry{
				debtEntry(settlementUserA, settlementUserB, 1),
				debtEntry(settlementUserB, settlementUserA, math.MaxInt64),
			},
			want: []Transfer{
				transfer(
					settlementUserB,
					settlementUserA,
					math.MaxInt64-1,
				),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := (PairwiseCalculator{}).Calculate(test.entries)
			if err != nil {
				t.Fatalf("Calculate: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("transfers = %#v, want %#v", got, test.want)
			}
		})
	}
}

func debtEntry(fromUserID string, toUserID string, amount int64) DebtEntry {
	return DebtEntry{
		FromUserID:  fromUserID,
		ToUserID:    toUserID,
		AmountCents: amount,
		Currency:    "USD",
	}
}

func transfer(fromUserID string, toUserID string, amount int64) Transfer {
	return Transfer{
		FromUserID:  fromUserID,
		ToUserID:    toUserID,
		AmountCents: amount,
		Currency:    "USD",
	}
}
