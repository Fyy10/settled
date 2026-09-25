package auth

import (
	"net/http"
	"time"
)

const (
	SessionCookieName = "settled_session"
	sessionCookiePath = "/api"
)

var clearedSessionExpiration = time.Unix(0, 0).UTC()

type SessionCookies struct {
	domain   string
	secure   bool
	sameSite http.SameSite
}

func NewSessionCookies(
	domain string,
	secure bool,
	sameSite http.SameSite,
) SessionCookies {
	return SessionCookies{
		domain:   domain,
		secure:   secure,
		sameSite: sameSite,
	}
}

func (cookies SessionCookies) Set(
	writer http.ResponseWriter,
	token string,
	expiresAt time.Time,
) {
	http.SetCookie(writer, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     sessionCookiePath,
		Domain:   cookies.domain,
		Expires:  expiresAt.UTC(),
		MaxAge:   int(SessionLifetime / time.Second),
		HttpOnly: true,
		Secure:   cookies.secure,
		SameSite: cookies.sameSite,
	})
}

func (cookies SessionCookies) Clear(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     sessionCookiePath,
		Domain:   cookies.domain,
		Expires:  clearedSessionExpiration,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cookies.secure,
		SameSite: cookies.sameSite,
	})
}
