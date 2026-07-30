package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/expenses"
)

const expenseHandlerExpenseID = "22222222-3333-4444-8555-666666666666"

func TestListExpensesReturnsExactStableServiceOrder(t *testing.T) {
	t.Parallel()

	first := expenseHandlerTestExpense()
	second := first
	second.ID = "11111111-2222-4333-8444-555555555555"
	second.Description = "Earlier"
	second.ExpenseDate = second.ExpenseDate.AddDate(0, 0, -1)
	result := expenses.ListResult{
		Expenses: []expenses.Expense{first, second},
		Members: []expenses.MemberSummary{
			{UserID: groupHandlerActorID, DisplayName: "Alice"},
			{UserID: groupHandlerOtherID, DisplayName: "Bob"},
		},
	}
	listCalls := 0
	options := expenseHandlerTestOptions(fakeExpenseService{
		list: func(
			ctx context.Context,
			actorID string,
			groupID string,
		) (expenses.ListResult, error) {
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
			"/api/groups/"+strings.ToUpper(groupHandlerGroupID)+"/expenses",
			nil,
			false,
		),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body expensesResponse
	decodeGroupJSONResponse(t, response.Result(), &body)
	want := expensesResponse{
		Expenses: []expenseResponse{
			newExpenseResponse(first),
			newExpenseResponse(second),
		},
		Members: []expenseMemberResponse{
			{UserID: groupHandlerActorID, DisplayName: "Alice"},
			{UserID: groupHandlerOtherID, DisplayName: "Bob"},
		},
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %#v, want %#v", body, want)
	}
	assertExpenseListKeys(t, response.Body.Bytes())
	if listCalls != 1 {
		t.Errorf("List calls = %d, want 1", listCalls)
	}

	emptyOptions := expenseHandlerTestOptions(fakeExpenseService{
		list: func(
			context.Context,
			string,
			string,
		) (expenses.ListResult, error) {
			return expenses.ListResult{}, nil
		},
	})
	emptyAPI, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		emptyOptions,
	)
	emptyResponse := httptest.NewRecorder()
	emptyAPI.Handler().ServeHTTP(
		emptyResponse,
		groupHandlerRequest(
			http.MethodGet,
			"/api/groups/"+groupHandlerGroupID+"/expenses",
			nil,
			false,
		),
	)
	if emptyResponse.Code != http.StatusOK ||
		emptyResponse.Body.String() != "{\"expenses\":[],\"members\":[]}\n" {
		t.Errorf(
			"empty response = %d %q",
			emptyResponse.Code,
			emptyResponse.Body.String(),
		)
	}
}

func TestCreateExpenseAcceptsAllSplitModesAndReturnsExactSplits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		wantInput expenses.MutationInput
	}{
		{
			name: "equal",
			body: `{
				"paidByUserId":"` + groupHandlerActorID + `",
				"description":" Groceries ",
				"amountCents":10,
				"expenseDate":"2026-07-31",
				"splitMode":"equal",
				"participantUserIds":[
					"` + groupHandlerOtherID + `",
					"` + groupHandlerActorID + `"
				]
			}`,
			wantInput: expenses.MutationInput{
				PaidByUserID: groupHandlerActorID,
				Description:  " Groceries ",
				AmountCents:  10,
				ExpenseDate:  "2026-07-31",
				SplitInput: expenses.SplitInput{
					Mode: expenses.SplitModeEqual,
					ParticipantUserIDs: []string{
						groupHandlerOtherID,
						groupHandlerActorID,
					},
					Splits:           []expenses.ExactSplitInput{},
					PercentageSplits: []expenses.PercentageSplitInput{},
				},
			},
		},
		{
			name: "exact",
			body: `{
				"paidByUserId":"` + groupHandlerActorID + `",
				"description":"Groceries",
				"amountCents":10,
				"expenseDate":"2026-07-31",
				"splitMode":"exact",
				"splits":[
					{"userId":"` + groupHandlerOtherID + `","amountCents":6},
					{"userId":"` + groupHandlerActorID + `","amountCents":4}
				]
			}`,
			wantInput: expenses.MutationInput{
				PaidByUserID: groupHandlerActorID,
				Description:  "Groceries",
				AmountCents:  10,
				ExpenseDate:  "2026-07-31",
				SplitInput: expenses.SplitInput{
					Mode:               expenses.SplitModeExact,
					ParticipantUserIDs: []string{},
					Splits: []expenses.ExactSplitInput{
						{UserID: groupHandlerOtherID, AmountCents: 6},
						{UserID: groupHandlerActorID, AmountCents: 4},
					},
					PercentageSplits: []expenses.PercentageSplitInput{},
				},
			},
		},
		{
			name: "percentage",
			body: `{
				"paidByUserId":"` + groupHandlerActorID + `",
				"description":"Groceries",
				"amountCents":10,
				"expenseDate":"2026-07-31",
				"splitMode":"percentage",
				"percentageSplits":[
					{"userId":"` + groupHandlerOtherID + `","percentageBasisPoints":6000},
					{"userId":"` + groupHandlerActorID + `","percentageBasisPoints":4000}
				]
			}`,
			wantInput: expenses.MutationInput{
				PaidByUserID: groupHandlerActorID,
				Description:  "Groceries",
				AmountCents:  10,
				ExpenseDate:  "2026-07-31",
				SplitInput: expenses.SplitInput{
					Mode:               expenses.SplitModePercentage,
					ParticipantUserIDs: []string{},
					Splits:             []expenses.ExactSplitInput{},
					PercentageSplits: []expenses.PercentageSplitInput{
						{
							UserID:                groupHandlerOtherID,
							PercentageBasisPoints: 6000,
						},
						{
							UserID:                groupHandlerActorID,
							PercentageBasisPoints: 4000,
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			createCalls := 0
			wantExpense := expenseHandlerTestExpense()
			options := expenseHandlerTestOptions(fakeExpenseService{
				create: func(
					ctx context.Context,
					actorID string,
					groupID string,
					input expenses.MutationInput,
				) (expenses.Expense, error) {
					createCalls++
					if ctx == nil ||
						actorID != groupHandlerActorID ||
						groupID != groupHandlerGroupID ||
						!reflect.DeepEqual(input, test.wantInput) {
						t.Errorf(
							"Create inputs = (%v, %q, %q, %#v), want %#v",
							ctx,
							actorID,
							groupID,
							input,
							test.wantInput,
						)
					}
					return wantExpense, nil
				},
			})
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := groupHandlerRequest(
				http.MethodPost,
				"/api/groups/"+
					strings.ToUpper(groupHandlerGroupID)+
					"/expenses",
				strings.NewReader(test.body),
				true,
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusCreated {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
			var body expenseResponseEnvelope
			decodeGroupJSONResponse(t, response.Result(), &body)
			wantBody := expenseResponseEnvelope{
				Expense: newExpenseResponse(wantExpense),
			}
			if !reflect.DeepEqual(body, wantBody) {
				t.Errorf("body = %#v, want %#v", body, wantBody)
			}
			assertExpenseEnvelopeKeys(t, response.Body.Bytes())
			if createCalls != 1 {
				t.Errorf("Create calls = %d, want 1", createCalls)
			}
		})
	}
}

func TestExpenseMutationBodiesAreStrictAndValidationIs422(t *testing.T) {
	t.Parallel()

	validBody := validExpenseHandlerBody()
	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		serviceErr  error
		wantStatus  int
		wantCode    string
		wantCalls   int
	}{
		{
			name:        "create unknown field",
			method:      http.MethodPost,
			path:        "/api/groups/" + groupHandlerGroupID + "/expenses",
			body:        strings.TrimSuffix(validBody, "}") + `,"currency":"USD"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:   "create nested unknown field",
			method: http.MethodPost,
			path:   "/api/groups/" + groupHandlerGroupID + "/expenses",
			body: `{
				"paidByUserId":"` + groupHandlerActorID + `",
				"description":"Expense",
				"amountCents":10,
				"expenseDate":"2026-07-31",
				"splitMode":"exact",
				"splits":[{"userId":"` + groupHandlerActorID + `","amountCents":10,"position":1}]
			}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:        "create trailing JSON",
			method:      http.MethodPost,
			path:        "/api/groups/" + groupHandlerGroupID + "/expenses",
			body:        validBody + `{}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:       "create missing content type",
			method:     http.MethodPost,
			path:       "/api/groups/" + groupHandlerGroupID + "/expenses",
			body:       validBody,
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:        "replace unknown field",
			method:      http.MethodPut,
			path:        expenseHandlerItemPath(),
			body:        strings.TrimSuffix(validBody, "}") + `,"createdByUserId":"hidden"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:        "replace missing editable field",
			method:      http.MethodPut,
			path:        expenseHandlerItemPath(),
			body:        `{}`,
			contentType: "application/json",
			serviceErr: &expenses.ValidationError{Fields: map[string]string{
				"paidByUserId": "Paid-by user ID is required.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantCalls:  1,
		},
		{
			name:        "create validation",
			method:      http.MethodPost,
			path:        "/api/groups/" + groupHandlerGroupID + "/expenses",
			body:        validBody,
			contentType: "application/json",
			serviceErr: &expenses.ValidationError{Fields: map[string]string{
				"splits": "Exact split amounts must total amountCents.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantCalls:  1,
		},
		{
			name:        "hidden group",
			method:      http.MethodPost,
			path:        "/api/groups/" + groupHandlerGroupID + "/expenses",
			body:        validBody,
			contentType: "application/json",
			serviceErr:  expenses.ErrNotFound,
			wantStatus:  http.StatusNotFound,
			wantCode:    "not_found",
			wantCalls:   1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			service := fakeExpenseService{
				create: func(
					context.Context,
					string,
					string,
					expenses.MutationInput,
				) (expenses.Expense, error) {
					calls++
					return expenses.Expense{}, test.serviceErr
				},
				replace: func(
					context.Context,
					string,
					string,
					string,
					expenses.MutationInput,
				) (expenses.Expense, error) {
					calls++
					return expenses.Expense{}, test.serviceErr
				},
			}
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				expenseHandlerTestOptions(service),
			)
			request := groupHandlerRequest(
				test.method,
				test.path,
				strings.NewReader(test.body),
				true,
			)
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			assertAPIError(
				t,
				response.Result(),
				test.wantStatus,
				test.wantCode,
			)
			if calls != test.wantCalls {
				t.Errorf("service calls = %d, want %d", calls, test.wantCalls)
			}
		})
	}
}

func TestExpenseItemGetReplaceAndDelete(t *testing.T) {
	t.Parallel()

	wantExpense := expenseHandlerTestExpense()
	replaceInput := expenses.MutationInput{
		PaidByUserID: groupHandlerActorID,
		Description:  " Dinner ",
		AmountCents:  12,
		ExpenseDate:  "2026-08-01",
		SplitInput: expenses.SplitInput{
			Mode:               expenses.SplitModeEqual,
			ParticipantUserIDs: []string{groupHandlerOtherID, groupHandlerActorID},
			Splits:             []expenses.ExactSplitInput{},
			PercentageSplits:   []expenses.PercentageSplitInput{},
		},
	}
	var calls []string
	options := expenseHandlerTestOptions(fakeExpenseService{
		get: func(
			_ context.Context,
			actorID string,
			groupID string,
			expenseID string,
		) (expenses.Expense, error) {
			calls = append(calls, "get")
			assertExpenseServiceIDs(t, actorID, groupID, expenseID)
			return wantExpense, nil
		},
		replace: func(
			_ context.Context,
			actorID string,
			groupID string,
			expenseID string,
			input expenses.MutationInput,
		) (expenses.Expense, error) {
			calls = append(calls, "replace")
			assertExpenseServiceIDs(t, actorID, groupID, expenseID)
			if !reflect.DeepEqual(input, replaceInput) {
				t.Errorf("replace input = %#v, want %#v", input, replaceInput)
			}
			replaced := wantExpense
			replaced.Description = "Dinner"
			replaced.AmountCents = 12
			replaced.UpdatedAt = replaced.UpdatedAt.Add(time.Hour)
			return replaced, nil
		},
		delete: func(
			_ context.Context,
			actorID string,
			groupID string,
			expenseID string,
		) error {
			calls = append(calls, "delete")
			assertExpenseServiceIDs(t, actorID, groupID, expenseID)
			return nil
		},
	})
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)

	getResponse := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		getResponse,
		groupHandlerRequest(
			http.MethodGet,
			"/api/groups/"+
				strings.ToUpper(groupHandlerGroupID)+
				"/expenses/"+
				strings.ToUpper(expenseHandlerExpenseID),
			nil,
			false,
		),
	)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d: %s", getResponse.Code, getResponse.Body.String())
	}
	var gotBody expenseResponseEnvelope
	decodeGroupJSONResponse(t, getResponse.Result(), &gotBody)
	if !reflect.DeepEqual(gotBody, expenseResponseEnvelope{
		Expense: newExpenseResponse(wantExpense),
	}) {
		t.Errorf("get body = %#v", gotBody)
	}

	replaceResponse := httptest.NewRecorder()
	replaceRequest := groupHandlerRequest(
		http.MethodPut,
		expenseHandlerItemPath(),
		strings.NewReader(`{
			"paidByUserId":"`+groupHandlerActorID+`",
			"description":" Dinner ",
			"amountCents":12,
			"expenseDate":"2026-08-01",
			"splitMode":"equal",
			"participantUserIds":["`+groupHandlerOtherID+`","`+groupHandlerActorID+`"]
		}`),
		true,
	)
	replaceRequest.Header.Set("Content-Type", "application/json")
	api.Handler().ServeHTTP(replaceResponse, replaceRequest)
	if replaceResponse.Code != http.StatusOK {
		t.Fatalf(
			"replace status = %d: %s",
			replaceResponse.Code,
			replaceResponse.Body.String(),
		)
	}

	deleteResponse := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		deleteResponse,
		groupHandlerRequest(
			http.MethodDelete,
			expenseHandlerItemPath(),
			nil,
			true,
		),
	)
	assertEmptyNoContent(t, deleteResponse)
	if !reflect.DeepEqual(calls, []string{"get", "replace", "delete"}) {
		t.Errorf("calls = %v", calls)
	}
}

func TestExpenseHiddenStatesAndRepeatedDeleteAre404(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "list hidden group",
			method: http.MethodGet,
			path:   "/api/groups/" + groupHandlerGroupID + "/expenses",
		},
		{
			name:   "get cross-group child",
			method: http.MethodGet,
			path:   expenseHandlerItemPath(),
		},
		{
			name:   "replace deleted expense",
			method: http.MethodPut,
			path:   expenseHandlerItemPath(),
		},
		{
			name:   "repeated delete",
			method: http.MethodDelete,
			path:   expenseHandlerItemPath(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := expenseHandlerTestOptions(fakeExpenseService{
				list: func(
					context.Context,
					string,
					string,
				) (expenses.ListResult, error) {
					return expenses.ListResult{}, expenses.ErrNotFound
				},
				get: func(
					context.Context,
					string,
					string,
					string,
				) (expenses.Expense, error) {
					return expenses.Expense{}, expenses.ErrNotFound
				},
				replace: func(
					context.Context,
					string,
					string,
					string,
					expenses.MutationInput,
				) (expenses.Expense, error) {
					return expenses.Expense{}, expenses.ErrNotFound
				},
				delete: func(context.Context, string, string, string) error {
					return expenses.ErrNotFound
				},
			})
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			var body io.Reader
			unsafe := test.method == http.MethodPut ||
				test.method == http.MethodDelete
			if test.method == http.MethodPut {
				body = strings.NewReader(validExpenseHandlerBody())
			}
			request := groupHandlerRequest(
				test.method,
				test.path,
				body,
				unsafe,
			)
			if test.method == http.MethodPut {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			assertAPIError(
				t,
				response.Result(),
				http.StatusNotFound,
				"not_found",
			)
		})
	}
}

func TestExpenseRoutesRejectInvalidUUIDsBeforeService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		unsafe bool
	}{
		{
			name:   "list group ID",
			method: http.MethodGet,
			path:   "/api/groups/not-a-uuid/expenses",
		},
		{
			name:   "create group ID",
			method: http.MethodPost,
			path:   "/api/groups/not-a-uuid/expenses",
			unsafe: true,
		},
		{
			name:   "get group ID",
			method: http.MethodGet,
			path: "/api/groups/not-a-uuid/expenses/" +
				expenseHandlerExpenseID,
		},
		{
			name:   "get expense ID",
			method: http.MethodGet,
			path: "/api/groups/" +
				groupHandlerGroupID +
				"/expenses/not-a-uuid",
		},
		{
			name:   "replace expense ID",
			method: http.MethodPut,
			path: "/api/groups/" +
				groupHandlerGroupID +
				"/expenses/not-a-uuid",
			unsafe: true,
		},
		{
			name:   "delete expense ID",
			method: http.MethodDelete,
			path: "/api/groups/" +
				groupHandlerGroupID +
				"/expenses/not-a-uuid",
			unsafe: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			options := expenseHandlerTestOptions(fakeExpenseService{
				list: func(
					context.Context,
					string,
					string,
				) (expenses.ListResult, error) {
					calls++
					return expenses.ListResult{}, nil
				},
				create: func(
					context.Context,
					string,
					string,
					expenses.MutationInput,
				) (expenses.Expense, error) {
					calls++
					return expenses.Expense{}, nil
				},
				get: func(
					context.Context,
					string,
					string,
					string,
				) (expenses.Expense, error) {
					calls++
					return expenses.Expense{}, nil
				},
				replace: func(
					context.Context,
					string,
					string,
					string,
					expenses.MutationInput,
				) (expenses.Expense, error) {
					calls++
					return expenses.Expense{}, nil
				},
				delete: func(context.Context, string, string, string) error {
					calls++
					return nil
				},
			})
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := groupHandlerRequest(
				test.method,
				test.path,
				nil,
				test.unsafe,
			)
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			assertAPIError(
				t,
				response.Result(),
				http.StatusBadRequest,
				"bad_request",
			)
			if calls != 0 {
				t.Errorf("service calls = %d, want 0", calls)
			}
		})
	}
}

func TestDeleteExpenseRejectsBodyBeforeService(t *testing.T) {
	t.Parallel()

	deleteCalls := 0
	options := expenseHandlerTestOptions(fakeExpenseService{
		delete: func(context.Context, string, string, string) error {
			deleteCalls++
			return nil
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
			http.MethodDelete,
			expenseHandlerItemPath(),
			strings.NewReader("{}"),
			true,
		),
	)

	assertAPIError(
		t,
		response.Result(),
		http.StatusBadRequest,
		"bad_request",
	)
	if deleteCalls != 0 {
		t.Errorf("Delete calls = %d, want 0", deleteCalls)
	}
}

func TestExpenseRouteSecurityRunsBeforeService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		session    bool
		origin     bool
		csrfError  error
		wantStatus int
		wantCode   string
		wantCSRF   int
		wantCreate int
	}{
		{
			name:       "authentication first",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "origin second",
			session:    true,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "CSRF third",
			session:    true,
			origin:     true,
			csrfError:  auth.ErrCSRFInvalid,
			wantStatus: http.StatusForbidden,
			wantCode:   "csrf_invalid",
			wantCSRF:   1,
		},
		{
			name:       "handler last",
			session:    true,
			origin:     true,
			wantStatus: http.StatusCreated,
			wantCSRF:   1,
			wantCreate: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			csrfCalls := 0
			createCalls := 0
			options := expenseHandlerTestOptions(fakeExpenseService{
				create: func(
					context.Context,
					string,
					string,
					expenses.MutationInput,
				) (expenses.Expense, error) {
					createCalls++
					return expenseHandlerTestExpense(), nil
				},
			})
			options.CSRF = fakeCSRFProtector{
				validateAuthenticated: func(
					string,
					string,
					auth.Session,
				) error {
					csrfCalls++
					return test.csrfError
				},
			}
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := groupHandlerRequest(
				http.MethodPost,
				"/api/groups/"+groupHandlerGroupID+"/expenses",
				strings.NewReader(validExpenseHandlerBody()),
				true,
			)
			request.Header.Set("Content-Type", "application/json")
			if !test.session {
				removeCookie(request, auth.SessionCookieName)
			}
			if !test.origin {
				request.Header.Del("Origin")
			}
			response := httptest.NewRecorder()

			api.Handler().ServeHTTP(response, request)

			if test.wantCode == "" {
				if response.Code != test.wantStatus {
					t.Errorf(
						"status = %d, want %d: %s",
						response.Code,
						test.wantStatus,
						response.Body.String(),
					)
				}
			} else {
				assertAPIError(
					t,
					response.Result(),
					test.wantStatus,
					test.wantCode,
				)
			}
			if csrfCalls != test.wantCSRF {
				t.Errorf("CSRF calls = %d, want %d", csrfCalls, test.wantCSRF)
			}
			if createCalls != test.wantCreate {
				t.Errorf("Create calls = %d, want %d", createCalls, test.wantCreate)
			}
		})
	}
}

func TestExpenseReadRequiresAuthenticationBeforeService(t *testing.T) {
	t.Parallel()

	listCalls := 0
	options := expenseHandlerTestOptions(fakeExpenseService{
		list: func(
			context.Context,
			string,
			string,
		) (expenses.ListResult, error) {
			listCalls++
			return expenses.ListResult{}, nil
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
			"/api/groups/"+groupHandlerGroupID+"/expenses",
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

func expenseHandlerTestOptions(service fakeExpenseService) Options {
	options := groupHandlerTestOptions(fakeGroupService{})
	options.Expenses = service
	return options
}

func expenseHandlerTestExpense() expenses.Expense {
	return expenses.Expense{
		ID:           expenseHandlerExpenseID,
		GroupID:      groupHandlerGroupID,
		PaidByUserID: groupHandlerActorID,
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
		CreatedByUserID: groupHandlerOtherID,
		Splits: []expenses.Split{
			{UserID: groupHandlerOtherID, AmountCents: 6},
			{UserID: groupHandlerActorID, AmountCents: 4},
		},
		CreatedAt: groupHandlerTestNow,
		UpdatedAt: groupHandlerTestNow.Add(time.Hour),
	}
}

func validExpenseHandlerBody() string {
	return `{
		"paidByUserId":"` + groupHandlerActorID + `",
		"description":"Expense",
		"amountCents":10,
		"expenseDate":"2026-07-31",
		"splitMode":"exact",
		"splits":[{"userId":"` + groupHandlerActorID + `","amountCents":10}]
	}`
}

func expenseHandlerItemPath() string {
	return "/api/groups/" +
		groupHandlerGroupID +
		"/expenses/" +
		expenseHandlerExpenseID
}

func assertExpenseServiceIDs(
	t *testing.T,
	actorID string,
	groupID string,
	expenseID string,
) {
	t.Helper()
	if actorID != groupHandlerActorID ||
		groupID != groupHandlerGroupID ||
		expenseID != expenseHandlerExpenseID {
		t.Errorf(
			"service IDs = (%q, %q, %q)",
			actorID,
			groupID,
			expenseID,
		)
	}
}

func assertExpenseEnvelopeKeys(t *testing.T, body []byte) {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode expense envelope keys: %v", err)
	}
	if len(envelope) != 1 || envelope["expense"] == nil {
		t.Errorf("expense envelope keys = %v", envelope)
		return
	}
	assertExpenseKeys(t, envelope["expense"])
}

func assertExpenseListKeys(t *testing.T, body []byte) {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode expense list keys: %v", err)
	}
	if len(envelope) != 2 ||
		envelope["expenses"] == nil ||
		envelope["members"] == nil {
		t.Errorf("expense list keys = %v", envelope)
		return
	}
	var found []json.RawMessage
	if err := json.Unmarshal(envelope["expenses"], &found); err != nil {
		t.Fatalf("decode expenses: %v", err)
	}
	for _, expense := range found {
		assertExpenseKeys(t, expense)
	}
	var members []map[string]json.RawMessage
	if err := json.Unmarshal(envelope["members"], &members); err != nil {
		t.Fatalf("decode expense members: %v", err)
	}
	for _, member := range members {
		if len(member) != 2 ||
			member["userId"] == nil ||
			member["displayName"] == nil {
			t.Errorf("member keys = %v", member)
		}
	}
}

func assertExpenseKeys(t *testing.T, body []byte) {
	t.Helper()
	var expense map[string]json.RawMessage
	if err := json.Unmarshal(body, &expense); err != nil {
		t.Fatalf("decode expense keys: %v", err)
	}
	for _, field := range []string{
		"id",
		"groupId",
		"paidByUserId",
		"description",
		"amountCents",
		"currency",
		"expenseDate",
		"createdByUserId",
		"splits",
		"createdAt",
		"updatedAt",
	} {
		if expense[field] == nil {
			t.Errorf("expense lacks %q: %v", field, expense)
		}
	}
	if len(expense) != 11 {
		t.Errorf("expense keys = %v", expense)
	}
	var splits []map[string]json.RawMessage
	if err := json.Unmarshal(expense["splits"], &splits); err != nil {
		t.Fatalf("decode split keys: %v", err)
	}
	for _, split := range splits {
		if len(split) != 2 ||
			split["userId"] == nil ||
			split["amountCents"] == nil {
			t.Errorf("split keys = %v", split)
		}
	}
}
