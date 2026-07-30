package httpapi

import (
	"errors"
	"net/http"

	"github.com/Fyy10/settled/server/internal/auth"
)

type csrfResponse struct {
	Token string `json:"csrfToken"`
}

func (a *API) getCSRF(w http.ResponseWriter, request *http.Request) {
	var sessionPointer *auth.Session
	if session, ok := sessionFromContext(request); ok {
		sessionPointer = &session
	}

	existingCookie := singleCookieValue(request, auth.CSRFCookieName)
	binding, err := a.csrf.IssueOrReuse(existingCookie, sessionPointer)
	if errors.Is(err, auth.ErrInvalidCSRFConfiguration) && sessionPointer != nil {
		requestStateFromContext(request.Context()).userID = ""
		sessionPointer = nil
		binding, err = a.csrf.IssueOrReuse(existingCookie, nil)
	}
	if err != nil {
		a.handleError(w, request, err)
		return
	}
	if !binding.Reused {
		a.csrfCookies.Set(w, binding)
	}
	if err := writeJSON(w, http.StatusOK, csrfResponse{Token: binding.Token}); err != nil {
		a.logger.Error("encode CSRF response")
	}
}

func (a *API) requireAnonymousCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		cookieValue, token, err := csrfMaterials(request)
		if err == nil {
			err = a.csrf.ValidateAnonymous(cookieValue, token)
		}
		if err != nil {
			a.handleError(w, request, err)
			return
		}
		next.ServeHTTP(w, request)
	})
}

func (a *API) requireAuthenticatedCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		session, ok := sessionFromContext(request)
		if !ok {
			a.writeError(w, unauthorizedError)
			return
		}
		cookieValue, token, err := csrfMaterials(request)
		if err == nil {
			err = a.csrf.ValidateAuthenticated(cookieValue, token, session)
		}
		if err != nil {
			a.handleError(w, request, err)
			return
		}
		next.ServeHTTP(w, request)
	})
}

func csrfMaterials(request *http.Request) (string, string, error) {
	cookies := request.CookiesNamed(auth.CSRFCookieName)
	tokens := request.Header.Values(csrfTokenHeader)
	if len(cookies) == 0 || len(tokens) == 0 {
		return "", "", auth.ErrCSRFRequired
	}
	if len(cookies) != 1 || len(tokens) != 1 {
		return "", "", auth.ErrCSRFInvalid
	}
	if cookies[0].Value == "" || tokens[0] == "" {
		return "", "", auth.ErrCSRFRequired
	}
	return cookies[0].Value, tokens[0], nil
}

func singleCookieValue(request *http.Request, name string) string {
	cookies := request.CookiesNamed(name)
	if len(cookies) != 1 {
		return ""
	}
	return cookies[0].Value
}
