package groups

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/Fyy10/settled/server/internal/clock"
	"github.com/Fyy10/settled/server/internal/identifier"
	"github.com/Fyy10/settled/server/internal/input"
)

const (
	maxJoinCodeAttempts = 5
	joinCodeField       = "joinCode"
)

var (
	ErrInvalidServiceConfiguration = errors.New("invalid group service configuration")
	ErrJoinCodeAttemptsExhausted   = errors.New("join code attempts exhausted")
)

type ValidationError struct {
	Fields map[string]string
}

func (validation *ValidationError) Error() string {
	return "validation failed"
}

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

func (service *Service) Create(
	ctx context.Context,
	actorID string,
	name string,
) (Group, error) {
	normalizedName, err := input.NormalizeGroupName(name)
	if err != nil {
		return Group{}, validationError(err)
	}

	groupID, err := identifier.NewUUIDFrom(service.random)
	if err != nil {
		return Group{}, fmt.Errorf("generate group ID: %w", err)
	}
	now := service.clock.Now().UTC()
	for range maxJoinCodeAttempts {
		joinCode, err := identifier.NewJoinCodeFrom(service.random)
		if err != nil {
			return Group{}, fmt.Errorf("generate group join code: %w", err)
		}
		group, err := service.store.CreateGroup(ctx, NewGroup{
			ID:          groupID,
			Name:        normalizedName,
			JoinCode:    joinCode,
			OwnerUserID: actorID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		if errors.Is(err, ErrJoinCodeCollision) {
			continue
		}
		if err != nil {
			return Group{}, err
		}
		return group, nil
	}
	return Group{}, ErrJoinCodeAttemptsExhausted
}

func (service *Service) List(
	ctx context.Context,
	actorID string,
) ([]Group, error) {
	return service.store.ListGroups(ctx, actorID)
}

func (service *Service) Join(
	ctx context.Context,
	actorID string,
	joinCode string,
) (Group, error) {
	normalizedJoinCode, err := normalizeJoinCode(joinCode)
	if err != nil {
		return Group{}, err
	}
	return service.store.JoinGroup(ctx, JoinGroupInput{
		UserID:   actorID,
		JoinCode: normalizedJoinCode,
		JoinedAt: service.clock.Now().UTC(),
	})
}

func (service *Service) Get(
	ctx context.Context,
	actorID string,
	groupID string,
) (Detail, error) {
	return service.store.GetGroup(ctx, actorID, groupID)
}

func (service *Service) Rename(
	ctx context.Context,
	actorID string,
	groupID string,
	name string,
) (Group, error) {
	normalizedName, err := input.NormalizeGroupName(name)
	if err != nil {
		return Group{}, validationError(err)
	}
	return service.store.RenameGroup(ctx, RenameGroupInput{
		ActorID:   actorID,
		GroupID:   groupID,
		Name:      normalizedName,
		UpdatedAt: service.clock.Now().UTC(),
	})
}

func (service *Service) Dissolve(
	ctx context.Context,
	actorID string,
	groupID string,
) error {
	return service.store.DissolveGroup(ctx, DissolveGroupInput{
		ActorID:     actorID,
		GroupID:     groupID,
		DissolvedAt: service.clock.Now().UTC(),
	})
}

func (service *Service) GetJoinCode(
	ctx context.Context,
	actorID string,
	groupID string,
) (string, error) {
	return service.store.GetJoinCode(ctx, actorID, groupID)
}

func (service *Service) RemoveMember(
	ctx context.Context,
	actorID string,
	groupID string,
	userID string,
) error {
	return service.store.RemoveMember(ctx, RemoveMemberInput{
		ActorID:   actorID,
		GroupID:   groupID,
		UserID:    userID,
		RemovedAt: service.clock.Now().UTC(),
	})
}

func normalizeJoinCode(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", &ValidationError{Fields: map[string]string{
			joinCodeField: "Join code must be valid UTF-8.",
		}}
	}
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", &ValidationError{Fields: map[string]string{
			joinCodeField: "Join code is required.",
		}}
	}
	return normalized, nil
}

func validationError(err error) error {
	var fieldError *input.FieldError
	if !errors.As(err, &fieldError) {
		return err
	}
	return &ValidationError{Fields: map[string]string{
		fieldError.Field: fieldError.Message,
	}}
}
