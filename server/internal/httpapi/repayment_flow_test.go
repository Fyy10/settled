package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/clock"
	"github.com/Fyy10/settled/server/internal/repayments"
)

func TestRepaymentEndpointFlowExercisesAllFiveRoutes(t *testing.T) {
	now := time.Date(2026, time.August, 2, 18, 0, 0, 0, time.UTC)
	store := newFlowRepaymentStore(
		groupHandlerGroupID,
		groupHandlerActorID,
		map[string]string{
			groupHandlerActorID: "Alice",
			groupHandlerOtherID: "Bob",
		},
	)
	service, err := repayments.NewServiceFrom(
		store,
		clock.Fixed{Time: now},
		bytes.NewReader(bytes.Repeat([]byte{0x32}, 16)),
	)
	if err != nil {
		t.Fatalf("repayments.NewServiceFrom: %v", err)
	}
	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	options.Repayments = service
	options.Sessions = sessionValidatorFunc(func(token string) (auth.Session, error) {
		switch token {
		case "alice-repayment-session":
			return auth.Session{
				UserID: groupHandlerActorID,
				JWTID:  "alice-repayment-flow",
			}, nil
		case "bob-repayment-session":
			return auth.Session{
				UserID: groupHandlerOtherID,
				JWTID:  "bob-repayment-flow",
			}, nil
		default:
			return auth.Session{}, auth.ErrUnauthenticated
		}
	})
	options.Auth = fakeAuthService{
		findUser: func(
			_ context.Context,
			userID string,
		) (auth.User, error) {
			if _, active := store.displayNames[userID]; !active {
				return auth.User{}, auth.ErrUserNotFound
			}
			return auth.User{ID: userID}, nil
		},
	}
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)

	createResponse := repaymentFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodPost,
		repaymentHandlerCollectionPath(),
		`{
			"fromUserId":"`+groupHandlerOtherID+`",
			"toUserId":"`+groupHandlerActorID+`",
			"amountCents":2500,
			"note":"  Venmo  ",
			"repaymentDate":"2026-07-31"
		}`,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf(
			"create = %d %s",
			createResponse.Code,
			createResponse.Body.String(),
		)
	}
	assertRepaymentEnvelopeKeys(t, createResponse.Body.Bytes())
	var created repaymentResponseEnvelope
	decodeGroupJSONResponse(t, createResponse.Result(), &created)
	if created.Repayment.FromUserID != groupHandlerOtherID ||
		created.Repayment.ToUserID != groupHandlerActorID ||
		created.Repayment.CreatedByUserID != groupHandlerActorID ||
		created.Repayment.Note == nil ||
		*created.Repayment.Note != "Venmo" {
		t.Errorf("created repayment = %#v", created.Repayment)
	}
	repaymentID := created.Repayment.ID
	itemPath := repaymentHandlerCollectionPath() + "/" + repaymentID

	getResponse := repaymentFlowCall(
		t,
		api.Handler(),
		groupHandlerOtherID,
		http.MethodGet,
		itemPath,
		"",
	)
	if getResponse.Code != http.StatusOK {
		t.Fatalf(
			"member get = %d %s",
			getResponse.Code,
			getResponse.Body.String(),
		)
	}
	assertRepaymentEnvelopeKeys(t, getResponse.Body.Bytes())

	replaceResponse := repaymentFlowCall(
		t,
		api.Handler(),
		groupHandlerOtherID,
		http.MethodPut,
		itemPath,
		`{
			"fromUserId":"`+groupHandlerActorID+`",
			"toUserId":"`+groupHandlerOtherID+`",
			"amountCents":3000,
			"note":null,
			"repaymentDate":"2026-08-01"
		}`,
	)
	if replaceResponse.Code != http.StatusOK {
		t.Fatalf(
			"collaborative replace = %d %s",
			replaceResponse.Code,
			replaceResponse.Body.String(),
		)
	}
	assertRepaymentEnvelopeKeys(t, replaceResponse.Body.Bytes())
	var replaced repaymentResponseEnvelope
	decodeGroupJSONResponse(t, replaceResponse.Result(), &replaced)
	if replaced.Repayment.ID != created.Repayment.ID ||
		replaced.Repayment.GroupID != created.Repayment.GroupID ||
		replaced.Repayment.CreatedByUserID != groupHandlerActorID ||
		replaced.Repayment.CreatedAt != created.Repayment.CreatedAt ||
		replaced.Repayment.FromUserID != groupHandlerActorID ||
		replaced.Repayment.ToUserID != groupHandlerOtherID ||
		replaced.Repayment.AmountCents != 3000 ||
		replaced.Repayment.Note != nil {
		t.Errorf("replaced repayment = %#v", replaced.Repayment)
	}

	listResponse := repaymentFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodGet,
		repaymentHandlerCollectionPath(),
		"",
	)
	if listResponse.Code != http.StatusOK {
		t.Fatalf(
			"list = %d %s",
			listResponse.Code,
			listResponse.Body.String(),
		)
	}
	assertRepaymentListKeys(t, listResponse.Body.Bytes())
	var listed repaymentsResponse
	decodeGroupJSONResponse(t, listResponse.Result(), &listed)
	if len(listed.Repayments) != 1 ||
		listed.Repayments[0].ID != repaymentID ||
		len(listed.Members) != 2 ||
		listed.Members[0].UserID != groupHandlerActorID {
		t.Errorf("list response = %#v", listed)
	}

	crossGroupResponse := repaymentFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodGet,
		"/api/groups/bbbbbbbb-cccc-4ddd-8eee-ffffffffffff/repayments/"+
			repaymentID,
		"",
	)
	assertAPIError(
		t,
		crossGroupResponse.Result(),
		http.StatusNotFound,
		"not_found",
	)

	deleteResponse := repaymentFlowCall(
		t,
		api.Handler(),
		groupHandlerOtherID,
		http.MethodDelete,
		itemPath,
		"",
	)
	assertEmptyNoContent(t, deleteResponse)

	deletedResponse := repaymentFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodGet,
		itemPath,
		"",
	)
	assertAPIError(
		t,
		deletedResponse.Result(),
		http.StatusNotFound,
		"not_found",
	)
	repeatedDeleteResponse := repaymentFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodDelete,
		itemPath,
		"",
	)
	assertAPIError(
		t,
		repeatedDeleteResponse.Result(),
		http.StatusNotFound,
		"not_found",
	)

	emptyListResponse := repaymentFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodGet,
		repaymentHandlerCollectionPath(),
		"",
	)
	var emptyList repaymentsResponse
	decodeGroupJSONResponse(t, emptyListResponse.Result(), &emptyList)
	if len(emptyList.Repayments) != 0 || len(emptyList.Members) != 2 {
		t.Errorf("list after delete = %#v", emptyList)
	}
}

func repaymentFlowCall(
	t *testing.T,
	handler http.Handler,
	actorID string,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	var requestBody io.Reader
	if body != "" {
		requestBody = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, requestBody)
	session := "alice-repayment-session"
	if actorID == groupHandlerOtherID {
		session = "bob-repayment-session"
	}
	request.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: session,
	})
	if method == http.MethodPost ||
		method == http.MethodPut ||
		method == http.MethodDelete {
		request.Header.Set("Origin", "https://web.example")
		request.Header.Set(csrfTokenHeader, "repayment-flow-csrf")
		request.AddCookie(&http.Cookie{
			Name:  auth.CSRFCookieName,
			Value: "repayment-flow-csrf-cookie",
		})
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type flowRepaymentStore struct {
	mu           sync.Mutex
	groupID      string
	ownerID      string
	displayNames map[string]string
	records      map[string]*flowRepaymentRecord
}

type flowRepaymentRecord struct {
	repayment repayments.Repayment
	deleted   bool
}

func newFlowRepaymentStore(
	groupID string,
	ownerID string,
	displayNames map[string]string,
) *flowRepaymentStore {
	return &flowRepaymentStore{
		groupID:      groupID,
		ownerID:      ownerID,
		displayNames: displayNames,
		records:      make(map[string]*flowRepaymentRecord),
	}
}

func (store *flowRepaymentStore) CreateRepayment(
	_ context.Context,
	input repayments.CreateInput,
) (repayments.Repayment, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.visible(input.ActorID, input.GroupID) ||
		!store.participantsActive(input.FromUserID, input.ToUserID) {
		return repayments.Repayment{}, repayments.ErrNotFound
	}
	repayment := repayments.Repayment{
		ID:              input.ID,
		GroupID:         input.GroupID,
		FromUserID:      input.FromUserID,
		ToUserID:        input.ToUserID,
		AmountCents:     input.AmountCents,
		Currency:        input.Currency,
		Note:            input.Note,
		RepaymentDate:   input.RepaymentDate,
		CreatedByUserID: input.ActorID,
		CreatedAt:       input.CreatedAt,
		UpdatedAt:       input.UpdatedAt,
	}
	store.records[repayment.ID] = &flowRepaymentRecord{
		repayment: repayment,
	}
	return repayment, nil
}

func (store *flowRepaymentStore) ReplaceRepayment(
	_ context.Context,
	input repayments.ReplaceInput,
) (repayments.Repayment, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, found := store.records[input.RepaymentID]
	if !store.visible(input.ActorID, input.GroupID) ||
		!store.participantsActive(input.FromUserID, input.ToUserID) ||
		!found ||
		record.deleted ||
		record.repayment.GroupID != input.GroupID {
		return repayments.Repayment{}, repayments.ErrNotFound
	}
	record.repayment.FromUserID = input.FromUserID
	record.repayment.ToUserID = input.ToUserID
	record.repayment.AmountCents = input.AmountCents
	record.repayment.Currency = input.Currency
	record.repayment.Note = input.Note
	record.repayment.RepaymentDate = input.RepaymentDate
	record.repayment.UpdatedAt = input.UpdatedAt
	return record.repayment, nil
}

func (store *flowRepaymentStore) DeleteRepayment(
	_ context.Context,
	input repayments.DeleteInput,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, found := store.records[input.RepaymentID]
	if !store.visible(input.ActorID, input.GroupID) ||
		!found ||
		record.deleted ||
		record.repayment.GroupID != input.GroupID {
		return repayments.ErrNotFound
	}
	record.deleted = true
	record.repayment.UpdatedAt = input.DeletedAt
	return nil
}

func (store *flowRepaymentStore) GetRepayment(
	_ context.Context,
	actorID string,
	groupID string,
	repaymentID string,
) (repayments.Repayment, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, found := store.records[repaymentID]
	if !store.visible(actorID, groupID) ||
		!found ||
		record.deleted ||
		record.repayment.GroupID != groupID {
		return repayments.Repayment{}, repayments.ErrNotFound
	}
	return record.repayment, nil
}

func (store *flowRepaymentStore) ListRepayments(
	_ context.Context,
	actorID string,
	groupID string,
) (repayments.ListResult, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.visible(actorID, groupID) {
		return repayments.ListResult{}, repayments.ErrNotFound
	}
	found := make([]repayments.Repayment, 0, len(store.records))
	for _, record := range store.records {
		if !record.deleted && record.repayment.GroupID == groupID {
			found = append(found, record.repayment)
		}
	}
	sort.Slice(found, func(left, right int) bool {
		if !found[left].RepaymentDate.Equal(found[right].RepaymentDate) {
			return found[left].RepaymentDate.After(found[right].RepaymentDate)
		}
		if !found[left].CreatedAt.Equal(found[right].CreatedAt) {
			return found[left].CreatedAt.After(found[right].CreatedAt)
		}
		return found[left].ID > found[right].ID
	})
	members := make([]repayments.MemberSummary, 0, len(store.displayNames))
	for userID, displayName := range store.displayNames {
		members = append(members, repayments.MemberSummary{
			UserID:      userID,
			DisplayName: displayName,
		})
	}
	sort.Slice(members, func(left, right int) bool {
		if members[left].UserID == store.ownerID {
			return true
		}
		if members[right].UserID == store.ownerID {
			return false
		}
		if members[left].DisplayName != members[right].DisplayName {
			return members[left].DisplayName < members[right].DisplayName
		}
		return members[left].UserID < members[right].UserID
	})
	return repayments.ListResult{
		Repayments: found,
		Members:    members,
	}, nil
}

func (store *flowRepaymentStore) visible(actorID string, groupID string) bool {
	if groupID != store.groupID {
		return false
	}
	_, active := store.displayNames[actorID]
	return active
}

func (store *flowRepaymentStore) participantsActive(
	fromUserID string,
	toUserID string,
) bool {
	if fromUserID == toUserID {
		return false
	}
	_, fromActive := store.displayNames[fromUserID]
	_, toActive := store.displayNames[toUserID]
	return fromActive && toActive
}
