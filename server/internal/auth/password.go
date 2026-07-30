package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordHashMemory      uint32 = 65536
	passwordHashIterations  uint32 = 3
	passwordHashParallelism uint8  = 4
	passwordHashSaltLength         = 16
	passwordHashKeyLength   uint32 = 32
	maxPHCLength                   = 256
	dummyPassword                  = "settled-dummy-password-not-a-user"
)

var (
	ErrInvalidPasswordHash         = errors.New("invalid password hash")
	ErrInvalidPasswordRandomSource = errors.New("password random source is required")
)

type PasswordHasher struct {
	random    io.Reader
	dummyHash string
}

type passwordHashParameters struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

type parsedPasswordHash struct {
	parameters passwordHashParameters
	salt       []byte
	key        []byte
}

func NewPasswordHasher() (*PasswordHasher, error) {
	return NewPasswordHasherFrom(rand.Reader)
}

func NewPasswordHasherFrom(random io.Reader) (*PasswordHasher, error) {
	if random == nil {
		return nil, ErrInvalidPasswordRandomSource
	}

	hasher := &PasswordHasher{random: random}
	dummyHash, err := hasher.Hash(dummyPassword)
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}
	hasher.dummyHash = dummyHash
	return hasher, nil
}

func (hasher *PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, passwordHashSaltLength)
	if _, err := io.ReadFull(hasher.random, salt); err != nil {
		return "", fmt.Errorf("read password salt: %w", err)
	}

	passwordBytes := []byte(password)
	defer clear(passwordBytes)

	key := argon2.IDKey(
		passwordBytes,
		salt,
		passwordHashIterations,
		passwordHashMemory,
		passwordHashParallelism,
		passwordHashKeyLength,
	)
	defer clear(key)

	return encodePasswordHash(
		passwordHashParameters{
			memory:      passwordHashMemory,
			iterations:  passwordHashIterations,
			parallelism: passwordHashParallelism,
		},
		salt,
		key,
	), nil
}

func (hasher *PasswordHasher) Verify(password, encodedHash string) (bool, error) {
	parsed, err := parsePasswordHash(encodedHash)
	if err != nil {
		return false, ErrInvalidPasswordHash
	}
	defer clear(parsed.key)

	passwordBytes := []byte(password)
	defer clear(passwordBytes)

	actual := argon2.IDKey(
		passwordBytes,
		parsed.salt,
		parsed.parameters.iterations,
		parsed.parameters.memory,
		parsed.parameters.parallelism,
		uint32(len(parsed.key)),
	)
	defer clear(actual)

	return subtle.ConstantTimeCompare(actual, parsed.key) == 1, nil
}

func (hasher *PasswordHasher) VerifyDummy(password string) error {
	_, err := hasher.Verify(password, hasher.dummyHash)
	return err
}

func encodePasswordHash(
	parameters passwordHashParameters,
	salt []byte,
	key []byte,
) string {
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		parameters.memory,
		parameters.iterations,
		parameters.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

func parsePasswordHash(encodedHash string) (parsedPasswordHash, error) {
	if len(encodedHash) == 0 || len(encodedHash) > maxPHCLength {
		return parsedPasswordHash{}, ErrInvalidPasswordHash
	}

	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 ||
		parts[0] != "" ||
		parts[1] != "argon2id" ||
		parts[2] != "v=19" {
		return parsedPasswordHash{}, ErrInvalidPasswordHash
	}

	parameters, err := parsePasswordHashParameters(parts[3])
	if err != nil {
		return parsedPasswordHash{}, ErrInvalidPasswordHash
	}

	salt, err := decodeFixedBase64(parts[4], passwordHashSaltLength)
	if err != nil {
		return parsedPasswordHash{}, ErrInvalidPasswordHash
	}
	key, err := decodeFixedBase64(parts[5], int(passwordHashKeyLength))
	if err != nil {
		clear(salt)
		return parsedPasswordHash{}, ErrInvalidPasswordHash
	}

	return parsedPasswordHash{
		parameters: parameters,
		salt:       salt,
		key:        key,
	}, nil
}

func parsePasswordHashParameters(value string) (passwordHashParameters, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 3 {
		return passwordHashParameters{}, ErrInvalidPasswordHash
	}

	memory, err := parseParameter(parts[0], "m=", 32)
	if err != nil || memory != uint64(passwordHashMemory) {
		return passwordHashParameters{}, ErrInvalidPasswordHash
	}
	iterations, err := parseParameter(parts[1], "t=", 32)
	if err != nil || iterations != uint64(passwordHashIterations) {
		return passwordHashParameters{}, ErrInvalidPasswordHash
	}
	parallelism, err := parseParameter(parts[2], "p=", 8)
	if err != nil || parallelism != uint64(passwordHashParallelism) {
		return passwordHashParameters{}, ErrInvalidPasswordHash
	}

	return passwordHashParameters{
		memory:      uint32(memory),
		iterations:  uint32(iterations),
		parallelism: uint8(parallelism),
	}, nil
}

func parseParameter(value, prefix string, bitSize int) (uint64, error) {
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return 0, ErrInvalidPasswordHash
	}
	encoded := value[len(prefix):]
	parsed, err := strconv.ParseUint(encoded, 10, bitSize)
	if err != nil {
		return 0, ErrInvalidPasswordHash
	}
	if strconv.FormatUint(parsed, 10) != encoded {
		return 0, ErrInvalidPasswordHash
	}
	return parsed, nil
}

func decodeFixedBase64(value string, decodedLength int) ([]byte, error) {
	if len(value) != base64.RawStdEncoding.EncodedLen(decodedLength) {
		return nil, ErrInvalidPasswordHash
	}

	decoded := make([]byte, decodedLength)
	written, err := base64.RawStdEncoding.Strict().Decode(decoded, []byte(value))
	if err != nil || written != decodedLength {
		clear(decoded)
		return nil, ErrInvalidPasswordHash
	}
	return decoded, nil
}
