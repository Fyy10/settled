package auth

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Fyy10/settled/server/internal/clock"
)

var csrfTestNow = time.Date(
	2026,
	time.July,
	29,
	21,
	15,
	30,
	987654321,
	time.FixedZone("test", -7*60*60),
)

func TestCSRFAnonymousIssuanceAndValidation(t *testing.T) {
	t.Parallel()

	random := &fillReader{value: 0x5a}
	manager := newCSRFTestManager(t, csrfTestNow, random)

	binding, err := manager.IssueOrReuse("", nil)
	if err != nil {
		t.Fatalf("IssueOrReuse: %v", err)
	}

	wantNow := csrfTestNow.UTC().Truncate(time.Second)
	if binding.Reused {
		t.Error("Reused = true, want false")
	}
	if !binding.ExpiresAt.Equal(wantNow.Add(anonymousCSRFLifetime)) {
		t.Errorf(
			"ExpiresAt = %v, want %v",
			binding.ExpiresAt,
			wantNow.Add(anonymousCSRFLifetime),
		)
	}
	if binding.MaxAgeSeconds != int(anonymousCSRFLifetime/time.Second) {
		t.Errorf(
			"MaxAgeSeconds = %d, want %d",
			binding.MaxAgeSeconds,
			int(anonymousCSRFLifetime/time.Second),
		)
	}
	if random.bytesRead() != csrfNonceByteLength {
		t.Errorf("random bytes = %d, want %d", random.bytesRead(), csrfNonceByteLength)
	}

	parsed, valid := parseCSRFCookie(binding.CookieValue)
	if !valid {
		t.Fatalf("issued cookie is not canonical: %q", binding.CookieValue)
	}
	defer clearParsedCSRFCookie(parsed)
	if !bytes.Equal(parsed.nonce, bytes.Repeat([]byte{0x5a}, csrfNonceByteLength)) {
		t.Errorf("nonce = %x, want repeated 0x5a", parsed.nonce)
	}
	if parsed.expirationUnix != binding.ExpiresAt.Unix() {
		t.Errorf(
			"cookie expiration = %d, want %d",
			parsed.expirationUnix,
			binding.ExpiresAt.Unix(),
		)
	}
	if len(binding.Token) != len(csrfTokenVersion)+1+
		base64.RawURLEncoding.EncodedLen(csrfMACByteLength) {
		t.Errorf("token length = %d", len(binding.Token))
	}
	if err := manager.ValidateAnonymous(binding.CookieValue, binding.Token); err != nil {
		t.Fatalf("ValidateAnonymous: %v", err)
	}
}

func TestCSRFAuthenticatedIssuanceAndCrossReplicaValidation(t *testing.T) {
	t.Parallel()

	secret := csrfTestSecret(0x31)
	session := csrfTestSession(csrfTestNow, 0x41)
	issuer, err := NewCSRFManagerFrom(
		secret,
		clock.Fixed{Time: csrfTestNow},
		&fillReader{value: 0x51},
	)
	if err != nil {
		t.Fatalf("NewCSRFManagerFrom issuer: %v", err)
	}

	binding, err := issuer.RotateAuthenticated(session)
	if err != nil {
		t.Fatalf("RotateAuthenticated: %v", err)
	}
	if binding.Reused {
		t.Error("Reused = true, want false")
	}
	if !binding.ExpiresAt.Equal(session.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want session expiry %v", binding.ExpiresAt, session.ExpiresAt)
	}
	wantMaxAge := int(session.ExpiresAt.Unix() - csrfTestNow.UTC().Truncate(time.Second).Unix())
	if binding.MaxAgeSeconds != wantMaxAge {
		t.Errorf("MaxAgeSeconds = %d, want %d", binding.MaxAgeSeconds, wantMaxAge)
	}

	replica, err := NewCSRFManagerFrom(
		append([]byte(nil), secret...),
		clock.Fixed{Time: csrfTestNow},
		&fillReader{value: 0x61},
	)
	if err != nil {
		t.Fatalf("NewCSRFManagerFrom replica: %v", err)
	}
	if err := replica.ValidateAuthenticated(
		binding.CookieValue,
		binding.Token,
		session,
	); err != nil {
		t.Fatalf("replica ValidateAuthenticated: %v", err)
	}

	differentSecretManager := newCSRFTestManagerWithSecret(
		t,
		csrfTestNow,
		csrfTestSecret(0x32),
		&fillReader{},
	)
	if err := differentSecretManager.ValidateAuthenticated(
		binding.CookieValue,
		binding.Token,
		session,
	); err != ErrCSRFInvalid {
		t.Errorf("different-secret error = %v, want exact ErrCSRFInvalid", err)
	}
}

func TestCSRFManagerCopiesSecret(t *testing.T) {
	t.Parallel()

	secret := csrfTestSecret(0x71)
	replicaSecret := append([]byte(nil), secret...)
	manager := newCSRFTestManagerWithSecret(
		t,
		csrfTestNow,
		secret,
		&fillReader{value: 0x72},
	)
	clear(secret)

	binding, err := manager.RotateAnonymous()
	if err != nil {
		t.Fatalf("RotateAnonymous: %v", err)
	}
	replica := newCSRFTestManagerWithSecret(
		t,
		csrfTestNow,
		replicaSecret,
		&fillReader{},
	)
	if err := replica.ValidateAnonymous(binding.CookieValue, binding.Token); err != nil {
		t.Fatalf("replica ValidateAnonymous after caller secret mutation: %v", err)
	}
}

func TestNewCSRFManagerRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	validClock := clock.Fixed{Time: csrfTestNow}
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
			secret: make([]byte, minimumCSRFSecretLength-1),
			clock:  validClock,
			random: &fillReader{},
		},
		{
			name:   "nil clock",
			secret: csrfTestSecret(1),
			random: &fillReader{},
		},
		{
			name:   "nil random",
			secret: csrfTestSecret(1),
			clock:  validClock,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manager, err := NewCSRFManagerFrom(test.secret, test.clock, test.random)
			if err != ErrInvalidCSRFConfiguration {
				t.Errorf("error = %v, want exact ErrInvalidCSRFConfiguration", err)
			}
			if manager != nil {
				t.Errorf("manager = %v, want nil", manager)
			}
		})
	}
}

func TestCSRFIssueOrReuseAnonymous(t *testing.T) {
	t.Parallel()

	random := &fillReader{value: 0x21}
	manager := newCSRFTestManager(t, csrfTestNow, random)
	issued, err := manager.RotateAnonymous()
	if err != nil {
		t.Fatalf("RotateAnonymous: %v", err)
	}
	bytesAfterIssue := random.bytesRead()

	reused, err := manager.IssueOrReuse(issued.CookieValue, nil)
	if err != nil {
		t.Fatalf("IssueOrReuse: %v", err)
	}
	if !reused.Reused {
		t.Error("Reused = false, want true")
	}
	if reused.CookieValue != issued.CookieValue || reused.Token != issued.Token {
		t.Errorf("reused binding = %+v, want original %+v", reused, issued)
	}
	if random.bytesRead() != bytesAfterIssue {
		t.Errorf("reuse consumed random bytes: got %d, want %d", random.bytesRead(), bytesAfterIssue)
	}
}

func TestCSRFIssueOrReuseRotatesInvalidAnonymousCookies(t *testing.T) {
	t.Parallel()

	now := csrfTestNow.UTC().Truncate(time.Second)
	tests := []struct {
		name   string
		cookie func(*CSRFManager) string
	}{
		{name: "missing", cookie: func(*CSRFManager) string { return "" }},
		{name: "malformed", cookie: func(*CSRFManager) string { return "malformed" }},
		{
			name: "tampered MAC",
			cookie: func(manager *CSRFManager) string {
				value := signedCSRFCookie(manager, 0x11, now.Add(time.Hour).Unix())
				return tamperCSRFCookieMAC(t, value)
			},
		},
		{
			name: "expired",
			cookie: func(manager *CSRFManager) string {
				return signedCSRFCookie(manager, 0x11, now.Unix())
			},
		},
		{
			name: "lifetime above one hour",
			cookie: func(manager *CSRFManager) string {
				return signedCSRFCookie(
					manager,
					0x11,
					now.Add(anonymousCSRFLifetime+time.Second).Unix(),
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			random := &fillReader{value: 0x22}
			manager := newCSRFTestManager(t, now, random)
			existing := test.cookie(manager)

			binding, err := manager.IssueOrReuse(existing, nil)
			if err != nil {
				t.Fatalf("IssueOrReuse: %v", err)
			}
			if binding.Reused {
				t.Error("Reused = true, want false")
			}
			if binding.CookieValue == existing && existing != "" {
				t.Error("rotated cookie equals invalid existing cookie")
			}
			if random.bytesRead() != csrfNonceByteLength {
				t.Errorf("random bytes = %d, want %d", random.bytesRead(), csrfNonceByteLength)
			}
			if err := manager.ValidateAnonymous(
				binding.CookieValue,
				binding.Token,
			); err != nil {
				t.Fatalf("ValidateAnonymous rotated binding: %v", err)
			}
		})
	}
}

func TestCSRFIssueOrReuseAuthenticatedRequiresExactSessionExpiration(t *testing.T) {
	t.Parallel()

	now := csrfTestNow.UTC().Truncate(time.Second)
	session := csrfTestSession(now, 0x31)
	tests := []struct {
		name       string
		expiration int64
		wantReuse  bool
	}{
		{
			name:       "exact session expiration",
			expiration: session.ExpiresAt.Unix(),
			wantReuse:  true,
		},
		{
			name:       "before session expiration",
			expiration: session.ExpiresAt.Add(-time.Second).Unix(),
		},
		{
			name:       "after session expiration",
			expiration: session.ExpiresAt.Add(time.Second).Unix(),
		},
		{
			name:       "anonymous expiration",
			expiration: now.Add(anonymousCSRFLifetime).Unix(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			random := &fillReader{value: 0x32}
			manager := newCSRFTestManager(t, now, random)
			existing := signedCSRFCookie(
				manager,
				0x33,
				test.expiration,
			)

			binding, err := manager.IssueOrReuse(existing, &session)
			if err != nil {
				t.Fatalf("IssueOrReuse: %v", err)
			}
			if binding.Reused != test.wantReuse {
				t.Errorf("Reused = %v, want %v", binding.Reused, test.wantReuse)
			}
			if test.wantReuse {
				if binding.CookieValue != existing {
					t.Error("reused cookie changed")
				}
				if random.bytesRead() != 0 {
					t.Errorf("reuse consumed %d random bytes", random.bytesRead())
				}
			} else {
				if binding.CookieValue == existing {
					t.Error("rotated cookie did not change")
				}
				if random.bytesRead() != csrfNonceByteLength {
					t.Errorf(
						"rotation consumed %d random bytes, want %d",
						random.bytesRead(),
						csrfNonceByteLength,
					)
				}
			}
			if !binding.ExpiresAt.Equal(session.ExpiresAt) {
				t.Errorf(
					"ExpiresAt = %v, want %v",
					binding.ExpiresAt,
					session.ExpiresAt,
				)
			}
			if err := manager.ValidateAuthenticated(
				binding.CookieValue,
				binding.Token,
				session,
			); err != nil {
				t.Fatalf("ValidateAuthenticated: %v", err)
			}
		})
	}
}

func TestCSRFRebindsReusableCookieTokenToExactSession(t *testing.T) {
	t.Parallel()

	manager := newCSRFTestManager(t, csrfTestNow, &fillReader{value: 0x44})
	firstSession := csrfTestSession(csrfTestNow, 0x45)
	secondSession := csrfTestSession(csrfTestNow, 0x46)
	first, err := manager.RotateAuthenticated(firstSession)
	if err != nil {
		t.Fatalf("RotateAuthenticated first: %v", err)
	}

	rebound, err := manager.IssueOrReuse(first.CookieValue, &secondSession)
	if err != nil {
		t.Fatalf("IssueOrReuse second: %v", err)
	}
	if !rebound.Reused || rebound.CookieValue != first.CookieValue {
		t.Errorf("rebound binding did not reuse cookie: %+v", rebound)
	}
	if rebound.Token == first.Token {
		t.Error("token did not change for a different session JWT ID")
	}
	if err := manager.ValidateAuthenticated(
		first.CookieValue,
		first.Token,
		secondSession,
	); err != ErrCSRFInvalid {
		t.Errorf("old cross-session token error = %v, want ErrCSRFInvalid", err)
	}
	if err := manager.ValidateAuthenticated(
		rebound.CookieValue,
		rebound.Token,
		secondSession,
	); err != nil {
		t.Fatalf("ValidateAuthenticated rebound: %v", err)
	}
}

func TestCSRFRotationChangesNonceAndBinding(t *testing.T) {
	t.Parallel()

	randomBytes := append(
		bytes.Repeat([]byte{0x51}, csrfNonceByteLength),
		bytes.Repeat([]byte{0x52}, csrfNonceByteLength)...,
	)
	manager := newCSRFTestManager(t, csrfTestNow, bytes.NewReader(randomBytes))

	anonymous, err := manager.RotateAnonymous()
	if err != nil {
		t.Fatalf("RotateAnonymous: %v", err)
	}
	session := csrfTestSession(csrfTestNow, 0x53)
	authenticated, err := manager.RotateAuthenticated(session)
	if err != nil {
		t.Fatalf("RotateAuthenticated: %v", err)
	}
	if anonymous.CookieValue == authenticated.CookieValue {
		t.Error("rotation reused the same cookie")
	}
	if anonymous.Token == authenticated.Token {
		t.Error("rotation reused the same token")
	}
	if err := manager.ValidateAuthenticated(
		anonymous.CookieValue,
		anonymous.Token,
		session,
	); err != ErrCSRFInvalid {
		t.Errorf("anonymous token authenticated error = %v, want ErrCSRFInvalid", err)
	}
	if err := manager.ValidateAuthenticated(
		authenticated.CookieValue,
		authenticated.Token,
		session,
	); err != nil {
		t.Fatalf("ValidateAuthenticated rotated binding: %v", err)
	}
}

func TestCSRFLogoutRotationProducesAnonymousBinding(t *testing.T) {
	t.Parallel()

	randomBytes := append(
		bytes.Repeat([]byte{0x61}, csrfNonceByteLength),
		bytes.Repeat([]byte{0x62}, csrfNonceByteLength)...,
	)
	manager := newCSRFTestManager(t, csrfTestNow, bytes.NewReader(randomBytes))
	session := csrfTestSession(csrfTestNow, 0x63)

	authenticated, err := manager.RotateAuthenticated(session)
	if err != nil {
		t.Fatalf("RotateAuthenticated: %v", err)
	}
	if err := manager.ValidateAuthenticated(
		authenticated.CookieValue,
		authenticated.Token,
		session,
	); err != nil {
		t.Fatalf("pre-logout validation: %v", err)
	}

	anonymous, err := manager.RotateAnonymous()
	if err != nil {
		t.Fatalf("RotateAnonymous: %v", err)
	}
	if !anonymous.ExpiresAt.Equal(
		csrfTestNow.UTC().Truncate(time.Second).Add(anonymousCSRFLifetime),
	) {
		t.Errorf("anonymous expiration = %v", anonymous.ExpiresAt)
	}
	if err := manager.ValidateAnonymous(
		anonymous.CookieValue,
		anonymous.Token,
	); err != nil {
		t.Fatalf("post-logout anonymous validation: %v", err)
	}
	if err := manager.ValidateAnonymous(
		authenticated.CookieValue,
		authenticated.Token,
	); err != ErrCSRFInvalid {
		t.Errorf("old authenticated binding error = %v, want ErrCSRFInvalid", err)
	}
}

func TestCSRFValidationDistinguishesRequiredFromInvalid(t *testing.T) {
	t.Parallel()

	manager := newCSRFTestManager(t, csrfTestNow, &fillReader{value: 0x71})
	session := csrfTestSession(csrfTestNow, 0x72)
	binding, err := manager.RotateAuthenticated(session)
	if err != nil {
		t.Fatalf("RotateAuthenticated: %v", err)
	}

	requiredTests := []struct {
		name   string
		cookie string
		token  string
	}{
		{name: "both missing"},
		{name: "cookie missing", token: binding.Token},
		{name: "token missing", cookie: binding.CookieValue},
	}
	for _, test := range requiredTests {
		t.Run(test.name, func(t *testing.T) {
			if err := manager.ValidateAnonymous(
				test.cookie,
				test.token,
			); err != ErrCSRFRequired {
				t.Errorf("anonymous error = %v, want exact ErrCSRFRequired", err)
			}
			if err := manager.ValidateAuthenticated(
				test.cookie,
				test.token,
				Session{},
			); err != ErrCSRFRequired {
				t.Errorf("authenticated error = %v, want exact ErrCSRFRequired", err)
			}
		})
	}

	if err := manager.ValidateAnonymous(" ", " "); err != ErrCSRFInvalid {
		t.Errorf("malformed non-empty error = %v, want exact ErrCSRFInvalid", err)
	}
	if err := manager.ValidateAuthenticated(
		" ",
		" ",
		session,
	); err != ErrCSRFInvalid {
		t.Errorf("malformed authenticated error = %v, want exact ErrCSRFInvalid", err)
	}
}

func TestCSRFRejectsMalformedAndTamperedCookies(t *testing.T) {
	t.Parallel()

	manager := newCSRFTestManager(t, csrfTestNow, &fillReader{})
	now := csrfTestNow.UTC().Truncate(time.Second)
	validCookie := signedCSRFCookie(manager, 0x21, now.Add(time.Hour).Unix())
	validToken := signedCSRFToken(manager, validCookie, anonymousCSRFSubject)
	parts := strings.Split(validCookie, ".")

	noncanonicalNonce := parts[0][:len(parts[0])-1] + "B"
	noncanonicalMAC := parts[2][:len(parts[2])-1] + "B"
	tests := []struct {
		name   string
		cookie string
	}{
		{name: "whitespace", cookie: " "},
		{name: "oversized", cookie: strings.Repeat("x", 512)},
		{name: "missing field", cookie: parts[0] + "." + parts[1]},
		{name: "extra field", cookie: validCookie + ".extra"},
		{name: "empty nonce", cookie: "." + parts[1] + "." + parts[2]},
		{name: "short nonce", cookie: parts[0][:len(parts[0])-1] + "." + parts[1] + "." + parts[2]},
		{name: "long nonce", cookie: parts[0] + "A." + parts[1] + "." + parts[2]},
		{name: "padded nonce", cookie: parts[0] + "=." + parts[1] + "." + parts[2]},
		{name: "invalid nonce alphabet", cookie: strings.Repeat("/", len(parts[0])) + "." + parts[1] + "." + parts[2]},
		{name: "noncanonical nonce bits", cookie: noncanonicalNonce + "." + parts[1] + "." + parts[2]},
		{name: "empty expiration", cookie: parts[0] + ".." + parts[2]},
		{name: "zero expiration", cookie: parts[0] + ".0." + parts[2]},
		{name: "negative expiration", cookie: parts[0] + ".-1." + parts[2]},
		{name: "positive sign expiration", cookie: parts[0] + ".+1." + parts[2]},
		{name: "leading zero expiration", cookie: parts[0] + ".0" + parts[1] + "." + parts[2]},
		{name: "expiration overflow", cookie: parts[0] + ".9223372036854775808." + parts[2]},
		{name: "expiration whitespace", cookie: parts[0] + ". " + parts[1] + "." + parts[2]},
		{name: "empty MAC", cookie: parts[0] + "." + parts[1] + "."},
		{name: "short MAC", cookie: parts[0] + "." + parts[1] + "." + parts[2][:len(parts[2])-1]},
		{name: "long MAC", cookie: parts[0] + "." + parts[1] + "." + parts[2] + "A"},
		{name: "padded MAC", cookie: parts[0] + "." + parts[1] + "." + parts[2] + "="},
		{name: "invalid MAC alphabet", cookie: parts[0] + "." + parts[1] + "." + strings.Repeat("/", len(parts[2]))},
		{name: "noncanonical MAC bits", cookie: parts[0] + "." + parts[1] + "." + noncanonicalMAC},
		{name: "tampered canonical nonce", cookie: tamperCSRFCookieNonce(t, validCookie)},
		{name: "tampered canonical MAC", cookie: tamperCSRFCookieMAC(t, validCookie)},
		{name: "tampered expiration", cookie: parts[0] + "." + strings.TrimSuffix(parts[1], "0") + "1." + parts[2]},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := manager.ValidateAnonymous(
				test.cookie,
				validToken,
			); err != ErrCSRFInvalid {
				t.Errorf("error = %v, want exact ErrCSRFInvalid", err)
			}
		})
	}
}

func TestCSRFRejectsMalformedAndTamperedTokens(t *testing.T) {
	t.Parallel()

	manager := newCSRFTestManager(t, csrfTestNow, &fillReader{value: 0x31})
	binding, err := manager.RotateAnonymous()
	if err != nil {
		t.Fatalf("RotateAnonymous: %v", err)
	}
	parts := strings.Split(binding.Token, ".")
	noncanonicalMAC := parts[1][:len(parts[1])-1] + "B"
	tests := []struct {
		name  string
		token string
	}{
		{name: "whitespace", token: " "},
		{name: "oversized", token: strings.Repeat("x", 1<<20)},
		{name: "missing version", token: parts[1]},
		{name: "wrong version", token: "v2." + parts[1]},
		{name: "version case", token: "V1." + parts[1]},
		{name: "extra field", token: binding.Token + ".extra"},
		{name: "empty MAC", token: csrfTokenVersion + "."},
		{name: "short MAC", token: csrfTokenVersion + "." + parts[1][:len(parts[1])-1]},
		{name: "long MAC", token: csrfTokenVersion + "." + parts[1] + "A"},
		{name: "padded MAC", token: csrfTokenVersion + "." + parts[1] + "="},
		{name: "invalid MAC alphabet", token: csrfTokenVersion + "." + strings.Repeat("/", len(parts[1]))},
		{name: "noncanonical MAC bits", token: csrfTokenVersion + "." + noncanonicalMAC},
		{name: "tampered canonical MAC", token: tamperCSRFTokenMAC(t, binding.Token)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := manager.ValidateAnonymous(
				binding.CookieValue,
				test.token,
			); err != ErrCSRFInvalid {
				t.Errorf("error = %v, want exact ErrCSRFInvalid", err)
			}
		})
	}
}

func TestCSRFRejectsExpiredAndOverlongBindings(t *testing.T) {
	t.Parallel()

	now := csrfTestNow.UTC().Truncate(time.Second)
	manager := newCSRFTestManager(t, now, &fillReader{})
	tests := []struct {
		name       string
		expiration int64
	}{
		{name: "past", expiration: now.Add(-time.Second).Unix()},
		{name: "exactly now", expiration: now.Unix()},
		{
			name:       "anonymous lifetime one second too long",
			expiration: now.Add(anonymousCSRFLifetime + time.Second).Unix(),
		},
		{
			name:       "maximum integer expiration",
			expiration: math.MaxInt64,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cookieValue := signedCSRFCookie(
				manager,
				0x41,
				test.expiration,
			)
			token := signedCSRFToken(
				manager,
				cookieValue,
				anonymousCSRFSubject,
			)
			if err := manager.ValidateAnonymous(
				cookieValue,
				token,
			); err != ErrCSRFInvalid {
				t.Errorf("error = %v, want exact ErrCSRFInvalid", err)
			}
		})
	}
}

func TestCSRFRejectsWrongCookieAndSessionBindings(t *testing.T) {
	t.Parallel()

	randomBytes := append(
		bytes.Repeat([]byte{0x51}, csrfNonceByteLength),
		bytes.Repeat([]byte{0x52}, csrfNonceByteLength)...,
	)
	manager := newCSRFTestManager(t, csrfTestNow, bytes.NewReader(randomBytes))
	firstAnonymous, err := manager.RotateAnonymous()
	if err != nil {
		t.Fatalf("RotateAnonymous first: %v", err)
	}
	secondAnonymous, err := manager.RotateAnonymous()
	if err != nil {
		t.Fatalf("RotateAnonymous second: %v", err)
	}
	if err := manager.ValidateAnonymous(
		secondAnonymous.CookieValue,
		firstAnonymous.Token,
	); err != ErrCSRFInvalid {
		t.Errorf("wrong-cookie error = %v, want ErrCSRFInvalid", err)
	}

	firstSession := csrfTestSession(csrfTestNow, 0x53)
	secondSession := csrfTestSession(csrfTestNow, 0x54)
	authCookie := signedCSRFCookie(
		manager,
		0x55,
		firstSession.ExpiresAt.Unix(),
	)
	firstToken := signedCSRFToken(manager, authCookie, firstSession.JWTID)
	if err := manager.ValidateAuthenticated(
		authCookie,
		firstToken,
		secondSession,
	); err != ErrCSRFInvalid {
		t.Errorf("cross-session error = %v, want ErrCSRFInvalid", err)
	}
	if err := manager.ValidateAnonymous(
		authCookie,
		firstToken,
	); err != ErrCSRFInvalid {
		t.Errorf("authenticated-as-anonymous error = %v, want ErrCSRFInvalid", err)
	}

	anonymousToken := signedCSRFToken(
		manager,
		authCookie,
		anonymousCSRFSubject,
	)
	if err := manager.ValidateAuthenticated(
		authCookie,
		anonymousToken,
		firstSession,
	); err != ErrCSRFInvalid {
		t.Errorf("anonymous-as-authenticated error = %v, want ErrCSRFInvalid", err)
	}
}

func TestCSRFAuthenticatedValidationRequiresExactCookieExpiration(t *testing.T) {
	t.Parallel()

	manager := newCSRFTestManager(t, csrfTestNow, &fillReader{})
	session := csrfTestSession(csrfTestNow, 0x61)
	for _, expiration := range []int64{
		session.ExpiresAt.Add(-time.Second).Unix(),
		session.ExpiresAt.Add(time.Second).Unix(),
	} {
		cookieValue := signedCSRFCookie(manager, 0x62, expiration)
		token := signedCSRFToken(manager, cookieValue, session.JWTID)
		if err := manager.ValidateAuthenticated(
			cookieValue,
			token,
			session,
		); err != ErrCSRFInvalid {
			t.Errorf(
				"expiration %d error = %v, want ErrCSRFInvalid",
				expiration,
				err,
			)
		}
	}
}

func TestCSRFRejectsInvalidTrustedSessions(t *testing.T) {
	t.Parallel()

	now := csrfTestNow.UTC().Truncate(time.Second)
	validSession := csrfTestSession(now, 0x71)
	tests := []struct {
		name   string
		mutate func(*Session)
	}{
		{
			name: "missing JWT ID",
			mutate: func(session *Session) {
				session.JWTID = ""
			},
		},
		{
			name: "short JWT ID",
			mutate: func(session *Session) {
				session.JWTID = base64.RawURLEncoding.EncodeToString(make([]byte, 31))
			},
		},
		{
			name: "padded JWT ID",
			mutate: func(session *Session) {
				session.JWTID += "="
			},
		},
		{
			name: "zero issued at",
			mutate: func(session *Session) {
				session.IssuedAt = time.Time{}
			},
		},
		{
			name: "fractional issued at",
			mutate: func(session *Session) {
				session.IssuedAt = session.IssuedAt.Add(time.Nanosecond)
			},
		},
		{
			name: "issued beyond skew",
			mutate: func(session *Session) {
				session.IssuedAt = now.Add(SessionClockSkew + time.Second)
				session.ExpiresAt = session.IssuedAt.Add(SessionLifetime)
			},
		},
		{
			name: "zero expiration",
			mutate: func(session *Session) {
				session.ExpiresAt = time.Time{}
			},
		},
		{
			name: "fractional expiration",
			mutate: func(session *Session) {
				session.ExpiresAt = session.ExpiresAt.Add(time.Nanosecond)
			},
		},
		{
			name: "expired",
			mutate: func(session *Session) {
				session.ExpiresAt = now
			},
		},
		{
			name: "expiration before issued at",
			mutate: func(session *Session) {
				session.ExpiresAt = session.IssuedAt.Add(-time.Second)
			},
		},
		{
			name: "lifetime above limit",
			mutate: func(session *Session) {
				session.ExpiresAt = session.IssuedAt.Add(
					SessionLifetime + SessionClockSkew + time.Second,
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			session := validSession
			test.mutate(&session)
			manager := newCSRFTestManager(t, now, &fillReader{value: 0x72})

			if _, err := manager.RotateAuthenticated(
				session,
			); err != ErrInvalidCSRFConfiguration {
				t.Errorf("RotateAuthenticated error = %v, want configuration error", err)
			}
			if _, err := manager.IssueOrReuse(
				"",
				&session,
			); err != ErrInvalidCSRFConfiguration {
				t.Errorf("IssueOrReuse error = %v, want configuration error", err)
			}
			if err := manager.ValidateAuthenticated(
				"present",
				"present",
				session,
			); err != ErrCSRFInvalid {
				t.Errorf("ValidateAuthenticated error = %v, want ErrCSRFInvalid", err)
			}
		})
	}
}

func TestCSRFAllowsMaximumSessionLifetimeAndClockSkew(t *testing.T) {
	t.Parallel()

	now := csrfTestNow.UTC().Truncate(time.Second)
	session := csrfTestSession(now, 0x73)
	session.IssuedAt = now.Add(SessionClockSkew)
	session.ExpiresAt = session.IssuedAt.Add(SessionLifetime)
	manager := newCSRFTestManager(t, now, &fillReader{value: 0x74})

	binding, err := manager.RotateAuthenticated(session)
	if err != nil {
		t.Fatalf("RotateAuthenticated: %v", err)
	}
	if !binding.ExpiresAt.Equal(session.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", binding.ExpiresAt, session.ExpiresAt)
	}
	if binding.MaxAgeSeconds != int(
		(SessionLifetime+SessionClockSkew)/time.Second,
	) {
		t.Errorf("MaxAgeSeconds = %d", binding.MaxAgeSeconds)
	}
}

func TestCSRFRandomSourceFailures(t *testing.T) {
	t.Parallel()

	sourceError := errors.New("entropy unavailable")
	manager := newCSRFTestManager(
		t,
		csrfTestNow,
		failingReader{err: sourceError},
	)
	session := csrfTestSession(csrfTestNow, 0x81)
	tests := []struct {
		name  string
		issue func() (CSRFBinding, error)
	}{
		{name: "anonymous rotation", issue: manager.RotateAnonymous},
		{
			name: "authenticated rotation",
			issue: func() (CSRFBinding, error) {
				return manager.RotateAuthenticated(session)
			},
		},
		{
			name: "GET replacement",
			issue: func() (CSRFBinding, error) {
				return manager.IssueOrReuse("invalid", nil)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding, err := test.issue()
			if !errors.Is(err, sourceError) {
				t.Errorf("error = %v, want wrapped %v", err, sourceError)
			}
			if binding != (CSRFBinding{}) {
				t.Errorf("binding = %+v, want zero", binding)
			}
		})
	}

	shortSourceManager := newCSRFTestManager(
		t,
		csrfTestNow,
		bytes.NewReader(make([]byte, csrfNonceByteLength-1)),
	)
	if _, err := shortSourceManager.RotateAnonymous(); !errors.Is(
		err,
		io.ErrUnexpectedEOF,
	) {
		t.Errorf("short source error = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestCSRFValidReuseDoesNotReadFailingRandomSource(t *testing.T) {
	t.Parallel()

	issuer := newCSRFTestManager(t, csrfTestNow, &fillReader{value: 0x91})
	binding, err := issuer.RotateAnonymous()
	if err != nil {
		t.Fatalf("RotateAnonymous: %v", err)
	}
	reuser := newCSRFTestManager(
		t,
		csrfTestNow,
		failingReader{err: errors.New("must not read")},
	)
	reused, err := reuser.IssueOrReuse(binding.CookieValue, nil)
	if err != nil {
		t.Fatalf("IssueOrReuse: %v", err)
	}
	if !reused.Reused || reused.CookieValue != binding.CookieValue {
		t.Errorf("reused binding = %+v", reused)
	}
}

func TestCSRFLengthPrefixesSeparateAmbiguousFields(t *testing.T) {
	t.Parallel()

	manager := newCSRFTestManager(t, csrfTestNow, &fillReader{})
	first := manager.tokenMAC("ab", "c")
	second := manager.tokenMAC("a", "bc")
	defer clear(first)
	defer clear(second)
	if bytes.Equal(first, second) {
		t.Error("length-prefixed token fields produced the same MAC")
	}
}

func TestCSRFConcurrentIssuanceAndValidation(t *testing.T) {
	t.Parallel()

	manager, err := NewCSRFManager(
		csrfTestSecret(0xa1),
		clock.Fixed{Time: csrfTestNow},
	)
	if err != nil {
		t.Fatalf("NewCSRFManager: %v", err)
	}

	const workers = 16
	type result struct {
		binding CSRFBinding
		err     error
	}
	results := make(chan result, workers)
	for range workers {
		go func() {
			binding, issueErr := manager.RotateAnonymous()
			if issueErr == nil {
				issueErr = manager.ValidateAnonymous(
					binding.CookieValue,
					binding.Token,
				)
			}
			results <- result{binding: binding, err: issueErr}
		}()
	}

	cookies := make(map[string]struct{}, workers)
	for range workers {
		result := <-results
		if result.err != nil {
			t.Errorf("concurrent issuance/validation: %v", result.err)
			continue
		}
		cookies[result.binding.CookieValue] = struct{}{}
	}
	if len(cookies) != workers {
		t.Errorf("unique cookies = %d, want %d", len(cookies), workers)
	}
}

func TestCSRFCookiesSetExactAttributes(t *testing.T) {
	t.Parallel()

	cookies := NewCSRFCookies(
		"api.example.com",
		true,
		http.SameSiteNoneMode,
	)
	binding := CSRFBinding{
		CookieValue:   "binding.value.mac",
		ExpiresAt:     csrfTestNow.UTC().Truncate(time.Second).Add(time.Hour),
		MaxAgeSeconds: 3600,
	}
	response := httptest.NewRecorder()
	cookies.Set(response, binding)

	cookie := singleResponseCookie(t, response)
	assertCSRFCookieCommonAttributes(
		t,
		cookie,
		"api.example.com",
		true,
		http.SameSiteNoneMode,
	)
	if cookie.Value != binding.CookieValue {
		t.Errorf("Value = %q, want %q", cookie.Value, binding.CookieValue)
	}
	if cookie.MaxAge != binding.MaxAgeSeconds {
		t.Errorf("MaxAge = %d, want %d", cookie.MaxAge, binding.MaxAgeSeconds)
	}
	if !cookie.Expires.Equal(binding.ExpiresAt) {
		t.Errorf("Expires = %v, want %v", cookie.Expires, binding.ExpiresAt)
	}
	header := response.Header().Get("Set-Cookie")
	for _, fragment := range []string{
		"Max-Age=3600",
		"HttpOnly",
		"Secure",
		"SameSite=None",
	} {
		if !strings.Contains(header, fragment) {
			t.Errorf("Set-Cookie missing %q: %s", fragment, header)
		}
	}
}

func TestCSRFCookiesClearMatchesSetAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		domain   string
		secure   bool
		sameSite http.SameSite
	}{
		{
			name:     "production cross-site",
			domain:   "api.example.com",
			secure:   true,
			sameSite: http.SameSiteNoneMode,
		},
		{
			name:     "local host-only",
			secure:   false,
			sameSite: http.SameSiteLaxMode,
		},
		{
			name:     "strict",
			domain:   "localhost",
			secure:   true,
			sameSite: http.SameSiteStrictMode,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cookies := NewCSRFCookies(test.domain, test.secure, test.sameSite)
			response := httptest.NewRecorder()
			cookies.Clear(response)
			cookie := singleResponseCookie(t, response)
			assertCSRFCookieCommonAttributes(
				t,
				cookie,
				test.domain,
				test.secure,
				test.sameSite,
			)
			if cookie.Value != "" {
				t.Errorf("Value = %q, want empty", cookie.Value)
			}
			if cookie.MaxAge >= 0 {
				t.Errorf("MaxAge = %d, want negative", cookie.MaxAge)
			}
			if !cookie.Expires.Equal(clearedSessionExpiration) {
				t.Errorf(
					"Expires = %v, want %v",
					cookie.Expires,
					clearedSessionExpiration,
				)
			}
			if !cookie.Expires.Before(time.Now()) {
				t.Errorf("Expires = %v, want past", cookie.Expires)
			}

			header := response.Header().Get("Set-Cookie")
			if !strings.Contains(header, "Max-Age=0") {
				t.Errorf("clear Set-Cookie missing Max-Age=0: %s", header)
			}
			if test.domain == "" && strings.Contains(header, "Domain=") {
				t.Errorf("host-only cookie unexpectedly has Domain: %s", header)
			}
		})
	}
}

func newCSRFTestManager(
	t *testing.T,
	now time.Time,
	random io.Reader,
) *CSRFManager {
	t.Helper()
	return newCSRFTestManagerWithSecret(
		t,
		now,
		csrfTestSecret(0x11),
		random,
	)
}

func newCSRFTestManagerWithSecret(
	t *testing.T,
	now time.Time,
	secret []byte,
	random io.Reader,
) *CSRFManager {
	t.Helper()

	manager, err := NewCSRFManagerFrom(
		secret,
		clock.Fixed{Time: now},
		random,
	)
	if err != nil {
		t.Fatalf("NewCSRFManagerFrom: %v", err)
	}
	return manager
}

func csrfTestSecret(value byte) []byte {
	return bytes.Repeat([]byte{value}, minimumCSRFSecretLength)
}

func csrfTestSession(now time.Time, jwtIDByte byte) Session {
	now = now.UTC().Truncate(time.Second)
	return Session{
		UserID: testUserID,
		JWTID: base64.RawURLEncoding.EncodeToString(
			bytes.Repeat([]byte{jwtIDByte}, sessionJWTIDByteLength),
		),
		IssuedAt:  now,
		ExpiresAt: now.Add(SessionLifetime),
	}
}

func signedCSRFCookie(
	manager *CSRFManager,
	nonceByte byte,
	expirationUnix int64,
) string {
	nonce := bytes.Repeat([]byte{nonceByte}, csrfNonceByteLength)
	expirationText := strconv.FormatInt(expirationUnix, 10)
	mac := manager.cookieMAC(nonce, expirationText)
	defer clear(mac)
	return strings.Join(
		[]string{
			base64.RawURLEncoding.EncodeToString(nonce),
			expirationText,
			base64.RawURLEncoding.EncodeToString(mac),
		},
		".",
	)
}

func signedCSRFToken(
	manager *CSRFManager,
	cookieValue string,
	subject string,
) string {
	mac := manager.tokenMAC(cookieValue, subject)
	defer clear(mac)
	return csrfTokenVersion + "." + base64.RawURLEncoding.EncodeToString(mac)
}

func tamperCSRFCookieNonce(t *testing.T, value string) string {
	t.Helper()
	parts := strings.Split(value, ".")
	parts[0] = tamperCanonicalRawURL(t, parts[0])
	return strings.Join(parts, ".")
}

func tamperCSRFCookieMAC(t *testing.T, value string) string {
	t.Helper()
	parts := strings.Split(value, ".")
	parts[2] = tamperCanonicalRawURL(t, parts[2])
	return strings.Join(parts, ".")
}

func tamperCSRFTokenMAC(t *testing.T, value string) string {
	t.Helper()
	parts := strings.Split(value, ".")
	parts[1] = tamperCanonicalRawURL(t, parts[1])
	return strings.Join(parts, ".")
}

func tamperCanonicalRawURL(t *testing.T, value string) string {
	t.Helper()
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		t.Fatalf("decode canonical Base64URL: %v", err)
	}
	decoded[0] ^= 0x01
	return base64.RawURLEncoding.EncodeToString(decoded)
}

func clearParsedCSRFCookie(parsed parsedCSRFCookie) {
	clear(parsed.nonce)
	clear(parsed.mac)
}

func assertCSRFCookieCommonAttributes(
	t *testing.T,
	cookie *http.Cookie,
	domain string,
	secure bool,
	sameSite http.SameSite,
) {
	t.Helper()

	if cookie.Name != CSRFCookieName {
		t.Errorf("Name = %q, want %q", cookie.Name, CSRFCookieName)
	}
	if cookie.Path != sessionCookiePath {
		t.Errorf("Path = %q, want %q", cookie.Path, sessionCookiePath)
	}
	if cookie.Domain != domain {
		t.Errorf("Domain = %q, want %q", cookie.Domain, domain)
	}
	if !cookie.HttpOnly {
		t.Error("HttpOnly = false, want true")
	}
	if cookie.Secure != secure {
		t.Errorf("Secure = %v, want %v", cookie.Secure, secure)
	}
	if cookie.SameSite != sameSite {
		t.Errorf("SameSite = %v, want %v", cookie.SameSite, sameSite)
	}
}
