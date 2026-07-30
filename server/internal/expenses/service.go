package expenses

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Fyy10/settled/server/internal/clock"
	"github.com/Fyy10/settled/server/internal/identifier"
	"github.com/Fyy10/settled/server/internal/input"
	"github.com/Fyy10/settled/server/internal/money"
)

const (
	fieldPaidByUserID     = "paidByUserId"
	fieldExpenseDate      = "expenseDate"
	fieldSplitMode        = "splitMode"
	fieldParticipantIDs   = "participantUserIds"
	fieldSplits           = "splits"
	fieldPercentageSplits = "percentageSplits"
)

var ErrInvalidServiceConfiguration = errors.New(
	"invalid expense service configuration",
)

type Service struct {
	store  Store
	clock  clock.Clock
	random io.Reader
}

func NewService(store Store, serviceClock clock.Clock) (*Service, error) {
	return NewServiceFrom(store, serviceClock, rand.Reader)
}

func NewServiceFrom(
	store Store,
	serviceClock clock.Clock,
	random io.Reader,
) (*Service, error) {
	if store == nil || serviceClock == nil || random == nil {
		return nil, ErrInvalidServiceConfiguration
	}
	return &Service{
		store:  store,
		clock:  serviceClock,
		random: random,
	}, nil
}

func (service *Service) List(
	ctx context.Context,
	actorID string,
	groupID string,
) (ListResult, error) {
	return service.store.ListExpenses(ctx, actorID, groupID)
}

func (service *Service) Create(
	ctx context.Context,
	actorID string,
	groupID string,
	input MutationInput,
) (Expense, error) {
	validated, err := validateMutation(input)
	if err != nil {
		return Expense{}, err
	}
	expenseID, err := identifier.NewUUIDFrom(service.random)
	if err != nil {
		return Expense{}, fmt.Errorf("generate expense ID: %w", err)
	}
	now := service.clock.Now().UTC()
	return service.store.CreateExpense(ctx, CreateInput{
		ID:           expenseID,
		ActorID:      actorID,
		GroupID:      groupID,
		PaidByUserID: validated.paidByUserID,
		Description:  validated.description,
		AmountCents:  input.AmountCents,
		Currency:     money.CurrencyUSD,
		ExpenseDate:  validated.expenseDate,
		Splits:       validated.splits,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

func (service *Service) Get(
	ctx context.Context,
	actorID string,
	groupID string,
	expenseID string,
) (Expense, error) {
	return service.store.GetExpense(ctx, actorID, groupID, expenseID)
}

func (service *Service) Replace(
	ctx context.Context,
	actorID string,
	groupID string,
	expenseID string,
	input MutationInput,
) (Expense, error) {
	validated, err := validateMutation(input)
	if err != nil {
		return Expense{}, err
	}
	return service.store.ReplaceExpense(ctx, ReplaceInput{
		ActorID:      actorID,
		GroupID:      groupID,
		ExpenseID:    expenseID,
		PaidByUserID: validated.paidByUserID,
		Description:  validated.description,
		AmountCents:  input.AmountCents,
		Currency:     money.CurrencyUSD,
		ExpenseDate:  validated.expenseDate,
		Splits:       validated.splits,
		UpdatedAt:    service.clock.Now().UTC(),
	})
}

func (service *Service) Delete(
	ctx context.Context,
	actorID string,
	groupID string,
	expenseID string,
) error {
	return service.store.DeleteExpense(ctx, DeleteInput{
		ActorID:   actorID,
		GroupID:   groupID,
		ExpenseID: expenseID,
		DeletedAt: service.clock.Now().UTC(),
	})
}

type validatedMutation struct {
	paidByUserID string
	description  string
	expenseDate  time.Time
	splits       []Split
}

func validateMutation(value MutationInput) (validatedMutation, error) {
	paidByUserID, err := identifier.ParseUUID(value.PaidByUserID)
	if err != nil {
		return validatedMutation{}, fieldValidation(
			fieldPaidByUserID,
			"Paid-by user ID must be a valid UUID.",
		)
	}
	description, err := input.NormalizeExpenseDescription(value.Description)
	if err != nil {
		return validatedMutation{}, inputValidation(err)
	}
	if err := input.ValidateAmountCents(value.AmountCents); err != nil {
		return validatedMutation{}, inputValidation(err)
	}
	expenseDate, err := time.Parse(time.DateOnly, value.ExpenseDate)
	if err != nil {
		message := "Expense date must use YYYY-MM-DD."
		if value.ExpenseDate == "" {
			message = "Expense date is required."
		}
		return validatedMutation{}, fieldValidation(fieldExpenseDate, message)
	}
	splits, err := CalculateSplits(value.AmountCents, value.SplitInput)
	if err != nil {
		return validatedMutation{}, splitValidation(value.SplitInput.Mode, err)
	}
	return validatedMutation{
		paidByUserID: paidByUserID,
		description:  description,
		expenseDate:  expenseDate,
		splits:       splits,
	}, nil
}

func inputValidation(err error) error {
	var fieldError *input.FieldError
	if !errors.As(err, &fieldError) {
		return err
	}
	return fieldValidation(fieldError.Field, fieldError.Message)
}

func splitValidation(mode SplitMode, err error) error {
	field := splitField(mode)
	message := "Split input is invalid."

	switch {
	case errors.Is(err, ErrInvalidExpenseAmount):
		field = input.FieldAmountCents
		message = "Amount must be positive."
	case errors.Is(err, ErrUnsupportedSplitMode):
		field = fieldSplitMode
		message = "Split mode must be equal, exact, or percentage."
	case errors.Is(err, ErrConflictingSplitInputs):
		field = fieldSplitMode
		message = "Split input must contain fields only for the selected mode."
	case errors.Is(err, ErrParticipantsRequired):
		message = "At least one split participant is required."
	case errors.Is(err, ErrInvalidParticipantID):
		message = "Split participant IDs must be valid UUIDs."
	case errors.Is(err, ErrDuplicateParticipant):
		message = "Split participants must not contain duplicate users."
	case errors.Is(err, ErrSplitAmountNotPositive):
		field = fieldSplits
		message = "Exact split amounts must be positive."
	case errors.Is(err, ErrExactSplitTotalMismatch):
		field = fieldSplits
		message = "Exact split amounts must total amountCents."
	case errors.Is(err, ErrPercentageNotPositive):
		field = fieldPercentageSplits
		message = "Percentage basis points must be positive."
	case errors.Is(err, ErrPercentageTotalMismatch):
		field = fieldPercentageSplits
		message = "Percentage basis points must total 10000."
	case errors.Is(err, ErrCalculatedSplitNotPositive):
		message = "Every calculated split must be at least one cent."
	case errors.Is(err, ErrSplitArithmeticOverflow):
		message = "Split calculation exceeds supported limits."
	}
	return fieldValidation(field, message)
}

func splitField(mode SplitMode) string {
	switch mode {
	case SplitModeEqual:
		return fieldParticipantIDs
	case SplitModeExact:
		return fieldSplits
	case SplitModePercentage:
		return fieldPercentageSplits
	default:
		return fieldSplitMode
	}
}

func fieldValidation(field string, message string) error {
	return &ValidationError{Fields: map[string]string{field: message}}
}
