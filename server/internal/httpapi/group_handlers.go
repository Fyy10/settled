package httpapi

import (
	"fmt"
	"net/http"

	"github.com/Fyy10/settled/server/internal/groups"
	"github.com/Fyy10/settled/server/internal/identifier"
)

type createGroupRequest struct {
	Name *string `json:"name"`
}

type joinGroupRequest struct {
	JoinCode *string `json:"joinCode"`
}

type groupResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	OwnerUserID     string `json:"ownerUserId"`
	MemberCount     int64  `json:"memberCount"`
	CurrentUserRole string `json:"currentUserRole"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type groupMemberResponse struct {
	UserID      string `json:"userId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	JoinedAt    string `json:"joinedAt"`
}

type groupsResponse struct {
	Groups []groupResponse `json:"groups"`
}

type groupResponseEnvelope struct {
	Group groupResponse `json:"group"`
}

type groupDetailResponse struct {
	Group   groupResponse         `json:"group"`
	Members []groupMemberResponse `json:"members"`
}

func (a *API) listGroups(w http.ResponseWriter, request *http.Request) {
	actorID, ok := authenticatedUserID(request)
	if !ok {
		a.writeError(w, unauthorizedError)
		return
	}
	found, err := a.groupService.List(request.Context(), actorID)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	responseGroups := make([]groupResponse, len(found))
	for index, group := range found {
		responseGroups[index] = newGroupResponse(group)
	}
	if err := writeJSON(w, http.StatusOK, groupsResponse{
		Groups: responseGroups,
	}); err != nil {
		a.logger.Error("encode group list response")
	}
}

func (a *API) createGroup(w http.ResponseWriter, request *http.Request) {
	actorID, ok := authenticatedUserID(request)
	if !ok {
		a.writeError(w, unauthorizedError)
		return
	}
	var body createGroupRequest
	if err := decodeJSON(w, request, &body); err != nil {
		a.handleError(w, request, err)
		return
	}
	group, err := a.groupService.Create(
		request.Context(),
		actorID,
		stringValue(body.Name),
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeGroupResponse(w, http.StatusCreated, group)
}

func (a *API) joinGroup(w http.ResponseWriter, request *http.Request) {
	actorID, ok := authenticatedUserID(request)
	if !ok {
		a.writeError(w, unauthorizedError)
		return
	}
	var body joinGroupRequest
	if err := decodeJSON(w, request, &body); err != nil {
		a.handleError(w, request, err)
		return
	}
	group, err := a.groupService.Join(
		request.Context(),
		actorID,
		stringValue(body.JoinCode),
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeGroupResponse(w, http.StatusOK, group)
}

func (a *API) getGroup(w http.ResponseWriter, request *http.Request) {
	actorID, ok := authenticatedUserID(request)
	if !ok {
		a.writeError(w, unauthorizedError)
		return
	}
	groupID, err := identifier.ParseUUID(request.PathValue("groupId"))
	if err != nil {
		a.handleError(
			w,
			request,
			fmt.Errorf("%w: invalid groupId", ErrBadRequest),
		)
		return
	}
	detail, err := a.groupService.Get(
		request.Context(),
		actorID,
		groupID,
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	members := make([]groupMemberResponse, len(detail.Members))
	for index, member := range detail.Members {
		members[index] = newGroupMemberResponse(member)
	}
	if err := writeJSON(w, http.StatusOK, groupDetailResponse{
		Group:   newGroupResponse(detail.Group),
		Members: members,
	}); err != nil {
		a.logger.Error("encode group detail response")
	}
}

func (a *API) writeGroupResponse(
	w http.ResponseWriter,
	status int,
	group groups.Group,
) {
	if err := writeJSON(w, status, groupResponseEnvelope{
		Group: newGroupResponse(group),
	}); err != nil {
		a.logger.Error("encode group response")
	}
}

func newGroupResponse(group groups.Group) groupResponse {
	return groupResponse{
		ID:              group.ID,
		Name:            group.Name,
		OwnerUserID:     group.OwnerUserID,
		MemberCount:     group.MemberCount,
		CurrentUserRole: group.CurrentUserRole,
		CreatedAt:       formatTimestamp(group.CreatedAt),
		UpdatedAt:       formatTimestamp(group.UpdatedAt),
	}
}

func newGroupMemberResponse(member groups.Member) groupMemberResponse {
	return groupMemberResponse{
		UserID:      member.UserID,
		Email:       member.Email,
		DisplayName: member.DisplayName,
		Role:        member.Role,
		JoinedAt:    formatTimestamp(member.JoinedAt),
	}
}

func authenticatedUserID(request *http.Request) (string, bool) {
	user, ok := userFromContext(request)
	if !ok || user.ID == "" {
		return "", false
	}
	return user.ID, true
}
