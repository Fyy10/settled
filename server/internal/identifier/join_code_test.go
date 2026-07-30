package identifier

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestJoinCodeConstantsMatchContract(t *testing.T) {
	t.Parallel()

	if JoinCodeAlphabet != "23456789ABCDEFGHJKLMNPQRSTUVWXYZ" {
		t.Errorf("JoinCodeAlphabet = %q", JoinCodeAlphabet)
	}
	if JoinCodeLength != 12 {
		t.Errorf("JoinCodeLength = %d, want 12", JoinCodeLength)
	}
}

func TestNewJoinCodeFrom(t *testing.T) {
	t.Parallel()

	random := []byte{0, 1, 2, 3, 4, 5, 6, 7, 30, 31, 32, 255}
	code, err := NewJoinCodeFrom(bytes.NewReader(random))
	if err != nil {
		t.Fatalf("NewJoinCodeFrom: %v", err)
	}

	const want = "23456789YZ2Z"
	if code != want {
		t.Errorf("join code = %q, want %q", code, want)
	}
	if len(code) != JoinCodeLength {
		t.Errorf("join code length = %d, want %d", len(code), JoinCodeLength)
	}
	for _, character := range code {
		if !strings.ContainsRune(JoinCodeAlphabet, character) {
			t.Errorf("join code contains character %q outside documented alphabet", character)
		}
	}
}

func TestJoinCodeAlphabetDistribution(t *testing.T) {
	t.Parallel()

	random := make([]byte, 256)
	for index := range random {
		random[index] = byte(index)
	}
	code, err := randomFromAlphabet(bytes.NewReader(random), len(random), JoinCodeAlphabet)
	if err != nil {
		t.Fatalf("randomFromAlphabet: %v", err)
	}

	counts := make(map[byte]int, len(JoinCodeAlphabet))
	for index := range len(code) {
		counts[code[index]]++
	}
	for index := range len(JoinCodeAlphabet) {
		character := JoinCodeAlphabet[index]
		if counts[character] != 8 {
			t.Errorf("alphabet character %q count = %d, want 8", character, counts[character])
		}
	}
}

func TestRandomFromAlphabetRejectsBiasedCandidates(t *testing.T) {
	t.Parallel()

	code, err := randomFromAlphabet(
		bytes.NewReader([]byte{255, 0, 1, 2}),
		3,
		"ABC",
	)
	if err != nil {
		t.Fatalf("randomFromAlphabet: %v", err)
	}
	if code != "ABC" {
		t.Errorf("code = %q, want %q", code, "ABC")
	}
}

func TestNewJoinCodeFromPropagatesRandomSourceFailure(t *testing.T) {
	t.Parallel()

	sourceError := errors.New("entropy unavailable")
	tests := []struct {
		name   string
		reader io.Reader
		want   error
	}{
		{name: "nil reader", want: ErrInvalidRandomSource},
		{name: "source error", reader: errorReader{err: sourceError}, want: sourceError},
		{name: "short source", reader: bytes.NewReader([]byte{0}), want: io.EOF},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			code, err := NewJoinCodeFrom(test.reader)
			if !errors.Is(err, test.want) {
				t.Errorf("error = %v, want wrapped %v", err, test.want)
			}
			if code != "" {
				t.Errorf("join code = %q, want empty", code)
			}
		})
	}
}

func TestRandomFromAlphabetRejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		reader   io.Reader
		length   int
		alphabet string
		want     error
	}{
		{
			name:     "zero length",
			reader:   panicReader{},
			alphabet: JoinCodeAlphabet,
			want:     ErrInvalidRandomSize,
		},
		{
			name:     "nil reader",
			length:   1,
			alphabet: JoinCodeAlphabet,
			want:     ErrInvalidRandomSource,
		},
		{
			name:     "short alphabet",
			reader:   panicReader{},
			length:   1,
			alphabet: "A",
		},
		{
			name:     "long alphabet",
			reader:   panicReader{},
			length:   1,
			alphabet: strings.Repeat("A", 257),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			code, err := randomFromAlphabet(test.reader, test.length, test.alphabet)
			if err == nil {
				t.Fatal("randomFromAlphabet succeeded, want error")
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Errorf("error = %v, want wrapped %v", err, test.want)
			}
			if code != "" {
				t.Errorf("code = %q, want empty", code)
			}
		})
	}
}
