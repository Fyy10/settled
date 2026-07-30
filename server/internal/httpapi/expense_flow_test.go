package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/clock"
	"github.com/Fyy10/settled/server/internal/expenses"
)

func TestExpenseEndpointFlowExercisesAllFiveRoutes(t *testing.T) {
	now := time.Date(2026, time.August, 2, 18, 0, 0, 0, time.UTC)
	store := newFlowExpenseStore(
		groupHandlerGroupID,
		map[string]string{
			groupHandlerActorID: "Alice",
			groupHandlerOtherID: "Bob",
		},
	)
	service, err := expenses.NewServiceFrom(
		store,
		clock.Fixed{Time: now},
		bytes.NewReader(bytes.Repeat([]byte{0x31}, 16)),
	)
	if err != nil {
		t.Fatalf("expenses.NewServiceFrom: %v", err)
	}
	options := defaultTestOptions()
	options.AllowedOrigins = []string{"https://web.example"}
	options.Expenses = service
	options.Sessions = sessionValidatorFunc(func(token string) (auth.Session, error) {
		switch token {
		case "alice-session":
			return auth.Session{
				UserID: groupHandlerActorID,
				JWTID:  "alice-expense-flow",
			}, nil
		case "bob-session":
			return auth.Session{
				UserID: groupHandlerOtherID,
				JWTID:  "bob-expense-flow",
			}, nil
		default:
			return auth.Session{}, auth.ErrUnauthenticated
		}
	})
	options.Auth = fakeAuthService{
		findUser: func(
			_ context.Context,
			userID string,
		) (auth.User, error) {
			if _, active := store.displayNames[userID]; !active {
				return auth.User{}, auth.ErrUserNotFound
			}
			return auth.User{ID: userID}, nil
		},
	}
	api, _ := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		options,
	)

	createResponse := expenseFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodPost,
		"/api/groups/"+groupHandlerGroupID+"/expenses",
		`{
			"paidByUserId":"`+groupHandlerActorID+`",
			"description":" Groceries ",
			"amountCents":10,
			"expenseDate":"2026-07-31",
			"splitMode":"percentage",
			"percentageSplits":[
				{"userId":"`+groupHandlerOtherID+`","percentageBasisPoints":6000},
				{"userId":"`+groupHandlerActorID+`","percentageBasisPoints":4000}
			]
		}`,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf(
			"create = %d %s",
			createResponse.Code,
			createResponse.Body.String(),
		)
	}
	assertOnlyExactExpenseSplits(t, createResponse.Body.Bytes())
	var created expenseResponseEnvelope
	decodeGroupJSONResponse(t, createResponse.Result(), &created)
	if created.Expense.Description != "Groceries" ||
		!reflectExpenseSplits(
			created.Expense.Splits,
			[]expenseSplitResponse{
				{UserID: groupHandlerOtherID, AmountCents: 6},
				{UserID: groupHandlerActorID, AmountCents: 4},
			},
		) {
		t.Errorf("created expense = %#v", created.Expense)
	}
	expenseID := created.Expense.ID

	itemPath := "/api/groups/" +
		groupHandlerGroupID +
		"/expenses/" +
		expenseID
	getResponse := expenseFlowCall(
		t,
		api.Handler(),
		groupHandlerOtherID,
		http.MethodGet,
		itemPath,
		"",
	)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("member get = %d %s", getResponse.Code, getResponse.Body.String())
	}
	assertOnlyExactExpenseSplits(t, getResponse.Body.Bytes())

	replaceResponse := expenseFlowCall(
		t,
		api.Handler(),
		groupHandlerOtherID,
		http.MethodPut,
		itemPath,
		`{
			"paidByUserId":"`+groupHandlerOtherID+`",
			"description":"Dinner",
			"amountCents":11,
			"expenseDate":"2026-08-01",
			"splitMode":"equal",
			"participantUserIds":[
				"`+groupHandlerActorID+`",
				"`+groupHandlerOtherID+`"
			]
		}`,
	)
	if replaceResponse.Code != http.StatusOK {
		t.Fatalf(
			"collaborative replace = %d %s",
			replaceResponse.Code,
			replaceResponse.Body.String(),
		)
	}
	assertOnlyExactExpenseSplits(t, replaceResponse.Body.Bytes())
	var replaced expenseResponseEnvelope
	decodeGroupJSONResponse(t, replaceResponse.Result(), &replaced)
	if replaced.Expense.ID != created.Expense.ID ||
		replaced.Expense.CreatedByUserID != groupHandlerActorID ||
		replaced.Expense.CreatedAt != created.Expense.CreatedAt ||
		replaced.Expense.Description != "Dinner" ||
		!reflectExpenseSplits(
			replaced.Expense.Splits,
			[]expenseSplitResponse{
				{UserID: groupHandlerActorID, AmountCents: 6},
				{UserID: groupHandlerOtherID, AmountCents: 5},
			},
		) {
		t.Errorf("replaced expense = %#v", replaced.Expense)
	}

	listResponse := expenseFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodGet,
		"/api/groups/"+groupHandlerGroupID+"/expenses",
		"",
	)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list = %d %s", listResponse.Code, listResponse.Body.String())
	}
	assertOnlyExactExpenseSplits(t, listResponse.Body.Bytes())
	var listed expensesResponse
	decodeGroupJSONResponse(t, listResponse.Result(), &listed)
	if len(listed.Expenses) != 1 ||
		listed.Expenses[0].ID != expenseID ||
		len(listed.Members) != 2 {
		t.Errorf("list response = %#v", listed)
	}

	crossGroupResponse := expenseFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodGet,
		"/api/groups/bbbbbbbb-cccc-4ddd-8eee-ffffffffffff/expenses/"+
			expenseID,
		"",
	)
	assertAPIError(
		t,
		crossGroupResponse.Result(),
		http.StatusNotFound,
		"not_found",
	)

	deleteResponse := expenseFlowCall(
		t,
		api.Handler(),
		groupHandlerOtherID,
		http.MethodDelete,
		itemPath,
		"",
	)
	assertEmptyNoContent(t, deleteResponse)

	deletedResponse := expenseFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodGet,
		itemPath,
		"",
	)
	assertAPIError(
		t,
		deletedResponse.Result(),
		http.StatusNotFound,
		"not_found",
	)
	repeatedDeleteResponse := expenseFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodDelete,
		itemPath,
		"",
	)
	assertAPIError(
		t,
		repeatedDeleteResponse.Result(),
		http.StatusNotFound,
		"not_found",
	)

	emptyListResponse := expenseFlowCall(
		t,
		api.Handler(),
		groupHandlerActorID,
		http.MethodGet,
		"/api/groups/"+groupHandlerGroupID+"/expenses",
		"",
	)
	var emptyList expensesResponse
	decodeGroupJSONResponse(t, emptyListResponse.Result(), &emptyList)
	if len(emptyList.Expenses) != 0 || len(emptyList.Members) != 2 {
		t.Errorf("list after delete = %#v", emptyList)
	}
}

func expenseFlowCall(
	t *testing.T,
	handler http.Handler,
	actorID string,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	var requestBody io.Reader
	if body != "" {
		requestBody = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, requestBody)
	session := "alice-session"
	if actorID == groupHandlerOtherID {
		session = "bob-session"
	}
	request.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: session,
	})
	if method == http.MethodPost ||
		method == http.MethodPut ||
		method == http.MethodDelete {
		request.Header.Set("Origin", "https://web.example")
		request.Header.Set(csrfTokenHeader, "expense-flow-csrf")
		request.AddCookie(&http.Cookie{
			Name:  auth.CSRFCookieName,
			Value: "expense-flow-csrf-cookie",
		})
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type flowExpenseStore struct {
	mu           sync.Mutex
	groupID      string
	displayNames map[string]string
	records      map[string]*flowExpenseRecord
}

type flowExpenseRecord struct {
	expense expenses.Expense
	deleted bool
}

func newFlowExpenseStore(
	groupID string,
	displayNames map[string]string,
) *flowExpenseStore {
	return &flowExpenseStore{
		groupID:      groupID,
		displayNames: displayNames,
		records:      make(map[string]*flowExpenseRecord),
	}
}

func (store *flowExpenseStore) CreateExpense(
	_ context.Context,
	input expenses.CreateInput,
) (expenses.Expense, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.visible(input.ActorID, input.GroupID) ||
		!store.participantsActive(input.PaidByUserID, input.Splits) {
		return expenses.Expense{}, expenses.ErrNotFound
	}
	expense := expenses.Expense{
		ID:              input.ID,
		GroupID:         input.GroupID,
		PaidByUserID:    input.PaidByUserID,
		Description:     input.Description,
		AmountCents:     input.AmountCents,
		Currency:        input.Currency,
		ExpenseDate:     input.ExpenseDate,
		CreatedByUserID: input.ActorID,
		Splits:          copyExpenseSplits(input.Splits),
		CreatedAt:       input.CreatedAt,
		UpdatedAt:       input.UpdatedAt,
	}
	store.records[input.ID] = &flowExpenseRecord{expense: expense}
	return expense, nil
}

func (store *flowExpenseStore) ReplaceExpense(
	_ context.Context,
	input expenses.ReplaceInput,
) (expenses.Expense, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.visible(input.ActorID, input.GroupID) ||
		!store.participantsActive(input.PaidByUserID, input.Splits) {
		return expenses.Expense{}, expenses.ErrNotFound
	}
	record, found := store.records[input.ExpenseID]
	if !found ||
		record.deleted ||
		record.expense.GroupID != input.GroupID {
		return expenses.Expense{}, expenses.ErrNotFound
	}
	record.expense.PaidByUserID = input.PaidByUserID
	record.expense.Description = input.Description
	record.expense.AmountCents = input.AmountCents
	record.expense.Currency = input.Currency
	record.expense.ExpenseDate = input.ExpenseDate
	record.expense.Splits = copyExpenseSplits(input.Splits)
	record.expense.UpdatedAt = input.UpdatedAt
	return record.expense, nil
}

func (store *flowExpenseStore) DeleteExpense(
	_ context.Context,
	input expenses.DeleteInput,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.visible(input.ActorID, input.GroupID) {
		return expenses.ErrNotFound
	}
	record, found := store.records[input.ExpenseID]
	if !found ||
		record.deleted ||
		record.expense.GroupID != input.GroupID {
		return expenses.ErrNotFound
	}
	record.deleted = true
	record.expense.UpdatedAt = input.DeletedAt
	return nil
}

func (store *flowExpenseStore) GetExpense(
	_ context.Context,
	actorID string,
	groupID string,
	expenseID string,
) (expenses.Expense, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.visible(actorID, groupID) {
		return expenses.Expense{}, expenses.ErrNotFound
	}
	record, found := store.records[expenseID]
	if !found ||
		record.deleted ||
		record.expense.GroupID != groupID {
		return expenses.Expense{}, expenses.ErrNotFound
	}
	return copyFlowExpense(record.expense), nil
}

func (store *flowExpenseStore) ListExpenses(
	_ context.Context,
	actorID string,
	groupID string,
) (expenses.ListResult, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.visible(actorID, groupID) {
		return expenses.ListResult{}, expenses.ErrNotFound
	}
	found := make([]expenses.Expense, 0)
	for _, record := range store.records {
		if !record.deleted && record.expense.GroupID == groupID {
			found = append(found, copyFlowExpense(record.expense))
		}
	}
	sort.Slice(found, func(left, right int) bool {
		if !found[left].ExpenseDate.Equal(found[right].ExpenseDate) {
			return found[left].ExpenseDate.After(found[right].ExpenseDate)
		}
		if !found[left].CreatedAt.Equal(found[right].CreatedAt) {
			return found[left].CreatedAt.After(found[right].CreatedAt)
		}
		return found[left].ID > found[right].ID
	})
	members := make([]expenses.MemberSummary, 0, len(store.displayNames))
	for userID, displayName := range store.displayNames {
		members = append(members, expenses.MemberSummary{
			UserID:      userID,
			DisplayName: displayName,
		})
	}
	sort.Slice(members, func(left, right int) bool {
		return members[left].DisplayName < members[right].DisplayName
	})
	return expenses.ListResult{
		Expenses: found,
		Members:  members,
	}, nil
}

func (store *flowExpenseStore) visible(actorID string, groupID string) bool {
	if groupID != store.groupID {
		return false
	}
	_, active := store.displayNames[actorID]
	return active
}

func (store *flowExpenseStore) participantsActive(
	payerID string,
	splits []expenses.Split,
) bool {
	if _, active := store.displayNames[payerID]; !active {
		return false
	}
	for _, split := range splits {
		if _, active := store.displayNames[split.UserID]; !active {
			return false
		}
	}
	return true
}

func copyFlowExpense(value expenses.Expense) expenses.Expense {
	value.Splits = copyExpenseSplits(value.Splits)
	return value
}

func copyExpenseSplits(values []expenses.Split) []expenses.Split {
	copied := make([]expenses.Split, len(values))
	copy(copied, values)
	return copied
}

func assertOnlyExactExpenseSplits(t *testing.T, body []byte) {
	t.Helper()
	for _, forbidden := range []string{
		`"splitMode"`,
		`"participantUserIds"`,
		`"percentageSplits"`,
		`"percentageBasisPoints"`,
	} {
		if bytes.Contains(body, []byte(forbidden)) {
			t.Errorf("response contains request-only field %s: %s", forbidden, body)
		}
	}
}

func reflectExpenseSplits(
	got []expenseSplitResponse,
	want []expenseSplitResponse,
) bool {
	return reflect.DeepEqual(got, want)
}
