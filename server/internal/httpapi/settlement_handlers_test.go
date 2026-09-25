package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Fyy10/settled/server/internal/settlements"
)

func TestListSettlementsReturnsExactStableServiceOrder(t *testing.T) {
	t.Parallel()

	result := settlements.Result{
		Transfers: []settlements.Transfer{
			{
				FromUserID:  groupHandlerOtherID,
				ToUserID:    groupHandlerActorID,
				AmountCents: 3000,
				Currency:    "USD",
			},
			{
				FromUserID:  groupHandlerActorID,
				ToUserID:    groupHandlerOtherID,
				AmountCents: 800,
				Currency:    "USD",
			},
		},
		Members: []settlements.MemberSummary{
			{UserID: groupHandlerActorID, DisplayName: "Alice"},
			{UserID: groupHandlerOtherID, DisplayName: "Bob"},
		},
	}
	listCalls := 0
	options := settlementHandlerTestOptions(fakeSettlementService{
		list: func(
			ctx context.Context,
			actorID string,
			groupID string,
		) (settlements.Result, error) {
			listCalls++
			if ctx == nil ||
				actorID != groupHandlerActorID ||
				groupID != groupHandlerGroupID {
				t.Errorf("List inputs = (%v, %q, %q)", ctx, actorID, groupID)
			}
			return result, nil
		},
	})
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(
		response,
		groupHandlerRequest(
			http.MethodGet,
			"/api/groups/"+
				strings.ToUpper(groupHandlerGroupID)+
				"/settlements",
			nil,
			false,
		),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body settlementsResponse
	decodeGroupJSONResponse(t, response.Result(), &body)
	want := settlementsResponse{
		Settlements: []settlementResponse{
			{
				FromUserID:  groupHandlerOtherID,
				ToUserID:    groupHandlerActorID,
				AmountCents: 3000,
				Currency:    "USD",
			},
			{
				FromUserID:  groupHandlerActorID,
				ToUserID:    groupHandlerOtherID,
				AmountCents: 800,
				Currency:    "USD",
			},
		},
		Members: []settlementMemberResponse{
			{UserID: groupHandlerActorID, DisplayName: "Alice"},
			{UserID: groupHandlerOtherID, DisplayName: "Bob"},
		},
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %#v, want %#v", body, want)
	}
	assertSettlementResponseKeys(t, response.Body.Bytes())
	if listCalls != 1 {
		t.Errorf("List calls = %d, want 1", listCalls)
	}
}

func TestListSettlementsUsesEmptyArrays(t *testing.T) {
	t.Parallel()

	options := settlementHandlerTestOptions(fakeSettlementService{
		list: func(
			context.Context,
			string,
			string,
		) (settlements.Result, error) {
			return settlements.Result{}, nil
		},
	})
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		response,
		groupHandlerRequest(
			http.MethodGet,
			settlementHandlerPath(),
			nil,
			false,
		),
	)

	if response.Code != http.StatusOK ||
		response.Body.String() != "{\"settlements\":[],\"members\":[]}\n" {
		t.Errorf(
			"empty response = %d %q",
			response.Code,
			response.Body.String(),
		)
	}
}

func TestListSettlementsMapsHiddenAndInternalErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "hidden group or membership",
			serviceErr: settlements.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "calculator failure stays internal",
			serviceErr: settlements.ErrArithmeticOverflow,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := settlementHandlerTestOptions(fakeSettlementService{
				list: func(
					context.Context,
					string,
					string,
				) (settlements.Result, error) {
					return settlements.Result{}, test.serviceErr
				},
			})
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(
				response,
				groupHandlerRequest(
					http.MethodGet,
					settlementHandlerPath(),
					nil,
					false,
				),
			)

			assertAPIError(
				t,
				response.Result(),
				test.wantStatus,
				test.wantCode,
			)
			if strings.Contains(
				response.Body.String(),
				test.serviceErr.Error(),
			) {
				t.Errorf(
					"response exposes service error: %s",
					response.Body.String(),
				)
			}
		})
	}
}

func TestListSettlementsRejectsInvalidGroupUUIDBeforeService(t *testing.T) {
	t.Parallel()

	listCalls := 0
	options := settlementHandlerTestOptions(fakeSettlementService{
		list: func(
			context.Context,
			string,
			string,
		) (settlements.Result, error) {
			listCalls++
			return settlements.Result{}, nil
		},
	})
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		response,
		groupHandlerRequest(
			http.MethodGet,
			"/api/groups/not-a-uuid/settlements",
			nil,
			false,
		),
	)

	assertAPIError(
		t,
		response.Result(),
		http.StatusBadRequest,
		"bad_request",
	)
	if listCalls != 0 {
		t.Errorf("List calls = %d, want 0", listCalls)
	}
}

func TestListSettlementsRequiresAuthenticationBeforeService(t *testing.T) {
	t.Parallel()

	listCalls := 0
	options := settlementHandlerTestOptions(fakeSettlementService{
		list: func(
			context.Context,
			string,
			string,
		) (settlements.Result, error) {
			listCalls++
			return settlements.Result{}, nil
		},
	})
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			settlementHandlerPath(),
			nil,
		),
	)

	assertAPIError(
		t,
		response.Result(),
		http.StatusUnauthorized,
		"unauthorized",
	)
	if listCalls != 0 {
		t.Errorf("List calls = %d, want 0", listCalls)
	}
}

func settlementHandlerTestOptions(
	service fakeSettlementService,
) Options {
	options := groupHandlerTestOptions(fakeGroupService{})
	options.Settlements = service
	return options
}

func settlementHandlerPath() string {
	return "/api/groups/" + groupHandlerGroupID + "/settlements"
}

func assertSettlementResponseKeys(t *testing.T, body []byte) {
	t.Helper()

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode settlement response keys: %v", err)
	}
	if len(envelope) != 2 ||
		envelope["settlements"] == nil ||
		envelope["members"] == nil {
		t.Errorf("settlement envelope keys = %v", envelope)
		return
	}
	var transfers []map[string]json.RawMessage
	if err := json.Unmarshal(envelope["settlements"], &transfers); err != nil {
		t.Fatalf("decode settlements: %v", err)
	}
	for _, transfer := range transfers {
		if len(transfer) != 4 ||
			transfer["fromUserId"] == nil ||
			transfer["toUserId"] == nil ||
			transfer["amountCents"] == nil ||
			transfer["currency"] == nil {
			t.Errorf("settlement keys = %v", transfer)
		}
		for _, forbidden := range []string{
			"id",
			"groupId",
			"status",
			"optimized",
			"verified",
		} {
			if transfer[forbidden] != nil {
				t.Errorf("settlement contains forbidden field %q", forbidden)
			}
		}
	}
	var members []map[string]json.RawMessage
	if err := json.Unmarshal(envelope["members"], &members); err != nil {
		t.Fatalf("decode settlement members: %v", err)
	}
	for _, member := range members {
		if len(member) != 2 ||
			member["userId"] == nil ||
			member["displayName"] == nil {
			t.Errorf("member keys = %v", member)
		}
	}
}
