package validation

import (
	"strings"
	"unicode/utf8"
)

const (
	MaxEmailLength    = 256
	MinPasswordLength = 8
	MaxPasswordBytes  = 72
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NormalizeEmail trims spaces and converts the email to lowercase.
// email The email address to normalize.
// Returns The normalized email address.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// IsStorable checks if the string is valid UTF-8 and does not contain null bytes.
// s The string to check.
// Returns True if the string is storable, false otherwise.
func isStorable(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

// ValidateRegister checks if the provided email and password meet the required validation rules.
//
// Returns a ValidationError containing all validation problems, or nil if there are none.
func ValidateRegister(email, password string) error {
	var problems []string

	switch {
	case email == "":
		problems = append(problems, "email is required")
	case utf8.RuneCountInString(email) > MaxEmailLength:
		problems = append(problems, "email is too long")
	case !isStorable(email):
		problems = append(problems, "email contains invalid characters")
	}

	switch {
	case !utf8.ValidString(password):
		problems = append(problems, "password contains invalid characters")
	case utf8.RuneCountInString(password) < MinPasswordLength:
		problems = append(problems, "password is too short")
	case len(password) > MaxPasswordBytes:
		problems = append(problems, "password is too long")
	}

	if len(problems) == 0 {
		return nil
	}
	return &ValidationError{Message: strings.Join(problems, "; ")}
}
