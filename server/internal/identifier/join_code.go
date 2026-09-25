package identifier

import (
	"crypto/rand"
	"fmt"
	"io"
)

const (
	JoinCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	JoinCodeLength   = 12
)

func NewJoinCode() (string, error) {
	return NewJoinCodeFrom(rand.Reader)
}

func NewJoinCodeFrom(reader io.Reader) (string, error) {
	code, err := randomFromAlphabet(reader, JoinCodeLength, JoinCodeAlphabet)
	if err != nil {
		return "", fmt.Errorf("generate join code: %w", err)
	}
	return code, nil
}

func randomFromAlphabet(reader io.Reader, length int, alphabet string) (string, error) {
	if length <= 0 {
		return "", ErrInvalidRandomSize
	}
	if reader == nil {
		return "", ErrInvalidRandomSource
	}
	if len(alphabet) < 2 || len(alphabet) > 256 {
		return "", fmt.Errorf("alphabet size must be between 2 and 256")
	}

	limit := 256 - 256%len(alphabet)
	code := make([]byte, 0, length)
	var candidate [1]byte
	for len(code) < length {
		if err := readRandom(reader, candidate[:]); err != nil {
			return "", err
		}
		if int(candidate[0]) >= limit {
			continue
		}
		code = append(code, alphabet[int(candidate[0])%len(alphabet)])
	}
	return string(code), nil
}
