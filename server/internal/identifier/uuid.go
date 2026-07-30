package identifier

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrInvalidUUID = errors.New("invalid UUID")

func NewUUID() (string, error) {
	return NewUUIDFrom(rand.Reader)
}

func NewUUIDFrom(reader io.Reader) (string, error) {
	var value [16]byte
	if err := readRandom(reader, value[:]); err != nil {
		return "", fmt.Errorf("generate UUID: %w", err)
	}

	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return formatUUID(value), nil
}

func ParseUUID(value string) (string, error) {
	if len(value) != 36 ||
		value[8] != '-' ||
		value[13] != '-' ||
		value[18] != '-' ||
		value[23] != '-' {
		return "", ErrInvalidUUID
	}

	var decoded [16]byte
	offsets := [][2]int{
		{0, 8},
		{9, 13},
		{14, 18},
		{19, 23},
		{24, 36},
	}
	position := 0
	for _, offset := range offsets {
		written, err := hex.Decode(decoded[position:], []byte(value[offset[0]:offset[1]]))
		if err != nil {
			return "", ErrInvalidUUID
		}
		position += written
	}
	if position != len(decoded) || decoded[8]&0xc0 != 0x80 {
		return "", ErrInvalidUUID
	}

	return strings.ToLower(value), nil
}

func formatUUID(value [16]byte) string {
	var encoded [36]byte
	hex.Encode(encoded[0:8], value[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], value[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], value[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], value[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], value[10:16])
	return string(encoded[:])
}
