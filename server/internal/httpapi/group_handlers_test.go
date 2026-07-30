package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/groups"
)

const (
	groupHandlerActorID = "00112233-4455-4677-8899-aabbccddeeff"
	groupHandlerOtherID = "11112222-3333-4444-8555-666677778888"
	groupHandlerGroupID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
)

var groupHandlerTestNow = time.Date(
	2026,
	time.July,
	31,
	17,
	4,
	5,
	123456789,
	time.UTC,
)

func TestListGroupsReturnsStableServiceOrderAndEmptyArray(t *testing.T) {
	t.Parallel()

	first := groupHandlerTestGroup()
	first.ID = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	first.Name = "Most Recent"
	first.UpdatedAt = first.UpdatedAt.Add(time.Hour)
	second := groupHandlerTestGroup()
	second.Name = "Earlier"
	wantGroups := []groups.Group{first, second}
	listCalls := 0
	options := groupHandlerTestOptions(fakeGroupService{
		list: func(
			ctx context.Context,
			actorID string,
		) ([]groups.Group, error) {
			listCalls++
			if actorID != groupHandlerActorID {
				t.Errorf("actor ID = %q", actorID)
			}
			if ctx == nil {
				t.Error("context is nil")
			}
			return wantGroups, nil
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
		groupHandlerRequest(http.MethodGet, "/api/groups", nil, false),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	rawBody := response.Body.String()
	if strings.Contains(rawBody, "joinCode") {
		t.Errorf("list response exposes join code: %s", rawBody)
	}
	var body groupsResponse
	decodeGroupJSONResponse(t, response.Result(), &body)
	want := groupsResponse{Groups: []groupResponse{
		newGroupResponse(first),
		newGroupResponse(second),
	}}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %+v, want %+v", body, want)
	}
	if listCalls != 1 {
		t.Errorf("List calls = %d, want 1", listCalls)
	}
	assertGroupListKeys(t, []byte(rawBody))

	emptyOptions := groupHandlerTestOptions(fakeGroupService{
		list: func(context.Context, string) ([]groups.Group, error) {
			return nil, nil
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
		groupHandlerRequest(http.MethodGet, "/api/groups", nil, false),
	)
	if emptyResponse.Code != http.StatusOK ||
		emptyResponse.Body.String() != "{\"groups\":[]}\n" {
		t.Errorf(
			"empty response = %d %q, want 200 with array",
			emptyResponse.Code,
			emptyResponse.Body.String(),
		)
	}
}

func TestCreateGroupReturnsStrictSummaryWithoutJoinCode(t *testing.T) {
	t.Parallel()

	group := groupHandlerTestGroup()
	privateJoinCode := "PRIVATECODE2"
	events := make([]string, 0, 3)
	options := groupHandlerTestOptions(fakeGroupService{
		create: func(
			_ context.Context,
			actorID string,
			name string,
		) (groups.Group, error) {
			events = append(events, "create")
			if actorID != groupHandlerActorID || name != " Lake Trip " {
				t.Errorf("Create inputs = (%q, %q)", actorID, name)
			}
			return group, nil
		},
	})
	options.Auth = fakeAuthService{
		findUser: func(
			context.Context,
			string,
		) (auth.User, error) {
			events = append(events, "user")
			return auth.User{ID: groupHandlerActorID}, nil
		},
	}
	options.CSRF = fakeCSRFProtector{
		validateAuthenticated: func(
			string,
			string,
			auth.Session,
		) error {
			events = append(events, "csrf")
			return nil
		},
	}
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := groupHandlerRequest(
		http.MethodPost,
		"/api/groups",
		strings.NewReader(`{"name":" Lake Trip "}`),
		true,
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if !reflect.DeepEqual(events, []string{"user", "csrf", "create"}) {
		t.Errorf("events = %v", events)
	}
	rawBody := response.Body.String()
	if strings.Contains(rawBody, "joinCode") ||
		strings.Contains(rawBody, privateJoinCode) {
		t.Errorf("response exposes join code: %s", rawBody)
	}
	var body groupResponseEnvelope
	decodeGroupJSONResponse(t, response.Result(), &body)
	if body != (groupResponseEnvelope{Group: newGroupResponse(group)}) {
		t.Errorf("body = %+v", body)
	}
	assertGroupEnvelopeKeys(t, []byte(rawBody), false)
}

func TestCreateGroupStrictJSONAndValidation(t *testing.T) {
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
			body:        `{"name":"Lake Trip","joinCode":"not-accepted"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:        "trailing value",
			body:        `{"name":"Lake Trip"} {}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:       "missing content type",
			body:       `{"name":"Lake Trip"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:        "missing name",
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			createCalls := 0
			options := groupHandlerTestOptions(fakeGroupService{
				create: func(
					context.Context,
					string,
					string,
				) (groups.Group, error) {
					createCalls++
					return groups.Group{}, test.serviceErr
				},
			})
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := groupHandlerRequest(
				http.MethodPost,
				"/api/groups",
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
			if createCalls != test.wantCalls {
				t.Errorf("Create calls = %d, want %d", createCalls, test.wantCalls)
			}
		})
	}
}

func TestJoinGroupSuccessAndErrorMappings(t *testing.T) {
	t.Parallel()

	group := groupHandlerTestGroup()
	group.MemberCount = 2
	group.CurrentUserRole = groups.RoleMember
	privateJoinCode := " PRIVATECODE2 "
	options := groupHandlerTestOptions(fakeGroupService{
		join: func(
			_ context.Context,
			actorID string,
			joinCode string,
		) (groups.Group, error) {
			if actorID != groupHandlerActorID || joinCode != privateJoinCode {
				t.Errorf("Join inputs = (%q, %q)", actorID, joinCode)
			}
			return group, nil
		},
	})
	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := groupHandlerRequest(
		http.MethodPost,
		"/api/groups/join",
		strings.NewReader(`{"joinCode":" PRIVATECODE2 "}`),
		true,
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	rawBody := response.Body.String()
	if strings.Contains(rawBody, privateJoinCode) ||
		strings.Contains(rawBody, strings.TrimSpace(privateJoinCode)) ||
		strings.Contains(logs.String(), strings.TrimSpace(privateJoinCode)) {
		t.Error("join code appears in response or logs")
	}
	var body groupResponseEnvelope
	decodeGroupJSONResponse(t, response.Result(), &body)
	if body != (groupResponseEnvelope{Group: newGroupResponse(group)}) {
		t.Errorf("body = %+v", body)
	}
	assertGroupEnvelopeKeys(t, []byte(rawBody), false)

	sourceError := errors.New("database unavailable")
	errorTests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name: "validation",
			err: &groups.ValidationError{Fields: map[string]string{
				"joinCode": "Join code is required.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
		},
		{
			name:       "unknown or dissolved code",
			err:        groups.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "store failure",
			err:        sourceError,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}
	for _, test := range errorTests {
		t.Run(test.name, func(t *testing.T) {
			errorOptions := groupHandlerTestOptions(fakeGroupService{
				join: func(
					context.Context,
					string,
					string,
				) (groups.Group, error) {
					return groups.Group{}, test.err
				},
			})
			errorAPI, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				errorOptions,
			)
			errorRequest := groupHandlerRequest(
				http.MethodPost,
				"/api/groups/join",
				strings.NewReader(`{"joinCode":"CODE"}`),
				true,
			)
			errorRequest.Header.Set("Content-Type", "application/json")
			errorResponse := httptest.NewRecorder()

			errorAPI.Handler().ServeHTTP(errorResponse, errorRequest)

			assertAPIError(
				t,
				errorResponse.Result(),
				test.wantStatus,
				test.wantCode,
			)
		})
	}
}

func TestJoinGroupStrictJSONAndValidation(t *testing.T) {
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
			body:        `{"joinCode":"CODE","groupId":"not-accepted"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:        "trailing value",
			body:        `{"joinCode":"CODE"} {}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "bad_request",
		},
		{
			name:       "missing content type",
			body:       `{"joinCode":"CODE"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:        "omitted join code",
			body:        `{}`,
			contentType: "application/json",
			serviceErr: &groups.ValidationError{Fields: map[string]string{
				"joinCode": "Join code is required.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantCalls:  1,
		},
		{
			name:        "null join code",
			body:        `{"joinCode":null}`,
			contentType: "application/json",
			serviceErr: &groups.ValidationError{Fields: map[string]string{
				"joinCode": "Join code is required.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantCalls:  1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			joinCalls := 0
			options := groupHandlerTestOptions(fakeGroupService{
				join: func(
					context.Context,
					string,
					string,
				) (groups.Group, error) {
					joinCalls++
					return groups.Group{}, test.serviceErr
				},
			})
			api, _ := newTestAPI(
				t,
				pingerFunc(func(context.Context) error { return nil }),
				options,
			)
			request := groupHandlerRequest(
				http.MethodPost,
				"/api/groups/join",
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
			if joinCalls != test.wantCalls {
				t.Errorf("Join calls = %d, want %d", joinCalls, test.wantCalls)
			}
		})
	}
}

func TestGetGroupCanonicalizesPathAndPreservesMemberOrder(t *testing.T) {
	t.Parallel()

	group := groupHandlerTestGroup()
	detail := groups.Detail{
		Group: group,
		Members: []groups.Member{
			{
				UserID:      groupHandlerActorID,
				Email:       "owner@example.com",
				DisplayName: "Zed Owner",
				Role:        groups.RoleOwner,
				JoinedAt:    groupHandlerTestNow,
			},
			{
				UserID:      groupHandlerOtherID,
				Email:       "alice@example.com",
				DisplayName: "alice",
				Role:        groups.RoleMember,
				JoinedAt:    groupHandlerTestNow.Add(time.Minute),
			},
		},
	}
	getCalls := 0
	options := groupHandlerTestOptions(fakeGroupService{
		get: func(
			_ context.Context,
			actorID string,
			groupID string,
		) (groups.Detail, error) {
			getCalls++
			if actorID != groupHandlerActorID ||
				groupID != groupHandlerGroupID {
				t.Errorf("Get inputs = (%q, %q)", actorID, groupID)
			}
			return detail, nil
		},
	})
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := groupHandlerRequest(
		http.MethodGet,
		"/api/groups/"+strings.ToUpper(groupHandlerGroupID),
		nil,
		false,
	)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body groupDetailResponse
	decodeGroupJSONResponse(t, response.Result(), &body)
	wantMembers := []groupMemberResponse{
		newGroupMemberResponse(detail.Members[0]),
		newGroupMemberResponse(detail.Members[1]),
	}
	if body.Group != newGroupResponse(group) ||
		!reflect.DeepEqual(body.Members, wantMembers) {
		t.Errorf("body = %+v", body)
	}
	if getCalls != 1 {
		t.Errorf("Get calls = %d, want 1", getCalls)
	}
	assertGroupEnvelopeKeys(t, []byte(response.Body.String()), true)
}

func TestGetGroupRejectsInvalidUUIDBeforeService(t *testing.T) {
	t.Parallel()

	tests := []string{
		"not-a-uuid",
		"00112233-4455-4677-0899-aabbccddeeff",
		"00112233445546778899aabbccddeeff",
	}
	for _, groupID := range tests {
		t.Run(groupID, func(t *testing.T) {
			getCalls := 0
			options := groupHandlerTestOptions(fakeGroupService{
				get: func(
					context.Context,
					string,
					string,
				) (groups.Detail, error) {
					getCalls++
					return groups.Detail{}, nil
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
					"/api/groups/"+groupID,
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
			if getCalls != 0 {
				t.Errorf("Get calls = %d, want 0", getCalls)
			}
		})
	}
}

func TestGetGroupHiddenStatesHaveIdenticalResponses(t *testing.T) {
	t.Parallel()

	var bodies []string
	for _, state := range []string{"missing", "dissolved", "non-member"} {
		options := groupHandlerTestOptions(fakeGroupService{
			get: func(
				context.Context,
				string,
				string,
			) (groups.Detail, error) {
				return groups.Detail{}, groups.ErrNotFound
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
				"/api/groups/"+groupHandlerGroupID,
				nil,
				false,
			),
		)
		if response.Code != http.StatusNotFound {
			t.Errorf("%s status = %d", state, response.Code)
		}
		bodies = append(bodies, response.Body.String())
	}
	for index := 1; index < len(bodies); index++ {
		if bodies[index] != bodies[0] {
			t.Errorf("hidden response %d differs: %q != %q", index, bodies[index], bodies[0])
		}
	}
}

func TestGroupRouteSecurityOrder(t *testing.T) {
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
			name:       "authentication before origin",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "origin before CSRF",
			session:    true,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "CSRF before handler",
			session:    true,
			origin:     true,
			csrfError:  auth.ErrCSRFRequired,
			wantStatus: http.StatusForbidden,
			wantCode:   "csrf_required",
			wantCSRF:   1,
		},
		{
			name:       "valid",
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
			options := groupHandlerTestOptions(fakeGroupService{
				create: func(
					context.Context,
					string,
					string,
				) (groups.Group, error) {
					createCalls++
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
				http.MethodPost,
				"/api/groups",
				strings.NewReader(`{"name":"Lake Trip"}`),
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

func TestGroupReadsRequireAuthenticationBeforeService(t *testing.T) {
	t.Parallel()

	listCalls := 0
	options := groupHandlerTestOptions(fakeGroupService{
		list: func(context.Context, string) ([]groups.Group, error) {
			listCalls++
			return nil, nil
		},
	})
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)
	request := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, request)

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

func groupHandlerTestOptions(service fakeGroupService) Options {
	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	options.Groups = service
	options.Sessions = sessionValidatorFunc(
		func(token string) (auth.Session, error) {
			if token != "valid-session" {
				return auth.Session{}, auth.ErrUnauthenticated
			}
			return auth.Session{
				UserID: groupHandlerActorID,
				JWTID:  "group-handler-jwt-id",
			}, nil
		},
	)
	options.Auth = fakeAuthService{
		findUser: func(
			_ context.Context,
			userID string,
		) (auth.User, error) {
			if userID != groupHandlerActorID {
				return auth.User{}, auth.ErrUserNotFound
			}
			return auth.User{ID: userID}, nil
		},
	}
	return options
}

func groupHandlerRequest(
	method string,
	path string,
	body io.Reader,
	unsafe bool,
) *http.Request {
	request := httptest.NewRequest(method, path, body)
	request.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: "valid-session",
	})
	if unsafe {
		request.Header.Set("Origin", "https://web.example")
		request.Header.Set(csrfTokenHeader, "session-csrf-token")
		request.AddCookie(&http.Cookie{
			Name:  auth.CSRFCookieName,
			Value: "session-csrf-cookie",
		})
	}
	return request
}

func removeCookie(request *http.Request, name string) {
	cookies := request.Cookies()
	request.Header.Del("Cookie")
	for _, cookie := range cookies {
		if cookie.Name != name {
			request.AddCookie(cookie)
		}
	}
}

func decodeGroupJSONResponse(
	t *testing.T,
	response *http.Response,
	destination any,
) {
	t.Helper()
	defer response.Body.Close()
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(destination); err != nil {
		t.Fatalf("decode group response: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		t.Errorf("unexpected trailing response JSON: %v", err)
	}
}

func assertGroupEnvelopeKeys(
	t *testing.T,
	body []byte,
	detail bool,
) {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode response keys: %v", err)
	}
	wantEnvelopeFields := 1
	if detail {
		wantEnvelopeFields = 2
	}
	if len(envelope) != wantEnvelopeFields || envelope["group"] == nil {
		t.Errorf("envelope keys = %v", envelope)
	}
	var group map[string]json.RawMessage
	if err := json.Unmarshal(envelope["group"], &group); err != nil {
		t.Fatalf("decode group keys: %v", err)
	}
	for _, field := range []string{
		"id",
		"name",
		"ownerUserId",
		"memberCount",
		"currentUserRole",
		"createdAt",
		"updatedAt",
	} {
		if group[field] == nil {
			t.Errorf("group lacks %q: %v", field, group)
		}
	}
	if len(group) != 7 || group["joinCode"] != nil {
		t.Errorf("group keys = %v", group)
	}
	if detail {
		var members []map[string]json.RawMessage
		if err := json.Unmarshal(envelope["members"], &members); err != nil {
			t.Fatalf("decode member keys: %v", err)
		}
		for _, member := range members {
			if len(member) != 5 {
				t.Errorf("member keys = %v", member)
			}
			for _, field := range []string{
				"userId",
				"email",
				"displayName",
				"role",
				"joinedAt",
			} {
				if member[field] == nil {
					t.Errorf("member lacks %q: %v", field, member)
				}
			}
		}
	}
}

func assertGroupListKeys(t *testing.T, body []byte) {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode list response keys: %v", err)
	}
	if len(envelope) != 1 || envelope["groups"] == nil {
		t.Errorf("list envelope keys = %v", envelope)
	}
	var responseGroups []map[string]json.RawMessage
	if err := json.Unmarshal(envelope["groups"], &responseGroups); err != nil {
		t.Fatalf("decode list group keys: %v", err)
	}
	for _, group := range responseGroups {
		if len(group) != 7 || group["joinCode"] != nil {
			t.Errorf("list group keys = %v", group)
		}
		for _, field := range []string{
			"id",
			"name",
			"ownerUserId",
			"memberCount",
			"currentUserRole",
			"createdAt",
			"updatedAt",
		} {
			if group[field] == nil {
				t.Errorf("list group lacks %q: %v", field, group)
			}
		}
	}
}

func groupHandlerTestGroup() groups.Group {
	return groups.Group{
		ID:              groupHandlerGroupID,
		Name:            "Lake Trip",
		OwnerUserID:     groupHandlerActorID,
		MemberCount:     1,
		CurrentUserRole: groups.RoleOwner,
		CreatedAt:       groupHandlerTestNow,
		UpdatedAt:       groupHandlerTestNow,
	}
}
