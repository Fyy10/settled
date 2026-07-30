package input

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	maxASCII := strings.Repeat("a", 252) + "@b"
	maxUnicode := strings.Repeat("É", 126) + "@B"
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "trim and lowercase",
			value: "\u2003Alice@EXAMPLE.COM\u2003",
			want:  "alice@example.com",
		},
		{name: "minimum bytes", value: "a@b", want: "a@b"},
		{name: "maximum ASCII bytes", value: maxASCII, want: maxASCII},
		{
			name:  "maximum Unicode bytes",
			value: maxUnicode,
			want:  strings.Repeat("é", 126) + "@b",
		},
		{
			name:  "local dots are permitted",
			value: ".local..part@domain",
			want:  ".local..part@domain",
		},
		{
			name:  "domain dot is not required",
			value: "person@localhost",
			want:  "person@localhost",
		},
		{
			name:  "ordinary internal space is not permitted",
			value: "person @example.com",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeEmail(test.value)
			if test.want == "" {
				requireFieldError(t, err, FieldEmail)
				return
			}
			if err != nil {
				t.Fatalf("NormalizeEmail: %v", err)
			}
			if got != test.want {
				t.Errorf("NormalizeEmail() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeEmailRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "empty"},
		{name: "Unicode whitespace only", value: "\u2003"},
		{name: "below minimum bytes", value: "a@"},
		{name: "above maximum bytes", value: strings.Repeat("a", 253) + "@b"},
		{name: "missing at", value: "example.com"},
		{name: "multiple at", value: "a@@b"},
		{name: "empty local", value: "@example.com"},
		{name: "empty domain", value: "person@"},
		{name: "domain leading dot", value: "person@.example"},
		{name: "domain trailing dot", value: "person@example."},
		{name: "domain consecutive dots", value: "person@example..com"},
		{name: "ordinary whitespace", value: "person @example.com"},
		{name: "Unicode whitespace", value: "person@\u00a0example.com"},
		{name: "tab", value: "person@\texample.com"},
		{name: "NUL", value: "person@\x00example.com"},
		{name: "bidi control", value: "person@\u202eexample.com"},
		{name: "invalid UTF-8", value: string([]byte{'a', '@', 0xff})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeEmail(test.value)
			requireFieldError(t, err, FieldEmail)
			if got != "" {
				t.Errorf("NormalizeEmail() = %q, want empty", got)
			}
		})
	}
}

func TestRequiredTextNormalizers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		normalize func(string) (string, error)
		field     string
		maxRunes  int
	}{
		{
			name:      "display name",
			normalize: NormalizeDisplayName,
			field:     FieldDisplayName,
			maxRunes:  120,
		},
		{
			name:      "group name",
			normalize: NormalizeGroupName,
			field:     FieldGroupName,
			maxRunes:  160,
		},
		{
			name:      "expense description",
			normalize: NormalizeExpenseDescription,
			field:     FieldExpenseDescription,
			maxRunes:  240,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			t.Run("trim Unicode whitespace", func(t *testing.T) {
				got, err := test.normalize("\u2003Settled value\u2003")
				if err != nil {
					t.Fatalf("normalize: %v", err)
				}
				if got != "Settled value" {
					t.Errorf("normalized value = %q", got)
				}
			})

			t.Run("maximum Unicode code points", func(t *testing.T) {
				value := strings.Repeat("界", test.maxRunes)
				got, err := test.normalize(value)
				if err != nil {
					t.Fatalf("normalize: %v", err)
				}
				if got != value {
					t.Errorf("normalized value changed at maximum length")
				}
			})

			t.Run("ordinary internal spaces", func(t *testing.T) {
				const value = "one two"
				got, err := test.normalize(value)
				if err != nil {
					t.Fatalf("normalize: %v", err)
				}
				if got != value {
					t.Errorf("normalized value = %q, want %q", got, value)
				}
			})

			invalidValues := []struct {
				name  string
				value string
			}{
				{name: "empty"},
				{name: "whitespace only", value: "\u2003 \t"},
				{name: "above maximum code points", value: strings.Repeat("界", test.maxRunes+1)},
				{name: "embedded newline", value: "one\ntwo"},
				{name: "embedded tab", value: "one\ttwo"},
				{name: "embedded DEL", value: "one\x7ftwo"},
				{name: "embedded bidi control", value: "one\u202etwo"},
				{name: "invalid UTF-8", value: string([]byte{'a', 0xff})},
			}
			for _, invalidValue := range invalidValues {
				t.Run(invalidValue.name, func(t *testing.T) {
					got, err := test.normalize(invalidValue.value)
					requireFieldError(t, err, test.field)
					if got != "" {
						t.Errorf("normalized value = %q, want empty", got)
					}
				})
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
	}{
		{name: "minimum bytes", password: "aaaaaaaa"},
		{name: "maximum bytes", password: strings.Repeat("a", 128)},
		{name: "Unicode measured as bytes", password: "éééé"},
		{name: "leading space is not trimmed", password: " abcdefg"},
		{name: "no complexity requirement", password: "11111111"},
		{name: "non-NUL control allowed", password: "abc\ndefg"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidatePassword(test.password); err != nil {
				t.Errorf("ValidatePassword: %v", err)
			}
		})
	}
}

func TestValidatePasswordRejectsInvalidValuesWithoutExposure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
	}{
		{name: "below minimum bytes", password: "1234567"},
		{name: "Unicode below minimum bytes", password: "ééé"},
		{name: "above maximum bytes", password: strings.Repeat("private", 19)},
		{name: "NUL", password: "abcd\x00efg"},
		{name: "invalid UTF-8", password: string([]byte{'1', '2', '3', '4', '5', '6', '7', 0xff})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePassword(test.password)
			requireFieldError(t, err, FieldPassword)
			if strings.Contains(err.Error(), test.password) {
				t.Errorf("error exposes password value: %v", err)
			}
		})
	}
}

func TestNormalizeRepaymentNote(t *testing.T) {
	t.Parallel()

	maximum := strings.Repeat("界", 240)
	tests := []struct {
		name  string
		value *string
		want  *string
	}{
		{name: "missing"},
		{name: "empty", value: stringPointer("")},
		{name: "whitespace only", value: stringPointer("\u2003 \t")},
		{
			name:  "trimmed value",
			value: stringPointer("\u2003Paid with Venmo\u2003"),
			want:  stringPointer("Paid with Venmo"),
		},
		{name: "maximum Unicode code points", value: &maximum, want: &maximum},
		{
			name:  "ordinary internal spaces",
			value: stringPointer("paid in cash"),
			want:  stringPointer("paid in cash"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeRepaymentNote(test.value)
			if err != nil {
				t.Fatalf("NormalizeRepaymentNote: %v", err)
			}
			switch {
			case got == nil && test.want == nil:
			case got == nil || test.want == nil:
				t.Errorf("NormalizeRepaymentNote() = %v, want %v", got, test.want)
			case *got != *test.want:
				t.Errorf("NormalizeRepaymentNote() = %q, want %q", *got, *test.want)
			}
		})
	}
}

func TestNormalizeRepaymentNoteRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "above maximum code points", value: strings.Repeat("界", 241)},
		{name: "embedded newline", value: "paid\ncash"},
		{name: "embedded tab", value: "paid\tcash"},
		{name: "embedded DEL", value: "paid\x7fcash"},
		{name: "embedded bidi control", value: "paid\u202ecash"},
		{name: "invalid UTF-8", value: string([]byte{'a', 0xff})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeRepaymentNote(&test.value)
			requireFieldError(t, err, FieldRepaymentNote)
			if got != nil {
				t.Errorf("NormalizeRepaymentNote() = %q, want nil", *got)
			}
		})
	}
}

func TestValidateAmountCents(t *testing.T) {
	t.Parallel()

	for _, cents := range []int64{1, math.MaxInt64} {
		if err := ValidateAmountCents(cents); err != nil {
			t.Errorf("ValidateAmountCents(%d): %v", cents, err)
		}
	}
	for _, cents := range []int64{math.MinInt64, -1, 0} {
		err := ValidateAmountCents(cents)
		requireFieldError(t, err, FieldAmountCents)
	}
}

func TestFieldErrorUsesStableFieldAndSafeMessage(t *testing.T) {
	t.Parallel()

	err := &FieldError{
		Field:   FieldDisplayName,
		Message: "Display name is required.",
	}
	if err.Error() != "displayName: Display name is required." {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestFieldKeysMatchAPIRequestNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "email", got: FieldEmail, want: "email"},
		{name: "display name", got: FieldDisplayName, want: "displayName"},
		{name: "password", got: FieldPassword, want: "password"},
		{name: "group name", got: FieldGroupName, want: "name"},
		{
			name: "expense description",
			got:  FieldExpenseDescription,
			want: "description",
		},
		{name: "repayment note", got: FieldRepaymentNote, want: "note"},
		{name: "amount cents", got: FieldAmountCents, want: "amountCents"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.got != test.want {
				t.Errorf("field key = %q, want %q", test.got, test.want)
			}
		})
	}
}

func requireFieldError(t *testing.T, err error, field string) {
	t.Helper()

	if err == nil {
		t.Fatal("error = nil, want FieldError")
	}
	var fieldError *FieldError
	if !errors.As(err, &fieldError) {
		t.Fatalf("error type = %T, want *FieldError", err)
	}
	if fieldError.Field != field {
		t.Errorf("field = %q, want %q", fieldError.Field, field)
	}
	if fieldError.Message == "" {
		t.Error("message is empty")
	}
}

func stringPointer(value string) *string {
	return &value
}
