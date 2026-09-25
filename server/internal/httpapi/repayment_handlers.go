package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/Fyy10/settled/server/internal/repayments"
)

type repaymentRequest struct {
	FromUserID    *string               `json:"fromUserId"`
	ToUserID      *string               `json:"toUserId"`
	AmountCents   *int64                `json:"amountCents"`
	Note          nullableStringRequest `json:"note"`
	RepaymentDate *string               `json:"repaymentDate"`
}

type nullableStringRequest struct {
	Present bool
	Value   *string
}

func (value *nullableStringRequest) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Value = nil
		return nil
	}
	var decoded string
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value.Value = &decoded
	return nil
}

type repaymentResponse struct {
	ID              string  `json:"id"`
	GroupID         string  `json:"groupId"`
	FromUserID      string  `json:"fromUserId"`
	ToUserID        string  `json:"toUserId"`
	AmountCents     int64   `json:"amountCents"`
	Currency        string  `json:"currency"`
	Note            *string `json:"note"`
	RepaymentDate   string  `json:"repaymentDate"`
	CreatedByUserID string  `json:"createdByUserId"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type repaymentMemberResponse struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
}

type repaymentResponseEnvelope struct {
	Repayment repaymentResponse `json:"repayment"`
}

type repaymentsResponse struct {
	Repayments []repaymentResponse       `json:"repayments"`
	Members    []repaymentMemberResponse `json:"members"`
}

func (a *API) listRepayments(w http.ResponseWriter, request *http.Request) {
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
	result, err := a.repaymentService.List(
		request.Context(),
		actorID,
		groupID,
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	found := make([]repaymentResponse, len(result.Repayments))
	for index, repayment := range result.Repayments {
		found[index] = newRepaymentResponse(repayment)
	}
	members := make([]repaymentMemberResponse, len(result.Members))
	for index, member := range result.Members {
		members[index] = repaymentMemberResponse{
			UserID:      member.UserID,
			DisplayName: member.DisplayName,
		}
	}
	if err := writeJSON(w, http.StatusOK, repaymentsResponse{
		Repayments: found,
		Members:    members,
	}); err != nil {
		a.logger.Error("encode repayment list response")
	}
}

func (a *API) createRepayment(w http.ResponseWriter, request *http.Request) {
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
	var body repaymentRequest
	if err := decodeJSON(w, request, &body); err != nil {
		a.handleError(w, request, err)
		return
	}
	repayment, err := a.repaymentService.Create(
		request.Context(),
		actorID,
		groupID,
		newRepaymentMutationInput(body),
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeRepaymentResponse(w, http.StatusCreated, repayment)
}

func (a *API) getRepayment(w http.ResponseWriter, request *http.Request) {
	actorID, groupID, repaymentID, ok := a.repaymentPath(w, request)
	if !ok {
		return
	}
	repayment, err := a.repaymentService.Get(
		request.Context(),
		actorID,
		groupID,
		repaymentID,
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeRepaymentResponse(w, http.StatusOK, repayment)
}

func (a *API) replaceRepayment(
	w http.ResponseWriter,
	request *http.Request,
) {
	actorID, groupID, repaymentID, ok := a.repaymentPath(w, request)
	if !ok {
		return
	}
	var body repaymentRequest
	if err := decodeJSON(w, request, &body); err != nil {
		a.handleError(w, request, err)
		return
	}
	repayment, err := a.repaymentService.Replace(
		request.Context(),
		actorID,
		groupID,
		repaymentID,
		newRepaymentMutationInput(body),
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeRepaymentResponse(w, http.StatusOK, repayment)
}

func (a *API) deleteRepayment(w http.ResponseWriter, request *http.Request) {
	actorID, groupID, repaymentID, ok := a.repaymentPath(w, request)
	if !ok {
		return
	}
	if err := requireEmptyBody(request); err != nil {
		a.handleError(w, request, err)
		return
	}
	if err := a.repaymentService.Delete(
		request.Context(),
		actorID,
		groupID,
		repaymentID,
	); err != nil {
		a.handleError(w, request, err)
		return
	}
	writeNoContent(w)
}

func (a *API) repaymentPath(
	w http.ResponseWriter,
	request *http.Request,
) (string, string, string, bool) {
	actorID, ok := authenticatedUserID(request)
	if !ok {
		a.writeError(w, unauthorizedError)
		return "", "", "", false
	}
	groupID, err := pathUUID(request, "groupId")
	if err != nil {
		a.handleError(w, request, err)
		return "", "", "", false
	}
	repaymentID, err := pathUUID(request, "repaymentId")
	if err != nil {
		a.handleError(w, request, err)
		return "", "", "", false
	}
	return actorID, groupID, repaymentID, true
}

func (a *API) writeRepaymentResponse(
	w http.ResponseWriter,
	status int,
	repayment repayments.Repayment,
) {
	if err := writeJSON(w, status, repaymentResponseEnvelope{
		Repayment: newRepaymentResponse(repayment),
	}); err != nil {
		a.logger.Error("encode repayment response")
	}
}

func newRepaymentResponse(repayment repayments.Repayment) repaymentResponse {
	return repaymentResponse{
		ID:              repayment.ID,
		GroupID:         repayment.GroupID,
		FromUserID:      repayment.FromUserID,
		ToUserID:        repayment.ToUserID,
		AmountCents:     repayment.AmountCents,
		Currency:        repayment.Currency,
		Note:            repayment.Note,
		RepaymentDate:   formatDate(repayment.RepaymentDate),
		CreatedByUserID: repayment.CreatedByUserID,
		CreatedAt:       formatTimestamp(repayment.CreatedAt),
		UpdatedAt:       formatTimestamp(repayment.UpdatedAt),
	}
}

func newRepaymentMutationInput(
	body repaymentRequest,
) repayments.MutationInput {
	return repayments.MutationInput{
		FromUserID:    stringValue(body.FromUserID),
		ToUserID:      stringValue(body.ToUserID),
		AmountCents:   int64Value(body.AmountCents),
		Note:          body.Note.Value,
		NotePresent:   body.Note.Present,
		RepaymentDate: stringValue(body.RepaymentDate),
	}
}
