package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/settlements"
)

const (
	settlementFlowCarolID    = "22223333-4444-4555-8666-777788889999"
	settlementFlowOutsiderID = "33334444-5555-4666-8777-88889999aaaa"
)

func TestSettlementEndpointFlowUsesProductionCalculator(t *testing.T) {
	store := &flowSettlementStore{
		groupID: groupHandlerGroupID,
		activeMembers: map[string]string{
			groupHandlerActorID:   "Alice",
			groupHandlerOtherID:   "Bob",
			settlementFlowCarolID: "Carol",
		},
		entries: []settlements.DebtEntry{
			settlementFlowDebt(groupHandlerOtherID, groupHandlerActorID, 2000),
			settlementFlowDebt(settlementFlowCarolID, groupHandlerActorID, 3000),
			settlementFlowDebt(groupHandlerOtherID, groupHandlerActorID, 1000),
			settlementFlowDebt(groupHandlerActorID, groupHandlerOtherID, 1200),
			settlementFlowDebt(groupHandlerActorID, groupHandlerOtherID, 1000),
		},
		members: []settlements.MemberSummary{
			{UserID: groupHandlerActorID, DisplayName: "Alice"},
			{UserID: groupHandlerOtherID, DisplayName: "Bob"},
			{UserID: settlementFlowCarolID, DisplayName: "Carol"},
		},
	}
	service, err := settlements.NewService(
		store,
		settlements.PairwiseCalculator{},
	)
	if err != nil {
		t.Fatalf("settlements.NewService: %v", err)
	}
	options := defaultTestOptions()
	options.Settlements = service
	options.Sessions = sessionValidatorFunc(func(token string) (auth.Session, error) {
		switch token {
		case "alice-settlement-session":
			return auth.Session{
				UserID: groupHandlerActorID,
				JWTID:  "alice-settlement-flow",
			}, nil
		case "bob-settlement-session":
			return auth.Session{
				UserID: groupHandlerOtherID,
				JWTID:  "bob-settlement-flow",
			}, nil
		case "outsider-settlement-session":
			return auth.Session{
				UserID: settlementFlowOutsiderID,
				JWTID:  "outsider-settlement-flow",
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
			switch userID {
			case groupHandlerActorID,
				groupHandlerOtherID,
				settlementFlowOutsiderID:
				return auth.User{ID: userID}, nil
			default:
				return auth.User{}, auth.ErrUserNotFound
			}
		},
	}
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)

	want := settlementsResponse{
		Settlements: []settlementResponse{
			{
				FromUserID:  settlementFlowCarolID,
				ToUserID:    groupHandlerActorID,
				AmountCents: 3000,
				Currency:    "USD",
			},
			{
				FromUserID:  groupHandlerOtherID,
				ToUserID:    groupHandlerActorID,
				AmountCents: 800,
				Currency:    "USD",
			},
		},
		Members: []settlementMemberResponse{
			{UserID: groupHandlerActorID, DisplayName: "Alice"},
			{UserID: groupHandlerOtherID, DisplayName: "Bob"},
			{UserID: settlementFlowCarolID, DisplayName: "Carol"},
		},
	}
	for _, actorID := range []string{
		groupHandlerActorID,
		groupHandlerOtherID,
	} {
		response := settlementFlowCall(
			t,
			api.Handler(),
			actorID,
			settlementHandlerPath(),
		)
		if response.Code != http.StatusOK {
			t.Fatalf(
				"member %s response = %d %s",
				actorID,
				response.Code,
				response.Body.String(),
			)
		}
		assertSettlementResponseKeys(t, response.Body.Bytes())
		var got settlementsResponse
		decodeGroupJSONResponse(t, response.Result(), &got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("member %s body = %#v, want %#v", actorID, got, want)
		}
	}

	outsiderResponse := settlementFlowCall(
		t,
		api.Handler(),
		settlementFlowOutsiderID,
		settlementHandlerPath(),
	)
	assertAPIError(
		t,
		outsiderResponse.Result(),
		http.StatusNotFound,
		"not_found",
	)

	crossGroupResponse := settlementFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		"/api/groups/bbbbbbbb-cccc-4ddd-8eee-ffffffffffff/settlements",
	)
	assertAPIError(
		t,
		crossGroupResponse.Result(),
		http.StatusNotFound,
		"not_found",
	)

	if store.calls != 4 {
		t.Errorf("Store calls = %d, want 4", store.calls)
	}
}

func settlementFlowCall(
	t *testing.T,
	handler http.Handler,
	actorID string,
	path string,
) *httptest.ResponseRecorder {
	t.Helper()

	session := "alice-settlement-session"
	switch actorID {
	case groupHandlerOtherID:
		session = "bob-settlement-session"
	case settlementFlowOutsiderID:
		session = "outsider-settlement-session"
	}
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: session,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type flowSettlementStore struct {
	groupID       string
	activeMembers map[string]string
	entries       []settlements.DebtEntry
	members       []settlements.MemberSummary
	calls         int
}

func (store *flowSettlementStore) ListDebtEntries(
	_ context.Context,
	actorID string,
	groupID string,
) ([]settlements.DebtEntry, []settlements.MemberSummary, error) {
	store.calls++
	if groupID != store.groupID {
		return nil, nil, settlements.ErrNotFound
	}
	if _, active := store.activeMembers[actorID]; !active {
		return nil, nil, settlements.ErrNotFound
	}
	return append([]settlements.DebtEntry(nil), store.entries...),
		append([]settlements.MemberSummary(nil), store.members...),
		nil
}

func settlementFlowDebt(
	fromUserID string,
	toUserID string,
	amountCents int64,
) settlements.DebtEntry {
	return settlements.DebtEntry{
		FromUserID:  fromUserID,
		ToUserID:    toUserID,
		AmountCents: amountCents,
		Currency:    "USD",
	}
}
