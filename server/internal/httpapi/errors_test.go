package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/expenses"
	"github.com/Fyy10/settled/server/internal/groups"
)

func TestErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantFields map[string]string
	}{
		{
			name:       "bad request",
			err:        errors.Join(errors.New("decode"), ErrBadRequest),
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:       "unauthorized",
			err:        ErrUnauthorized,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "JWT unauthenticated",
			err:        auth.ErrUnauthenticated,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "invalid credentials",
			err:        auth.ErrInvalidCredentials,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "deleted authenticated user",
			err:        auth.ErrUserNotFound,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "forbidden",
			err:        ErrForbidden,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "CSRF required",
			err:        auth.ErrCSRFRequired,
			wantStatus: http.StatusForbidden,
			wantCode:   "csrf_required",
		},
		{
			name:       "CSRF invalid",
			err:        auth.ErrCSRFInvalid,
			wantStatus: http.StatusForbidden,
			wantCode:   "csrf_invalid",
		},
		{
			name:       "not found",
			err:        ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "hidden group",
			err:        groups.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "hidden expense",
			err:        expenses.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "conflict",
			err:        ErrConflict,
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
		{
			name:       "duplicate email",
			err:        auth.ErrDuplicateEmail,
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
		{
			name:       "validation",
			err:        &ValidationError{Fields: map[string]string{"email": "Email is required."}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantFields: map[string]string{"email": "Email is required."},
		},
		{
			name: "group validation",
			err: &groups.ValidationError{Fields: map[string]string{
				"name": "Group name is required.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantFields: map[string]string{"name": "Group name is required."},
		},
		{
			name: "expense validation",
			err: &expenses.ValidationError{Fields: map[string]string{
				"expenseDate": "Expense date is required.",
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantFields: map[string]string{
				"expenseDate": "Expense date is required.",
			},
		},
		{
			name:       "join code collision exhaustion remains internal",
			err:        groups.ErrJoinCodeAttemptsExhausted,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
		{
			name:       "unknown",
			err:        errors.New("private internal detail"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := errorFor(test.err)
			if spec.status != test.wantStatus || spec.code != test.wantCode {
				t.Errorf(
					"errorFor = (%d, %q), want (%d, %q)",
					spec.status,
					spec.code,
					test.wantStatus,
					test.wantCode,
				)
			}
			if spec.message == "" {
				t.Error("message is empty")
			}
			if !reflect.DeepEqual(spec.fields, test.wantFields) {
				t.Errorf("fields = %v, want %v", spec.fields, test.wantFields)
			}
		})
	}
}

func TestHandleErrorDoesNotExposeInternalDetail(t *testing.T) {
	t.Parallel()

	api, logs := newTestAPI(
		t,
		pingerFunc(func(context.Context) error { return nil }),
		defaultTestOptions(),
	)
	request := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	state := &requestState{requestID: "request-id"}
	request = request.WithContext(context.WithValue(
		request.Context(),
		requestStateContextKey{},
		state,
	))
	response := httptest.NewRecorder()
	privateError := errors.New("private internal detail")

	api.handleError(response, request, privateError)

	if strings.Contains(response.Body.String(), privateError.Error()) {
		t.Errorf("client response exposes private detail: %s", response.Body.String())
	}
	assertAPIError(
		t,
		response.Result(),
		http.StatusInternalServerError,
		"internal_error",
	)
	if logs.Len() == 0 {
		t.Error("internal error was not logged")
	}
}
