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

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/groups"
)

func TestRenameGroupReturnsExactSummary(t *testing.T) {
	t.Parallel()

	group := groupHandlerTestGroup()
	group.Name = "Beach Trip"
	group.UpdatedAt = group.UpdatedAt.AddDate(0, 0, 1)
	renameCalls := 0
	options := groupHandlerTestOptions(fakeGroupService{
		rename: func(
			_ context.Context,
			actorID string,
			groupID string,
			name string,
		) (groups.Group, error) {
			renameCalls++
			if actorID != groupHandlerActorID ||
				groupID != groupHandlerGroupID ||
				name != " Beach Trip " {
				t.Errorf(
					"Rename inputs = (%q, %q, %q)",
					actorID,
					groupID,
					name,
				)
			}
			return group, nil
		},
	})
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := groupHandlerRequest(
		http.MethodPatch,
		"/api/groups/"+strings.ToUpper(groupHandlerGroupID),
		strings.NewReader(`{"name":" Beach Trip "}`),
		true,
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	rawBody := response.Body.String()
	if strings.Contains(rawBody, "joinCode") {
		t.Errorf("rename response exposes join code: %s", rawBody)
	}
	var body groupResponseEnvelope
	decodeGroupJSONResponse(t, response.Result(), &body)
	if body != (groupResponseEnvelope{Group: newGroupResponse(group)}) {
		t.Errorf("body = %+v", body)
	}
	assertGroupEnvelopeKeys(t, []byte(rawBody), false)
	if renameCalls != 1 {
		t.Errorf("Rename calls = %d, want 1", renameCalls)
	}
}

func TestRenameGroupStrictJSONValidationAndErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		contentType string
		serviceErr  error
		wantStatus  int
		wantCode    string
		wantCalls   int
	}{
		{
			name:        "unknown field",
			body:        `{"name":"Beach Trip","joinCode":"not-accepted"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:        "trailing JSON",
			body:        `{"name":"Beach Trip"} {}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:       "missing content type",
			body:       `{"name":"Beach Trip"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:        "omitted name",
			body:        `{}`,
			contentType: "application/json",
			serviceErr: &groups.ValidationError{Fields: map[string]string{
				"name": "Group name is required.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantCalls:  1,
		},
		{
			name:        "null name",
			body:        `{"name":null}`,
			contentType: "application/json",
			serviceErr: &groups.ValidationError{Fields: map[string]string{
				"name": "Group name is required.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantCalls:  1,
		},
		{
			name:        "active member is not owner",
			body:        `{"name":"Beach Trip"}`,
			contentType: "application/json",
			serviceErr:  groups.ErrForbidden,
			wantStatus:  http.StatusForbidden,
			wantCode:    "forbidden",
			wantCalls:   1,
		},
		{
			name:        "hidden group",
			body:        `{"name":"Beach Trip"}`,
			contentType: "application/json",
			serviceErr:  groups.ErrNotFound,
			wantStatus:  http.StatusNotFound,
			wantCode:    "not_found",
			wantCalls:   1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			renameCalls := 0
			options := groupHandlerTestOptions(fakeGroupService{
				rename: func(
					context.Context,
					string,
					string,
					string,
				) (groups.Group, error) {
					renameCalls++
					return groups.Group{}, test.serviceErr
				},
			})
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := groupHandlerRequest(
				http.MethodPatch,
				"/api/groups/"+groupHandlerGroupID,
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
			if renameCalls != test.wantCalls {
				t.Errorf("Rename calls = %d, want %d", renameCalls, test.wantCalls)
			}
		})
	}
}

func TestDissolveGroupReturnsEmpty204AndMapsOwnerState(t *testing.T) {
	t.Parallel()

	dissolveCalls := 0
	options := groupHandlerTestOptions(fakeGroupService{
		dissolve: func(
			_ context.Context,
			actorID string,
			groupID string,
		) error {
			dissolveCalls++
			if actorID != groupHandlerActorID || groupID != groupHandlerGroupID {
				t.Errorf("Dissolve inputs = (%q, %q)", actorID, groupID)
			}
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
			"/api/groups/"+strings.ToUpper(groupHandlerGroupID),
			nil,
			true,
		),
	)

	assertEmptyNoContent(t, response)
	if dissolveCalls != 1 {
		t.Errorf("Dissolve calls = %d, want 1", dissolveCalls)
	}

	for _, test := range []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "member forbidden",
			err:        groups.ErrForbidden,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "missing dissolved or hidden",
			err:        groups.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			errorOptions := groupHandlerTestOptions(fakeGroupService{
				dissolve: func(context.Context, string, string) error {
					return test.err
				},
			})
			errorAPI, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				errorOptions,
			)
			errorResponse := httptest.NewRecorder()
			errorAPI.Handler().ServeHTTP(
				errorResponse,
				groupHandlerRequest(
					http.MethodDelete,
					"/api/groups/"+groupHandlerGroupID,
					nil,
					true,
				),
			)
			assertAPIError(
				t,
				errorResponse.Result(),
				test.wantStatus,
				test.wantCode,
			)
		})
	}
}

func TestGetGroupJoinCodeOwnerOnlyAndNotLogged(t *testing.T) {
	t.Parallel()

	const privateJoinCode = "PRIVATECODE2"
	getCalls := 0
	options := groupHandlerTestOptions(fakeGroupService{
		getJoinCode: func(
			_ context.Context,
			actorID string,
			groupID string,
		) (string, error) {
			getCalls++
			if actorID != groupHandlerActorID || groupID != groupHandlerGroupID {
				t.Errorf("GetJoinCode inputs = (%q, %q)", actorID, groupID)
			}
			return privateJoinCode, nil
		},
	})
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := groupHandlerRequest(
		http.MethodGet,
		"/api/groups/"+strings.ToUpper(groupHandlerGroupID)+"/join-code",
		nil,
		false,
	)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body groupJoinCodeResponse
	decodeGroupJSONResponse(t, response.Result(), &body)
	if body.JoinCode != privateJoinCode {
		t.Errorf("body = %+v", body)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &keys); err != nil {
		t.Fatalf("decode response keys: %v", err)
	}
	if len(keys) != 1 || keys["joinCode"] == nil {
		t.Errorf("response keys = %v", keys)
	}
	if strings.Contains(logs.String(), privateJoinCode) {
		t.Errorf("access log exposes join code: %s", logs.String())
	}
	if getCalls != 1 {
		t.Errorf("GetJoinCode calls = %d, want 1", getCalls)
	}

	for _, test := range []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "member forbidden",
			err:        groups.ErrForbidden,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "non-member dissolved or missing",
			err:        groups.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			errorOptions := groupHandlerTestOptions(fakeGroupService{
				getJoinCode: func(
					context.Context,
					string,
					string,
				) (string, error) {
					return "", test.err
				},
			})
			errorAPI, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				errorOptions,
			)
			errorResponse := httptest.NewRecorder()
			errorAPI.Handler().ServeHTTP(
				errorResponse,
				groupHandlerRequest(
					http.MethodGet,
					"/api/groups/"+groupHandlerGroupID+"/join-code",
					nil,
					false,
				),
			)
			assertAPIError(
				t,
				errorResponse.Result(),
				test.wantStatus,
				test.wantCode,
			)
		})
	}
}

func TestRemoveGroupMemberReturns204AndExactErrorDistinctions(t *testing.T) {
	t.Parallel()

	removeCalls := 0
	options := groupHandlerTestOptions(fakeGroupService{
		removeMember: func(
			_ context.Context,
			actorID string,
			groupID string,
			userID string,
		) error {
			removeCalls++
			if actorID != groupHandlerActorID ||
				groupID != groupHandlerGroupID ||
				userID != groupHandlerOtherID {
				t.Errorf(
					"RemoveMember inputs = (%q, %q, %q)",
					actorID,
					groupID,
					userID,
				)
			}
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
			"/api/groups/"+
				strings.ToUpper(groupHandlerGroupID)+
				"/members/"+
				strings.ToUpper(groupHandlerOtherID),
			nil,
			true,
		),
	)

	assertEmptyNoContent(t, response)
	if removeCalls != 1 {
		t.Errorf("RemoveMember calls = %d, want 1", removeCalls)
	}

	errorTests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "member actor forbidden",
			err:        groups.ErrForbidden,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "owner self-removal forbidden",
			err:        groups.ErrForbidden,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "target member in use",
			err:        groups.ErrMemberInUse,
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
		{
			name:       "group or member hidden",
			err:        groups.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}
	for _, test := range errorTests {
		t.Run(test.name, func(t *testing.T) {
			errorOptions := groupHandlerTestOptions(fakeGroupService{
				removeMember: func(
					context.Context,
					string,
					string,
					string,
				) error {
					return test.err
				},
			})
			errorAPI, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				errorOptions,
			)
			errorResponse := httptest.NewRecorder()
			errorAPI.Handler().ServeHTTP(
				errorResponse,
				groupHandlerRequest(
					http.MethodDelete,
					"/api/groups/"+
						groupHandlerGroupID+
						"/members/"+
						groupHandlerOtherID,
					nil,
					true,
				),
			)
			assertAPIError(
				t,
				errorResponse.Result(),
				test.wantStatus,
				test.wantCode,
			)
		})
	}
}

func TestOwnerRoutesRejectInvalidUUIDsBeforeService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		unsafe bool
	}{
		{
			name:   "rename group ID",
			method: http.MethodPatch,
			path:   "/api/groups/not-a-uuid",
			body:   `{"name":"Beach Trip"}`,
			unsafe: true,
		},
		{
			name:   "dissolve group ID",
			method: http.MethodDelete,
			path:   "/api/groups/not-a-uuid",
			unsafe: true,
		},
		{
			name:   "join-code group ID",
			method: http.MethodGet,
			path:   "/api/groups/not-a-uuid/join-code",
		},
		{
			name:   "remove group ID",
			method: http.MethodDelete,
			path:   "/api/groups/not-a-uuid/members/" + groupHandlerOtherID,
			unsafe: true,
		},
		{
			name:   "remove user ID",
			method: http.MethodDelete,
			path: "/api/groups/" +
				groupHandlerGroupID +
				"/members/not-a-uuid",
			unsafe: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			options := groupHandlerTestOptions(fakeGroupService{
				rename: func(
					context.Context,
					string,
					string,
					string,
				) (groups.Group, error) {
					calls++
					return groups.Group{}, nil
				},
				dissolve: func(context.Context, string, string) error {
					calls++
					return nil
				},
				getJoinCode: func(
					context.Context,
					string,
					string,
				) (string, error) {
					calls++
					return "CODE", nil
				},
				removeMember: func(
					context.Context,
					string,
					string,
					string,
				) error {
					calls++
					return nil
				},
			})
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			var body io.Reader
			if test.body != "" {
				body = strings.NewReader(test.body)
			}
			request := groupHandlerRequest(
				test.method,
				test.path,
				body,
				test.unsafe,
			)
			if test.method == http.MethodPatch {
				request.Header.Set("Content-Type", "application/json")
			}
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

func TestOwnerDeleteRoutesRejectBodiesBeforeService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "dissolve", path: "/api/groups/" + groupHandlerGroupID},
		{
			name: "remove member",
			path: "/api/groups/" +
				groupHandlerGroupID +
				"/members/" +
				groupHandlerOtherID,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			options := groupHandlerTestOptions(fakeGroupService{
				dissolve: func(context.Context, string, string) error {
					calls++
					return nil
				},
				removeMember: func(
					context.Context,
					string,
					string,
					string,
				) error {
					calls++
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
					test.path,
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
			if calls != 0 {
				t.Errorf("service calls = %d, want 0", calls)
			}
		})
	}
}

func TestOwnerUnsafeRouteSecurityOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		session    bool
		origin     bool
		csrfError  error
		wantStatus int
		wantCode   string
		wantCSRF   int
		wantRename int
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
			csrfError:  auth.ErrCSRFRequired,
			wantStatus: http.StatusForbidden,
			wantCode:   "csrf_required",
			wantCSRF:   1,
		},
		{
			name:       "handler last",
			session:    true,
			origin:     true,
			wantStatus: http.StatusOK,
			wantCSRF:   1,
			wantRename: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			csrfCalls := 0
			renameCalls := 0
			options := groupHandlerTestOptions(fakeGroupService{
				rename: func(
					context.Context,
					string,
					string,
					string,
				) (groups.Group, error) {
					renameCalls++
					return groupHandlerTestGroup(), nil
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
				http.MethodPatch,
				"/api/groups/"+groupHandlerGroupID,
				strings.NewReader(`{"name":"Beach Trip"}`),
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
			if renameCalls != test.wantRename {
				t.Errorf("Rename calls = %d, want %d", renameCalls, test.wantRename)
			}
		})
	}
}

func assertEmptyNoContent(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204: %s", response.Code, response.Body.String())
	}
	if response.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", response.Body.String())
	}
	if response.Header().Get("Content-Type") != "" {
		t.Errorf("Content-Type = %q, want empty", response.Header().Get("Content-Type"))
	}
}

func TestOwnerErrorBodiesPreserve403404409Distinctions(t *testing.T) {
	t.Parallel()

	specs := []struct {
		err    error
		status int
		code   string
	}{
		{err: groups.ErrForbidden, status: http.StatusForbidden, code: "forbidden"},
		{err: groups.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
		{err: groups.ErrMemberInUse, status: http.StatusConflict, code: "conflict"},
	}
	for _, spec := range specs {
		got := errorFor(spec.err)
		if got.status != spec.status || got.code != spec.code {
			t.Errorf(
				"error %v maps to (%d, %q), want (%d, %q)",
				spec.err,
				got.status,
				got.code,
				spec.status,
				spec.code,
			)
		}
	}
}

func TestJoinCodeErrorResponsesNeverEchoCode(t *testing.T) {
	t.Parallel()

	const privateJoinCode = "PRIVATECODE2"
	options := groupHandlerTestOptions(fakeGroupService{
		getJoinCode: func(
			context.Context,
			string,
			string,
		) (string, error) {
			return "", groups.ErrForbidden
		},
	})
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := groupHandlerRequest(
		http.MethodGet,
		"/api/groups/"+groupHandlerGroupID+"/join-code",
		nil,
		false,
	)
	request.Header.Set("X-Private-Test-Value", privateJoinCode)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if strings.Contains(response.Body.String(), privateJoinCode) ||
		strings.Contains(logs.String(), privateJoinCode) {
		t.Error("join code leaked through error response or access log")
	}
	assertAPIError(
		t,
		response.Result(),
		http.StatusForbidden,
		"forbidden",
	)
}

func TestOwnerHiddenResponsesAreIndistinguishable(t *testing.T) {
	t.Parallel()

	var bodies []string
	for range []string{"missing", "dissolved", "removed actor"} {
		options := groupHandlerTestOptions(fakeGroupService{
			getJoinCode: func(
				context.Context,
				string,
				string,
			) (string, error) {
				return "", groups.ErrNotFound
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
				"/api/groups/"+groupHandlerGroupID+"/join-code",
				nil,
				false,
			),
		)
		bodies = append(bodies, response.Body.String())
	}
	if !reflect.DeepEqual(
		bodies,
		[]string{bodies[0], bodies[0], bodies[0]},
	) {
		t.Errorf("hidden response bodies differ: %q", bodies)
	}
}
