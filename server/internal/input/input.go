package input

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Fyy10/settled/server/internal/money"
)

const (
	FieldEmail              = "email"
	FieldDisplayName        = "displayName"
	FieldPassword           = "password"
	FieldGroupName          = "name"
	FieldExpenseDescription = "description"
	FieldRepaymentNote      = "note"
	FieldAmountCents        = "amountCents"
)

type FieldError struct {
	Field   string
	Message string
}

func (err *FieldError) Error() string {
	return fmt.Sprintf("%s: %s", err.Field, err.Message)
}

func NormalizeEmail(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", invalid(FieldEmail, "Email must be valid UTF-8.")
	}

	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", invalid(FieldEmail, "Email is required.")
	}
	if len(normalized) < 3 || len(normalized) > 254 {
		return "", invalid(FieldEmail, "Email must be between 3 and 254 bytes.")
	}
	if strings.Count(normalized, "@") != 1 {
		return "", invalid(FieldEmail, "Email must contain exactly one @.")
	}

	local, domain, _ := strings.Cut(normalized, "@")
	if local == "" || domain == "" {
		return "", invalid(FieldEmail, "Email local and domain parts are required.")
	}
	if strings.HasPrefix(domain, ".") ||
		strings.HasSuffix(domain, ".") ||
		strings.Contains(domain, "..") {
		return "", invalid(FieldEmail, "Email domain dots are invalid.")
	}
	for _, character := range normalized {
		if unicode.IsSpace(character) || isDisallowedControl(character) {
			return "", invalid(FieldEmail, "Email must not contain whitespace or control characters.")
		}
	}

	return normalized, nil
}

func NormalizeDisplayName(value string) (string, error) {
	return normalizeRequiredText(
		value,
		FieldDisplayName,
		"Display name",
		120,
	)
}

func ValidatePassword(value string) error {
	if !utf8.ValidString(value) {
		return invalid(FieldPassword, "Password must be valid UTF-8.")
	}
	if len(value) < 8 || len(value) > 128 {
		return invalid(FieldPassword, "Password must be between 8 and 128 bytes.")
	}
	if strings.IndexByte(value, 0) >= 0 {
		return invalid(FieldPassword, "Password must not contain NUL bytes.")
	}
	return nil
}

func NormalizeGroupName(value string) (string, error) {
	return normalizeRequiredText(value, FieldGroupName, "Group name", 160)
}

func NormalizeExpenseDescription(value string) (string, error) {
	return normalizeRequiredText(
		value,
		FieldExpenseDescription,
		"Expense description",
		240,
	)
}

func NormalizeRepaymentNote(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	if !utf8.ValidString(*value) {
		return nil, invalid(FieldRepaymentNote, "Repayment note must be valid UTF-8.")
	}

	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil, nil
	}
	if containsControl(normalized) {
		return nil, invalid(
			FieldRepaymentNote,
			"Repayment note must not contain control characters.",
		)
	}
	if utf8.RuneCountInString(normalized) > 240 {
		return nil, invalid(
			FieldRepaymentNote,
			"Repayment note must not exceed 240 characters.",
		)
	}

	return &normalized, nil
}

func ValidateAmountCents(cents int64) error {
	if err := money.ValidatePositive(cents); err != nil {
		return invalid(FieldAmountCents, "Amount must be positive.")
	}
	return nil
}

func normalizeRequiredText(
	value string,
	field string,
	label string,
	maxRunes int,
) (string, error) {
	if !utf8.ValidString(value) {
		return "", invalid(field, label+" must be valid UTF-8.")
	}

	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", invalid(field, label+" is required.")
	}
	if containsControl(normalized) {
		return "", invalid(field, label+" must not contain control characters.")
	}
	if utf8.RuneCountInString(normalized) > maxRunes {
		return "", invalid(
			field,
			fmt.Sprintf("%s must not exceed %d characters.", label, maxRunes),
		)
	}
	return normalized, nil
}

func containsControl(value string) bool {
	for _, character := range value {
		if isDisallowedControl(character) {
			return true
		}
	}
	return false
}

func isDisallowedControl(character rune) bool {
	return unicode.IsControl(character) || unicode.Is(unicode.Bidi_Control, character)
}

func invalid(field, message string) error {
	return &FieldError{
		Field:   field,
		Message: message,
	}
}
