package httpapi

import "net/http"

type groupJoinCodeResponse struct {
	JoinCode string `json:"joinCode"`
}

func (a *API) renameGroup(w http.ResponseWriter, request *http.Request) {
	actorID, ok := authenticatedUserID(request)
	if !ok {
		a.writeError(w, unauthorizedError)
		return
	}
	groupID, err := pathUUID(request, "groupId")
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	var body createGroupRequest
	if err := decodeJSON(w, request, &body); err != nil {
		a.handleError(w, request, err)
		return
	}
	group, err := a.groupService.Rename(
		request.Context(),
		actorID,
		groupID,
		stringValue(body.Name),
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeGroupResponse(w, http.StatusOK, group)
}

func (a *API) dissolveGroup(w http.ResponseWriter, request *http.Request) {
	actorID, ok := authenticatedUserID(request)
	if !ok {
		a.writeError(w, unauthorizedError)
		return
	}
	groupID, err := pathUUID(request, "groupId")
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	if err := requireEmptyBody(request); err != nil {
		a.handleError(w, request, err)
		return
	}
	if err := a.groupService.Dissolve(
		request.Context(),
		actorID,
		groupID,
	); err != nil {
		a.handleError(w, request, err)
		return
	}
	writeNoContent(w)
}

func (a *API) getGroupJoinCode(
	w http.ResponseWriter,
	request *http.Request,
) {
	actorID, ok := authenticatedUserID(request)
	if !ok {
		a.writeError(w, unauthorizedError)
		return
	}
	groupID, err := pathUUID(request, "groupId")
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	joinCode, err := a.groupService.GetJoinCode(
		request.Context(),
		actorID,
		groupID,
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	if err := writeJSON(w, http.StatusOK, groupJoinCodeResponse{
		JoinCode: joinCode,
	}); err != nil {
		a.logger.Error("encode group join code response")
	}
}

func (a *API) removeGroupMember(
	w http.ResponseWriter,
	request *http.Request,
) {
	actorID, ok := authenticatedUserID(request)
	if !ok {
		a.writeError(w, unauthorizedError)
		return
	}
	groupID, err := pathUUID(request, "groupId")
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	userID, err := pathUUID(request, "userId")
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	if err := requireEmptyBody(request); err != nil {
		a.handleError(w, request, err)
		return
	}
	if err := a.groupService.RemoveMember(
		request.Context(),
		actorID,
		groupID,
		userID,
	); err != nil {
		a.handleError(w, request, err)
		return
	}
	writeNoContent(w)
}
