package settlements

import (
	"context"
	"errors"
	"fmt"
)

var ErrInvalidServiceConfiguration = errors.New(
	"invalid settlement service configuration",
)

type Service struct {
	store      Store
	calculator Calculator
}

func NewService(store Store, calculator Calculator) (*Service, error) {
	if store == nil || calculator == nil {
		return nil, ErrInvalidServiceConfiguration
	}
	return &Service{
		store:      store,
		calculator: calculator,
	}, nil
}

func (service *Service) List(
	ctx context.Context,
	actorID string,
	groupID string,
) (Result, error) {
	entries, members, err := service.store.ListDebtEntries(
		ctx,
		actorID,
		groupID,
	)
	if err != nil {
		return Result{}, err
	}
	transfers, err := service.calculator.Calculate(entries)
	if err != nil {
		return Result{}, fmt.Errorf("calculate settlements: %w", err)
	}
	if transfers == nil {
		transfers = make([]Transfer, 0)
	}
	if members == nil {
		members = make([]MemberSummary, 0)
	}
	return Result{
		Transfers: transfers,
		Members:   members,
	}, nil
}
