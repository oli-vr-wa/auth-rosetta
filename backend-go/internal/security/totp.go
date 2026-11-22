package security

import (
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const totpIssuer = "AuthRosetta"

var totpParams = totp.ValidateOpts{
	Period:    30,
	Skew:      1,
	Digits:    otp.DigitsSix,
	Algorithm: otp.AlgorithmSHA1,
}

type TOTPEnrollment struct {
	Secret string
	URI    string
}

// NewTOTPEnrollment generates a new TOTP enrollment for the given account email.
// It returns the TOTP secret and the corresponding URI.
// If an error occurs during generation, it returns an empty TOTPEnrolment and the error.
func NewTOTPEnrollment(accountEmail string) (TOTPEnrollment, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: accountEmail,
		Period:      totpParams.Period,
		SecretSize:  20,
		Digits:      totpParams.Digits,
		Algorithm:   totpParams.Algorithm,
	})
	if err != nil {
		return TOTPEnrollment{}, fmt.Errorf("generating TOTP secret: %w", err)
	}
	return TOTPEnrollment{Secret: key.Secret(), URI: key.URL()}, nil
}

// VerifyTOTP verifies the given TOTP code against the secret at the specified time.
// It returns true if the code is valid, otherwise it returns false.
func VerifyTOTP(secret, code string, at time.Time) bool {
	if !isSixASCIIDigits(code) {
		return false
	}
	ok, err := totp.ValidateCustom(code, secret, at, totpParams)
	return err == nil && ok
}

// isSixASCIIDigits checks if the given string consists of exactly six ASCII digits.
// It returns true if the string is exactly six characters long and all characters are ASCII digits ('0'-'9'), otherwise it returns false.
func isSixASCIIDigits(s string) bool {
	if len(s) != 6 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
