package expenses

import (
	"errors"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/Fyy10/settled/server/internal/identifier"
)

const (
	participantA = "00000000-0000-4000-8000-000000000001"
	participantB = "00000000-0000-4000-8000-000000000002"
	participantC = "00000000-0000-4000-8000-000000000003"
	participantD = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

func TestSplitModeValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode SplitMode
		want string
	}{
		{mode: SplitModeEqual, want: "equal"},
		{mode: SplitModeExact, want: "exact"},
		{mode: SplitModePercentage, want: "percentage"},
	}
	for _, test := range tests {
		if string(test.mode) != test.want {
			t.Errorf("SplitMode = %q, want %q", test.mode, test.want)
		}
	}
}

func TestCalculateSplitsSuccessfulCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		amountCents int64
		input       SplitInput
		want        []Split
	}{
		{
			name:        "equal exact division",
			amountCents: 9,
			input: SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{participantA, participantB, participantC},
			},
			want: []Split{
				{UserID: participantA, AmountCents: 3},
				{UserID: participantB, AmountCents: 3},
				{UserID: participantC, AmountCents: 3},
			},
		},
		{
			name:        "equal remainder follows request order",
			amountCents: 10,
			input: SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{participantC, participantA, participantB},
			},
			want: []Split{
				{UserID: participantC, AmountCents: 4},
				{UserID: participantA, AmountCents: 3},
				{UserID: participantB, AmountCents: 3},
			},
		},
		{
			name:        "equal maximum amount",
			amountCents: math.MaxInt64,
			input: SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{participantA, participantB, participantC},
			},
			want: []Split{
				{UserID: participantA, AmountCents: 3_074_457_345_618_258_603},
				{UserID: participantB, AmountCents: 3_074_457_345_618_258_602},
				{UserID: participantC, AmountCents: 3_074_457_345_618_258_602},
			},
		},
		{
			name:        "exact preserves order and normalizes UUID",
			amountCents: 10,
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: strings.ToUpper(participantD), AmountCents: 6},
					{UserID: participantA, AmountCents: 4},
				},
			},
			want: []Split{
				{UserID: participantD, AmountCents: 6},
				{UserID: participantA, AmountCents: 4},
			},
		},
		{
			name:        "exact maximum amount",
			amountCents: math.MaxInt64,
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: participantA, AmountCents: math.MaxInt64 - 1},
					{UserID: participantB, AmountCents: 1},
				},
			},
			want: []Split{
				{UserID: participantA, AmountCents: math.MaxInt64 - 1},
				{UserID: participantB, AmountCents: 1},
			},
		},
		{
			name:        "percentage exact division",
			amountCents: 200,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: 2_500},
					{UserID: participantB, PercentageBasisPoints: 7_500},
				},
			},
			want: []Split{
				{UserID: participantA, AmountCents: 50},
				{UserID: participantB, AmountCents: 150},
			},
		},
		{
			name:        "percentage remainder follows request order",
			amountCents: 5_400,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantC, PercentageBasisPoints: 3_334},
					{UserID: participantA, PercentageBasisPoints: 3_333},
					{UserID: participantB, PercentageBasisPoints: 3_333},
				},
			},
			want: []Split{
				{UserID: participantC, AmountCents: 1_801},
				{UserID: participantA, AmountCents: 1_800},
				{UserID: participantB, AmountCents: 1_799},
			},
		},
		{
			name:        "percentage remainder rescues floor zero",
			amountCents: 2,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: 1},
					{UserID: participantB, PercentageBasisPoints: 9_999},
				},
			},
			want: []Split{
				{UserID: participantA, AmountCents: 1},
				{UserID: participantB, AmountCents: 1},
			},
		},
		{
			name:        "percentage maximum amount avoids multiplication overflow",
			amountCents: math.MaxInt64,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: 5_000},
					{UserID: participantB, PercentageBasisPoints: 5_000},
				},
			},
			want: []Split{
				{UserID: participantA, AmountCents: 4_611_686_018_427_387_904},
				{UserID: participantB, AmountCents: 4_611_686_018_427_387_903},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := CalculateSplits(test.amountCents, test.input)
			if err != nil {
				t.Fatalf("CalculateSplits: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("splits = %#v, want %#v", got, test.want)
			}
			assertSplitInvariants(t, got, test.amountCents)
		})
	}
}

func TestCalculateSplitsRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	invalidVariantUUID := "00000000-0000-4000-0000-000000000001"
	tests := []struct {
		name        string
		amountCents int64
		input       SplitInput
		wantError   error
	}{
		{
			name:        "zero expense amount",
			amountCents: 0,
			input: SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{participantA},
			},
			wantError: ErrInvalidExpenseAmount,
		},
		{
			name:        "negative expense amount",
			amountCents: -1,
			input: SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{participantA},
			},
			wantError: ErrInvalidExpenseAmount,
		},
		{
			name:        "empty mode",
			amountCents: 1,
			input:       SplitInput{},
			wantError:   ErrUnsupportedSplitMode,
		},
		{
			name:        "unsupported mode",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitMode("weighted"),
			},
			wantError: ErrUnsupportedSplitMode,
		},
		{
			name:        "equal with exact fields",
			amountCents: 1,
			input: SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{participantA},
				Splits: []ExactSplitInput{
					{UserID: participantA, AmountCents: 1},
				},
			},
			wantError: ErrConflictingSplitInputs,
		},
		{
			name:        "exact with percentage fields",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: participantA, AmountCents: 1},
				},
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: 10_000},
				},
			},
			wantError: ErrConflictingSplitInputs,
		},
		{
			name:        "percentage with equal fields",
			amountCents: 1,
			input: SplitInput{
				Mode:               SplitModePercentage,
				ParticipantUserIDs: []string{participantA},
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: 10_000},
				},
			},
			wantError: ErrConflictingSplitInputs,
		},
		{
			name:        "equal participants required",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModeEqual,
			},
			wantError: ErrParticipantsRequired,
		},
		{
			name:        "exact participants required",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModeExact,
			},
			wantError: ErrParticipantsRequired,
		},
		{
			name:        "percentage participants required",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModePercentage,
			},
			wantError: ErrParticipantsRequired,
		},
		{
			name:        "equal invalid UUID",
			amountCents: 1,
			input: SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{"not-a-uuid"},
			},
			wantError: ErrInvalidParticipantID,
		},
		{
			name:        "exact invalid RFC variant",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: invalidVariantUUID, AmountCents: 1},
				},
			},
			wantError: ErrInvalidParticipantID,
		},
		{
			name:        "percentage invalid UUID",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: "", PercentageBasisPoints: 10_000},
				},
			},
			wantError: ErrInvalidParticipantID,
		},
		{
			name:        "equal duplicate participant",
			amountCents: 2,
			input: SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{participantD, strings.ToUpper(participantD)},
			},
			wantError: ErrDuplicateParticipant,
		},
		{
			name:        "exact duplicate participant",
			amountCents: 2,
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: participantA, AmountCents: 1},
					{UserID: participantA, AmountCents: 1},
				},
			},
			wantError: ErrDuplicateParticipant,
		},
		{
			name:        "percentage duplicate participant",
			amountCents: 2,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: 5_000},
					{UserID: participantA, PercentageBasisPoints: 5_000},
				},
			},
			wantError: ErrDuplicateParticipant,
		},
		{
			name:        "equal amount smaller than participant count",
			amountCents: 2,
			input: SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{participantA, participantB, participantC},
			},
			wantError: ErrCalculatedSplitNotPositive,
		},
		{
			name:        "exact zero amount",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: participantA, AmountCents: 0},
				},
			},
			wantError: ErrSplitAmountNotPositive,
		},
		{
			name:        "exact negative amount",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: participantA, AmountCents: -1},
				},
			},
			wantError: ErrSplitAmountNotPositive,
		},
		{
			name:        "exact total below expense amount",
			amountCents: 2,
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: participantA, AmountCents: 1},
				},
			},
			wantError: ErrExactSplitTotalMismatch,
		},
		{
			name:        "exact total above expense amount",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: participantA, AmountCents: 2},
				},
			},
			wantError: ErrExactSplitTotalMismatch,
		},
		{
			name:        "exact sum overflow",
			amountCents: math.MaxInt64,
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: participantA, AmountCents: math.MaxInt64},
					{UserID: participantB, AmountCents: 1},
				},
			},
			wantError: ErrSplitArithmeticOverflow,
		},
		{
			name:        "percentage zero basis points",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: 0},
				},
			},
			wantError: ErrPercentageNotPositive,
		},
		{
			name:        "percentage negative basis points",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: -1},
				},
			},
			wantError: ErrPercentageNotPositive,
		},
		{
			name:        "percentage total below 10000",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: 9_999},
				},
			},
			wantError: ErrPercentageTotalMismatch,
		},
		{
			name:        "percentage total above 10000",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: 10_001},
				},
			},
			wantError: ErrPercentageTotalMismatch,
		},
		{
			name:        "percentage total overflow",
			amountCents: 1,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: math.MaxInt64},
					{UserID: participantB, PercentageBasisPoints: 1},
				},
			},
			wantError: ErrSplitArithmeticOverflow,
		},
		{
			name:        "percentage zero output after remainder",
			amountCents: 2,
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantA, PercentageBasisPoints: 9_999},
					{UserID: participantB, PercentageBasisPoints: 1},
				},
			},
			wantError: ErrCalculatedSplitNotPositive,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := CalculateSplits(test.amountCents, test.input)
			if !errors.Is(err, test.wantError) {
				t.Errorf("error = %v, want %v", err, test.wantError)
			}
			if got != nil {
				t.Errorf("splits = %#v, want nil", got)
			}
		})
	}
}

func TestCalculateSplitsCrossModeParity(t *testing.T) {
	t.Parallel()

	const amountCents int64 = 10_003
	want := []Split{
		{UserID: participantC, AmountCents: 2_501},
		{UserID: participantA, AmountCents: 2_501},
		{UserID: participantD, AmountCents: 2_501},
		{UserID: participantB, AmountCents: 2_500},
	}
	inputs := []SplitInput{
		{
			Mode: SplitModeEqual,
			ParticipantUserIDs: []string{
				participantC,
				participantA,
				participantD,
				participantB,
			},
		},
		{
			Mode: SplitModeExact,
			Splits: []ExactSplitInput{
				{UserID: participantC, AmountCents: 2_501},
				{UserID: participantA, AmountCents: 2_501},
				{UserID: participantD, AmountCents: 2_501},
				{UserID: participantB, AmountCents: 2_500},
			},
		},
		{
			Mode: SplitModePercentage,
			PercentageSplits: []PercentageSplitInput{
				{UserID: participantC, PercentageBasisPoints: 2_500},
				{UserID: participantA, PercentageBasisPoints: 2_500},
				{UserID: participantD, PercentageBasisPoints: 2_500},
				{UserID: participantB, PercentageBasisPoints: 2_500},
			},
		},
	}

	for _, input := range inputs {
		got, err := CalculateSplits(amountCents, input)
		if err != nil {
			t.Fatalf("CalculateSplits(%q): %v", input.Mode, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("CalculateSplits(%q) = %#v, want %#v", input.Mode, got, want)
		}
	}
}

func TestPercentageEveryTwoParticipantBasisPointAllocation(t *testing.T) {
	t.Parallel()

	amounts := []int64{2, 3, 9_999, 10_001, math.MaxInt64}
	for _, amountCents := range amounts {
		for firstBasisPoints := int64(1); firstBasisPoints < percentageBasisPointTotal; firstBasisPoints++ {
			basisPoints := []int64{
				firstBasisPoints,
				percentageBasisPointTotal - firstBasisPoints,
			}
			wantAmounts := percentageOracle(amountCents, basisPoints)
			got, err := CalculateSplits(amountCents, SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{
						UserID:                participantA,
						PercentageBasisPoints: basisPoints[0],
					},
					{
						UserID:                participantB,
						PercentageBasisPoints: basisPoints[1],
					},
				},
			})

			if wantAmounts[0] == 0 || wantAmounts[1] == 0 {
				if !errors.Is(err, ErrCalculatedSplitNotPositive) {
					t.Fatalf(
						"amount %d, basis points %v: error = %v, want zero-output error",
						amountCents,
						basisPoints,
						err,
					)
				}
				if got != nil {
					t.Fatalf(
						"amount %d, basis points %v: splits = %#v, want nil",
						amountCents,
						basisPoints,
						got,
					)
				}
				continue
			}

			if err != nil {
				t.Fatalf(
					"amount %d, basis points %v: CalculateSplits: %v",
					amountCents,
					basisPoints,
					err,
				)
			}
			gotAmounts := []int64{got[0].AmountCents, got[1].AmountCents}
			if !reflect.DeepEqual(gotAmounts, wantAmounts) {
				t.Fatalf(
					"amount %d, basis points %v: amounts = %v, want %v",
					amountCents,
					basisPoints,
					gotAmounts,
					wantAmounts,
				)
			}
			assertSplitInvariants(t, got, amountCents)
		}
	}
}

func TestCalculateSplitsDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	input := SplitInput{
		Mode: SplitModePercentage,
		PercentageSplits: []PercentageSplitInput{
			{UserID: strings.ToUpper(participantD), PercentageBasisPoints: 3_334},
			{UserID: participantA, PercentageBasisPoints: 3_333},
			{UserID: participantB, PercentageBasisPoints: 3_333},
		},
	}
	original := SplitInput{
		Mode: input.Mode,
		PercentageSplits: append(
			[]PercentageSplitInput(nil),
			input.PercentageSplits...,
		),
	}

	if _, err := CalculateSplits(100, input); err != nil {
		t.Fatalf("CalculateSplits: %v", err)
	}
	if !reflect.DeepEqual(input, original) {
		t.Errorf("input mutated: got %#v, want %#v", input, original)
	}
}

func percentageOracle(amountCents int64, basisPoints []int64) []int64 {
	denominator := big.NewInt(percentageBasisPointTotal)
	amount := big.NewInt(amountCents)
	shares := make([]int64, len(basisPoints))
	var floorTotal int64
	for index, value := range basisPoints {
		product := new(big.Int).Mul(amount, big.NewInt(value))
		shares[index] = new(big.Int).Quo(product, denominator).Int64()
		floorTotal += shares[index]
	}
	remainder := amountCents - floorTotal
	for index := int64(0); index < remainder; index++ {
		shares[index]++
	}
	return shares
}

func assertSplitInvariants(t *testing.T, splits []Split, wantTotal int64) {
	t.Helper()

	if len(splits) == 0 {
		t.Fatal("successful split result is empty")
	}
	seen := make(map[string]struct{}, len(splits))
	total := new(big.Int)
	for index, split := range splits {
		if split.AmountCents <= 0 {
			t.Errorf("split %d amount = %d, want positive", index, split.AmountCents)
		}
		canonical, err := identifier.ParseUUID(split.UserID)
		if err != nil {
			t.Errorf("split %d user ID is invalid: %v", index, err)
		} else if canonical != split.UserID {
			t.Errorf("split %d user ID = %q, want canonical %q", index, split.UserID, canonical)
		}
		if _, exists := seen[split.UserID]; exists {
			t.Errorf("split %d duplicates user %q", index, split.UserID)
		}
		seen[split.UserID] = struct{}{}
		total.Add(total, big.NewInt(split.AmountCents))
	}
	if total.Cmp(big.NewInt(wantTotal)) != 0 {
		t.Errorf("split total = %s, want %d", total, wantTotal)
	}
}
