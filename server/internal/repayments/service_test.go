package repayments

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
	repaymentServiceActorID     = "00112233-4455-4677-8899-aabbccddeeff"
	repaymentServiceFromUserID  = "11112222-3333-4444-8555-666677778888"
	repaymentServiceToUserID    = "99990000-aaaa-4bbb-8ccc-ddddeeeeffff"
	repaymentServiceGroupID     = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	repaymentServiceRepaymentID = "22222222-3333-4444-8555-666666666666"
)

var repaymentServiceNow = time.Date(
	2026,
	time.August,
	3,
	10,
	20,
	30,
	456789123,
	time.FixedZone("test", -7*60*60),
)

type repaymentStoreStub struct {
	create  func(context.Context, CreateInput) (Repayment, error)
	replace func(context.Context, ReplaceInput) (Repayment, error)
	delete  func(context.Context, DeleteInput) error
	get     func(context.Context, string, string, string) (Repayment, error)
	list    func(context.Context, string, string) (ListResult, error)
}

func (store repaymentStoreStub) CreateRepayment(
	ctx context.Context,
	input CreateInput,
) (Repayment, error) {
	if store.create == nil {
		return Repayment{}, errors.New("unexpected CreateRepayment call")
	}
	return store.create(ctx, input)
}

func (store repaymentStoreStub) ReplaceRepayment(
	ctx context.Context,
	input ReplaceInput,
) (Repayment, error) {
	if store.replace == nil {
		return Repayment{}, errors.New("unexpected ReplaceRepayment call")
	}
	return store.replace(ctx, input)
}

func (store repaymentStoreStub) DeleteRepayment(
	ctx context.Context,
	input DeleteInput,
) error {
	if store.delete == nil {
		return errors.New("unexpected DeleteRepayment call")
	}
	return store.delete(ctx, input)
}

func (store repaymentStoreStub) GetRepayment(
	ctx context.Context,
	actorID string,
	groupID string,
	repaymentID string,
) (Repayment, error) {
	if store.get == nil {
		return Repayment{}, errors.New("unexpected GetRepayment call")
	}
	return store.get(ctx, actorID, groupID, repaymentID)
}

func (store repaymentStoreStub) ListRepayments(
	ctx context.Context,
	actorID string,
	groupID string,
) (ListResult, error) {
	if store.list == nil {
		return ListResult{}, errors.New("unexpected ListRepayments call")
	}
	return store.list(ctx, actorID, groupID)
}

func TestServiceCreateBuildsNormalizedAtomicInput(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(
		context.Background(),
		repaymentServiceContextKey{},
		"create",
	)
	wantRepayment := repaymentServiceTestRepayment()
	storeCalls := 0
	service := newRepaymentServiceForTest(
		t,
		repaymentStoreStub{
			create: func(
				gotContext context.Context,
				input CreateInput,
			) (Repayment, error) {
				storeCalls++
				if gotContext != ctx {
					t.Error("CreateRepayment context was not preserved")
				}
				wantNote := "Venmo"
				wantInput := CreateInput{
					ID:          "31313131-3131-4131-b131-313131313131",
					ActorID:     repaymentServiceActorID,
					GroupID:     repaymentServiceGroupID,
					FromUserID:  repaymentServiceFromUserID,
					ToUserID:    repaymentServiceToUserID,
					AmountCents: 2_000,
					Currency:    "USD",
					Note:        &wantNote,
					RepaymentDate: time.Date(
						2026,
						time.July,
						31,
						0,
						0,
						0,
						0,
						time.UTC,
					),
					CreatedAt: repaymentServiceNow.UTC(),
					UpdatedAt: repaymentServiceNow.UTC(),
				}
				if !reflect.DeepEqual(input, wantInput) {
					t.Errorf("CreateRepayment input = %#v, want %#v", input, wantInput)
				}
				return wantRepayment, nil
			},
		},
		bytes.NewReader(bytes.Repeat([]byte{0x31}, 16)),
	)
	note := "  Venmo  "

	got, err := service.Create(
		ctx,
		repaymentServiceActorID,
		repaymentServiceGroupID,
		MutationInput{
			FromUserID:    strings.ToUpper(repaymentServiceFromUserID),
			ToUserID:      strings.ToUpper(repaymentServiceToUserID),
			AmountCents:   2_000,
			Note:          &note,
			NotePresent:   true,
			RepaymentDate: "2026-07-31",
		},
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !reflect.DeepEqual(got, wantRepayment) {
		t.Errorf("repayment = %#v, want %#v", got, wantRepayment)
	}
	if storeCalls != 1 {
		t.Errorf("CreateRepayment calls = %d, want 1", storeCalls)
	}
}

func TestServiceCreateNormalizesNullableNotes(t *testing.T) {
	t.Parallel()

	empty := ""
	blank := " \t "
	value := " Cash "
	tests := []struct {
		name        string
		note        *string
		notePresent bool
		want        *string
	}{
		{name: "missing"},
		{name: "null", notePresent: true},
		{name: "empty", note: &empty, notePresent: true},
		{name: "whitespace", note: &blank, notePresent: true},
		{name: "trimmed value", note: &value, notePresent: true, want: stringPointer("Cash")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newRepaymentServiceForTest(
				t,
				repaymentStoreStub{
					create: func(
						_ context.Context,
						input CreateInput,
					) (Repayment, error) {
						if !reflect.DeepEqual(input.Note, test.want) {
							t.Errorf("note = %#v, want %#v", input.Note, test.want)
						}
						return Repayment{Note: input.Note}, nil
					},
				},
				bytes.NewReader(bytes.Repeat([]byte{0x32}, 16)),
			)
			mutation := validRepaymentMutation()
			mutation.Note = test.note
			mutation.NotePresent = test.notePresent

			if _, err := service.Create(
				context.Background(),
				repaymentServiceActorID,
				repaymentServiceGroupID,
				mutation,
			); err != nil {
				t.Fatalf("Create: %v", err)
			}
		})
	}
}

func TestServiceReplaceRequiresNotePresenceAndAllowsNull(t *testing.T) {
	t.Parallel()

	withoutNote := validRepaymentMutation()
	withoutNote.NotePresent = false
	service := newRepaymentServiceForTest(
		t,
		repaymentStoreStub{
			replace: func(
				context.Context,
				ReplaceInput,
			) (Repayment, error) {
				t.Fatal("ReplaceRepayment called for omitted note")
				return Repayment{}, nil
			},
		},
		repaymentPanicReader{},
	)
	_, err := service.Replace(
		context.Background(),
		repaymentServiceActorID,
		repaymentServiceGroupID,
		repaymentServiceRepaymentID,
		withoutNote,
	)
	assertRepaymentValidationField(t, err, fieldNote)

	ctx := context.WithValue(
		context.Background(),
		repaymentServiceContextKey{},
		"replace",
	)
	wantRepayment := repaymentServiceTestRepayment()
	service = newRepaymentServiceForTest(
		t,
		repaymentStoreStub{
			replace: func(
				gotContext context.Context,
				input ReplaceInput,
			) (Repayment, error) {
				wantInput := ReplaceInput{
					ActorID:     repaymentServiceActorID,
					GroupID:     repaymentServiceGroupID,
					RepaymentID: repaymentServiceRepaymentID,
					FromUserID:  repaymentServiceFromUserID,
					ToUserID:    repaymentServiceToUserID,
					AmountCents: 2_000,
					Currency:    "USD",
					Note:        nil,
					RepaymentDate: time.Date(
						2026,
						time.July,
						31,
						0,
						0,
						0,
						0,
						time.UTC,
					),
					UpdatedAt: repaymentServiceNow.UTC(),
				}
				if gotContext != ctx || !reflect.DeepEqual(input, wantInput) {
					t.Errorf(
						"ReplaceRepayment input = (%v, %#v), want (%v, %#v)",
						gotContext,
						input,
						ctx,
						wantInput,
					)
				}
				return wantRepayment, nil
			},
		},
		repaymentPanicReader{},
	)
	nullNote := validRepaymentMutation()
	nullNote.Note = nil
	nullNote.NotePresent = true
	got, err := service.Replace(
		ctx,
		repaymentServiceActorID,
		repaymentServiceGroupID,
		repaymentServiceRepaymentID,
		nullNote,
	)
	if err != nil {
		t.Fatalf("Replace null note: %v", err)
	}
	if !reflect.DeepEqual(got, wantRepayment) {
		t.Errorf("repayment = %#v, want %#v", got, wantRepayment)
	}
}

func TestServiceValidationPreventsStoreAndRandomUse(t *testing.T) {
	t.Parallel()

	tooLongNote := strings.Repeat("界", 241)
	controlNote := "bad\nnote"
	tests := []struct {
		name      string
		mutate    func(*MutationInput)
		wantField string
	}{
		{
			name: "sender required",
			mutate: func(input *MutationInput) {
				input.FromUserID = ""
			},
			wantField: fieldFromUserID,
		},
		{
			name: "recipient required",
			mutate: func(input *MutationInput) {
				input.ToUserID = ""
			},
			wantField: fieldToUserID,
		},
		{
			name: "users must differ after UUID normalization",
			mutate: func(input *MutationInput) {
				input.ToUserID = strings.ToUpper(input.FromUserID)
			},
			wantField: fieldToUserID,
		},
		{
			name: "amount positive",
			mutate: func(input *MutationInput) {
				input.AmountCents = 0
			},
			wantField: "amountCents",
		},
		{
			name: "note length",
			mutate: func(input *MutationInput) {
				input.Note = &tooLongNote
			},
			wantField: fieldNote,
		},
		{
			name: "note controls",
			mutate: func(input *MutationInput) {
				input.Note = &controlNote
			},
			wantField: fieldNote,
		},
		{
			name: "date required",
			mutate: func(input *MutationInput) {
				input.RepaymentDate = ""
			},
			wantField: fieldRepaymentDate,
		},
		{
			name: "date strict format",
			mutate: func(input *MutationInput) {
				input.RepaymentDate = "07/31/2026"
			},
			wantField: fieldRepaymentDate,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validRepaymentMutation()
			test.mutate(&input)
			service := newRepaymentServiceForTest(
				t,
				repaymentStoreStub{
					create: func(
						context.Context,
						CreateInput,
					) (Repayment, error) {
						t.Fatal("CreateRepayment was called")
						return Repayment{}, nil
					},
					replace: func(
						context.Context,
						ReplaceInput,
					) (Repayment, error) {
						t.Fatal("ReplaceRepayment was called")
						return Repayment{}, nil
					},
				},
				repaymentPanicReader{},
			)
			_, createErr := service.Create(
				context.Background(),
				repaymentServiceActorID,
				repaymentServiceGroupID,
				input,
			)
			assertRepaymentValidationField(t, createErr, test.wantField)
			_, replaceErr := service.Replace(
				context.Background(),
				repaymentServiceActorID,
				repaymentServiceGroupID,
				repaymentServiceRepaymentID,
				input,
			)
			assertRepaymentValidationField(t, replaceErr, test.wantField)
		})
	}
}

func TestServiceReadAndDeleteDelegation(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(
		context.Background(),
		repaymentServiceContextKey{},
		"read-delete",
	)
	wantRepayment := repaymentServiceTestRepayment()
	wantList := ListResult{
		Repayments: []Repayment{wantRepayment},
		Members: []MemberSummary{{
			UserID:      repaymentServiceActorID,
			DisplayName: "Alice",
		}},
	}
	var calls []string
	service := newRepaymentServiceForTest(
		t,
		repaymentStoreStub{
			list: func(
				gotContext context.Context,
				actorID string,
				groupID string,
			) (ListResult, error) {
				calls = append(calls, "list")
				if gotContext != ctx ||
					actorID != repaymentServiceActorID ||
					groupID != repaymentServiceGroupID {
					t.Errorf("ListRepayments inputs = (%v, %q, %q)", gotContext, actorID, groupID)
				}
				return wantList, nil
			},
			get: func(
				gotContext context.Context,
				actorID string,
				groupID string,
				repaymentID string,
			) (Repayment, error) {
				calls = append(calls, "get")
				if gotContext != ctx ||
					actorID != repaymentServiceActorID ||
					groupID != repaymentServiceGroupID ||
					repaymentID != repaymentServiceRepaymentID {
					t.Errorf(
						"GetRepayment inputs = (%v, %q, %q, %q)",
						gotContext,
						actorID,
						groupID,
						repaymentID,
					)
				}
				return wantRepayment, nil
			},
			delete: func(
				gotContext context.Context,
				input DeleteInput,
			) error {
				calls = append(calls, "delete")
				wantInput := DeleteInput{
					ActorID:     repaymentServiceActorID,
					GroupID:     repaymentServiceGroupID,
					RepaymentID: repaymentServiceRepaymentID,
					DeletedAt:   repaymentServiceNow.UTC(),
				}
				if gotContext != ctx || !reflect.DeepEqual(input, wantInput) {
					t.Errorf("DeleteRepayment input = (%v, %#v)", gotContext, input)
				}
				return nil
			},
		},
		repaymentPanicReader{},
	)

	gotList, err := service.List(
		ctx,
		repaymentServiceActorID,
		repaymentServiceGroupID,
	)
	if err != nil || !reflect.DeepEqual(gotList, wantList) {
		t.Errorf("List = (%#v, %v), want (%#v, nil)", gotList, err, wantList)
	}
	gotRepayment, err := service.Get(
		ctx,
		repaymentServiceActorID,
		repaymentServiceGroupID,
		repaymentServiceRepaymentID,
	)
	if err != nil || !reflect.DeepEqual(gotRepayment, wantRepayment) {
		t.Errorf("Get = (%#v, %v), want (%#v, nil)", gotRepayment, err, wantRepayment)
	}
	if err := service.Delete(
		ctx,
		repaymentServiceActorID,
		repaymentServiceGroupID,
		repaymentServiceRepaymentID,
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
	service := newRepaymentServiceForTest(
		t,
		repaymentStoreStub{
			create: func(context.Context, CreateInput) (Repayment, error) {
				return Repayment{}, storeError
			},
			replace: func(context.Context, ReplaceInput) (Repayment, error) {
				return Repayment{}, storeError
			},
			delete: func(context.Context, DeleteInput) error {
				return storeError
			},
			get: func(
				context.Context,
				string,
				string,
				string,
			) (Repayment, error) {
				return Repayment{}, storeError
			},
			list: func(context.Context, string, string) (ListResult, error) {
				return ListResult{}, storeError
			},
		},
		bytes.NewReader(bytes.Repeat([]byte{0x33}, 16)),
	)
	mutation := validRepaymentMutation()

	if _, err := service.Create(
		context.Background(),
		repaymentServiceActorID,
		repaymentServiceGroupID,
		mutation,
	); !errors.Is(err, storeError) {
		t.Errorf("Create error = %v, want store error", err)
	}
	if _, err := service.Replace(
		context.Background(),
		repaymentServiceActorID,
		repaymentServiceGroupID,
		repaymentServiceRepaymentID,
		mutation,
	); !errors.Is(err, storeError) {
		t.Errorf("Replace error = %v, want store error", err)
	}
	if err := service.Delete(
		context.Background(),
		repaymentServiceActorID,
		repaymentServiceGroupID,
		repaymentServiceRepaymentID,
	); !errors.Is(err, storeError) {
		t.Errorf("Delete error = %v, want store error", err)
	}
	if _, err := service.Get(
		context.Background(),
		repaymentServiceActorID,
		repaymentServiceGroupID,
		repaymentServiceRepaymentID,
	); !errors.Is(err, storeError) {
		t.Errorf("Get error = %v, want store error", err)
	}
	if _, err := service.List(
		context.Background(),
		repaymentServiceActorID,
		repaymentServiceGroupID,
	); !errors.Is(err, storeError) {
		t.Errorf("List error = %v, want store error", err)
	}

	randomError := errors.New("entropy unavailable")
	randomService := newRepaymentServiceForTest(
		t,
		repaymentStoreStub{
			create: func(context.Context, CreateInput) (Repayment, error) {
				t.Fatal("CreateRepayment called after random failure")
				return Repayment{}, nil
			},
		},
		repaymentErrorReader{err: randomError},
	)
	if _, err := randomService.Create(
		context.Background(),
		repaymentServiceActorID,
		repaymentServiceGroupID,
		mutation,
	); !errors.Is(err, randomError) {
		t.Errorf("random error = %v, want wrapped %v", err, randomError)
	}
}

func TestNewServiceRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	validStore := repaymentStoreStub{}
	validClock := clock.Fixed{Time: repaymentServiceNow}
	tests := []struct {
		name   string
		store  Store
		clock  clock.Clock
		random io.Reader
	}{
		{
			name:   "nil store",
			clock:  validClock,
			random: repaymentPanicReader{},
		},
		{
			name:   "nil clock",
			store:  validStore,
			random: repaymentPanicReader{},
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

func validRepaymentMutation() MutationInput {
	note := "Venmo"
	return MutationInput{
		FromUserID:    repaymentServiceFromUserID,
		ToUserID:      repaymentServiceToUserID,
		AmountCents:   2_000,
		Note:          &note,
		NotePresent:   true,
		RepaymentDate: "2026-07-31",
	}
}

func newRepaymentServiceForTest(
	t *testing.T,
	store Store,
	random io.Reader,
) *Service {
	t.Helper()
	service, err := NewServiceFrom(
		store,
		clock.Fixed{Time: repaymentServiceNow},
		random,
	)
	if err != nil {
		t.Fatalf("NewServiceFrom: %v", err)
	}
	return service
}

func repaymentServiceTestRepayment() Repayment {
	note := "Venmo"
	return Repayment{
		ID:          repaymentServiceRepaymentID,
		GroupID:     repaymentServiceGroupID,
		FromUserID:  repaymentServiceFromUserID,
		ToUserID:    repaymentServiceToUserID,
		AmountCents: 2_000,
		Currency:    "USD",
		Note:        &note,
		RepaymentDate: time.Date(
			2026,
			time.July,
			31,
			0,
			0,
			0,
			0,
			time.UTC,
		),
		CreatedByUserID: repaymentServiceActorID,
		CreatedAt:       repaymentServiceNow.UTC(),
		UpdatedAt:       repaymentServiceNow.UTC(),
	}
}

func assertRepaymentValidationField(
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

func stringPointer(value string) *string {
	return &value
}

type repaymentServiceContextKey struct{}

type repaymentPanicReader struct{}

func (repaymentPanicReader) Read([]byte) (int, error) {
	panic("unexpected random read")
}

type repaymentErrorReader struct {
	err error
}

func (reader repaymentErrorReader) Read([]byte) (int, error) {
	return 0, reader.err
}
