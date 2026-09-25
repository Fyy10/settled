package auth

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Fyy10/settled/server/internal/clock"
)

const testUserID = "00112233-4455-4677-8899-aabbccddeeff"

var testSessionNow = time.Date(
	2026,
	time.July,
	29,
	20,
	15,
	30,
	987654321,
	time.FixedZone("test", -7*60*60),
)

func TestSessionManagerIssueAndCrossReplicaValidation(t *testing.T) {
	t.Parallel()

	secret := testJWTSecret(0x11)
	manager := newTestSessionManager(
		t,
		secret,
		bytes.NewReader(make([]byte, sessionJWTIDByteLength)),
	)

	issued, err := manager.Issue("00112233-4455-4677-8899-AABBCCDDEEFF")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	wantIssuedAt := testSessionNow.UTC().Truncate(time.Second)
	wantJWTID := base64.RawURLEncoding.EncodeToString(
		make([]byte, sessionJWTIDByteLength),
	)
	if issued.Token == "" {
		t.Fatal("issued token is empty")
	}
	if issued.UserID != testUserID {
		t.Errorf("UserID = %q, want %q", issued.UserID, testUserID)
	}
	if issued.JWTID != wantJWTID {
		t.Errorf("JWTID = %q, want %q", issued.JWTID, wantJWTID)
	}
	if !issued.IssuedAt.Equal(wantIssuedAt) {
		t.Errorf("IssuedAt = %v, want %v", issued.IssuedAt, wantIssuedAt)
	}
	if !issued.ExpiresAt.Equal(wantIssuedAt.Add(SessionLifetime)) {
		t.Errorf(
			"ExpiresAt = %v, want %v",
			issued.ExpiresAt,
			wantIssuedAt.Add(SessionLifetime),
		)
	}
	if len(strings.Split(issued.Token, ".")) != 3 || strings.Contains(issued.Token, "=") {
		t.Errorf("token is not compact unpadded JWT: %q", issued.Token)
	}

	mapClaims := jwt.MapClaims{}
	token, _, err := jwt.NewParser().ParseUnverified(issued.Token, mapClaims)
	if err != nil {
		t.Fatalf("ParseUnverified: %v", err)
	}
	if token.Method != jwt.SigningMethodHS256 {
		t.Errorf("signing method = %v, want HS256", token.Method)
	}
	wantClaimKeys := map[string]bool{
		"aud": true,
		"exp": true,
		"iat": true,
		"iss": true,
		"jti": true,
		"nbf": true,
		"sub": true,
	}
	if len(mapClaims) != len(wantClaimKeys) {
		t.Errorf("claim count = %d, want %d: %v", len(mapClaims), len(wantClaimKeys), mapClaims)
	}
	for key := range mapClaims {
		if !wantClaimKeys[key] {
			t.Errorf("unexpected claim %q", key)
		}
	}

	replica := newTestSessionManager(t, secret, &fillReader{value: 0x22})
	validated, err := replica.Validate(issued.Token)
	if err != nil {
		t.Fatalf("replica Validate: %v", err)
	}
	if validated != issued.Session {
		t.Errorf("validated session = %+v, want %+v", validated, issued.Session)
	}
}

func TestSessionManagerCopiesSigningSecret(t *testing.T) {
	t.Parallel()

	secret := testJWTSecret(0x31)
	manager := newTestSessionManager(t, secret, &fillReader{value: 0x44})
	clear(secret)

	issued, err := manager.Issue(testUserID)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := manager.Validate(issued.Token); err != nil {
		t.Fatalf("Validate after caller secret mutation: %v", err)
	}
}

func TestNewSessionManagerRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	validClock := clock.Fixed{Time: testSessionNow}
	tests := []struct {
		name   string
		secret []byte
		clock  clock.Clock
		random io.Reader
	}{
		{
			name:   "missing secret",
			clock:  validClock,
			random: &fillReader{},
		},
		{
			name:   "short secret",
			secret: make([]byte, minimumJWTSecretLength-1),
			clock:  validClock,
			random: &fillReader{},
		},
		{
			name:   "nil clock",
			secret: testJWTSecret(1),
			random: &fillReader{},
		},
		{
			name:   "nil random source",
			secret: testJWTSecret(1),
			clock:  validClock,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manager, err := NewSessionManagerFrom(test.secret, test.clock, test.random)
			if err != ErrInvalidSessionConfiguration {
				t.Errorf("error = %v, want exact ErrInvalidSessionConfiguration", err)
			}
			if manager != nil {
				t.Errorf("manager = %v, want nil", manager)
			}
		})
	}
}

func TestSessionManagerIssueRejectsInvalidSubjectAndRandomFailure(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"not-a-uuid",
		"00112233-4455-4677-0099-aabbccddeeff",
		"00112233-4455-4677-8899-aabbccddeef",
	}
	for _, subject := range tests {
		manager := newTestSessionManager(t, testJWTSecret(1), &fillReader{})
		issued, err := manager.Issue(subject)
		if err != ErrInvalidSessionConfiguration {
			t.Errorf("Issue(%q) error = %v", subject, err)
		}
		if issued.Token != "" {
			t.Errorf("Issue(%q) returned token", subject)
		}
	}

	sourceError := errors.New("entropy unavailable")
	manager := newTestSessionManager(
		t,
		testJWTSecret(1),
		failingReader{err: sourceError},
	)
	issued, err := manager.Issue(testUserID)
	if !errors.Is(err, sourceError) {
		t.Errorf("random failure error = %v, want wrapped %v", err, sourceError)
	}
	if issued.Token != "" {
		t.Error("random failure returned token")
	}
}

func TestSessionManagerValidationFailuresAreIndistinguishable(t *testing.T) {
	t.Parallel()

	manager := newTestSessionManager(t, testJWTSecret(0x11), &fillReader{})
	now := testSessionNow.UTC().Truncate(time.Second)
	differentSecret := testJWTSecret(0x22)

	tests := []struct {
		name  string
		token func(*testing.T) string
	}{
		{name: "empty", token: func(*testing.T) string { return "" }},
		{
			name:  "oversized",
			token: func(*testing.T) string { return strings.Repeat("x", maxSessionTokenLength+1) },
		},
		{name: "malformed", token: func(*testing.T) string { return "not-a-token" }},
		{
			name: "wrong signature",
			token: func(t *testing.T) string {
				return signSessionClaims(t, validSessionClaims(now), jwt.SigningMethodHS256, differentSecret)
			},
		},
		{
			name: "wrong algorithm",
			token: func(t *testing.T) string {
				return signSessionClaims(t, validSessionClaims(now), jwt.SigningMethodHS384, testJWTSecret(0x11))
			},
		},
		{
			name: "none algorithm",
			token: func(t *testing.T) string {
				return signSessionClaims(
					t,
					validSessionClaims(now),
					jwt.SigningMethodNone,
					jwt.UnsafeAllowNoneSignatureType,
				)
			},
		},
		{
			name: "missing issuer",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.Issuer = ""
			}),
		},
		{
			name: "wrong issuer",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.Issuer = "other-api"
			}),
		},
		{
			name: "missing audience",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.Audience = nil
			}),
		},
		{
			name: "wrong audience",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.Audience = jwt.ClaimStrings{"other-web"}
			}),
		},
		{
			name: "extra audience",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.Audience = jwt.ClaimStrings{sessionAudience, "other-web"}
			}),
		},
		{
			name: "missing subject",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.Subject = ""
			}),
		},
		{
			name: "invalid subject",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.Subject = "not-a-uuid"
			}),
		},
		{
			name: "non-RFC variant subject",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.Subject = "00112233-4455-4677-0099-aabbccddeeff"
			}),
		},
		{
			name: "noncanonical subject",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.Subject = "00112233-4455-4677-8899-AABBCCDDEEFF"
			}),
		},
		{
			name: "missing JWT ID",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.ID = ""
			}),
		},
		{
			name: "short JWT ID",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.ID = base64.RawURLEncoding.EncodeToString(make([]byte, 31))
			}),
		},
		{
			name: "padded JWT ID",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.ID += "="
			}),
		},
		{
			name: "standard Base64 JWT ID",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.ID = base64.RawStdEncoding.EncodeToString(
					bytes.Repeat([]byte{0xff}, sessionJWTIDByteLength),
				)
			}),
		},
		{
			name: "missing expiration",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.ExpiresAt = nil
			}),
		},
		{
			name: "missing issued at",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.IssuedAt = nil
			}),
		},
		{
			name: "missing not before",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.NotBefore = nil
			}),
		},
		{
			name: "expired beyond skew",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.IssuedAt = jwt.NewNumericDate(now.Add(-time.Hour))
				claims.NotBefore = jwt.NewNumericDate(now.Add(-time.Hour))
				claims.ExpiresAt = jwt.NewNumericDate(now.Add(-SessionClockSkew - time.Second))
			}),
		},
		{
			name: "issued in future beyond skew",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				future := now.Add(SessionClockSkew + time.Second)
				claims.IssuedAt = jwt.NewNumericDate(future)
				claims.NotBefore = jwt.NewNumericDate(future)
				claims.ExpiresAt = jwt.NewNumericDate(future.Add(SessionLifetime))
			}),
		},
		{
			name: "not before differs from issued at",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.NotBefore = jwt.NewNumericDate(now.Add(time.Second))
			}),
		},
		{
			name: "expiration not after issued at",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.ExpiresAt = jwt.NewNumericDate(now)
			}),
		},
		{
			name: "excessive lifetime",
			token: mutateAndSign(now, func(claims *sessionClaims) {
				claims.ExpiresAt = jwt.NewNumericDate(
					now.Add(SessionLifetime + SessionClockSkew + time.Second),
				)
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			encodedToken := test.token(t)
			session, err := manager.Validate(encodedToken)
			if err != ErrUnauthenticated {
				t.Errorf("error = %v, want exact ErrUnauthenticated", err)
			}
			if session != (Session{}) {
				t.Errorf("session = %+v, want zero", session)
			}
			if err != nil && encodedToken != "" && strings.Contains(err.Error(), encodedToken) {
				t.Errorf("error exposes token: %v", err)
			}
		})
	}
}

func TestSessionManagerAllowsOnlyDocumentedClockSkewAndLifetime(t *testing.T) {
	t.Parallel()

	manager := newTestSessionManager(t, testJWTSecret(0x11), &fillReader{})
	now := testSessionNow.UTC().Truncate(time.Second)
	tests := []struct {
		name   string
		mutate func(*sessionClaims)
	}{
		{
			name: "issued and active within future skew",
			mutate: func(claims *sessionClaims) {
				future := now.Add(SessionClockSkew - time.Second)
				claims.IssuedAt = jwt.NewNumericDate(future)
				claims.NotBefore = jwt.NewNumericDate(future)
				claims.ExpiresAt = jwt.NewNumericDate(future.Add(SessionLifetime))
			},
		},
		{
			name: "expired within skew",
			mutate: func(claims *sessionClaims) {
				claims.IssuedAt = jwt.NewNumericDate(now.Add(-time.Hour))
				claims.NotBefore = jwt.NewNumericDate(now.Add(-time.Hour))
				claims.ExpiresAt = jwt.NewNumericDate(now.Add(-SessionClockSkew + time.Second))
			},
		},
		{
			name: "maximum lifetime with skew",
			mutate: func(claims *sessionClaims) {
				claims.ExpiresAt = jwt.NewNumericDate(
					now.Add(SessionLifetime + SessionClockSkew),
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := validSessionClaims(now)
			test.mutate(&claims)
			encodedToken := signSessionClaims(
				t,
				claims,
				jwt.SigningMethodHS256,
				testJWTSecret(0x11),
			)

			if _, err := manager.Validate(encodedToken); err != nil {
				t.Errorf("Validate: %v", err)
			}
		})
	}
}

func TestSessionManagerConcurrentValidation(t *testing.T) {
	t.Parallel()

	manager := newTestSessionManager(t, testJWTSecret(0x55), &fillReader{value: 0x33})
	issued, err := manager.Issue(testUserID)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	const workers = 16
	results := make(chan error, workers)
	for range workers {
		go func() {
			session, validateErr := manager.Validate(issued.Token)
			if validateErr == nil && session != issued.Session {
				validateErr = errors.New("validated session differs from issued session")
			}
			results <- validateErr
		}()
	}
	for range workers {
		if err := <-results; err != nil {
			t.Errorf("concurrent Validate: %v", err)
		}
	}
}

func newTestSessionManager(
	t *testing.T,
	secret []byte,
	random interface{ Read([]byte) (int, error) },
) *SessionManager {
	t.Helper()

	manager, err := NewSessionManagerFrom(
		secret,
		clock.Fixed{Time: testSessionNow},
		random,
	)
	if err != nil {
		t.Fatalf("NewSessionManagerFrom: %v", err)
	}
	return manager
}

func testJWTSecret(value byte) []byte {
	return bytes.Repeat([]byte{value}, minimumJWTSecretLength)
}

func validSessionClaims(now time.Time) sessionClaims {
	now = now.UTC().Truncate(time.Second)
	return sessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    sessionIssuer,
			Subject:   testUserID,
			Audience:  jwt.ClaimStrings{sessionAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(SessionLifetime)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID: base64.RawURLEncoding.EncodeToString(
				make([]byte, sessionJWTIDByteLength),
			),
		},
	}
}

func mutateAndSign(
	now time.Time,
	mutate func(*sessionClaims),
) func(*testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		claims := validSessionClaims(now)
		mutate(&claims)
		return signSessionClaims(
			t,
			claims,
			jwt.SigningMethodHS256,
			testJWTSecret(0x11),
		)
	}
}

func signSessionClaims(
	t *testing.T,
	claims sessionClaims,
	method jwt.SigningMethod,
	key any,
) string {
	t.Helper()

	encodedToken, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return encodedToken
}
