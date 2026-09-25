package settlements

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type settlementStoreFunc func(
	context.Context,
	string,
	string,
) ([]DebtEntry, []MemberSummary, error)

func (store settlementStoreFunc) ListDebtEntries(
	ctx context.Context,
	actorID string,
	groupID string,
) ([]DebtEntry, []MemberSummary, error) {
	return store(ctx, actorID, groupID)
}

type calculatorFunc func([]DebtEntry) ([]Transfer, error)

func (calculate calculatorFunc) Calculate(
	entries []DebtEntry,
) ([]Transfer, error) {
	return calculate(entries)
}

func TestServiceListLoadsRawEntriesAndInvokesCalculator(t *testing.T) {
	t.Parallel()

	entries := []DebtEntry{
		debtEntry(settlementUserB, settlementUserA, 3000),
		debtEntry(settlementUserA, settlementUserB, 2200),
	}
	members := []MemberSummary{
		{UserID: settlementUserA, DisplayName: "Alice"},
		{UserID: settlementUserB, DisplayName: "Bob"},
	}
	transfers := []Transfer{
		transfer(settlementUserB, settlementUserA, 800),
	}
	storeCalls := 0
	calculatorCalls := 0
	service, err := NewService(
		settlementStoreFunc(func(
			ctx context.Context,
			actorID string,
			groupID string,
		) ([]DebtEntry, []MemberSummary, error) {
			storeCalls++
			if ctx == nil ||
				actorID != settlementUserC ||
				groupID != settlementUserD {
				t.Errorf(
					"store inputs = (%v, %q, %q)",
					ctx,
					actorID,
					groupID,
				)
			}
			return entries, members, nil
		}),
		calculatorFunc(func(got []DebtEntry) ([]Transfer, error) {
			calculatorCalls++
			if !reflect.DeepEqual(got, entries) {
				t.Errorf("calculator entries = %#v, want %#v", got, entries)
			}
			return transfers, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	got, err := service.List(
		context.Background(),
		settlementUserC,
		settlementUserD,
	)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := Result{
		Transfers: transfers,
		Members:   members,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("result = %#v, want %#v", got, want)
	}
	if storeCalls != 1 || calculatorCalls != 1 {
		t.Errorf(
			"calls = (store %d, calculator %d), want (1, 1)",
			storeCalls,
			calculatorCalls,
		)
	}
}

func TestServiceListPreservesStoreErrorWithoutCalculating(t *testing.T) {
	t.Parallel()

	calculatorCalls := 0
	service, err := NewService(
		settlementStoreFunc(func(
			context.Context,
			string,
			string,
		) ([]DebtEntry, []MemberSummary, error) {
			return nil, nil, ErrNotFound
		}),
		calculatorFunc(func([]DebtEntry) ([]Transfer, error) {
			calculatorCalls++
			return nil, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.List(
		context.Background(),
		settlementUserA,
		settlementUserD,
	)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if calculatorCalls != 0 {
		t.Errorf("calculator calls = %d, want 0", calculatorCalls)
	}
}

func TestServiceListWrapsCalculatorError(t *testing.T) {
	t.Parallel()

	service, err := NewService(
		settlementStoreFunc(func(
			context.Context,
			string,
			string,
		) ([]DebtEntry, []MemberSummary, error) {
			return []DebtEntry{
				debtEntry(settlementUserA, settlementUserB, 1),
			}, nil, nil
		}),
		calculatorFunc(func([]DebtEntry) ([]Transfer, error) {
			return nil, ErrArithmeticOverflow
		}),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.List(
		context.Background(),
		settlementUserA,
		settlementUserD,
	)
	if !errors.Is(err, ErrArithmeticOverflow) {
		t.Errorf("error = %v, want ErrArithmeticOverflow", err)
	}
}

func TestServiceListNormalizesNilResultSlices(t *testing.T) {
	t.Parallel()

	service, err := NewService(
		settlementStoreFunc(func(
			context.Context,
			string,
			string,
		) ([]DebtEntry, []MemberSummary, error) {
			return nil, nil, nil
		}),
		calculatorFunc(func(entries []DebtEntry) ([]Transfer, error) {
			if entries != nil {
				t.Errorf("entries = %#v, want nil", entries)
			}
			return nil, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	got, err := service.List(
		context.Background(),
		settlementUserA,
		settlementUserD,
	)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Transfers == nil ||
		len(got.Transfers) != 0 ||
		got.Members == nil ||
		len(got.Members) != 0 {
		t.Errorf("result = %#v, want non-nil empty slices", got)
	}
}

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	validStore := settlementStoreFunc(func(
		context.Context,
		string,
		string,
	) ([]DebtEntry, []MemberSummary, error) {
		return nil, nil, nil
	})
	validCalculator := calculatorFunc(func(
		[]DebtEntry,
	) ([]Transfer, error) {
		return nil, nil
	})
	tests := []struct {
		name       string
		store      Store
		calculator Calculator
	}{
		{name: "missing store", calculator: validCalculator},
		{name: "missing calculator", store: validStore},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service, err := NewService(test.store, test.calculator)
			if !errors.Is(err, ErrInvalidServiceConfiguration) {
				t.Errorf(
					"error = %v, want ErrInvalidServiceConfiguration",
					err,
				)
			}
			if service != nil {
				t.Errorf("service = %#v, want nil", service)
			}
		})
	}
}
