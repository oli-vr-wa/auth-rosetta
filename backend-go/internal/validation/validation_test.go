package validation

import (
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	got := NormalizeEmail("  Bob@Example.COM")
	want := "bob@example.com"
	if got != want {
		t.Errorf("NormalizeEmail = %q, want %q", got, want)
	}
}

func TestValidateRegister(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
	}{
		{"valid", "a@b.com", "password1", false},
		{"empty email", "", "password1", true},
		{"email 256 chars ok", strings.Repeat("a", 256), "password1", false},
		{"email 257 chars rejected", strings.Repeat("a", 257), "password1", true},
		{"NUL in email", "a\x00@b.com", "password1", true},
		{"password 7 chars", "a@b.com", "1234567", true},
		{"password 8 chars", "a@b.com", "12345678", false},
		{"password 72 bytes ok", "a@b.com", strings.Repeat("é", 36), false},
		{"password 74 bytes rejected", "a@b.com", strings.Repeat("é", 37), true},
		{"invalid utf-8 password", "a@b.com", "\xff\xfe\xfd\xfc\xfb\xfa\xf9\xf8", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRegister(tc.email, tc.password)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateRegister(%q, %q) error = %v, wantErr %v", tc.email, tc.password, err, tc.wantErr)
			}
		})
	}
}

func TestValidateRegisterMessages(t *testing.T) {
	tests := []struct{ name, email, password, want string }{
		{"empty email", "", "password1", "Email is required."},
		{"long email", strings.Repeat("a", 257), "password1", "Email must be at most 256 characters"},
		{"NUL in email", "a\x00@b.com", "password1", "Email contains invalid characters."},
		{"short password", "a@b.com", "short", "Password must be at least 8 characters"},
		{"long password", "a@b.com", strings.Repeat("a", 73), "Password must be at most 72 bytes"},
		{"invalid UTF-8 password", "a@b.com", "\xff\xfe\xfd\xfc\xfb\xfa\xf9\xf8", "Password contains invalid characters."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRegister(tc.email, tc.password)
			if err == nil || err.Error() != tc.want {
				t.Errorf("error = %v, want %q", err, tc.want)
			}
		})
	}

	// Both problems are reported together.
	err := ValidateRegister("", "short")
	for _, want := range []string{"Email is required.", "Password must be at least 8 characters"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("combined error %v lacks %q", err, want)
		}
	}
}

func TestValidateRegisterJoinsProblems(t *testing.T) {
	err := ValidateRegister("", "short")
	if err == nil || !strings.Contains(err.Error(), "; ") {
		t.Errorf("expected joined problems, got %v", err)
	}
}
