package money

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestCurrencyUSD(t *testing.T) {
	t.Parallel()

	if CurrencyUSD != "USD" {
		t.Errorf("CurrencyUSD = %q, want USD", CurrencyUSD)
	}
}

func TestValidatePositive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cents   int64
		wantErr bool
	}{
		{name: "minimum positive", cents: 1},
		{name: "maximum positive", cents: math.MaxInt64},
		{name: "zero", wantErr: true},
		{name: "negative one", cents: -1, wantErr: true},
		{name: "minimum integer", cents: math.MinInt64, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePositive(test.cents)
			if test.wantErr && !errors.Is(err, ErrNotPositive) {
				t.Errorf("error = %v, want ErrNotPositive", err)
			}
			if !test.wantErr && err != nil {
				t.Errorf("error = %v, want nil", err)
			}
		})
	}
}

func TestAddMatchesArbitraryPrecisionAtBoundaries(t *testing.T) {
	t.Parallel()

	values := boundaryValues()
	for _, left := range values {
		for _, right := range values {
			want := new(big.Int).Add(big.NewInt(left), big.NewInt(right))
			got, err := Add(left, right)
			if !want.IsInt64() {
				if !errors.Is(err, ErrOverflow) {
					t.Errorf("Add(%d, %d) error = %v, want ErrOverflow", left, right, err)
				}
				continue
			}
			if err != nil {
				t.Errorf("Add(%d, %d) error = %v", left, right, err)
				continue
			}
			if got != want.Int64() {
				t.Errorf("Add(%d, %d) = %d, want %d", left, right, got, want.Int64())
			}
		}
	}
}

func TestMultiplyMatchesArbitraryPrecisionAtBoundaries(t *testing.T) {
	t.Parallel()

	values := boundaryValues()
	for _, left := range values {
		for _, right := range values {
			want := new(big.Int).Mul(big.NewInt(left), big.NewInt(right))
			got, err := Multiply(left, right)
			if !want.IsInt64() {
				if !errors.Is(err, ErrOverflow) {
					t.Errorf(
						"Multiply(%d, %d) error = %v, want ErrOverflow",
						left,
						right,
						err,
					)
				}
				continue
			}
			if err != nil {
				t.Errorf("Multiply(%d, %d) error = %v", left, right, err)
				continue
			}
			if got != want.Int64() {
				t.Errorf(
					"Multiply(%d, %d) = %d, want %d",
					left,
					right,
					got,
					want.Int64(),
				)
			}
		}
	}
}

func boundaryValues() []int64 {
	return []int64{
		math.MinInt64,
		math.MinInt64 + 1,
		-3037000500,
		-3037000499,
		-2,
		-1,
		0,
		1,
		2,
		3037000499,
		3037000500,
		math.MaxInt64 - 1,
		math.MaxInt64,
	}
}
