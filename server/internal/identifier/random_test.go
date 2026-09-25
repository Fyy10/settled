package identifier

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestNewRandomBase64URLFrom(t *testing.T) {
	t.Parallel()

	got, err := NewRandomBase64URLFrom(bytes.NewReader([]byte{0xfb, 0xff}), 2)
	if err != nil {
		t.Fatalf("NewRandomBase64URLFrom: %v", err)
	}
	if got != "-_8" {
		t.Errorf("ID = %q, want %q", got, "-_8")
	}
	if strings.Contains(got, "=") {
		t.Errorf("ID = %q, must be unpadded", got)
	}
}

func TestNewRandomHexFrom(t *testing.T) {
	t.Parallel()

	got, err := NewRandomHexFrom(bytes.NewReader([]byte{0xab, 0xcd, 0xef}), 3)
	if err != nil {
		t.Fatalf("NewRandomHexFrom: %v", err)
	}
	if got != "abcdef" {
		t.Errorf("ID = %q, want %q", got, "abcdef")
	}
}

func TestRandomIDLength(t *testing.T) {
	t.Parallel()

	base64ID, err := NewRandomBase64URLFrom(bytes.NewReader(make([]byte, 32)), 32)
	if err != nil {
		t.Fatalf("NewRandomBase64URLFrom: %v", err)
	}
	if len(base64ID) != 43 {
		t.Errorf("base64url ID length = %d, want 43", len(base64ID))
	}

	hexID, err := NewRandomHexFrom(bytes.NewReader(make([]byte, 16)), 16)
	if err != nil {
		t.Fatalf("NewRandomHexFrom: %v", err)
	}
	if len(hexID) != 32 {
		t.Errorf("hex ID length = %d, want 32", len(hexID))
	}
}

func TestRandomIDRejectsInvalidSizeWithoutReading(t *testing.T) {
	t.Parallel()

	for _, size := range []int{-1, 0} {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			t.Parallel()

			if value, err := NewRandomBase64URLFrom(panicReader{}, size); !errors.Is(
				err,
				ErrInvalidRandomSize,
			) || value != "" {
				t.Errorf("base64url result = (%q, %v)", value, err)
			}
			if value, err := NewRandomHexFrom(panicReader{}, size); !errors.Is(
				err,
				ErrInvalidRandomSize,
			) || value != "" {
				t.Errorf("hex result = (%q, %v)", value, err)
			}
		})
	}
}

func TestRandomIDPropagatesRandomSourceFailure(t *testing.T) {
	t.Parallel()

	sourceError := errors.New("entropy unavailable")
	tests := []struct {
		name      string
		newReader func() io.Reader
		want      error
	}{
		{
			name:      "nil reader",
			newReader: func() io.Reader { return nil },
			want:      ErrInvalidRandomSource,
		},
		{
			name:      "source error",
			newReader: func() io.Reader { return errorReader{err: sourceError} },
			want:      sourceError,
		},
		{
			name:      "short source",
			newReader: func() io.Reader { return bytes.NewReader([]byte{1}) },
			want:      io.ErrUnexpectedEOF,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if value, err := NewRandomBase64URLFrom(test.newReader(), 2); !errors.Is(
				err,
				test.want,
			) || value != "" {
				t.Errorf("base64url result = (%q, %v), want error %v", value, err, test.want)
			}
			if value, err := NewRandomHexFrom(test.newReader(), 2); !errors.Is(
				err,
				test.want,
			) || value != "" {
				t.Errorf("hex result = (%q, %v), want error %v", value, err, test.want)
			}
		})
	}
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("random source must not be read")
}
