package expenses

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/Fyy10/settled/server/internal/identifier"
	"github.com/Fyy10/settled/server/internal/money"
)

const percentageBasisPointTotal int64 = 10_000

var (
	ErrInvalidExpenseAmount       = errors.New("expense amount must be positive")
	ErrUnsupportedSplitMode       = errors.New("unsupported split mode")
	ErrConflictingSplitInputs     = errors.New("split input contains fields for another mode")
	ErrParticipantsRequired       = errors.New("at least one split participant is required")
	ErrInvalidParticipantID       = errors.New("split participant ID is invalid")
	ErrDuplicateParticipant       = errors.New("split participant is duplicated")
	ErrSplitAmountNotPositive     = errors.New("exact split amount must be positive")
	ErrExactSplitTotalMismatch    = errors.New("exact split total must equal the expense amount")
	ErrPercentageNotPositive      = errors.New("percentage basis points must be positive")
	ErrPercentageTotalMismatch    = errors.New("percentage basis points must total 10000")
	ErrSplitArithmeticOverflow    = errors.New("split arithmetic overflow")
	ErrCalculatedSplitNotPositive = errors.New("calculated split amount must be positive")
)

// CalculateSplits converts one discriminated split input to exact cent amounts.
func CalculateSplits(amountCents int64, input SplitInput) ([]Split, error) {
	if err := money.ValidatePositive(amountCents); err != nil {
		return nil, ErrInvalidExpenseAmount
	}
	if err := validateSplitVariant(input); err != nil {
		return nil, err
	}

	switch input.Mode {
	case SplitModeEqual:
		return calculateEqualSplits(amountCents, input.ParticipantUserIDs)
	case SplitModeExact:
		return calculateExactSplits(amountCents, input.Splits)
	case SplitModePercentage:
		return calculatePercentageSplits(amountCents, input.PercentageSplits)
	default:
		return nil, ErrUnsupportedSplitMode
	}
}

func validateSplitVariant(input SplitInput) error {
	switch input.Mode {
	case SplitModeEqual:
		if len(input.Splits) != 0 || len(input.PercentageSplits) != 0 {
			return ErrConflictingSplitInputs
		}
	case SplitModeExact:
		if len(input.ParticipantUserIDs) != 0 ||
			len(input.PercentageSplits) != 0 {
			return ErrConflictingSplitInputs
		}
	case SplitModePercentage:
		if len(input.ParticipantUserIDs) != 0 || len(input.Splits) != 0 {
			return ErrConflictingSplitInputs
		}
	default:
		return ErrUnsupportedSplitMode
	}
	return nil
}

func calculateEqualSplits(
	amountCents int64,
	participantUserIDs []string,
) ([]Split, error) {
	userIDs, err := normalizeParticipantIDs(participantUserIDs)
	if err != nil {
		return nil, err
	}

	participantCount := int64(len(userIDs))
	if amountCents < participantCount {
		return nil, ErrCalculatedSplitNotPositive
	}

	base := amountCents / participantCount
	remainder := amountCents % participantCount
	amounts := make([]int64, len(userIDs))
	for index := range amounts {
		amounts[index] = base
		if int64(index) < remainder {
			amounts[index]++
		}
	}

	return exactSplits(amountCents, userIDs, amounts)
}

func calculateExactSplits(
	amountCents int64,
	inputs []ExactSplitInput,
) ([]Split, error) {
	rawUserIDs := make([]string, len(inputs))
	for index := range inputs {
		rawUserIDs[index] = inputs[index].UserID
	}
	userIDs, err := normalizeParticipantIDs(rawUserIDs)
	if err != nil {
		return nil, err
	}

	amounts := make([]int64, len(inputs))
	var total int64
	for index, input := range inputs {
		if err := money.ValidatePositive(input.AmountCents); err != nil {
			return nil, fmt.Errorf(
				"%w at index %d",
				ErrSplitAmountNotPositive,
				index,
			)
		}
		total, err = money.Add(total, input.AmountCents)
		if err != nil {
			return nil, ErrSplitArithmeticOverflow
		}
		amounts[index] = input.AmountCents
	}
	if total != amountCents {
		return nil, ErrExactSplitTotalMismatch
	}

	return exactSplits(amountCents, userIDs, amounts)
}

func calculatePercentageSplits(
	amountCents int64,
	inputs []PercentageSplitInput,
) ([]Split, error) {
	rawUserIDs := make([]string, len(inputs))
	for index := range inputs {
		rawUserIDs[index] = inputs[index].UserID
	}
	userIDs, err := normalizeParticipantIDs(rawUserIDs)
	if err != nil {
		return nil, err
	}

	var basisPointTotal int64
	for index, input := range inputs {
		if input.PercentageBasisPoints <= 0 {
			return nil, fmt.Errorf(
				"%w at index %d",
				ErrPercentageNotPositive,
				index,
			)
		}
		basisPointTotal, err = money.Add(
			basisPointTotal,
			input.PercentageBasisPoints,
		)
		if err != nil {
			return nil, ErrSplitArithmeticOverflow
		}
	}
	if basisPointTotal != percentageBasisPointTotal {
		return nil, ErrPercentageTotalMismatch
	}

	denominator := big.NewInt(percentageBasisPointTotal)
	total := big.NewInt(amountCents)
	amounts := make([]int64, len(inputs))
	var floorTotal int64
	for index, input := range inputs {
		product := new(big.Int).Mul(
			total,
			big.NewInt(input.PercentageBasisPoints),
		)
		quotient := new(big.Int).Quo(product, denominator)
		if !quotient.IsInt64() {
			return nil, ErrSplitArithmeticOverflow
		}
		amounts[index] = quotient.Int64()
		floorTotal, err = money.Add(floorTotal, amounts[index])
		if err != nil {
			return nil, ErrSplitArithmeticOverflow
		}
	}

	remainder := amountCents - floorTotal
	if remainder < 0 || remainder >= int64(len(amounts)) {
		return nil, ErrSplitArithmeticOverflow
	}
	for index := int64(0); index < remainder; index++ {
		amounts[index]++
	}

	return exactSplits(amountCents, userIDs, amounts)
}

func normalizeParticipantIDs(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, ErrParticipantsRequired
	}

	normalized := make([]string, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		userID, err := identifier.ParseUUID(value)
		if err != nil {
			return nil, fmt.Errorf("%w at index %d", ErrInvalidParticipantID, index)
		}
		if _, exists := seen[userID]; exists {
			return nil, fmt.Errorf("%w at index %d", ErrDuplicateParticipant, index)
		}
		seen[userID] = struct{}{}
		normalized[index] = userID
	}
	return normalized, nil
}

func exactSplits(
	amountCents int64,
	userIDs []string,
	amounts []int64,
) ([]Split, error) {
	splits := make([]Split, len(userIDs))
	var total int64
	for index, userID := range userIDs {
		if err := money.ValidatePositive(amounts[index]); err != nil {
			return nil, fmt.Errorf(
				"%w at index %d",
				ErrCalculatedSplitNotPositive,
				index,
			)
		}
		var err error
		total, err = money.Add(total, amounts[index])
		if err != nil {
			return nil, ErrSplitArithmeticOverflow
		}
		splits[index] = Split{
			UserID:      userID,
			AmountCents: amounts[index],
		}
	}
	if total != amountCents {
		return nil, ErrExactSplitTotalMismatch
	}
	return splits, nil
}
