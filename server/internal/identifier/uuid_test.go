package identifier

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestNewUUIDFromSetsVersionVariantAndCanonicalFormat(t *testing.T) {
	t.Parallel()

	uuid, err := NewUUIDFrom(bytes.NewReader(bytes.Repeat([]byte{0xff}, 16)))
	if err != nil {
		t.Fatalf("NewUUIDFrom: %v", err)
	}

	const want = "ffffffff-ffff-4fff-bfff-ffffffffffff"
	if uuid != want {
		t.Errorf("UUID = %q, want %q", uuid, want)
	}
}

func TestNewUUIDFromUsesAllRandomBytes(t *testing.T) {
	t.Parallel()

	random := []byte{
		0x00, 0x11, 0x22, 0x33,
		0x44, 0x55,
		0xa6, 0x77,
		0x08, 0x99,
		0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
	}
	uuid, err := NewUUIDFrom(bytes.NewReader(random))
	if err != nil {
		t.Fatalf("NewUUIDFrom: %v", err)
	}

	const want = "00112233-4455-4677-8899-aabbccddeeff"
	if uuid != want {
		t.Errorf("UUID = %q, want %q", uuid, want)
	}
}

func TestNewUUIDFromPropagatesRandomSourceFailure(t *testing.T) {
	t.Parallel()

	sourceError := errors.New("entropy unavailable")
	tests := []struct {
		name   string
		reader io.Reader
		want   error
	}{
		{
			name:   "nil reader",
			reader: nil,
			want:   ErrInvalidRandomSource,
		},
		{
			name:   "source error",
			reader: errorReader{err: sourceError},
			want:   sourceError,
		},
		{
			name:   "short source",
			reader: strings.NewReader("short"),
			want:   io.ErrUnexpectedEOF,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			uuid, err := NewUUIDFrom(test.reader)
			if !errors.Is(err, test.want) {
				t.Errorf("error = %v, want wrapped %v", err, test.want)
			}
			if uuid != "" {
				t.Errorf("UUID = %q, want empty", uuid)
			}
		})
	}
}

func TestParseUUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "lowercase version four",
			value: "00112233-4455-4677-8899-aabbccddeeff",
			want:  "00112233-4455-4677-8899-aabbccddeeff",
		},
		{
			name:  "uppercase",
			value: "00112233-4455-4677-8899-AABBCCDDEEFF",
			want:  "00112233-4455-4677-8899-aabbccddeeff",
		},
		{
			name:  "non-v4 with RFC 4122 variant",
			value: "00112233-4455-1677-8899-aabbccddeeff",
			want:  "00112233-4455-1677-8899-aabbccddeeff",
		},
		{
			name:  "highest RFC 4122 variant byte",
			value: "00112233-4455-4677-BF99-AABBCCDDEEFF",
			want:  "00112233-4455-4677-bf99-aabbccddeeff",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseUUID(test.value)
			if err != nil {
				t.Fatalf("ParseUUID: %v", err)
			}
			if got != test.want {
				t.Errorf("ParseUUID() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseUUIDRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "empty"},
		{name: "too short", value: "00112233-4455-4677-8899-aabbccddeef"},
		{name: "too long", value: "00112233-4455-4677-8899-aabbccddeeff0"},
		{name: "missing hyphens", value: "00112233445546778899aabbccddeeff"},
		{name: "wrong first hyphen", value: "0011223-34455-4677-8899-aabbccddeeff"},
		{name: "wrong second hyphen", value: "00112233-44554-677-8899-aabbccddeeff"},
		{name: "wrong third hyphen", value: "00112233-4455-46778-899-aabbccddeeff"},
		{name: "wrong fourth hyphen", value: "00112233-4455-4677-8899a-abbccddeeff"},
		{name: "non hexadecimal", value: "00112233-4455-4677-8899-aabbccddeefg"},
		{name: "braces", value: "{0112233-4455-4677-8899-aabbccddeeff}"},
		{name: "NCS variant", value: "00112233-4455-4677-0099-aabbccddeeff"},
		{name: "reserved Microsoft variant", value: "00112233-4455-4677-c099-aabbccddeeff"},
		{name: "future reserved variant", value: "00112233-4455-4677-e099-aabbccddeeff"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseUUID(test.value)
			if !errors.Is(err, ErrInvalidUUID) {
				t.Errorf("error = %v, want ErrInvalidUUID", err)
			}
			if got != "" {
				t.Errorf("ParseUUID() = %q, want empty", got)
			}
		})
	}
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}
