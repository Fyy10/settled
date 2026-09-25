package money

import (
	"errors"
	"math"
)

const CurrencyUSD = "USD"

var (
	ErrNotPositive = errors.New("money must be positive")
	ErrOverflow    = errors.New("money arithmetic overflow")
)

func ValidatePositive(cents int64) error {
	if cents <= 0 {
		return ErrNotPositive
	}
	return nil
}

func Add(left, right int64) (int64, error) {
	if (right > 0 && left > math.MaxInt64-right) ||
		(right < 0 && left < math.MinInt64-right) {
		return 0, ErrOverflow
	}
	return left + right, nil
}

func Multiply(left, right int64) (int64, error) {
	switch {
	case left == 0 || right == 0:
		return 0, nil
	case left == -1:
		if right == math.MinInt64 {
			return 0, ErrOverflow
		}
	case right == -1:
		if left == math.MinInt64 {
			return 0, ErrOverflow
		}
	case left > 0 && right > 0:
		if left > math.MaxInt64/right {
			return 0, ErrOverflow
		}
	case left > 0 && right < 0:
		if right < math.MinInt64/left {
			return 0, ErrOverflow
		}
	case left < 0 && right > 0:
		if left < math.MinInt64/right {
			return 0, ErrOverflow
		}
	case left < 0 && right < 0:
		if left < math.MaxInt64/right {
			return 0, ErrOverflow
		}
	}

	return left * right, nil
}
