package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Fyy10/settled/server/internal/auth"
	"github.com/Fyy10/settled/server/internal/expenses"
	"github.com/Fyy10/settled/server/internal/groups"
)

var (
	ErrBadRequest   = errors.New("bad request")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

type ValidationError = auth.ValidationError

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type errorSpec struct {
	status  int
	code    string
	message string
	fields  map[string]string
}

var (
	badRequestError = errorSpec{
		status:  http.StatusBadRequest,
		code:    "bad_request",
		message: "The request is invalid.",
	}
	unauthorizedError = errorSpec{
		status:  http.StatusUnauthorized,
		code:    "unauthorized",
		message: "Authentication is required.",
	}
	forbiddenError = errorSpec{
		status:  http.StatusForbidden,
		code:    "forbidden",
		message: "You are not allowed to perform this action.",
	}
	csrfRequiredError = errorSpec{
		status:  http.StatusForbidden,
		code:    "csrf_required",
		message: "A CSRF token is required.",
	}
	csrfInvalidError = errorSpec{
		status:  http.StatusForbidden,
		code:    "csrf_invalid",
		message: "The CSRF token is invalid.",
	}
	notFoundError = errorSpec{
		status:  http.StatusNotFound,
		code:    "not_found",
		message: "The requested resource was not found.",
	}
	methodNotAllowedError = errorSpec{
		status:  http.StatusMethodNotAllowed,
		code:    "method_not_allowed",
		message: "The request method is not allowed.",
	}
	conflictError = errorSpec{
		status:  http.StatusConflict,
		code:    "conflict",
		message: "The request conflicts with the current state.",
	}
	validationFailedError = errorSpec{
		status:  http.StatusUnprocessableEntity,
		code:    "validation_failed",
		message: "One or more fields are invalid.",
	}
	internalError = errorSpec{
		status:  http.StatusInternalServerError,
		code:    "internal_error",
		message: "An unexpected error occurred.",
	}
)

func (a *API) handleError(
	w http.ResponseWriter,
	request *http.Request,
	err error,
) {
	spec := errorFor(err)
	if spec.status == http.StatusInternalServerError {
		a.logger.Error(
			"HTTP request failed",
			slog.String("request_id", requestIDFromContext(request.Context())),
			slog.Any("error", err),
		)
	}
	a.writeError(w, spec)
}

func errorFor(err error) errorSpec {
	var authValidation *auth.ValidationError
	var groupValidation *groups.ValidationError
	var expenseValidation *expenses.ValidationError
	switch {
	case errors.As(err, &authValidation):
		spec := validationFailedError
		spec.fields = authValidation.Fields
		return spec
	case errors.As(err, &groupValidation):
		spec := validationFailedError
		spec.fields = groupValidation.Fields
		return spec
	case errors.As(err, &expenseValidation):
		spec := validationFailedError
		spec.fields = expenseValidation.Fields
		return spec
	case errors.Is(err, ErrBadRequest):
		return badRequestError
	case errors.Is(err, ErrUnauthorized),
		errors.Is(err, auth.ErrUnauthenticated),
		errors.Is(err, auth.ErrInvalidCredentials),
		errors.Is(err, auth.ErrUserNotFound):
		return unauthorizedError
	case errors.Is(err, auth.ErrCSRFRequired):
		return csrfRequiredError
	case errors.Is(err, auth.ErrCSRFInvalid):
		return csrfInvalidError
	case errors.Is(err, ErrForbidden), errors.Is(err, groups.ErrForbidden):
		return forbiddenError
	case errors.Is(err, ErrNotFound),
		errors.Is(err, groups.ErrNotFound),
		errors.Is(err, expenses.ErrNotFound):
		return notFoundError
	case errors.Is(err, ErrConflict),
		errors.Is(err, auth.ErrDuplicateEmail),
		errors.Is(err, groups.ErrMemberInUse):
		return conflictError
	default:
		return internalError
	}
}

func (a *API) writeError(w http.ResponseWriter, spec errorSpec) {
	err := writeJSON(w, spec.status, errorEnvelope{
		Error: errorBody{
			Code:    spec.code,
			Message: spec.message,
			Fields:  spec.fields,
		},
	})
	if err != nil {
		a.logger.Error("encode HTTP error response")
	}
}
