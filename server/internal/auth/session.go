package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Fyy10/settled/server/internal/clock"
	"github.com/Fyy10/settled/server/internal/identifier"
)

const (
	SessionLifetime        = 168 * time.Hour
	SessionClockSkew       = 30 * time.Second
	sessionIssuer          = "settled-api"
	sessionAudience        = "settled-web"
	sessionJWTIDByteLength = 32
	minimumJWTSecretLength = 32
	maxSessionTokenLength  = 4096
)

var (
	ErrUnauthenticated             = errors.New("unauthenticated")
	ErrInvalidSessionConfiguration = errors.New("invalid session configuration")
)

type Session struct {
	UserID    string
	JWTID     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type IssuedSession struct {
	Token string
	Session
}

type SessionManager struct {
	secret []byte
	clock  clock.Clock
	random io.Reader
}

type sessionClaims struct {
	jwt.RegisteredClaims
}

func NewSessionManager(secret []byte, sessionClock clock.Clock) (*SessionManager, error) {
	return NewSessionManagerFrom(secret, sessionClock, rand.Reader)
}

func NewSessionManagerFrom(
	secret []byte,
	sessionClock clock.Clock,
	random io.Reader,
) (*SessionManager, error) {
	if len(secret) < minimumJWTSecretLength || sessionClock == nil || random == nil {
		return nil, ErrInvalidSessionConfiguration
	}

	return &SessionManager{
		secret: append([]byte(nil), secret...),
		clock:  sessionClock,
		random: random,
	}, nil
}

func (manager *SessionManager) Issue(userID string) (IssuedSession, error) {
	canonicalUserID, err := identifier.ParseUUID(userID)
	if err != nil {
		return IssuedSession{}, ErrInvalidSessionConfiguration
	}

	jwtID, err := identifier.NewRandomBase64URLFrom(
		manager.random,
		sessionJWTIDByteLength,
	)
	if err != nil {
		return IssuedSession{}, err
	}

	issuedAt := manager.clock.Now().UTC().Truncate(time.Second)
	expiresAt := issuedAt.Add(SessionLifetime)
	claims := sessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    sessionIssuer,
			Subject:   canonicalUserID,
			Audience:  jwt.ClaimStrings{sessionAudience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ID:        jwtID,
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(manager.secret)
	if err != nil {
		return IssuedSession{}, errors.New("sign session token")
	}

	return IssuedSession{
		Token: token,
		Session: Session{
			UserID:    canonicalUserID,
			JWTID:     jwtID,
			IssuedAt:  issuedAt,
			ExpiresAt: expiresAt,
		},
	}, nil
}

func (manager *SessionManager) Validate(encodedToken string) (Session, error) {
	if encodedToken == "" || len(encodedToken) > maxSessionTokenLength {
		return Session{}, ErrUnauthenticated
	}

	now := manager.clock.Now().UTC()
	claims := &sessionClaims{}
	token, err := jwt.ParseWithClaims(
		encodedToken,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrUnauthenticated
			}
			return manager.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(sessionIssuer),
		jwt.WithAudience(sessionAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(SessionClockSkew),
		jwt.WithTimeFunc(func() time.Time { return now }),
		jwt.WithStrictDecoding(),
	)
	if err != nil || token == nil || !token.Valid {
		return Session{}, ErrUnauthenticated
	}

	session, valid := validateSessionClaims(claims)
	if !valid {
		return Session{}, ErrUnauthenticated
	}
	return session, nil
}

func validateSessionClaims(claims *sessionClaims) (Session, bool) {
	if claims == nil ||
		claims.ExpiresAt == nil ||
		claims.NotBefore == nil ||
		claims.IssuedAt == nil ||
		claims.Issuer != sessionIssuer ||
		len(claims.Audience) != 1 ||
		claims.Audience[0] != sessionAudience {
		return Session{}, false
	}

	canonicalUserID, err := identifier.ParseUUID(claims.Subject)
	if err != nil || canonicalUserID != claims.Subject {
		return Session{}, false
	}
	if !validJWTID(claims.ID) {
		return Session{}, false
	}

	issuedAt := claims.IssuedAt.Time.UTC()
	notBefore := claims.NotBefore.Time.UTC()
	expiresAt := claims.ExpiresAt.Time.UTC()
	if !notBefore.Equal(issuedAt) ||
		!expiresAt.After(issuedAt) ||
		expiresAt.Sub(issuedAt) > SessionLifetime+SessionClockSkew {
		return Session{}, false
	}

	return Session{
		UserID:    canonicalUserID,
		JWTID:     claims.ID,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
	}, true
}

func validJWTID(value string) bool {
	if len(value) != base64.RawURLEncoding.EncodedLen(sessionJWTIDByteLength) {
		return false
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(decoded) != sessionJWTIDByteLength {
		return false
	}
	return base64.RawURLEncoding.EncodeToString(decoded) == value
}
