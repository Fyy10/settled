package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Fyy10/settled/server/internal/clock"
)

const (
	CSRFCookieName               = "settled_csrf"
	anonymousCSRFLifetime        = time.Hour
	csrfCookieVersion       byte = 1
	csrfTokenVersion             = "v1"
	csrfTokenMACLabel            = "token-v1"
	csrfNonceByteLength          = 32
	csrfMACByteLength            = sha256.Size
	minimumCSRFSecretLength      = 32
	maxCSRFExpirationLength      = 19
)

var (
	ErrCSRFRequired             = errors.New("CSRF material is required")
	ErrCSRFInvalid              = errors.New("CSRF material is invalid")
	ErrInvalidCSRFConfiguration = errors.New("invalid CSRF configuration")
)

type CSRFBinding struct {
	CookieValue   string
	Token         string
	ExpiresAt     time.Time
	MaxAgeSeconds int
	Reused        bool
}

type CSRFManager struct {
	secret []byte
	clock  clock.Clock
	random io.Reader
}

type CSRFCookies struct {
	domain   string
	secure   bool
	sameSite http.SameSite
}

type parsedCSRFCookie struct {
	value          string
	nonce          []byte
	expirationText string
	expirationUnix int64
	mac            []byte
}

func NewCSRFManager(
	secret []byte,
	csrfClock clock.Clock,
) (*CSRFManager, error) {
	return NewCSRFManagerFrom(secret, csrfClock, rand.Reader)
}

func NewCSRFManagerFrom(
	secret []byte,
	csrfClock clock.Clock,
	random io.Reader,
) (*CSRFManager, error) {
	if len(secret) < minimumCSRFSecretLength || csrfClock == nil || random == nil {
		return nil, ErrInvalidCSRFConfiguration
	}

	return &CSRFManager{
		secret: append([]byte(nil), secret...),
		clock:  csrfClock,
		random: random,
	}, nil
}

// IssueOrReuse returns a binding for GET /api/auth/csrf.
func (manager *CSRFManager) IssueOrReuse(
	existingCookieValue string,
	session *Session,
) (CSRFBinding, error) {
	now := manager.now()
	subject := anonymousCSRFSubject
	expirationUnix, err := anonymousCSRFExpiration(now)
	if err != nil {
		return CSRFBinding{}, err
	}

	if session != nil {
		sessionSubject, sessionExpiration, valid := validCSRFSession(*session, now)
		if !valid {
			return CSRFBinding{}, ErrInvalidCSRFConfiguration
		}
		subject = sessionSubject
		expirationUnix = sessionExpiration
	}

	if parsed, valid := manager.reusableCookie(
		existingCookieValue,
		now,
		expirationUnix,
		session != nil,
	); valid {
		return manager.bindingFromCookie(parsed, subject, now, true)
	}

	return manager.newBinding(subject, expirationUnix, now)
}

// RotateAnonymous always creates a fresh one-hour anonymous binding.
func (manager *CSRFManager) RotateAnonymous() (CSRFBinding, error) {
	now := manager.now()
	expirationUnix, err := anonymousCSRFExpiration(now)
	if err != nil {
		return CSRFBinding{}, err
	}
	return manager.newBinding(anonymousCSRFSubject, expirationUnix, now)
}

// RotateAuthenticated always creates a fresh binding for a validated session.
func (manager *CSRFManager) RotateAuthenticated(
	session Session,
) (CSRFBinding, error) {
	now := manager.now()
	subject, expirationUnix, valid := validCSRFSession(session, now)
	if !valid {
		return CSRFBinding{}, ErrInvalidCSRFConfiguration
	}
	return manager.newBinding(subject, expirationUnix, now)
}

func (manager *CSRFManager) ValidateAnonymous(
	cookieValue string,
	headerToken string,
) error {
	if cookieValue == "" || headerToken == "" {
		return ErrCSRFRequired
	}

	now := manager.now()
	parsed, valid := manager.authenticateCookie(cookieValue)
	if !valid ||
		parsed.expirationUnix <= now.Unix() ||
		parsed.expirationUnix-now.Unix() > int64(anonymousCSRFLifetime/time.Second) {
		return ErrCSRFInvalid
	}
	if !manager.authenticateToken(
		headerToken,
		parsed.value,
		anonymousCSRFSubject,
	) {
		return ErrCSRFInvalid
	}
	return nil
}

func (manager *CSRFManager) ValidateAuthenticated(
	cookieValue string,
	headerToken string,
	session Session,
) error {
	if cookieValue == "" || headerToken == "" {
		return ErrCSRFRequired
	}

	now := manager.now()
	subject, sessionExpiration, valid := validCSRFSession(session, now)
	if !valid {
		return ErrCSRFInvalid
	}

	parsed, valid := manager.authenticateCookie(cookieValue)
	if !valid ||
		parsed.expirationUnix <= now.Unix() ||
		parsed.expirationUnix != sessionExpiration {
		return ErrCSRFInvalid
	}
	if !manager.authenticateToken(headerToken, parsed.value, subject) {
		return ErrCSRFInvalid
	}
	return nil
}

func NewCSRFCookies(
	domain string,
	secure bool,
	sameSite http.SameSite,
) CSRFCookies {
	return CSRFCookies{
		domain:   domain,
		secure:   secure,
		sameSite: sameSite,
	}
}

func (cookies CSRFCookies) Set(
	writer http.ResponseWriter,
	binding CSRFBinding,
) {
	http.SetCookie(writer, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    binding.CookieValue,
		Path:     sessionCookiePath,
		Domain:   cookies.domain,
		Expires:  binding.ExpiresAt.UTC(),
		MaxAge:   binding.MaxAgeSeconds,
		HttpOnly: true,
		Secure:   cookies.secure,
		SameSite: cookies.sameSite,
	})
}

func (cookies CSRFCookies) Clear(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{
		Name:     CSRFCookieName,
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

const anonymousCSRFSubject = "anonymous"

func (manager *CSRFManager) now() time.Time {
	return manager.clock.Now().UTC().Truncate(time.Second)
}

func anonymousCSRFExpiration(now time.Time) (int64, error) {
	nowUnix := now.Unix()
	lifetimeSeconds := int64(anonymousCSRFLifetime / time.Second)
	if nowUnix <= 0 || nowUnix > math.MaxInt64-lifetimeSeconds {
		return 0, ErrInvalidCSRFConfiguration
	}
	return nowUnix + lifetimeSeconds, nil
}

func validCSRFSession(
	session Session,
	now time.Time,
) (string, int64, bool) {
	if !validJWTID(session.JWTID) ||
		session.IssuedAt.IsZero() ||
		session.ExpiresAt.IsZero() ||
		!session.IssuedAt.Equal(session.IssuedAt.UTC().Truncate(time.Second)) ||
		!session.ExpiresAt.Equal(session.ExpiresAt.UTC().Truncate(time.Second)) {
		return "", 0, false
	}

	issuedAt := session.IssuedAt.UTC()
	expiresAt := session.ExpiresAt.UTC()
	if issuedAt.After(now.Add(SessionClockSkew)) ||
		!expiresAt.After(now) ||
		!expiresAt.After(issuedAt) ||
		expiresAt.Sub(issuedAt) > SessionLifetime+SessionClockSkew {
		return "", 0, false
	}

	maxRemaining := int64((SessionLifetime + SessionClockSkew) / time.Second)
	if expiresAt.Unix()-now.Unix() > maxRemaining {
		return "", 0, false
	}
	return session.JWTID, expiresAt.Unix(), true
}

func (manager *CSRFManager) reusableCookie(
	value string,
	now time.Time,
	expectedSessionExpiration int64,
	authenticated bool,
) (parsedCSRFCookie, bool) {
	if value == "" {
		return parsedCSRFCookie{}, false
	}

	parsed, valid := manager.authenticateCookie(value)
	if !valid || parsed.expirationUnix <= now.Unix() {
		return parsedCSRFCookie{}, false
	}
	if authenticated {
		if parsed.expirationUnix != expectedSessionExpiration {
			return parsedCSRFCookie{}, false
		}
		return parsed, true
	}

	if parsed.expirationUnix-now.Unix() >
		int64(anonymousCSRFLifetime/time.Second) {
		return parsedCSRFCookie{}, false
	}
	return parsed, true
}

func (manager *CSRFManager) newBinding(
	subject string,
	expirationUnix int64,
	now time.Time,
) (CSRFBinding, error) {
	nonce := make([]byte, csrfNonceByteLength)
	if _, err := io.ReadFull(manager.random, nonce); err != nil {
		return CSRFBinding{}, fmt.Errorf("read CSRF nonce: %w", err)
	}
	defer clear(nonce)

	expirationText := strconv.FormatInt(expirationUnix, 10)
	encodedNonce := base64.RawURLEncoding.EncodeToString(nonce)
	cookieMAC := manager.cookieMAC(nonce, expirationText)
	defer clear(cookieMAC)

	cookieValue := strings.Join(
		[]string{
			encodedNonce,
			expirationText,
			base64.RawURLEncoding.EncodeToString(cookieMAC),
		},
		".",
	)
	parsed := parsedCSRFCookie{
		value:          cookieValue,
		expirationText: expirationText,
		expirationUnix: expirationUnix,
	}
	return manager.bindingFromCookie(parsed, subject, now, false)
}

func (manager *CSRFManager) bindingFromCookie(
	parsed parsedCSRFCookie,
	subject string,
	now time.Time,
	reused bool,
) (CSRFBinding, error) {
	maxAgeSeconds := parsed.expirationUnix - now.Unix()
	if maxAgeSeconds <= 0 || maxAgeSeconds > int64(math.MaxInt) {
		return CSRFBinding{}, ErrInvalidCSRFConfiguration
	}

	tokenMAC := manager.tokenMAC(parsed.value, subject)
	defer clear(tokenMAC)
	return CSRFBinding{
		CookieValue: parsed.value,
		Token: csrfTokenVersion + "." +
			base64.RawURLEncoding.EncodeToString(tokenMAC),
		ExpiresAt:     time.Unix(parsed.expirationUnix, 0).UTC(),
		MaxAgeSeconds: int(maxAgeSeconds),
		Reused:        reused,
	}, nil
}

func (manager *CSRFManager) authenticateCookie(
	value string,
) (parsedCSRFCookie, bool) {
	parsed, valid := parseCSRFCookie(value)
	if !valid {
		return parsedCSRFCookie{}, false
	}

	expectedMAC := manager.cookieMAC(parsed.nonce, parsed.expirationText)
	defer clear(expectedMAC)
	if subtle.ConstantTimeCompare(expectedMAC, parsed.mac) != 1 {
		return parsedCSRFCookie{}, false
	}
	return parsed, true
}

func parseCSRFCookie(value string) (parsedCSRFCookie, bool) {
	maxLength := base64.RawURLEncoding.EncodedLen(csrfNonceByteLength) +
		1 +
		maxCSRFExpirationLength +
		1 +
		base64.RawURLEncoding.EncodedLen(csrfMACByteLength)
	if len(value) == 0 || len(value) > maxLength {
		return parsedCSRFCookie{}, false
	}

	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return parsedCSRFCookie{}, false
	}
	nonce, valid := decodeCanonicalRawURL(parts[0], csrfNonceByteLength)
	if !valid {
		return parsedCSRFCookie{}, false
	}
	mac, valid := decodeCanonicalRawURL(parts[2], csrfMACByteLength)
	if !valid {
		clear(nonce)
		return parsedCSRFCookie{}, false
	}

	expirationUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil ||
		expirationUnix <= 0 ||
		strconv.FormatInt(expirationUnix, 10) != parts[1] {
		clear(nonce)
		clear(mac)
		return parsedCSRFCookie{}, false
	}

	return parsedCSRFCookie{
		value:          value,
		nonce:          nonce,
		expirationText: parts[1],
		expirationUnix: expirationUnix,
		mac:            mac,
	}, true
}

func (manager *CSRFManager) authenticateToken(
	value string,
	cookieValue string,
	subject string,
) bool {
	expectedLength := len(csrfTokenVersion) +
		1 +
		base64.RawURLEncoding.EncodedLen(csrfMACByteLength)
	if len(value) != expectedLength {
		return false
	}

	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] != csrfTokenVersion {
		return false
	}
	actualMAC, valid := decodeCanonicalRawURL(parts[1], csrfMACByteLength)
	if !valid {
		return false
	}
	defer clear(actualMAC)

	expectedMAC := manager.tokenMAC(cookieValue, subject)
	defer clear(expectedMAC)
	return subtle.ConstantTimeCompare(expectedMAC, actualMAC) == 1
}

func (manager *CSRFManager) cookieMAC(
	nonce []byte,
	expirationText string,
) []byte {
	mac := hmac.New(sha256.New, manager.secret)
	_, _ = mac.Write([]byte{csrfCookieVersion})
	writeLengthPrefixed(mac, nonce)
	writeLengthPrefixed(mac, []byte(expirationText))
	return mac.Sum(nil)
}

func (manager *CSRFManager) tokenMAC(
	cookieValue string,
	subject string,
) []byte {
	mac := hmac.New(sha256.New, manager.secret)
	writeLengthPrefixed(mac, []byte(csrfTokenMACLabel))
	writeLengthPrefixed(mac, []byte(cookieValue))
	writeLengthPrefixed(mac, []byte(subject))
	return mac.Sum(nil)
}

func writeLengthPrefixed(destination hash.Hash, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = destination.Write(length[:])
	_, _ = destination.Write(value)
}

func decodeCanonicalRawURL(
	value string,
	decodedLength int,
) ([]byte, bool) {
	if len(value) != base64.RawURLEncoding.EncodedLen(decodedLength) {
		return nil, false
	}

	decoded := make([]byte, decodedLength)
	written, err := base64.RawURLEncoding.Strict().Decode(
		decoded,
		[]byte(value),
	)
	if err != nil ||
		written != decodedLength ||
		base64.RawURLEncoding.EncodeToString(decoded) != value {
		clear(decoded)
		return nil, false
	}
	return decoded, true
}
