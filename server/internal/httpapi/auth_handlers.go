package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/Fyy10/settled/server/internal/auth"
)

type registerRequest struct {
	Email       *string `json:"email"`
	Password    *string `json:"password"`
	DisplayName *string `json:"displayName"`
}

type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type authResponse struct {
	CSRFToken string       `json:"csrfToken"`
	User      userResponse `json:"user"`
}

type meResponse struct {
	User userResponse `json:"user"`
}

func (a *API) registerUser(w http.ResponseWriter, request *http.Request) {
	var body registerRequest
	if err := decodeJSON(w, request, &body); err != nil {
		a.handleError(w, request, err)
		return
	}

	result, err := a.authService.Register(request.Context(), auth.RegisterInput{
		Email:       stringValue(body.Email),
		Password:    stringValue(body.Password),
		DisplayName: stringValue(body.DisplayName),
	})
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeAuthResult(w, request, http.StatusCreated, result)
}

func (a *API) login(w http.ResponseWriter, request *http.Request) {
	email, password, err := basicCredentials(request)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	if err := requireEmptyBody(request); err != nil {
		a.handleError(w, request, err)
		return
	}

	result, err := a.authService.Login(request.Context(), email, password)
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	a.writeAuthResult(w, request, http.StatusOK, result)
}

func (a *API) logout(w http.ResponseWriter, request *http.Request) {
	if err := requireEmptyBody(request); err != nil {
		a.handleError(w, request, err)
		return
	}
	binding, err := a.csrf.RotateAnonymous()
	if err != nil {
		a.handleError(w, request, err)
		return
	}

	a.sessionCookies.Clear(w)
	a.csrfCookies.Set(w, binding)
	writeNoContent(w)
}

func (a *API) me(w http.ResponseWriter, request *http.Request) {
	user, ok := userFromContext(request)
	if !ok {
		a.writeError(w, unauthorizedError)
		return
	}
	if err := writeJSON(w, http.StatusOK, meResponse{
		User: newUserResponse(user),
	}); err != nil {
		a.logger.Error("encode current user response")
	}
}

func (a *API) writeAuthResult(
	w http.ResponseWriter,
	request *http.Request,
	status int,
	result auth.AuthResult,
) {
	binding, err := a.csrf.RotateAuthenticated(result.Session.Session)
	if err != nil {
		a.handleError(w, request, err)
		return
	}

	a.sessionCookies.Set(w, result.Session.Token, result.Session.ExpiresAt)
	a.csrfCookies.Set(w, binding)
	if err := writeJSON(w, status, authResponse{
		CSRFToken: binding.Token,
		User:      newUserResponse(result.User),
	}); err != nil {
		a.logger.Error("encode authentication response")
	}
}

func newUserResponse(user auth.User) userResponse {
	return userResponse{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   formatTimestamp(user.CreatedAt),
		UpdatedAt:   formatTimestamp(user.UpdatedAt),
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func basicCredentials(request *http.Request) (string, string, error) {
	values := request.Header.Values("Authorization")
	if len(values) == 0 {
		return "", "", ErrUnauthorized
	}
	if len(values) != 1 {
		return "", "", fmt.Errorf(
			"%w: Authorization must occur exactly once",
			ErrBadRequest,
		)
	}

	email, password, ok := request.BasicAuth()
	if !ok || !utf8.ValidString(email) || !utf8.ValidString(password) {
		return "", "", fmt.Errorf("%w: malformed Basic Auth", ErrBadRequest)
	}
	return email, password, nil
}

func requireEmptyBody(request *http.Request) error {
	if request.Body == nil {
		return nil
	}
	value, err := io.ReadAll(io.LimitReader(request.Body, 1))
	if err != nil || len(value) != 0 {
		return fmt.Errorf("%w: request body must be empty", ErrBadRequest)
	}
	return nil
}
