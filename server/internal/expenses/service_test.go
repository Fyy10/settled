package expenses

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/clock"
)

const (
	expenseServiceActorID   = "00112233-4455-4677-8899-aabbccddeeff"
	expenseServiceGroupID   = "11111111-2222-4333-8444-555555555555"
	expenseServiceExpenseID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
)

var expenseServiceNow = time.Date(
	2026,
	time.August,
	2,
	9,
	15,
	30,
	456789123,
	time.FixedZone("test", -7*60*60),
)

type expenseStoreStub struct {
	create  func(context.Context, CreateInput) (Expense, error)
	replace func(context.Context, ReplaceInput) (Expense, error)
	delete  func(context.Context, DeleteInput) error
	get     func(context.Context, string, string, string) (Expense, error)
	list    func(context.Context, string, string) (ListResult, error)
}

func (store expenseStoreStub) CreateExpense(
	ctx context.Context,
	input CreateInput,
) (Expense, error) {
	if store.create == nil {
		return Expense{}, errors.New("unexpected CreateExpense call")
	}
	return store.create(ctx, input)
}

func (store expenseStoreStub) ReplaceExpense(
	ctx context.Context,
	input ReplaceInput,
) (Expense, error) {
	if store.replace == nil {
		return Expense{}, errors.New("unexpected ReplaceExpense call")
	}
	return store.replace(ctx, input)
}

func (store expenseStoreStub) DeleteExpense(
	ctx context.Context,
	input DeleteInput,
) error {
	if store.delete == nil {
		return errors.New("unexpected DeleteExpense call")
	}
	return store.delete(ctx, input)
}

func (store expenseStoreStub) GetExpense(
	ctx context.Context,
	actorID string,
	groupID string,
	expenseID string,
) (Expense, error) {
	if store.get == nil {
		return Expense{}, errors.New("unexpected GetExpense call")
	}
	return store.get(ctx, actorID, groupID, expenseID)
}

func (store expenseStoreStub) ListExpenses(
	ctx context.Context,
	actorID string,
	groupID string,
) (ListResult, error) {
	if store.list == nil {
		return ListResult{}, errors.New("unexpected ListExpenses call")
	}
	return store.list(ctx, actorID, groupID)
}

func TestServiceCreateBuildsValidatedAtomicInput(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(
		context.Background(),
		expenseServiceContextKey{},
		"create",
	)
	wantExpense := expenseServiceTestExpense()
	storeCalls := 0
	service := newExpenseServiceForTest(
		t,
		expenseStoreStub{
			create: func(
				gotContext context.Context,
				input CreateInput,
			) (Expense, error) {
				storeCalls++
				if gotContext != ctx {
					t.Error("CreateExpense context was not preserved")
				}
				wantInput := CreateInput{
					ID:           "11111111-1111-4111-9111-111111111111",
					ActorID:      expenseServiceActorID,
					GroupID:      expenseServiceGroupID,
					PaidByUserID: participantD,
					Description:  "Groceries",
					AmountCents:  10,
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
					Splits: []Split{
						{UserID: participantC, AmountCents: 4},
						{UserID: participantA, AmountCents: 3},
						{UserID: participantB, AmountCents: 3},
					},
					CreatedAt: expenseServiceNow.UTC(),
					UpdatedAt: expenseServiceNow.UTC(),
				}
				if !reflect.DeepEqual(input, wantInput) {
					t.Errorf("CreateExpense input = %#v, want %#v", input, wantInput)
				}
				return wantExpense, nil
			},
		},
		bytes.NewReader(bytes.Repeat([]byte{0x11}, 16)),
	)

	got, err := service.Create(
		ctx,
		expenseServiceActorID,
		expenseServiceGroupID,
		MutationInput{
			PaidByUserID: strings.ToUpper(participantD),
			Description:  "  Groceries  ",
			AmountCents:  10,
			ExpenseDate:  "2026-07-31",
			SplitInput: SplitInput{
				Mode: SplitModeEqual,
				ParticipantUserIDs: []string{
					participantC,
					participantA,
					participantB,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !reflect.DeepEqual(got, wantExpense) {
		t.Errorf("expense = %#v, want %#v", got, wantExpense)
	}
	if storeCalls != 1 {
		t.Errorf("CreateExpense calls = %d, want 1", storeCalls)
	}
}

func TestServiceCreateCalculatesEverySplitModeBeforeStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input SplitInput
		want  []Split
	}{
		{
			name: "equal",
			input: SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{participantA, participantB, participantC},
			},
			want: []Split{
				{UserID: participantA, AmountCents: 4},
				{UserID: participantB, AmountCents: 3},
				{UserID: participantC, AmountCents: 3},
			},
		},
		{
			name: "exact",
			input: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: participantB, AmountCents: 6},
					{UserID: participantA, AmountCents: 4},
				},
			},
			want: []Split{
				{UserID: participantB, AmountCents: 6},
				{UserID: participantA, AmountCents: 4},
			},
		},
		{
			name: "percentage",
			input: SplitInput{
				Mode: SplitModePercentage,
				PercentageSplits: []PercentageSplitInput{
					{UserID: participantC, PercentageBasisPoints: 3_334},
					{UserID: participantA, PercentageBasisPoints: 3_333},
					{UserID: participantB, PercentageBasisPoints: 3_333},
				},
			},
			want: []Split{
				{UserID: participantC, AmountCents: 4},
				{UserID: participantA, AmountCents: 3},
				{UserID: participantB, AmountCents: 3},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newExpenseServiceForTest(
				t,
				expenseStoreStub{
					create: func(
						_ context.Context,
						input CreateInput,
					) (Expense, error) {
						if !reflect.DeepEqual(input.Splits, test.want) {
							t.Errorf("splits = %#v, want %#v", input.Splits, test.want)
						}
						return Expense{Splits: input.Splits}, nil
					},
				},
				bytes.NewReader(bytes.Repeat([]byte{0x12}, 16)),
			)
			_, err := service.Create(
				context.Background(),
				expenseServiceActorID,
				expenseServiceGroupID,
				validExpenseMutation(test.input),
			)
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
		})
	}
}

func TestServiceMutationValidationPreventsStoreAndRandomUse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*MutationInput)
		wantField string
	}{
		{
			name: "payer required",
			mutate: func(input *MutationInput) {
				input.PaidByUserID = ""
			},
			wantField: fieldPaidByUserID,
		},
		{
			name: "description required",
			mutate: func(input *MutationInput) {
				input.Description = " "
			},
			wantField: "description",
		},
		{
			name: "amount positive",
			mutate: func(input *MutationInput) {
				input.AmountCents = 0
			},
			wantField: "amountCents",
		},
		{
			name: "date required",
			mutate: func(input *MutationInput) {
				input.ExpenseDate = ""
			},
			wantField: fieldExpenseDate,
		},
		{
			name: "date exact format",
			mutate: func(input *MutationInput) {
				input.ExpenseDate = "07/31/2026"
			},
			wantField: fieldExpenseDate,
		},
		{
			name: "split mode required",
			mutate: func(input *MutationInput) {
				input.SplitInput = SplitInput{}
			},
			wantField: fieldSplitMode,
		},
		{
			name: "participants required",
			mutate: func(input *MutationInput) {
				input.SplitInput.ParticipantUserIDs = nil
			},
			wantField: fieldParticipantIDs,
		},
		{
			name: "duplicate participants",
			mutate: func(input *MutationInput) {
				input.SplitInput.ParticipantUserIDs = []string{
					participantA,
					participantA,
				}
			},
			wantField: fieldParticipantIDs,
		},
		{
			name: "conflicting split inputs",
			mutate: func(input *MutationInput) {
				input.SplitInput.Splits = []ExactSplitInput{{
					UserID:      participantA,
					AmountCents: input.AmountCents,
				}}
			},
			wantField: fieldSplitMode,
		},
		{
			name: "exact total",
			mutate: func(input *MutationInput) {
				input.SplitInput = SplitInput{
					Mode: SplitModeExact,
					Splits: []ExactSplitInput{{
						UserID:      participantA,
						AmountCents: input.AmountCents - 1,
					}},
				}
			},
			wantField: fieldSplits,
		},
		{
			name: "percentage total",
			mutate: func(input *MutationInput) {
				input.SplitInput = SplitInput{
					Mode: SplitModePercentage,
					PercentageSplits: []PercentageSplitInput{{
						UserID:                participantA,
						PercentageBasisPoints: 9_999,
					}},
				}
			},
			wantField: fieldPercentageSplits,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validExpenseMutation(SplitInput{
				Mode:               SplitModeEqual,
				ParticipantUserIDs: []string{participantA},
			})
			test.mutate(&input)
			service := newExpenseServiceForTest(
				t,
				expenseStoreStub{
					create: func(
						context.Context,
						CreateInput,
					) (Expense, error) {
						t.Fatal("CreateExpense was called")
						return Expense{}, nil
					},
					replace: func(
						context.Context,
						ReplaceInput,
					) (Expense, error) {
						t.Fatal("ReplaceExpense was called")
						return Expense{}, nil
					},
				},
				expensePanicReader{},
			)

			_, createErr := service.Create(
				context.Background(),
				expenseServiceActorID,
				expenseServiceGroupID,
				input,
			)
			assertExpenseValidationField(t, createErr, test.wantField)

			_, replaceErr := service.Replace(
				context.Background(),
				expenseServiceActorID,
				expenseServiceGroupID,
				expenseServiceExpenseID,
				input,
			)
			assertExpenseValidationField(t, replaceErr, test.wantField)
		})
	}
}

func TestServiceReplaceBuildsFullReplacementInput(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(
		context.Background(),
		expenseServiceContextKey{},
		"replace",
	)
	wantExpense := expenseServiceTestExpense()
	service := newExpenseServiceForTest(
		t,
		expenseStoreStub{
			replace: func(
				gotContext context.Context,
				input ReplaceInput,
			) (Expense, error) {
				if gotContext != ctx {
					t.Error("ReplaceExpense context was not preserved")
				}
				wantInput := ReplaceInput{
					ActorID:      expenseServiceActorID,
					GroupID:      expenseServiceGroupID,
					ExpenseID:    expenseServiceExpenseID,
					PaidByUserID: participantA,
					Description:  "Dinner",
					AmountCents:  10,
					Currency:     "USD",
					ExpenseDate: time.Date(
						2026,
						time.August,
						1,
						0,
						0,
						0,
						0,
						time.UTC,
					),
					Splits: []Split{
						{UserID: participantB, AmountCents: 6},
						{UserID: participantA, AmountCents: 4},
					},
					UpdatedAt: expenseServiceNow.UTC(),
				}
				if !reflect.DeepEqual(input, wantInput) {
					t.Errorf("ReplaceExpense input = %#v, want %#v", input, wantInput)
				}
				return wantExpense, nil
			},
		},
		expensePanicReader{},
	)

	got, err := service.Replace(
		ctx,
		expenseServiceActorID,
		expenseServiceGroupID,
		expenseServiceExpenseID,
		MutationInput{
			PaidByUserID: strings.ToUpper(participantA),
			Description:  " Dinner ",
			AmountCents:  10,
			ExpenseDate:  "2026-08-01",
			SplitInput: SplitInput{
				Mode: SplitModeExact,
				Splits: []ExactSplitInput{
					{UserID: participantB, AmountCents: 6},
					{UserID: participantA, AmountCents: 4},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if !reflect.DeepEqual(got, wantExpense) {
		t.Errorf("expense = %#v, want %#v", got, wantExpense)
	}
}

func TestServiceReadAndDeleteDelegation(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(
		context.Background(),
		expenseServiceContextKey{},
		"read-delete",
	)
	wantExpense := expenseServiceTestExpense()
	wantList := ListResult{
		Expenses: []Expense{wantExpense},
		Members: []MemberSummary{{
			UserID:      expenseServiceActorID,
			DisplayName: "Alice",
		}},
	}
	var calls []string
	service := newExpenseServiceForTest(
		t,
		expenseStoreStub{
			list: func(
				gotContext context.Context,
				actorID string,
				groupID string,
			) (ListResult, error) {
				calls = append(calls, "list")
				if gotContext != ctx ||
					actorID != expenseServiceActorID ||
					groupID != expenseServiceGroupID {
					t.Errorf("ListExpenses inputs = (%v, %q, %q)", gotContext, actorID, groupID)
				}
				return wantList, nil
			},
			get: func(
				gotContext context.Context,
				actorID string,
				groupID string,
				expenseID string,
			) (Expense, error) {
				calls = append(calls, "get")
				if gotContext != ctx ||
					actorID != expenseServiceActorID ||
					groupID != expenseServiceGroupID ||
					expenseID != expenseServiceExpenseID {
					t.Errorf(
						"GetExpense inputs = (%v, %q, %q, %q)",
						gotContext,
						actorID,
						groupID,
						expenseID,
					)
				}
				return wantExpense, nil
			},
			delete: func(
				gotContext context.Context,
				input DeleteInput,
			) error {
				calls = append(calls, "delete")
				wantInput := DeleteInput{
					ActorID:   expenseServiceActorID,
					GroupID:   expenseServiceGroupID,
					ExpenseID: expenseServiceExpenseID,
					DeletedAt: expenseServiceNow.UTC(),
				}
				if gotContext != ctx || !reflect.DeepEqual(input, wantInput) {
					t.Errorf("DeleteExpense input = (%v, %#v), want (%v, %#v)", gotContext, input, ctx, wantInput)
				}
				return nil
			},
		},
		expensePanicReader{},
	)

	gotList, err := service.List(ctx, expenseServiceActorID, expenseServiceGroupID)
	if err != nil || !reflect.DeepEqual(gotList, wantList) {
		t.Errorf("List = (%#v, %v), want (%#v, nil)", gotList, err, wantList)
	}
	gotExpense, err := service.Get(
		ctx,
		expenseServiceActorID,
		expenseServiceGroupID,
		expenseServiceExpenseID,
	)
	if err != nil || !reflect.DeepEqual(gotExpense, wantExpense) {
		t.Errorf("Get = (%#v, %v), want (%#v, nil)", gotExpense, err, wantExpense)
	}
	if err := service.Delete(
		ctx,
		expenseServiceActorID,
		expenseServiceGroupID,
		expenseServiceExpenseID,
	); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"list", "get", "delete"}) {
		t.Errorf("calls = %v", calls)
	}
}

func TestServicePropagatesStoreAndRandomErrors(t *testing.T) {
	t.Parallel()

	storeError := errors.New("store unavailable")
	service := newExpenseServiceForTest(
		t,
		expenseStoreStub{
			create: func(context.Context, CreateInput) (Expense, error) {
				return Expense{}, storeError
			},
			replace: func(context.Context, ReplaceInput) (Expense, error) {
				return Expense{}, storeError
			},
			delete: func(context.Context, DeleteInput) error {
				return storeError
			},
			get: func(
				context.Context,
				string,
				string,
				string,
			) (Expense, error) {
				return Expense{}, storeError
			},
			list: func(context.Context, string, string) (ListResult, error) {
				return ListResult{}, storeError
			},
		},
		bytes.NewReader(bytes.Repeat([]byte{0x13}, 16)),
	)
	mutation := validExpenseMutation(SplitInput{
		Mode:               SplitModeEqual,
		ParticipantUserIDs: []string{participantA},
	})

	if _, err := service.Create(
		context.Background(),
		expenseServiceActorID,
		expenseServiceGroupID,
		mutation,
	); !errors.Is(err, storeError) {
		t.Errorf("Create error = %v, want store error", err)
	}
	if _, err := service.Replace(
		context.Background(),
		expenseServiceActorID,
		expenseServiceGroupID,
		expenseServiceExpenseID,
		mutation,
	); !errors.Is(err, storeError) {
		t.Errorf("Replace error = %v, want store error", err)
	}
	if err := service.Delete(
		context.Background(),
		expenseServiceActorID,
		expenseServiceGroupID,
		expenseServiceExpenseID,
	); !errors.Is(err, storeError) {
		t.Errorf("Delete error = %v, want store error", err)
	}
	if _, err := service.Get(
		context.Background(),
		expenseServiceActorID,
		expenseServiceGroupID,
		expenseServiceExpenseID,
	); !errors.Is(err, storeError) {
		t.Errorf("Get error = %v, want store error", err)
	}
	if _, err := service.List(
		context.Background(),
		expenseServiceActorID,
		expenseServiceGroupID,
	); !errors.Is(err, storeError) {
		t.Errorf("List error = %v, want store error", err)
	}

	randomError := errors.New("entropy unavailable")
	randomService := newExpenseServiceForTest(
		t,
		expenseStoreStub{
			create: func(context.Context, CreateInput) (Expense, error) {
				t.Fatal("CreateExpense was called after random failure")
				return Expense{}, nil
			},
		},
		expenseErrorReader{err: randomError},
	)
	if _, err := randomService.Create(
		context.Background(),
		expenseServiceActorID,
		expenseServiceGroupID,
		mutation,
	); !errors.Is(err, randomError) {
		t.Errorf("random error = %v, want wrapped %v", err, randomError)
	}
}

func TestNewServiceRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	validStore := expenseStoreStub{}
	validClock := clock.Fixed{Time: expenseServiceNow}
	tests := []struct {
		name   string
		store  Store
		clock  clock.Clock
		random io.Reader
	}{
		{
			name:   "nil store",
			clock:  validClock,
			random: expensePanicReader{},
		},
		{
			name:   "nil clock",
			store:  validStore,
			random: expensePanicReader{},
		},
		{
			name:  "nil random",
			store: validStore,
			clock: validClock,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewServiceFrom(test.store, test.clock, test.random)
			if !errors.Is(err, ErrInvalidServiceConfiguration) {
				t.Errorf("error = %v, want ErrInvalidServiceConfiguration", err)
			}
			if service != nil {
				t.Errorf("service = %#v, want nil", service)
			}
		})
	}
}

func validExpenseMutation(splitInput SplitInput) MutationInput {
	return MutationInput{
		PaidByUserID: participantA,
		Description:  "Expense",
		AmountCents:  10,
		ExpenseDate:  "2026-07-31",
		SplitInput:   splitInput,
	}
}

func newExpenseServiceForTest(
	t *testing.T,
	store Store,
	random io.Reader,
) *Service {
	t.Helper()
	service, err := NewServiceFrom(
		store,
		clock.Fixed{Time: expenseServiceNow},
		random,
	)
	if err != nil {
		t.Fatalf("NewServiceFrom: %v", err)
	}
	return service
}

func expenseServiceTestExpense() Expense {
	return Expense{
		ID:              expenseServiceExpenseID,
		GroupID:         expenseServiceGroupID,
		PaidByUserID:    participantA,
		Description:     "Expense",
		AmountCents:     10,
		Currency:        "USD",
		ExpenseDate:     time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC),
		CreatedByUserID: expenseServiceActorID,
		Splits: []Split{{
			UserID:      participantA,
			AmountCents: 10,
		}},
		CreatedAt: expenseServiceNow.UTC(),
		UpdatedAt: expenseServiceNow.UTC(),
	}
}

func assertExpenseValidationField(
	t *testing.T,
	err error,
	wantField string,
) {
	t.Helper()
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
	if len(validation.Fields) != 1 || validation.Fields[wantField] == "" {
		t.Errorf("validation fields = %v, want only %q", validation.Fields, wantField)
	}
}

type expenseServiceContextKey struct{}

type expensePanicReader struct{}

func (expensePanicReader) Read([]byte) (int, error) {
	panic("unexpected random read")
}

type expenseErrorReader struct {
	err error
}

func (reader expenseErrorReader) Read([]byte) (int, error) {
	return 0, reader.err
}
