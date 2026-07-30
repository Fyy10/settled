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
	"github.com/Fyy10/settled/server/internal/repayments"
)

const repaymentHandlerRepaymentID = "22222222-3333-4444-8555-666666666666"

func TestListRepaymentsReturnsExactStableServiceOrder(t *testing.T) {
	t.Parallel()

	first := repaymentHandlerTestRepayment()
	second := first
	second.ID = "11111111-2222-4333-8444-555555555555"
	second.RepaymentDate = second.RepaymentDate.AddDate(0, 0, -1)
	result := repayments.ListResult{
		Repayments: []repayments.Repayment{first, second},
		Members: []repayments.MemberSummary{
			{UserID: groupHandlerActorID, DisplayName: "Alice"},
			{UserID: groupHandlerOtherID, DisplayName: "Bob"},
		},
	}
	calls := 0
	options := repaymentHandlerTestOptions(fakeRepaymentService{
		list: func(
			ctx context.Context,
			actorID string,
			groupID string,
		) (repayments.ListResult, error) {
			calls++
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
				"/repayments",
			nil,
			false,
		),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body repaymentsResponse
	decodeGroupJSONResponse(t, response.Result(), &body)
	want := repaymentsResponse{
		Repayments: []repaymentResponse{
			newRepaymentResponse(first),
			newRepaymentResponse(second),
		},
		Members: []repaymentMemberResponse{
			{UserID: groupHandlerActorID, DisplayName: "Alice"},
			{UserID: groupHandlerOtherID, DisplayName: "Bob"},
		},
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %#v, want %#v", body, want)
	}
	assertRepaymentListKeys(t, response.Body.Bytes())
	if calls != 1 {
		t.Errorf("List calls = %d, want 1", calls)
	}
}

func TestListRepaymentsUsesEmptyArrays(t *testing.T) {
	t.Parallel()

	options := repaymentHandlerTestOptions(fakeRepaymentService{
		list: func(
			context.Context,
			string,
			string,
		) (repayments.ListResult, error) {
			return repayments.ListResult{}, nil
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
			"/api/groups/"+groupHandlerGroupID+"/repayments",
			nil,
			false,
		),
	)

	if response.Code != http.StatusOK ||
		response.Body.String() != "{\"repayments\":[],\"members\":[]}\n" {
		t.Errorf(
			"empty response = %d %q",
			response.Code,
			response.Body.String(),
		)
	}
}

func TestCreateRepaymentPreservesNullableNoteSemantics(t *testing.T) {
	t.Parallel()

	blank := " \t "
	text := " Venmo "
	tests := []struct {
		name     string
		noteJSON string
		wantNote *string
	}{
		{name: "missing"},
		{name: "null", noteJSON: `,"note":null`},
		{name: "blank", noteJSON: `,"note":" \t "`, wantNote: &blank},
		{name: "text", noteJSON: `,"note":" Venmo "`, wantNote: &text},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			createCalls := 0
			options := repaymentHandlerTestOptions(fakeRepaymentService{
				create: func(
					ctx context.Context,
					actorID string,
					groupID string,
					input repayments.MutationInput,
				) (repayments.Repayment, error) {
					createCalls++
					if ctx == nil ||
						actorID != groupHandlerActorID ||
						groupID != groupHandlerGroupID {
						t.Errorf(
							"Create inputs = (%v, %q, %q)",
							ctx,
							actorID,
							groupID,
						)
					}
					want := repayments.MutationInput{
						FromUserID:    groupHandlerOtherID,
						ToUserID:      groupHandlerActorID,
						AmountCents:   2500,
						Note:          test.wantNote,
						NotePresent:   test.noteJSON != "",
						RepaymentDate: "2026-07-31",
					}
					if !reflect.DeepEqual(input, want) {
						t.Errorf("input = %#v, want %#v", input, want)
					}
					return repaymentHandlerTestRepayment(), nil
				},
			})
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			response := httptest.NewRecorder()
			request := groupHandlerRequest(
				http.MethodPost,
				"/api/groups/"+groupHandlerGroupID+"/repayments",
				strings.NewReader(
					`{"fromUserId":"`+
						groupHandlerOtherID+
						`","toUserId":"`+
						groupHandlerActorID+
						`","amountCents":2500`+
						test.noteJSON+
						`,"repaymentDate":"2026-07-31"}`,
				),
				true,
			)
			request.Header.Set("Content-Type", "application/json")

			api.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusCreated {
				t.Fatalf(
					"status = %d: %s",
					response.Code,
					response.Body.String(),
				)
			}
			assertRepaymentEnvelopeKeys(t, response.Body.Bytes())
			if createCalls != 1 {
				t.Errorf("Create calls = %d, want 1", createCalls)
			}
		})
	}
}

func TestRepaymentResponseAlwaysIncludesNullableNote(t *testing.T) {
	t.Parallel()

	repayment := repaymentHandlerTestRepayment()
	repayment.Note = nil
	options := repaymentHandlerTestOptions(fakeRepaymentService{
		get: func(
			context.Context,
			string,
			string,
			string,
		) (repayments.Repayment, error) {
			return repayment, nil
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
			repaymentHandlerItemPath(),
			nil,
			false,
		),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var envelope map[string]map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := string(envelope["repayment"]["note"]); got != "null" {
		t.Errorf("note = %s, want null", got)
	}
	assertRepaymentEnvelopeKeys(t, response.Body.Bytes())
}

func TestRepaymentItemGetReplaceAndDelete(t *testing.T) {
	t.Parallel()

	wantRepayment := repaymentHandlerTestRepayment()
	var calls []string
	options := repaymentHandlerTestOptions(fakeRepaymentService{
		get: func(
			_ context.Context,
			actorID string,
			groupID string,
			repaymentID string,
		) (repayments.Repayment, error) {
			calls = append(calls, "get")
			assertRepaymentServiceIDs(t, actorID, groupID, repaymentID)
			return wantRepayment, nil
		},
		replace: func(
			_ context.Context,
			actorID string,
			groupID string,
			repaymentID string,
			input repayments.MutationInput,
		) (repayments.Repayment, error) {
			calls = append(calls, "replace")
			assertRepaymentServiceIDs(t, actorID, groupID, repaymentID)
			want := repayments.MutationInput{
				FromUserID:    groupHandlerActorID,
				ToUserID:      groupHandlerOtherID,
				AmountCents:   3000,
				NotePresent:   true,
				RepaymentDate: "2026-08-01",
			}
			if !reflect.DeepEqual(input, want) {
				t.Errorf("replace input = %#v, want %#v", input, want)
			}
			replaced := wantRepayment
			replaced.FromUserID = groupHandlerActorID
			replaced.ToUserID = groupHandlerOtherID
			replaced.AmountCents = 3000
			replaced.Note = nil
			replaced.UpdatedAt = replaced.UpdatedAt.Add(time.Hour)
			return replaced, nil
		},
		delete: func(
			_ context.Context,
			actorID string,
			groupID string,
			repaymentID string,
		) error {
			calls = append(calls, "delete")
			assertRepaymentServiceIDs(t, actorID, groupID, repaymentID)
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
				"/repayments/"+
				strings.ToUpper(repaymentHandlerRepaymentID),
			nil,
			false,
		),
	)
	if getResponse.Code != http.StatusOK {
		t.Fatalf(
			"get status = %d: %s",
			getResponse.Code,
			getResponse.Body.String(),
		)
	}
	assertRepaymentEnvelopeKeys(t, getResponse.Body.Bytes())

	replaceResponse := httptest.NewRecorder()
	replaceRequest := groupHandlerRequest(
		http.MethodPut,
		repaymentHandlerItemPath(),
		strings.NewReader(`{
			"fromUserId":"`+groupHandlerActorID+`",
			"toUserId":"`+groupHandlerOtherID+`",
			"amountCents":3000,
			"note":null,
			"repaymentDate":"2026-08-01"
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
	assertRepaymentEnvelopeKeys(t, replaceResponse.Body.Bytes())

	deleteResponse := httptest.NewRecorder()
	api.Handler().ServeHTTP(
		deleteResponse,
		groupHandlerRequest(
			http.MethodDelete,
			repaymentHandlerItemPath(),
			nil,
			true,
		),
	)
	assertEmptyNoContent(t, deleteResponse)
	if !reflect.DeepEqual(calls, []string{"get", "replace", "delete"}) {
		t.Errorf("calls = %v", calls)
	}
}

func TestRepaymentMutationBodiesAreStrictAndValidationIs422(t *testing.T) {
	t.Parallel()

	validBody := validRepaymentHandlerBody()
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
			name:        "create unknown currency",
			method:      http.MethodPost,
			path:        repaymentHandlerCollectionPath(),
			body:        strings.TrimSuffix(validBody, "}") + `,"currency":"USD"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:        "create derived creator field",
			method:      http.MethodPost,
			path:        repaymentHandlerCollectionPath(),
			body:        strings.TrimSuffix(validBody, "}") + `,"createdByUserId":"x"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:        "create non-string note",
			method:      http.MethodPost,
			path:        repaymentHandlerCollectionPath(),
			body:        strings.Replace(validBody, `"note":"Venmo"`, `"note":{}`, 1),
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:        "create trailing JSON",
			method:      http.MethodPost,
			path:        repaymentHandlerCollectionPath(),
			body:        validBody + `{}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:       "create missing content type",
			method:     http.MethodPost,
			path:       repaymentHandlerCollectionPath(),
			body:       validBody,
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:        "replace omits required note",
			method:      http.MethodPut,
			path:        repaymentHandlerItemPath(),
			body:        strings.Replace(validBody, `,"note":"Venmo"`, "", 1),
			contentType: "application/json",
			serviceErr: &repayments.ValidationError{Fields: map[string]string{
				"note": "Note must be included in a full replacement.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantCalls:  1,
		},
		{
			name:        "create domain validation",
			method:      http.MethodPost,
			path:        repaymentHandlerCollectionPath(),
			body:        validBody,
			contentType: "application/json",
			serviceErr: &repayments.ValidationError{Fields: map[string]string{
				"amountCents": "Amount must be greater than zero.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantCalls:  1,
		},
		{
			name:        "hidden group",
			method:      http.MethodPost,
			path:        repaymentHandlerCollectionPath(),
			body:        validBody,
			contentType: "application/json",
			serviceErr:  repayments.ErrNotFound,
			wantStatus:  http.StatusNotFound,
			wantCode:    "not_found",
			wantCalls:   1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			service := fakeRepaymentService{
				create: func(
					context.Context,
					string,
					string,
					repayments.MutationInput,
				) (repayments.Repayment, error) {
					calls++
					return repayments.Repayment{}, test.serviceErr
				},
				replace: func(
					context.Context,
					string,
					string,
					string,
					repayments.MutationInput,
				) (repayments.Repayment, error) {
					calls++
					return repayments.Repayment{}, test.serviceErr
				},
			}
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				repaymentHandlerTestOptions(service),
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

func TestRepaymentHiddenStatesAndRepeatedDeleteAre404(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "list hidden group",
			method: http.MethodGet,
			path:   repaymentHandlerCollectionPath(),
		},
		{
			name:   "get cross-group child",
			method: http.MethodGet,
			path:   repaymentHandlerItemPath(),
		},
		{
			name:   "replace deleted repayment",
			method: http.MethodPut,
			path:   repaymentHandlerItemPath(),
		},
		{
			name:   "repeated delete",
			method: http.MethodDelete,
			path:   repaymentHandlerItemPath(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := repaymentHandlerTestOptions(fakeRepaymentService{
				list: func(
					context.Context,
					string,
					string,
				) (repayments.ListResult, error) {
					return repayments.ListResult{}, repayments.ErrNotFound
				},
				get: func(
					context.Context,
					string,
					string,
					string,
				) (repayments.Repayment, error) {
					return repayments.Repayment{}, repayments.ErrNotFound
				},
				replace: func(
					context.Context,
					string,
					string,
					string,
					repayments.MutationInput,
				) (repayments.Repayment, error) {
					return repayments.Repayment{}, repayments.ErrNotFound
				},
				delete: func(context.Context, string, string, string) error {
					return repayments.ErrNotFound
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
				body = strings.NewReader(validRepaymentHandlerBody())
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

func TestRepaymentRoutesRejectInvalidUUIDsBeforeService(t *testing.T) {
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
			path:   "/api/groups/not-a-uuid/repayments",
		},
		{
			name:   "create group ID",
			method: http.MethodPost,
			path:   "/api/groups/not-a-uuid/repayments",
			unsafe: true,
		},
		{
			name:   "get group ID",
			method: http.MethodGet,
			path: "/api/groups/not-a-uuid/repayments/" +
				repaymentHandlerRepaymentID,
		},
		{
			name:   "get repayment ID",
			method: http.MethodGet,
			path: repaymentHandlerCollectionPath() +
				"/not-a-uuid",
		},
		{
			name:   "replace repayment ID",
			method: http.MethodPut,
			path: repaymentHandlerCollectionPath() +
				"/not-a-uuid",
			unsafe: true,
		},
		{
			name:   "delete repayment ID",
			method: http.MethodDelete,
			path: repaymentHandlerCollectionPath() +
				"/not-a-uuid",
			unsafe: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			options := repaymentHandlerTestOptions(fakeRepaymentService{
				list: func(
					context.Context,
					string,
					string,
				) (repayments.ListResult, error) {
					calls++
					return repayments.ListResult{}, nil
				},
				create: func(
					context.Context,
					string,
					string,
					repayments.MutationInput,
				) (repayments.Repayment, error) {
					calls++
					return repayments.Repayment{}, nil
				},
				get: func(
					context.Context,
					string,
					string,
					string,
				) (repayments.Repayment, error) {
					calls++
					return repayments.Repayment{}, nil
				},
				replace: func(
					context.Context,
					string,
					string,
					string,
					repayments.MutationInput,
				) (repayments.Repayment, error) {
					calls++
					return repayments.Repayment{}, nil
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

func TestDeleteRepaymentRejectsBodyBeforeService(t *testing.T) {
	t.Parallel()

	deleteCalls := 0
	options := repaymentHandlerTestOptions(fakeRepaymentService{
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
			repaymentHandlerItemPath(),
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

func TestRepaymentRouteSecurityRunsBeforeService(t *testing.T) {
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
			options := repaymentHandlerTestOptions(fakeRepaymentService{
				create: func(
					context.Context,
					string,
					string,
					repayments.MutationInput,
				) (repayments.Repayment, error) {
					createCalls++
					return repaymentHandlerTestRepayment(), nil
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
				repaymentHandlerCollectionPath(),
				strings.NewReader(validRepaymentHandlerBody()),
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

func TestRepaymentReadRequiresAuthenticationBeforeService(t *testing.T) {
	t.Parallel()

	listCalls := 0
	options := repaymentHandlerTestOptions(fakeRepaymentService{
		list: func(
			context.Context,
			string,
			string,
		) (repayments.ListResult, error) {
			listCalls++
			return repayments.ListResult{}, nil
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
			repaymentHandlerCollectionPath(),
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

func repaymentHandlerTestOptions(service fakeRepaymentService) Options {
	options := groupHandlerTestOptions(fakeGroupService{})
	options.Repayments = service
	return options
}

func repaymentHandlerTestRepayment() repayments.Repayment {
	note := "Venmo"
	return repayments.Repayment{
		ID:              repaymentHandlerRepaymentID,
		GroupID:         groupHandlerGroupID,
		FromUserID:      groupHandlerOtherID,
		ToUserID:        groupHandlerActorID,
		AmountCents:     2500,
		Currency:        "USD",
		Note:            &note,
		RepaymentDate:   time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC),
		CreatedByUserID: groupHandlerActorID,
		CreatedAt:       groupHandlerTestNow,
		UpdatedAt:       groupHandlerTestNow.Add(time.Hour),
	}
}

func validRepaymentHandlerBody() string {
	return `{
		"fromUserId":"` + groupHandlerOtherID + `",
		"toUserId":"` + groupHandlerActorID + `",
		"amountCents":2500,
		"note":"Venmo",
		"repaymentDate":"2026-07-31"
	}`
}

func repaymentHandlerCollectionPath() string {
	return "/api/groups/" + groupHandlerGroupID + "/repayments"
}

func repaymentHandlerItemPath() string {
	return repaymentHandlerCollectionPath() +
		"/" +
		repaymentHandlerRepaymentID
}

func assertRepaymentServiceIDs(
	t *testing.T,
	actorID string,
	groupID string,
	repaymentID string,
) {
	t.Helper()
	if actorID != groupHandlerActorID ||
		groupID != groupHandlerGroupID ||
		repaymentID != repaymentHandlerRepaymentID {
		t.Errorf(
			"service IDs = (%q, %q, %q)",
			actorID,
			groupID,
			repaymentID,
		)
	}
}

func assertRepaymentEnvelopeKeys(t *testing.T, body []byte) {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode repayment envelope keys: %v", err)
	}
	if len(envelope) != 1 || envelope["repayment"] == nil {
		t.Errorf("repayment envelope keys = %v", envelope)
		return
	}
	assertRepaymentKeys(t, envelope["repayment"])
}

func assertRepaymentListKeys(t *testing.T, body []byte) {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode repayment list keys: %v", err)
	}
	if len(envelope) != 2 ||
		envelope["repayments"] == nil ||
		envelope["members"] == nil {
		t.Errorf("repayment list keys = %v", envelope)
		return
	}
	var found []json.RawMessage
	if err := json.Unmarshal(envelope["repayments"], &found); err != nil {
		t.Fatalf("decode repayments: %v", err)
	}
	for _, repayment := range found {
		assertRepaymentKeys(t, repayment)
	}
	var members []map[string]json.RawMessage
	if err := json.Unmarshal(envelope["members"], &members); err != nil {
		t.Fatalf("decode repayment members: %v", err)
	}
	for _, member := range members {
		if len(member) != 2 ||
			member["userId"] == nil ||
			member["displayName"] == nil {
			t.Errorf("member keys = %v", member)
		}
	}
}

func assertRepaymentKeys(t *testing.T, body []byte) {
	t.Helper()
	var repayment map[string]json.RawMessage
	if err := json.Unmarshal(body, &repayment); err != nil {
		t.Fatalf("decode repayment keys: %v", err)
	}
	for _, field := range []string{
		"id",
		"groupId",
		"fromUserId",
		"toUserId",
		"amountCents",
		"currency",
		"note",
		"repaymentDate",
		"createdByUserId",
		"createdAt",
		"updatedAt",
	} {
		if repayment[field] == nil {
			t.Errorf("repayment lacks %q: %v", field, repayment)
		}
	}
	if len(repayment) != 11 {
		t.Errorf("repayment keys = %v", repayment)
	}
	for _, forbidden := range []string{
		"sent",
		"verified",
		"status",
		"processedAt",
	} {
		if repayment[forbidden] != nil {
			t.Errorf("repayment contains forbidden field %q", forbidden)
		}
	}
}
