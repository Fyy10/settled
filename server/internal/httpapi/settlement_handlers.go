package httpapi

import (
	"net/http"

	"github.com/Fyy10/settled/server/internal/settlements"
)

type settlementResponse struct {
	FromUserID  string `json:"fromUserId"`
	ToUserID    string `json:"toUserId"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
}

type settlementMemberResponse struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
}

type settlementsResponse struct {
	Settlements []settlementResponse       `json:"settlements"`
	Members     []settlementMemberResponse `json:"members"`
}

func (a *API) listSettlements(w http.ResponseWriter, request *http.Request) {
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
	result, err := a.settlementService.List(
		request.Context(),
		actorID,
		groupID,
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	found := make([]settlementResponse, len(result.Transfers))
	for index, transfer := range result.Transfers {
		found[index] = newSettlementResponse(transfer)
	}
	members := make([]settlementMemberResponse, len(result.Members))
	for index, member := range result.Members {
		members[index] = settlementMemberResponse{
			UserID:      member.UserID,
			DisplayName: member.DisplayName,
		}
	}
	if err := writeJSON(w, http.StatusOK, settlementsResponse{
		Settlements: found,
		Members:     members,
	}); err != nil {
		a.logger.Error("encode settlement response")
	}
}

func newSettlementResponse(
	transfer settlements.Transfer,
) settlementResponse {
	return settlementResponse{
		FromUserID:  transfer.FromUserID,
		ToUserID:    transfer.ToUserID,
		AmountCents: transfer.AmountCents,
		Currency:    transfer.Currency,
	}
}
