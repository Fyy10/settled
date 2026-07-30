package auth

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPasswordHasherHashAndVerify(t *testing.T) {
	randomBytes := make([]byte, 0, passwordHashSaltLength*3)
	randomBytes = append(randomBytes, bytes.Repeat([]byte{0}, passwordHashSaltLength)...)
	randomBytes = append(randomBytes, bytes.Repeat([]byte{1}, passwordHashSaltLength)...)
	randomBytes = append(randomBytes, bytes.Repeat([]byte{2}, passwordHashSaltLength)...)
	hasher, err := NewPasswordHasherFrom(bytes.NewReader(randomBytes))
	if err != nil {
		t.Fatalf("NewPasswordHasherFrom: %v", err)
	}

	const password = "correct horse battery staple"
	encodedHash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(
		encodedHash,
		"$argon2id$v=19$m=65536,t=3,p=4$",
	) {
		t.Errorf("hash has unexpected PHC prefix: %q", encodedHash)
	}
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		t.Fatalf("PHC field count = %d, want 6", len(parts))
	}
	if strings.Contains(parts[4], "=") || strings.Contains(parts[5], "=") {
		t.Errorf("PHC uses padded Base64: %q", encodedHash)
	}

	parsed, err := parsePasswordHash(encodedHash)
	if err != nil {
		t.Fatalf("parsePasswordHash: %v", err)
	}
	if !bytes.Equal(parsed.salt, bytes.Repeat([]byte{1}, passwordHashSaltLength)) {
		t.Errorf("salt = %x, want deterministic salt bytes", parsed.salt)
	}
	if len(parsed.key) != int(passwordHashKeyLength) {
		t.Errorf("key length = %d, want %d", len(parsed.key), passwordHashKeyLength)
	}

	matched, err := hasher.Verify(password, encodedHash)
	if err != nil {
		t.Fatalf("Verify correct password: %v", err)
	}
	if !matched {
		t.Error("correct password did not match")
	}

	matched, err = hasher.Verify("wrong password", encodedHash)
	if err != nil {
		t.Fatalf("Verify incorrect password: %v", err)
	}
	if matched {
		t.Error("incorrect password matched")
	}

	secondHash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash second password: %v", err)
	}
	if secondHash == encodedHash {
		t.Error("two salts produced identical encoded hashes")
	}
}

func TestPasswordHasherCreatesOneProcessDummyHash(t *testing.T) {
	random := &fillReader{value: 0x5a}
	hasher, err := NewPasswordHasherFrom(random)
	if err != nil {
		t.Fatalf("NewPasswordHasherFrom: %v", err)
	}

	if random.bytesRead() != passwordHashSaltLength {
		t.Errorf(
			"constructor random bytes = %d, want %d",
			random.bytesRead(),
			passwordHashSaltLength,
		)
	}
	if hasher.dummyHash == "" {
		t.Fatal("dummy hash is empty")
	}

	if err := hasher.VerifyDummy("password for unknown email"); err != nil {
		t.Fatalf("VerifyDummy: %v", err)
	}
	if random.bytesRead() != passwordHashSaltLength {
		t.Errorf("VerifyDummy generated another dummy salt; bytes read = %d", random.bytesRead())
	}
}

func TestPasswordHasherRandomSourceFailuresAreSafe(t *testing.T) {
	if hasher, err := NewPasswordHasherFrom(nil); !errors.Is(
		err,
		ErrInvalidPasswordRandomSource,
	) || hasher != nil {
		t.Errorf("nil source result = (%v, %v)", hasher, err)
	}

	sourceError := errors.New("entropy unavailable")
	if hasher, err := NewPasswordHasherFrom(failingReader{err: sourceError}); !errors.Is(
		err,
		sourceError,
	) || hasher != nil {
		t.Errorf("failing source result = (%v, %v)", hasher, err)
	}

	hasher, err := NewPasswordHasherFrom(
		bytes.NewReader(make([]byte, passwordHashSaltLength)),
	)
	if err != nil {
		t.Fatalf("NewPasswordHasherFrom limited source: %v", err)
	}
	const password = "private password value"
	encodedHash, err := hasher.Hash(password)
	if !errors.Is(err, io.EOF) {
		t.Errorf("Hash error = %v, want wrapped EOF", err)
	}
	if encodedHash != "" {
		t.Errorf("Hash = %q, want empty", encodedHash)
	}
	if strings.Contains(err.Error(), password) {
		t.Errorf("Hash error exposes password: %v", err)
	}
}

func TestParsePasswordHashRejectsMalformedAndHostileValues(t *testing.T) {
	salt := base64.RawStdEncoding.EncodeToString(make([]byte, passwordHashSaltLength))
	key := base64.RawStdEncoding.EncodeToString(make([]byte, passwordHashKeyLength))
	valid := "$argon2id$v=19$m=65536,t=3,p=4$" + salt + "$" + key

	tests := []struct {
		name  string
		value string
	}{
		{name: "empty"},
		{name: "oversized", value: strings.Repeat("x", maxPHCLength+1)},
		{name: "missing leading delimiter", value: strings.TrimPrefix(valid, "$")},
		{name: "missing field", value: strings.TrimSuffix(valid, "$"+key)},
		{name: "extra field", value: valid + "$extra"},
		{name: "leading whitespace", value: " " + valid},
		{name: "trailing whitespace", value: valid + " "},
		{name: "wrong algorithm", value: strings.Replace(valid, "argon2id", "argon2i", 1)},
		{name: "algorithm case", value: strings.Replace(valid, "argon2id", "Argon2id", 1)},
		{name: "missing version", value: strings.Replace(valid, "v=19", "19", 1)},
		{name: "wrong version", value: strings.Replace(valid, "v=19", "v=18", 1)},
		{name: "noncanonical version", value: strings.Replace(valid, "v=19", "v=019", 1)},
		{name: "parameters reordered", value: strings.Replace(valid, "m=65536,t=3,p=4", "t=3,m=65536,p=4", 1)},
		{name: "missing parameter", value: strings.Replace(valid, "m=65536,t=3,p=4", "m=65536,t=3", 1)},
		{name: "extra parameter", value: strings.Replace(valid, "m=65536,t=3,p=4", "m=65536,t=3,p=4,x=1", 1)},
		{name: "duplicate parameter", value: strings.Replace(valid, "m=65536,t=3,p=4", "m=65536,t=3,t=3", 1)},
		{name: "wrong parameter key", value: strings.Replace(valid, "m=65536", "memory=65536", 1)},
		{name: "memory zero", value: strings.Replace(valid, "m=65536", "m=0", 1)},
		{name: "memory below bound", value: strings.Replace(valid, "m=65536", "m=65535", 1)},
		{name: "memory above bound", value: strings.Replace(valid, "m=65536", "m=65537", 1)},
		{name: "memory hostile", value: strings.Replace(valid, "m=65536", "m=4294967295", 1)},
		{name: "memory overflow", value: strings.Replace(valid, "m=65536", "m=4294967296", 1)},
		{name: "memory leading zero", value: strings.Replace(valid, "m=65536", "m=065536", 1)},
		{name: "memory sign", value: strings.Replace(valid, "m=65536", "m=+65536", 1)},
		{name: "iterations zero", value: strings.Replace(valid, "t=3", "t=0", 1)},
		{name: "iterations below bound", value: strings.Replace(valid, "t=3", "t=2", 1)},
		{name: "iterations above bound", value: strings.Replace(valid, "t=3", "t=4", 1)},
		{name: "iterations hostile", value: strings.Replace(valid, "t=3", "t=4294967295", 1)},
		{name: "parallelism zero", value: strings.Replace(valid, "p=4", "p=0", 1)},
		{name: "parallelism below bound", value: strings.Replace(valid, "p=4", "p=3", 1)},
		{name: "parallelism above bound", value: strings.Replace(valid, "p=4", "p=5", 1)},
		{name: "parallelism overflow", value: strings.Replace(valid, "p=4", "p=256", 1)},
		{name: "empty salt", value: strings.Replace(valid, "$"+salt+"$", "$$", 1)},
		{name: "short salt", value: strings.Replace(valid, salt, salt[:len(salt)-1], 1)},
		{name: "long salt", value: strings.Replace(valid, salt, salt+"A", 1)},
		{name: "padded salt", value: strings.Replace(valid, salt, salt+"=", 1)},
		{name: "invalid salt alphabet", value: strings.Replace(valid, salt, strings.Repeat("_", len(salt)), 1)},
		{name: "noncanonical salt bits", value: strings.Replace(valid, salt, salt[:len(salt)-1]+"B", 1)},
		{name: "empty key", value: strings.TrimSuffix(valid, key)},
		{name: "short key", value: strings.TrimSuffix(valid, key[len(key)-1:])},
		{name: "long key", value: valid + "A"},
		{name: "padded key", value: valid + "="},
		{name: "invalid key alphabet", value: strings.TrimSuffix(valid, key) + strings.Repeat("_", len(key))},
		{name: "noncanonical key bits", value: valid[:len(valid)-1] + "B"},
	}

	hasher := &PasswordHasher{}
	const password = "private password value"
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parsePasswordHash(test.value); !errors.Is(
				err,
				ErrInvalidPasswordHash,
			) {
				t.Errorf("parse error = %v, want ErrInvalidPasswordHash", err)
			}

			matched, err := hasher.Verify(password, test.value)
			if matched {
				t.Error("malformed password hash matched")
			}
			if err != ErrInvalidPasswordHash {
				t.Errorf("Verify error = %v, want exact ErrInvalidPasswordHash", err)
			}
			if err != nil &&
				(strings.Contains(err.Error(), password) ||
					(test.value != "" && strings.Contains(err.Error(), test.value))) {
				t.Errorf("Verify error exposes password or hash: %v", err)
			}
		})
	}
}

func TestParsePasswordHashAcceptsOnlyFixedParameters(t *testing.T) {
	salt := bytes.Repeat([]byte{0x11}, passwordHashSaltLength)
	key := bytes.Repeat([]byte{0x22}, int(passwordHashKeyLength))
	encoded := encodePasswordHash(
		passwordHashParameters{
			memory:      passwordHashMemory,
			iterations:  passwordHashIterations,
			parallelism: passwordHashParallelism,
		},
		salt,
		key,
	)

	parsed, err := parsePasswordHash(encoded)
	if err != nil {
		t.Fatalf("parsePasswordHash: %v", err)
	}
	if parsed.parameters.memory != passwordHashMemory ||
		parsed.parameters.iterations != passwordHashIterations ||
		parsed.parameters.parallelism != passwordHashParallelism {
		t.Errorf("parameters = %+v", parsed.parameters)
	}
	if !bytes.Equal(parsed.salt, salt) {
		t.Errorf("salt = %x, want %x", parsed.salt, salt)
	}
	if !bytes.Equal(parsed.key, key) {
		t.Errorf("key = %x, want %x", parsed.key, key)
	}
}

func TestPasswordHasherConcurrentVerification(t *testing.T) {
	hasher, err := NewPasswordHasherFrom(&fillReader{value: 0x3c})
	if err != nil {
		t.Fatalf("NewPasswordHasherFrom: %v", err)
	}
	encodedHash, err := hasher.Hash("correct password")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	type result struct {
		matched bool
		err     error
	}
	results := make(chan result, 2)
	for _, password := range []string{"correct password", "wrong password"} {
		go func() {
			matched, verifyErr := hasher.Verify(password, encodedHash)
			results <- result{matched: matched, err: verifyErr}
		}()
	}

	matches := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Errorf("Verify: %v", result.err)
		}
		if result.matched {
			matches++
		}
	}
	if matches != 1 {
		t.Errorf("matching verifications = %d, want 1", matches)
	}
}
