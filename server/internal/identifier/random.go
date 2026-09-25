package identifier

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidRandomSize   = errors.New("random size must be positive")
	ErrInvalidRandomSource = errors.New("random source is required")
)

func NewRandomBase64URL(byteLength int) (string, error) {
	return NewRandomBase64URLFrom(rand.Reader, byteLength)
}

func NewRandomBase64URLFrom(reader io.Reader, byteLength int) (string, error) {
	random, err := randomBytes(reader, byteLength)
	if err != nil {
		return "", fmt.Errorf("generate base64url ID: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func NewRandomHex(byteLength int) (string, error) {
	return NewRandomHexFrom(rand.Reader, byteLength)
}

func NewRandomHexFrom(reader io.Reader, byteLength int) (string, error) {
	random, err := randomBytes(reader, byteLength)
	if err != nil {
		return "", fmt.Errorf("generate hexadecimal ID: %w", err)
	}
	return hex.EncodeToString(random), nil
}

func randomBytes(reader io.Reader, byteLength int) ([]byte, error) {
	if byteLength <= 0 {
		return nil, ErrInvalidRandomSize
	}

	random := make([]byte, byteLength)
	if err := readRandom(reader, random); err != nil {
		return nil, err
	}
	return random, nil
}

func readRandom(reader io.Reader, destination []byte) error {
	if reader == nil {
		return ErrInvalidRandomSource
	}
	if _, err := io.ReadFull(reader, destination); err != nil {
		return fmt.Errorf("read cryptographic randomness: %w", err)
	}
	return nil
}
