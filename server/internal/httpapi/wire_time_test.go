package httpapi

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  time.Time
	}{
		{
			name:  "ordinary date",
			value: "2026-07-29",
			want:  time.Date(2026, time.July, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "leap day",
			value: "2024-02-29",
			want:  time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseDate(test.value)
			if err != nil {
				t.Fatalf("parseDate: %v", err)
			}
			if !got.Equal(test.want) || got.Location() != time.UTC {
				t.Errorf("parseDate() = %v, want %v in UTC", got, test.want)
			}
		})
	}
}

func TestParseDateRejectsNonStrictValues(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"2026",
		"2026-07",
		"2026-7-29",
		"2026-07-9",
		"2026-02-29",
		"2024-02-30",
		"2026-13-01",
		"2026-00-01",
		"2026-07-29T00:00:00Z",
		"2026-07-29Z",
		" 2026-07-29",
		"2026-07-29 ",
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if got, err := parseDate(value); err == nil {
				t.Errorf("parseDate(%q) = %v, want error", value, got)
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("date", 9*60*60)
	value := time.Date(2026, time.July, 29, 23, 59, 59, 0, location)
	if got := formatDate(value); got != "2026-07-29" {
		t.Errorf("formatDate() = %q, want 2026-07-29", got)
	}
}

func TestFormatTimestampConvertsToUTCAndPreservesInstant(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("source", -7*60*60)
	values := []time.Time{
		time.Date(2026, time.July, 29, 18, 30, 45, 0, location),
		time.Date(2026, time.July, 29, 18, 30, 45, 123000000, location),
		time.Date(2026, time.July, 29, 18, 30, 45, 123456000, location),
		time.Date(2026, time.July, 29, 18, 30, 45, 123456789, location),
	}

	for _, value := range values {
		formatted := formatTimestamp(value)
		parsed, err := time.Parse(time.RFC3339Nano, formatted)
		if err != nil {
			t.Fatalf("parse formatted timestamp %q: %v", formatted, err)
		}
		if !parsed.Equal(value) {
			t.Errorf("formatted timestamp instant = %v, want %v", parsed, value)
		}
		if parsed.Location() != time.UTC {
			t.Errorf("formatted timestamp location = %v, want UTC", parsed.Location())
		}
		if formatted[len(formatted)-1] != 'Z' {
			t.Errorf("formatted timestamp = %q, want UTC Z suffix", formatted)
		}
	}
}
