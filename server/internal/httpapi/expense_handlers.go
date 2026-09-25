package httpapi

import (
	"net/http"

	"github.com/Fyy10/settled/server/internal/expenses"
)

type expenseRequest struct {
	PaidByUserID       *string                  `json:"paidByUserId"`
	Description        *string                  `json:"description"`
	AmountCents        *int64                   `json:"amountCents"`
	ExpenseDate        *string                  `json:"expenseDate"`
	SplitMode          *expenses.SplitMode      `json:"splitMode"`
	ParticipantUserIDs []string                 `json:"participantUserIds"`
	Splits             []exactSplitRequest      `json:"splits"`
	PercentageSplits   []percentageSplitRequest `json:"percentageSplits"`
}

type exactSplitRequest struct {
	UserID      *string `json:"userId"`
	AmountCents *int64  `json:"amountCents"`
}

type percentageSplitRequest struct {
	UserID                *string `json:"userId"`
	PercentageBasisPoints *int64  `json:"percentageBasisPoints"`
}

type expenseSplitResponse struct {
	UserID      string `json:"userId"`
	AmountCents int64  `json:"amountCents"`
}

type expenseResponse struct {
	ID              string                 `json:"id"`
	GroupID         string                 `json:"groupId"`
	PaidByUserID    string                 `json:"paidByUserId"`
	Description     string                 `json:"description"`
	AmountCents     int64                  `json:"amountCents"`
	Currency        string                 `json:"currency"`
	ExpenseDate     string                 `json:"expenseDate"`
	CreatedByUserID string                 `json:"createdByUserId"`
	Splits          []expenseSplitResponse `json:"splits"`
	CreatedAt       string                 `json:"createdAt"`
	UpdatedAt       string                 `json:"updatedAt"`
}

type expenseMemberResponse struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
}

type expenseResponseEnvelope struct {
	Expense expenseResponse `json:"expense"`
}

type expensesResponse struct {
	Expenses []expenseResponse       `json:"expenses"`
	Members  []expenseMemberResponse `json:"members"`
}

func (a *API) listExpenses(w http.ResponseWriter, request *http.Request) {
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
	result, err := a.expenseService.List(
		request.Context(),
		actorID,
		groupID,
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	responseExpenses := make([]expenseResponse, len(result.Expenses))
	for index, expense := range result.Expenses {
		responseExpenses[index] = newExpenseResponse(expense)
	}
	members := make([]expenseMemberResponse, len(result.Members))
	for index, member := range result.Members {
		members[index] = expenseMemberResponse{
			UserID:      member.UserID,
			DisplayName: member.DisplayName,
		}
	}
	if err := writeJSON(w, http.StatusOK, expensesResponse{
		Expenses: responseExpenses,
		Members:  members,
	}); err != nil {
		a.logger.Error("encode expense list response")
	}
}

func (a *API) createExpense(w http.ResponseWriter, request *http.Request) {
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
	var body expenseRequest
	if err := decodeJSON(w, request, &body); err != nil {
		a.handleError(w, request, err)
		return
	}
	expense, err := a.expenseService.Create(
		request.Context(),
		actorID,
		groupID,
		newExpenseMutationInput(body),
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeExpenseResponse(w, http.StatusCreated, expense)
}

func (a *API) getExpense(w http.ResponseWriter, request *http.Request) {
	actorID, groupID, expenseID, ok := a.expensePath(w, request)
	if !ok {
		return
	}
	expense, err := a.expenseService.Get(
		request.Context(),
		actorID,
		groupID,
		expenseID,
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeExpenseResponse(w, http.StatusOK, expense)
}

func (a *API) replaceExpense(w http.ResponseWriter, request *http.Request) {
	actorID, groupID, expenseID, ok := a.expensePath(w, request)
	if !ok {
		return
	}
	var body expenseRequest
	if err := decodeJSON(w, request, &body); err != nil {
		a.handleError(w, request, err)
		return
	}
	expense, err := a.expenseService.Replace(
		request.Context(),
		actorID,
		groupID,
		expenseID,
		newExpenseMutationInput(body),
	)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeExpenseResponse(w, http.StatusOK, expense)
}

func (a *API) deleteExpense(w http.ResponseWriter, request *http.Request) {
	actorID, groupID, expenseID, ok := a.expensePath(w, request)
	if !ok {
		return
	}
	if err := requireEmptyBody(request); err != nil {
		a.handleError(w, request, err)
		return
	}
	if err := a.expenseService.Delete(
		request.Context(),
		actorID,
		groupID,
		expenseID,
	); err != nil {
		a.handleError(w, request, err)
		return
	}
	writeNoContent(w)
}

func (a *API) expensePath(
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
	expenseID, err := pathUUID(request, "expenseId")
	if err != nil {
		a.handleError(w, request, err)
		return "", "", "", false
	}
	return actorID, groupID, expenseID, true
}

func (a *API) writeExpenseResponse(
	w http.ResponseWriter,
	status int,
	expense expenses.Expense,
) {
	if err := writeJSON(w, status, expenseResponseEnvelope{
		Expense: newExpenseResponse(expense),
	}); err != nil {
		a.logger.Error("encode expense response")
	}
}

func newExpenseResponse(expense expenses.Expense) expenseResponse {
	splits := make([]expenseSplitResponse, len(expense.Splits))
	for index, split := range expense.Splits {
		splits[index] = expenseSplitResponse{
			UserID:      split.UserID,
			AmountCents: split.AmountCents,
		}
	}
	return expenseResponse{
		ID:              expense.ID,
		GroupID:         expense.GroupID,
		PaidByUserID:    expense.PaidByUserID,
		Description:     expense.Description,
		AmountCents:     expense.AmountCents,
		Currency:        expense.Currency,
		ExpenseDate:     formatDate(expense.ExpenseDate),
		CreatedByUserID: expense.CreatedByUserID,
		Splits:          splits,
		CreatedAt:       formatTimestamp(expense.CreatedAt),
		UpdatedAt:       formatTimestamp(expense.UpdatedAt),
	}
}

func newExpenseMutationInput(body expenseRequest) expenses.MutationInput {
	participantUserIDs := make([]string, len(body.ParticipantUserIDs))
	copy(participantUserIDs, body.ParticipantUserIDs)
	exactSplits := make([]expenses.ExactSplitInput, len(body.Splits))
	for index, split := range body.Splits {
		exactSplits[index] = expenses.ExactSplitInput{
			UserID:      stringValue(split.UserID),
			AmountCents: int64Value(split.AmountCents),
		}
	}
	percentageSplits := make(
		[]expenses.PercentageSplitInput,
		len(body.PercentageSplits),
	)
	for index, split := range body.PercentageSplits {
		percentageSplits[index] = expenses.PercentageSplitInput{
			UserID: stringValue(split.UserID),
			PercentageBasisPoints: int64Value(
				split.PercentageBasisPoints,
			),
		}
	}
	return expenses.MutationInput{
		PaidByUserID: stringValue(body.PaidByUserID),
		Description:  stringValue(body.Description),
		AmountCents:  int64Value(body.AmountCents),
		ExpenseDate:  stringValue(body.ExpenseDate),
		SplitInput: expenses.SplitInput{
			Mode:               splitModeValue(body.SplitMode),
			ParticipantUserIDs: participantUserIDs,
			Splits:             exactSplits,
			PercentageSplits:   percentageSplits,
		},
	}
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func splitModeValue(value *expenses.SplitMode) expenses.SplitMode {
	if value == nil {
		return ""
	}
	return *value
}
