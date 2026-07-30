package groups

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/clock"
)

var groupServiceTestNow = time.Date(
	2026,
	time.July,
	31,
	9,
	20,
	11,
	987654321,
	time.FixedZone("test", -7*60*60),
)

const (
	groupServiceActorID = "00112233-4455-4677-8899-aabbccddeeff"
	groupServiceGroupID = "00000000-0000-4000-8000-000000000000"
)

type storeStub struct {
	createGroup func(context.Context, NewGroup) (Group, error)
	listGroups  func(context.Context, string) ([]Group, error)
	joinGroup   func(context.Context, JoinGroupInput) (Group, error)
	getGroup    func(context.Context, string, string) (Detail, error)
}

func (store storeStub) CreateGroup(
	ctx context.Context,
	input NewGroup,
) (Group, error) {
	if store.createGroup == nil {
		return Group{}, errors.New("unexpected CreateGroup call")
	}
	return store.createGroup(ctx, input)
}

func (store storeStub) ListGroups(
	ctx context.Context,
	actorID string,
) ([]Group, error) {
	if store.listGroups == nil {
		return nil, errors.New("unexpected ListGroups call")
	}
	return store.listGroups(ctx, actorID)
}

func (store storeStub) JoinGroup(
	ctx context.Context,
	input JoinGroupInput,
) (Group, error) {
	if store.joinGroup == nil {
		return Group{}, errors.New("unexpected JoinGroup call")
	}
	return store.joinGroup(ctx, input)
}

func (store storeStub) GetGroup(
	ctx context.Context,
	actorID string,
	groupID string,
) (Detail, error) {
	if store.getGroup == nil {
		return Detail{}, errors.New("unexpected GetGroup call")
	}
	return store.getGroup(ctx, actorID, groupID)
}

func TestServiceCreateNormalizesAndBuildsAtomicStoreInput(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), groupServiceContextKey{}, "value")
	wantGroup := groupServiceTestGroup()
	storeCalls := 0
	service := newGroupServiceForTest(
		t,
		storeStub{
			createGroup: func(
				gotContext context.Context,
				input NewGroup,
			) (Group, error) {
				storeCalls++
				if gotContext != ctx {
					t.Error("CreateGroup did not receive caller context")
				}
				wantInput := NewGroup{
					ID:          groupServiceGroupID,
					Name:        "Lake Trip",
					JoinCode:    strings.Repeat("2", 12),
					OwnerUserID: groupServiceActorID,
					CreatedAt:   groupServiceTestNow.UTC(),
					UpdatedAt:   groupServiceTestNow.UTC(),
				}
				if input != wantInput {
					t.Errorf("CreateGroup input = %+v, want %+v", input, wantInput)
				}
				return wantGroup, nil
			},
		},
		createRandom(0),
	)

	group, err := service.Create(
		ctx,
		groupServiceActorID,
		"\u2003Lake Trip\u2003",
	)

	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if group != wantGroup {
		t.Errorf("group = %+v, want %+v", group, wantGroup)
	}
	if storeCalls != 1 {
		t.Errorf("store calls = %d, want 1", storeCalls)
	}
}

func TestServiceCreateValidatesBeforeRandomOrStoreWork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "blank", value: "\u2003"},
		{name: "control", value: "Lake\nTrip"},
		{name: "too long", value: strings.Repeat("界", 161)},
		{name: "invalid UTF-8", value: string([]byte{'L', 0xff})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storeCalled := false
			service := newGroupServiceForTest(
				t,
				storeStub{
					createGroup: func(context.Context, NewGroup) (Group, error) {
						storeCalled = true
						return Group{}, nil
					},
				},
				errorReader{err: errors.New("random source must not be read")},
			)

			_, err := service.Create(
				context.Background(),
				groupServiceActorID,
				test.value,
			)

			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("error = %v, want ValidationError", err)
			}
			if validation.Fields["name"] == "" || len(validation.Fields) != 1 {
				t.Errorf("fields = %v", validation.Fields)
			}
			if storeCalled {
				t.Error("invalid group name reached Store")
			}
			if strings.Contains(err.Error(), test.value) {
				t.Errorf("error exposes invalid value: %v", err)
			}
		})
	}
}

func TestServiceCreateRetriesOnlyJoinCodeCollisions(t *testing.T) {
	t.Parallel()

	sourceError := errors.New("database unavailable")
	tests := []struct {
		name         string
		errors       []error
		wantCodes    []string
		wantError    error
		wantAttempts int
	}{
		{
			name: "collisions then success",
			errors: []error{
				fmt.Errorf("insert group: %w", ErrJoinCodeCollision),
				ErrJoinCodeCollision,
				nil,
			},
			wantCodes: []string{
				strings.Repeat("2", 12),
				strings.Repeat("3", 12),
				strings.Repeat("4", 12),
			},
			wantAttempts: 3,
		},
		{
			name:         "other store error stops",
			errors:       []error{sourceError},
			wantCodes:    []string{strings.Repeat("2", 12)},
			wantError:    sourceError,
			wantAttempts: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var inputs []NewGroup
			service := newGroupServiceForTest(
				t,
				storeStub{
					createGroup: func(
						_ context.Context,
						input NewGroup,
					) (Group, error) {
						inputs = append(inputs, input)
						err := test.errors[len(inputs)-1]
						if err != nil {
							return Group{}, err
						}
						return groupServiceTestGroup(), nil
					},
				},
				createRandom(0, 1, 2),
			)

			group, err := service.Create(
				context.Background(),
				groupServiceActorID,
				"Lake Trip",
			)

			if test.wantError != nil {
				if !errors.Is(err, test.wantError) {
					t.Errorf("error = %v, want %v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatalf("Create: %v", err)
				}
				if group != groupServiceTestGroup() {
					t.Errorf("group = %+v", group)
				}
			}
			if len(inputs) != test.wantAttempts {
				t.Fatalf("attempts = %d, want %d", len(inputs), test.wantAttempts)
			}
			codes := make([]string, len(inputs))
			for index, input := range inputs {
				codes[index] = input.JoinCode
				if input.ID != groupServiceGroupID {
					t.Errorf("attempt %d group ID = %q", index, input.ID)
				}
				if !input.CreatedAt.Equal(groupServiceTestNow.UTC()) ||
					!input.UpdatedAt.Equal(groupServiceTestNow.UTC()) {
					t.Errorf("attempt %d timestamps = %+v", index, input)
				}
			}
			if !reflect.DeepEqual(codes, test.wantCodes) {
				t.Errorf("codes = %v, want %v", codes, test.wantCodes)
			}
		})
	}
}

func TestServiceCreateStopsAfterFiveJoinCodeCollisions(t *testing.T) {
	t.Parallel()

	var codes []string
	service := newGroupServiceForTest(
		t,
		storeStub{
			createGroup: func(
				_ context.Context,
				input NewGroup,
			) (Group, error) {
				codes = append(codes, input.JoinCode)
				return Group{}, ErrJoinCodeCollision
			},
		},
		createRandom(0, 1, 2, 3, 4),
	)

	_, err := service.Create(
		context.Background(),
		groupServiceActorID,
		"Lake Trip",
	)

	if !errors.Is(err, ErrJoinCodeAttemptsExhausted) {
		t.Errorf("error = %v, want ErrJoinCodeAttemptsExhausted", err)
	}
	if len(codes) != maxJoinCodeAttempts {
		t.Errorf("attempts = %d, want %d", len(codes), maxJoinCodeAttempts)
	}
	wantCodes := []string{
		strings.Repeat("2", 12),
		strings.Repeat("3", 12),
		strings.Repeat("4", 12),
		strings.Repeat("5", 12),
		strings.Repeat("6", 12),
	}
	if !reflect.DeepEqual(codes, wantCodes) {
		t.Errorf("codes = %v, want %v", codes, wantCodes)
	}
}

func TestServiceCreateRandomFailuresStopBeforeStore(t *testing.T) {
	t.Parallel()

	sourceError := errors.New("entropy unavailable")
	tests := []struct {
		name   string
		random io.Reader
	}{
		{name: "group ID", random: errorReader{err: sourceError}},
		{
			name: "join code",
			random: io.MultiReader(
				bytes.NewReader(make([]byte, 16)),
				errorReader{err: sourceError},
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storeCalls := 0
			service := newGroupServiceForTest(
				t,
				storeStub{
					createGroup: func(context.Context, NewGroup) (Group, error) {
						storeCalls++
						return Group{}, nil
					},
				},
				test.random,
			)

			_, err := service.Create(
				context.Background(),
				groupServiceActorID,
				"Lake Trip",
			)

			if !errors.Is(err, sourceError) {
				t.Errorf("error = %v, want wrapped source error", err)
			}
			if storeCalls != 0 {
				t.Errorf("Store calls = %d, want 0", storeCalls)
			}
		})
	}
}

func TestServiceJoinNormalizesAndUsesWorkflowTime(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), groupServiceContextKey{}, "value")
	wantGroup := groupServiceTestGroup()
	service := newGroupServiceForTest(
		t,
		storeStub{
			joinGroup: func(
				gotContext context.Context,
				input JoinGroupInput,
			) (Group, error) {
				if gotContext != ctx {
					t.Error("JoinGroup did not receive caller context")
				}
				want := JoinGroupInput{
					UserID:   groupServiceActorID,
					JoinCode: "AB-CD",
					JoinedAt: groupServiceTestNow.UTC(),
				}
				if input != want {
					t.Errorf("JoinGroup input = %+v, want %+v", input, want)
				}
				return wantGroup, nil
			},
		},
		bytes.NewReader(nil),
	)

	group, err := service.Join(
		ctx,
		groupServiceActorID,
		"\u2003ab-cd\u2003",
	)

	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if group != wantGroup {
		t.Errorf("group = %+v, want %+v", group, wantGroup)
	}
}

func TestServiceJoinValidationPreventsStoreWork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		wantField string
	}{
		{name: "empty", wantField: "Join code is required."},
		{name: "Unicode whitespace", value: "\u2003", wantField: "Join code is required."},
		{
			name:      "invalid UTF-8",
			value:     string([]byte{'A', 0xff}),
			wantField: "Join code must be valid UTF-8.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storeCalled := false
			service := newGroupServiceForTest(
				t,
				storeStub{
					joinGroup: func(
						context.Context,
						JoinGroupInput,
					) (Group, error) {
						storeCalled = true
						return Group{}, nil
					},
				},
				bytes.NewReader(nil),
			)

			_, err := service.Join(
				context.Background(),
				groupServiceActorID,
				test.value,
			)

			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("error = %v, want ValidationError", err)
			}
			if validation.Fields[joinCodeField] != test.wantField ||
				len(validation.Fields) != 1 {
				t.Errorf("fields = %v", validation.Fields)
			}
			if storeCalled {
				t.Error("invalid join code reached Store")
			}
			if test.value != "" && strings.Contains(err.Error(), test.value) {
				t.Errorf("error exposes join code: %v", err)
			}
		})
	}
}

func TestServiceDelegatesListAndGet(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), groupServiceContextKey{}, "value")
	wantGroups := []Group{groupServiceTestGroup()}
	wantDetail := Detail{
		Group: groupServiceTestGroup(),
		Members: []Member{{
			UserID:      groupServiceActorID,
			Email:       "alice@example.com",
			DisplayName: "Alice",
			Role:        RoleOwner,
			JoinedAt:    groupServiceTestNow.UTC(),
		}},
	}
	service := newGroupServiceForTest(
		t,
		storeStub{
			listGroups: func(
				gotContext context.Context,
				actorID string,
			) ([]Group, error) {
				if gotContext != ctx || actorID != groupServiceActorID {
					t.Errorf("ListGroups inputs = (%v, %q)", gotContext, actorID)
				}
				return wantGroups, nil
			},
			getGroup: func(
				gotContext context.Context,
				actorID string,
				groupID string,
			) (Detail, error) {
				if gotContext != ctx ||
					actorID != groupServiceActorID ||
					groupID != groupServiceGroupID {
					t.Errorf(
						"GetGroup inputs = (%v, %q, %q)",
						gotContext,
						actorID,
						groupID,
					)
				}
				return wantDetail, nil
			},
		},
		bytes.NewReader(nil),
	)

	groups, err := service.List(ctx, groupServiceActorID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !reflect.DeepEqual(groups, wantGroups) {
		t.Errorf("groups = %+v, want %+v", groups, wantGroups)
	}
	detail, err := service.Get(ctx, groupServiceActorID, groupServiceGroupID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(detail, wantDetail) {
		t.Errorf("detail = %+v, want %+v", detail, wantDetail)
	}
}

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	validStore := storeStub{}
	validClock := clock.Fixed{Time: groupServiceTestNow}
	tests := []struct {
		name   string
		store  Store
		clock  clock.Clock
		random io.Reader
	}{
		{name: "store", clock: validClock, random: bytes.NewReader(nil)},
		{name: "clock", store: validStore, random: bytes.NewReader(nil)},
		{name: "random", store: validStore, clock: validClock},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewServiceFrom(
				test.store,
				test.clock,
				test.random,
			)
			if service != nil || err != ErrInvalidServiceConfiguration {
				t.Errorf("result = (%v, %v)", service, err)
			}
		})
	}
}

func newGroupServiceForTest(
	t *testing.T,
	store Store,
	random io.Reader,
) *Service {
	t.Helper()
	service, err := NewServiceFrom(
		store,
		clock.Fixed{Time: groupServiceTestNow},
		random,
	)
	if err != nil {
		t.Fatalf("NewServiceFrom: %v", err)
	}
	return service
}

func createRandom(codeBytes ...byte) io.Reader {
	value := make([]byte, 16, 16+len(codeBytes)*12)
	for _, codeByte := range codeBytes {
		value = append(value, bytes.Repeat([]byte{codeByte}, 12)...)
	}
	return bytes.NewReader(value)
}

func groupServiceTestGroup() Group {
	return Group{
		ID:              groupServiceGroupID,
		Name:            "Lake Trip",
		OwnerUserID:     groupServiceActorID,
		MemberCount:     1,
		CurrentUserRole: RoleOwner,
		CreatedAt:       groupServiceTestNow.UTC(),
		UpdatedAt:       groupServiceTestNow.UTC(),
	}
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}

type groupServiceContextKey struct{}
