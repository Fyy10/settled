package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/clock"
	"github.com/Fyy10/settled/server/internal/groups"
)

func TestGroupFlowWithTwoIndependentCookieJars(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	fixedClock := clock.Fixed{Time: now}
	sessionRandom := append(
		bytes.Repeat([]byte{0x71}, 32),
		bytes.Repeat([]byte{0x72}, 32)...,
	)
	sessions, err := auth.NewSessionManagerFrom(
		testSecret(0x70),
		fixedClock,
		bytes.NewReader(sessionRandom),
	)
	if err != nil {
		t.Fatalf("NewSessionManagerFrom: %v", err)
	}
	aliceSession, err := sessions.Issue(groupHandlerActorID)
	if err != nil {
		t.Fatalf("issue Alice session: %v", err)
	}
	bobSession, err := sessions.Issue(groupHandlerOtherID)
	if err != nil {
		t.Fatalf("issue Bob session: %v", err)
	}
	csrf, err := auth.NewCSRFManagerFrom(
		testSecret(0x73),
		fixedClock,
		&repeatReader{value: 0x74},
	)
	if err != nil {
		t.Fatalf("NewCSRFManagerFrom: %v", err)
	}

	users := map[string]auth.User{
		groupHandlerActorID: {
			ID:          groupHandlerActorID,
			Email:       "alice@example.com",
			DisplayName: "Alice",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		groupHandlerOtherID: {
			ID:          groupHandlerOtherID,
			Email:       "bob@example.com",
			DisplayName: "Bob",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	groupStore := newFlowGroupStore(users)
	groupRandom := make([]byte, 0, 56)
	groupRandom = append(groupRandom, bytes.Repeat([]byte{0x10}, 16)...)
	groupRandom = append(groupRandom, bytes.Repeat([]byte{0x00}, 12)...)
	groupRandom = append(groupRandom, bytes.Repeat([]byte{0x20}, 16)...)
	groupRandom = append(groupRandom, bytes.Repeat([]byte{0x01}, 12)...)
	groupService, err := groups.NewServiceFrom(
		groupStore,
		fixedClock,
		bytes.NewReader(groupRandom),
	)
	if err != nil {
		t.Fatalf("groups.NewServiceFrom: %v", err)
	}
	api, err := New(
		pingerFunc(func(context.Context) error { return nil }),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Options{
			AllowedOrigins: []string{"https://web.example"},
			Auth: fakeAuthService{
				findUser: func(
					_ context.Context,
					userID string,
				) (auth.User, error) {
					user, found := users[userID]
					if !found {
						return auth.User{}, auth.ErrUserNotFound
					}
					return user, nil
				},
			},
			Groups:         groupService,
			Sessions:       sessions,
			CSRF:           csrf,
			SessionCookies: auth.NewSessionCookies("", false, http.SameSiteLaxMode),
			CSRFCookies:    auth.NewCSRFCookies("", false, http.SameSiteLaxMode),
			RequestIDBytes: &repeatReader{value: 0x75},
		},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const serverURL = "http://groups.test"
	aliceClient := newGroupFlowClient(
		t,
		api.Handler(),
		serverURL,
		aliceSession,
	)
	bobClient := newGroupFlowClient(
		t,
		api.Handler(),
		serverURL,
		bobSession,
	)

	aliceCSRF := flowGetCSRF(t, aliceClient, serverURL)
	firstStatus, firstBody := groupFlowCreate(
		t,
		aliceClient,
		serverURL,
		aliceCSRF,
		"Lake Trip",
	)
	if firstStatus != http.StatusCreated {
		t.Fatalf("first create = %d %s", firstStatus, firstBody)
	}
	var firstResponse groupResponseEnvelope
	if err := decodeFlowJSON(firstBody, &firstResponse); err != nil {
		t.Fatalf("decode first create: %v", err)
	}
	if strings.Contains(string(firstBody), "joinCode") {
		t.Errorf("create response exposes join code: %s", firstBody)
	}
	firstJoinCode := groupStore.joinCode(t, firstResponse.Group.ID)

	bobCSRF := flowGetCSRF(t, bobClient, serverURL)
	joinRequest := groupFlowUnsafeRequest(
		t,
		http.MethodPost,
		serverURL+"/api/groups/join",
		strings.NewReader(`{"joinCode":" `+strings.ToLower(firstJoinCode)+` "}`),
		bobCSRF,
	)
	joinStatus, joinBody := flowDo(t, bobClient, joinRequest)
	if joinStatus != http.StatusOK {
		t.Fatalf("join = %d %s", joinStatus, joinBody)
	}
	var joined groupResponseEnvelope
	if err := decodeFlowJSON(joinBody, &joined); err != nil {
		t.Fatalf("decode join: %v", err)
	}
	if joined.Group.ID != firstResponse.Group.ID ||
		joined.Group.MemberCount != 2 ||
		joined.Group.CurrentUserRole != groups.RoleMember {
		t.Errorf("joined group = %+v", joined.Group)
	}
	if strings.Contains(string(joinBody), firstJoinCode) {
		t.Errorf("join response exposes join code: %s", joinBody)
	}

	secondStatus, secondBody := groupFlowCreate(
		t,
		aliceClient,
		serverURL,
		aliceCSRF,
		"Private Group",
	)
	if secondStatus != http.StatusCreated {
		t.Fatalf("second create = %d %s", secondStatus, secondBody)
	}

	aliceGroups := groupFlowList(t, aliceClient, serverURL)
	bobGroups := groupFlowList(t, bobClient, serverURL)
	if len(aliceGroups) != 2 ||
		aliceGroups[0].Name != "Private Group" ||
		aliceGroups[1].ID != firstResponse.Group.ID {
		t.Errorf("Alice groups = %+v", aliceGroups)
	}
	if len(bobGroups) != 1 ||
		bobGroups[0].ID != firstResponse.Group.ID ||
		bobGroups[0].CurrentUserRole != groups.RoleMember {
		t.Errorf("Bob groups = %+v", bobGroups)
	}

	for name, client := range map[string]*http.Client{
		"Alice": aliceClient,
		"Bob":   bobClient,
	} {
		request, err := http.NewRequest(
			http.MethodGet,
			serverURL+"/api/groups/"+firstResponse.Group.ID,
			nil,
		)
		if err != nil {
			t.Fatalf("NewRequest detail: %v", err)
		}
		status, body := flowDo(t, client, request)
		if status != http.StatusOK {
			t.Fatalf("%s detail = %d %s", name, status, body)
		}
		var detail groupDetailResponse
		if err := decodeFlowJSON(body, &detail); err != nil {
			t.Fatalf("decode %s detail: %v", name, err)
		}
		if len(detail.Members) != 2 ||
			detail.Members[0].Role != groups.RoleOwner ||
			detail.Members[0].UserID != groupHandlerActorID ||
			detail.Members[1].UserID != groupHandlerOtherID {
			t.Errorf("%s detail members = %+v", name, detail.Members)
		}
	}

	memberJoinCodeRequest, err := http.NewRequest(
		http.MethodGet,
		serverURL+"/api/groups/"+firstResponse.Group.ID+"/join-code",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest member join code: %v", err)
	}
	memberJoinCodeStatus, memberJoinCodeBody := flowDo(
		t,
		bobClient,
		memberJoinCodeRequest,
	)
	if memberJoinCodeStatus != http.StatusForbidden {
		t.Errorf(
			"member join code = %d %s, want 403",
			memberJoinCodeStatus,
			memberJoinCodeBody,
		)
	}

	ownerJoinCodeRequest, err := http.NewRequest(
		http.MethodGet,
		serverURL+"/api/groups/"+firstResponse.Group.ID+"/join-code",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest owner join code: %v", err)
	}
	ownerJoinCodeStatus, ownerJoinCodeBody := flowDo(
		t,
		aliceClient,
		ownerJoinCodeRequest,
	)
	if ownerJoinCodeStatus != http.StatusOK {
		t.Fatalf(
			"owner join code = %d %s",
			ownerJoinCodeStatus,
			ownerJoinCodeBody,
		)
	}
	var ownerJoinCode groupJoinCodeResponse
	if err := decodeFlowJSON(ownerJoinCodeBody, &ownerJoinCode); err != nil {
		t.Fatalf("decode owner join code: %v", err)
	}
	if ownerJoinCode.JoinCode != firstJoinCode {
		t.Errorf(
			"owner join code = %q, want %q",
			ownerJoinCode.JoinCode,
			firstJoinCode,
		)
	}

	memberRenameRequest := groupFlowUnsafeRequest(
		t,
		http.MethodPatch,
		serverURL+"/api/groups/"+firstResponse.Group.ID,
		strings.NewReader(`{"name":"Member Rename"}`),
		bobCSRF,
	)
	memberRenameStatus, memberRenameBody := flowDo(
		t,
		bobClient,
		memberRenameRequest,
	)
	if memberRenameStatus != http.StatusForbidden {
		t.Errorf(
			"member rename = %d %s, want 403",
			memberRenameStatus,
			memberRenameBody,
		)
	}

	ownerRenameRequest := groupFlowUnsafeRequest(
		t,
		http.MethodPatch,
		serverURL+"/api/groups/"+firstResponse.Group.ID,
		strings.NewReader(`{"name":" Coastal Trip "}`),
		aliceCSRF,
	)
	ownerRenameStatus, ownerRenameBody := flowDo(
		t,
		aliceClient,
		ownerRenameRequest,
	)
	if ownerRenameStatus != http.StatusOK {
		t.Fatalf(
			"owner rename = %d %s",
			ownerRenameStatus,
			ownerRenameBody,
		)
	}
	var ownerRename groupResponseEnvelope
	if err := decodeFlowJSON(ownerRenameBody, &ownerRename); err != nil {
		t.Fatalf("decode owner rename: %v", err)
	}
	if ownerRename.Group.Name != "Coastal Trip" ||
		ownerRename.Group.MemberCount != 2 {
		t.Errorf("owner rename group = %+v", ownerRename.Group)
	}

	memberDissolveRequest := groupFlowUnsafeRequest(
		t,
		http.MethodDelete,
		serverURL+"/api/groups/"+firstResponse.Group.ID,
		nil,
		bobCSRF,
	)
	memberDissolveStatus, memberDissolveBody := flowDo(
		t,
		bobClient,
		memberDissolveRequest,
	)
	if memberDissolveStatus != http.StatusForbidden {
		t.Errorf(
			"member dissolve = %d %s, want 403",
			memberDissolveStatus,
			memberDissolveBody,
		)
	}

	memberRemoveRequest := groupFlowUnsafeRequest(
		t,
		http.MethodDelete,
		serverURL+"/api/groups/"+
			firstResponse.Group.ID+
			"/members/"+
			groupHandlerActorID,
		nil,
		bobCSRF,
	)
	memberRemoveStatus, memberRemoveBody := flowDo(
		t,
		bobClient,
		memberRemoveRequest,
	)
	if memberRemoveStatus != http.StatusForbidden {
		t.Errorf(
			"member removal = %d %s, want 403",
			memberRemoveStatus,
			memberRemoveBody,
		)
	}

	ownerRemoveRequest := groupFlowUnsafeRequest(
		t,
		http.MethodDelete,
		serverURL+"/api/groups/"+
			firstResponse.Group.ID+
			"/members/"+
			groupHandlerOtherID,
		nil,
		aliceCSRF,
	)
	ownerRemoveStatus, ownerRemoveBody := flowDo(
		t,
		aliceClient,
		ownerRemoveRequest,
	)
	if ownerRemoveStatus != http.StatusNoContent ||
		len(ownerRemoveBody) != 0 {
		t.Fatalf(
			"owner removal = %d %q, want empty 204",
			ownerRemoveStatus,
			ownerRemoveBody,
		)
	}

	if removedGroups := groupFlowList(t, bobClient, serverURL); len(removedGroups) != 0 {
		t.Errorf("removed member groups = %+v, want none", removedGroups)
	}
	removedDetailRequest, err := http.NewRequest(
		http.MethodGet,
		serverURL+"/api/groups/"+firstResponse.Group.ID,
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest removed member detail: %v", err)
	}
	removedDetailStatus, removedDetailBody := flowDo(
		t,
		bobClient,
		removedDetailRequest,
	)
	if removedDetailStatus != http.StatusNotFound {
		t.Errorf(
			"removed member detail = %d %s, want 404",
			removedDetailStatus,
			removedDetailBody,
		)
	}

	ownerDissolveRequest := groupFlowUnsafeRequest(
		t,
		http.MethodDelete,
		serverURL+"/api/groups/"+firstResponse.Group.ID,
		nil,
		aliceCSRF,
	)
	ownerDissolveStatus, ownerDissolveBody := flowDo(
		t,
		aliceClient,
		ownerDissolveRequest,
	)
	if ownerDissolveStatus != http.StatusNoContent ||
		len(ownerDissolveBody) != 0 {
		t.Fatalf(
			"owner dissolve = %d %q, want empty 204",
			ownerDissolveStatus,
			ownerDissolveBody,
		)
	}

	repeatedDissolveRequest := groupFlowUnsafeRequest(
		t,
		http.MethodDelete,
		serverURL+"/api/groups/"+firstResponse.Group.ID,
		nil,
		aliceCSRF,
	)
	repeatedDissolveStatus, repeatedDissolveBody := flowDo(
		t,
		aliceClient,
		repeatedDissolveRequest,
	)
	if repeatedDissolveStatus != http.StatusNotFound {
		t.Errorf(
			"repeated dissolve = %d %s, want 404",
			repeatedDissolveStatus,
			repeatedDissolveBody,
		)
	}

	remainingGroups := groupFlowList(t, aliceClient, serverURL)
	if len(remainingGroups) != 1 ||
		remainingGroups[0].Name != "Private Group" {
		t.Errorf("owner groups after dissolution = %+v", remainingGroups)
	}
}

func newGroupFlowClient(
	t *testing.T,
	handler http.Handler,
	serverURL string,
	session auth.IssuedSession,
) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	endpoint, err := url.Parse(serverURL + "/api/auth/csrf")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	jar.SetCookies(endpoint, []*http.Cookie{{
		Name:     auth.SessionCookieName,
		Value:    session.Token,
		Path:     "/api",
		Expires:  session.ExpiresAt,
		MaxAge:   int(auth.SessionLifetime / time.Second),
		HttpOnly: true,
	}})
	return &http.Client{
		Transport: handlerRoundTripper{handler: handler},
		Jar:       jar,
	}
}

func groupFlowCreate(
	t *testing.T,
	client *http.Client,
	serverURL string,
	csrfToken string,
	name string,
) (int, []byte) {
	t.Helper()
	body, err := json.Marshal(createGroupRequest{Name: &name})
	if err != nil {
		t.Fatalf("marshal create group: %v", err)
	}
	request := groupFlowUnsafeRequest(
		t,
		http.MethodPost,
		serverURL+"/api/groups",
		bytes.NewReader(body),
		csrfToken,
	)
	return flowDo(t, client, request)
}

func groupFlowUnsafeRequest(
	t *testing.T,
	method string,
	requestURL string,
	body io.Reader,
	csrfToken string,
) *http.Request {
	t.Helper()
	request, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	request.Header.Set("Origin", "https://web.example")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(csrfTokenHeader, csrfToken)
	return request
}

func groupFlowList(
	t *testing.T,
	client *http.Client,
	serverURL string,
) []groupResponse {
	t.Helper()
	request, err := http.NewRequest(
		http.MethodGet,
		serverURL+"/api/groups",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest list: %v", err)
	}
	status, body := flowDo(t, client, request)
	if status != http.StatusOK {
		t.Fatalf("list = %d %s", status, body)
	}
	var response groupsResponse
	if err := decodeFlowJSON(body, &response); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	return response.Groups
}

type flowGroupStore struct {
	mu      sync.Mutex
	users   map[string]auth.User
	records map[string]*flowGroupRecord
	byCode  map[string]string
}

type flowGroupRecord struct {
	input       groups.NewGroup
	memberships map[string]time.Time
	dissolved   bool
}

func newFlowGroupStore(users map[string]auth.User) *flowGroupStore {
	return &flowGroupStore{
		users:   users,
		records: make(map[string]*flowGroupRecord),
		byCode:  make(map[string]string),
	}
}

func (store *flowGroupStore) CreateGroup(
	_ context.Context,
	input groups.NewGroup,
) (groups.Group, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.byCode[input.JoinCode]; exists {
		return groups.Group{}, groups.ErrJoinCodeCollision
	}
	record := &flowGroupRecord{
		input: input,
		memberships: map[string]time.Time{
			input.OwnerUserID: input.CreatedAt,
		},
	}
	store.records[input.ID] = record
	store.byCode[input.JoinCode] = input.ID
	return flowGroupSummary(record, input.OwnerUserID), nil
}

func (store *flowGroupStore) ListGroups(
	_ context.Context,
	actorID string,
) ([]groups.Group, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]groups.Group, 0)
	for _, record := range store.records {
		if record.dissolved {
			continue
		}
		if _, visible := record.memberships[actorID]; visible {
			result = append(result, flowGroupSummary(record, actorID))
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if !result[left].UpdatedAt.Equal(result[right].UpdatedAt) {
			return result[left].UpdatedAt.After(result[right].UpdatedAt)
		}
		return result[left].ID > result[right].ID
	})
	return result, nil
}

func (store *flowGroupStore) JoinGroup(
	_ context.Context,
	input groups.JoinGroupInput,
) (groups.Group, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	groupID, found := store.byCode[input.JoinCode]
	if !found {
		return groups.Group{}, groups.ErrNotFound
	}
	record := store.records[groupID]
	if record.dissolved {
		return groups.Group{}, groups.ErrNotFound
	}
	if _, active := record.memberships[input.UserID]; !active {
		record.memberships[input.UserID] = input.JoinedAt
	}
	return flowGroupSummary(record, input.UserID), nil
}

func (store *flowGroupStore) GetGroup(
	_ context.Context,
	actorID string,
	groupID string,
) (groups.Detail, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, found := store.records[groupID]
	if !found || record.dissolved {
		return groups.Detail{}, groups.ErrNotFound
	}
	if _, visible := record.memberships[actorID]; !visible {
		return groups.Detail{}, groups.ErrNotFound
	}
	members := make([]groups.Member, 0, len(record.memberships))
	for userID, joinedAt := range record.memberships {
		user := store.users[userID]
		role := groups.RoleMember
		if userID == record.input.OwnerUserID {
			role = groups.RoleOwner
		}
		members = append(members, groups.Member{
			UserID:      userID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Role:        role,
			JoinedAt:    joinedAt,
		})
	}
	sort.Slice(members, func(left, right int) bool {
		if members[left].Role != members[right].Role {
			return members[left].Role == groups.RoleOwner
		}
		leftName := strings.ToLower(members[left].DisplayName)
		rightName := strings.ToLower(members[right].DisplayName)
		if leftName != rightName {
			return leftName < rightName
		}
		return members[left].UserID < members[right].UserID
	})
	return groups.Detail{
		Group:   flowGroupSummary(record, actorID),
		Members: members,
	}, nil
}

func (store *flowGroupStore) RenameGroup(
	_ context.Context,
	input groups.RenameGroupInput,
) (groups.Group, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, err := store.ownerRecord(input.ActorID, input.GroupID)
	if err != nil {
		return groups.Group{}, err
	}
	record.input.Name = input.Name
	record.input.UpdatedAt = input.UpdatedAt
	return flowGroupSummary(record, input.ActorID), nil
}

func (store *flowGroupStore) DissolveGroup(
	_ context.Context,
	input groups.DissolveGroupInput,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, err := store.ownerRecord(input.ActorID, input.GroupID)
	if err != nil {
		return err
	}
	record.dissolved = true
	record.input.UpdatedAt = input.DissolvedAt
	return nil
}

func (store *flowGroupStore) GetJoinCode(
	_ context.Context,
	actorID string,
	groupID string,
) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, err := store.ownerRecord(actorID, groupID)
	if err != nil {
		return "", err
	}
	return record.input.JoinCode, nil
}

func (store *flowGroupStore) RemoveMember(
	_ context.Context,
	input groups.RemoveMemberInput,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, err := store.ownerRecord(input.ActorID, input.GroupID)
	if err != nil {
		return err
	}
	if input.UserID == record.input.OwnerUserID {
		return groups.ErrForbidden
	}
	if _, found := record.memberships[input.UserID]; !found {
		return groups.ErrNotFound
	}
	delete(record.memberships, input.UserID)
	return nil
}

func (store *flowGroupStore) ownerRecord(
	actorID string,
	groupID string,
) (*flowGroupRecord, error) {
	record, found := store.records[groupID]
	if !found || record.dissolved {
		return nil, groups.ErrNotFound
	}
	if _, active := record.memberships[actorID]; !active {
		return nil, groups.ErrNotFound
	}
	if record.input.OwnerUserID != actorID {
		return nil, groups.ErrForbidden
	}
	return record, nil
}

func (store *flowGroupStore) joinCode(t *testing.T, groupID string) string {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()
	record, found := store.records[groupID]
	if !found {
		t.Fatalf("group %s is missing", groupID)
	}
	return record.input.JoinCode
}

func flowGroupSummary(
	record *flowGroupRecord,
	actorID string,
) groups.Group {
	role := groups.RoleMember
	if actorID == record.input.OwnerUserID {
		role = groups.RoleOwner
	}
	return groups.Group{
		ID:              record.input.ID,
		Name:            record.input.Name,
		OwnerUserID:     record.input.OwnerUserID,
		MemberCount:     int64(len(record.memberships)),
		CurrentUserRole: role,
		CreatedAt:       record.input.CreatedAt,
		UpdatedAt:       record.input.UpdatedAt,
	}
}
