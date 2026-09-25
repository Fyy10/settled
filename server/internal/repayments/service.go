package repayments

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
	fieldFromUserID    = "fromUserId"
	fieldToUserID      = "toUserId"
	fieldNote          = "note"
	fieldRepaymentDate = "repaymentDate"
)

var ErrInvalidServiceConfiguration = errors.New(
	"invalid repayment service configuration",
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
	return service.store.ListRepayments(ctx, actorID, groupID)
}

func (service *Service) Create(
	ctx context.Context,
	actorID string,
	groupID string,
	input MutationInput,
) (Repayment, error) {
	validated, err := validateMutation(input, false)
	if err != nil {
		return Repayment{}, err
	}
	repaymentID, err := identifier.NewUUIDFrom(service.random)
	if err != nil {
		return Repayment{}, fmt.Errorf("generate repayment ID: %w", err)
	}
	now := service.clock.Now().UTC()
	return service.store.CreateRepayment(ctx, CreateInput{
		ID:            repaymentID,
		ActorID:       actorID,
		GroupID:       groupID,
		FromUserID:    validated.fromUserID,
		ToUserID:      validated.toUserID,
		AmountCents:   input.AmountCents,
		Currency:      money.CurrencyUSD,
		Note:          validated.note,
		RepaymentDate: validated.repaymentDate,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
}

func (service *Service) Get(
	ctx context.Context,
	actorID string,
	groupID string,
	repaymentID string,
) (Repayment, error) {
	return service.store.GetRepayment(
		ctx,
		actorID,
		groupID,
		repaymentID,
	)
}

func (service *Service) Replace(
	ctx context.Context,
	actorID string,
	groupID string,
	repaymentID string,
	input MutationInput,
) (Repayment, error) {
	validated, err := validateMutation(input, true)
	if err != nil {
		return Repayment{}, err
	}
	return service.store.ReplaceRepayment(ctx, ReplaceInput{
		ActorID:       actorID,
		GroupID:       groupID,
		RepaymentID:   repaymentID,
		FromUserID:    validated.fromUserID,
		ToUserID:      validated.toUserID,
		AmountCents:   input.AmountCents,
		Currency:      money.CurrencyUSD,
		Note:          validated.note,
		RepaymentDate: validated.repaymentDate,
		UpdatedAt:     service.clock.Now().UTC(),
	})
}

func (service *Service) Delete(
	ctx context.Context,
	actorID string,
	groupID string,
	repaymentID string,
) error {
	return service.store.DeleteRepayment(ctx, DeleteInput{
		ActorID:     actorID,
		GroupID:     groupID,
		RepaymentID: repaymentID,
		DeletedAt:   service.clock.Now().UTC(),
	})
}

type validatedMutation struct {
	fromUserID    string
	toUserID      string
	note          *string
	repaymentDate time.Time
}

func validateMutation(
	value MutationInput,
	requireNote bool,
) (validatedMutation, error) {
	fromUserID, err := identifier.ParseUUID(value.FromUserID)
	if err != nil {
		return validatedMutation{}, fieldValidation(
			fieldFromUserID,
			"Sender user ID must be a valid UUID.",
		)
	}
	toUserID, err := identifier.ParseUUID(value.ToUserID)
	if err != nil {
		return validatedMutation{}, fieldValidation(
			fieldToUserID,
			"Recipient user ID must be a valid UUID.",
		)
	}
	if fromUserID == toUserID {
		return validatedMutation{}, fieldValidation(
			fieldToUserID,
			"Recipient must be different from sender.",
		)
	}
	if err := input.ValidateAmountCents(value.AmountCents); err != nil {
		return validatedMutation{}, inputValidation(err)
	}
	if requireNote && !value.NotePresent {
		return validatedMutation{}, fieldValidation(
			fieldNote,
			"Note must be included in a full replacement.",
		)
	}
	note, err := input.NormalizeRepaymentNote(value.Note)
	if err != nil {
		return validatedMutation{}, inputValidation(err)
	}
	repaymentDate, err := time.Parse(time.DateOnly, value.RepaymentDate)
	if err != nil {
		message := "Repayment date must use YYYY-MM-DD."
		if value.RepaymentDate == "" {
			message = "Repayment date is required."
		}
		return validatedMutation{}, fieldValidation(fieldRepaymentDate, message)
	}
	return validatedMutation{
		fromUserID:    fromUserID,
		toUserID:      toUserID,
		note:          note,
		repaymentDate: repaymentDate,
	}, nil
}

func inputValidation(err error) error {
	var fieldError *input.FieldError
	if !errors.As(err, &fieldError) {
		return err
	}
	return fieldValidation(fieldError.Field, fieldError.Message)
}

func fieldValidation(field string, message string) error {
	return &ValidationError{Fields: map[string]string{field: message}}
}
