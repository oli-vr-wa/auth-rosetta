package security

import (
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// Base32 of the ASCII string "12345678901234567890", the secret used by
// the RFC 6238 test vectors (SHA-1). The RFC lists 8-digit codes; the
// 6-digit codes below are their last six digits.
const rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func unix(sec int64) time.Time { return time.Unix(sec, 0) }

func TestRFC6238Vectors(t *testing.T) {
	tests := []struct {
		at   int64
		code string
	}{
		{59, "287082"},         // RFC: 94287082
		{1111111109, "081804"}, // RFC: 07081804 (leading zero kept)
		{1234567890, "005924"}, // RFC: 89005924 (two leading zeros)
	}
	for _, tc := range tests {
		if !VerifyTOTP(rfcSecret, tc.code, unix(tc.at)) {
			t.Errorf("code %s should be valid at t=%d", tc.code, tc.at)
		}
	}
}

func TestSkewWindowIsPlusMinusOneStep(t *testing.T) {
	const code = "287082" // the code for t=59 (step 1)
	for _, tc := range []struct {
		at   int64
		want bool
	}{
		{29, true},   // step 0: one step early
		{59, true},   // step 1: exact
		{89, true},   // step 2: one step late
		{119, false}, // step 3: two steps late
	} {
		if got := VerifyTOTP(rfcSecret, code, unix(tc.at)); got != tc.want {
			t.Errorf("t=%d: valid = %v, want %v", tc.at, got, tc.want)
		}
	}
}

func TestMalformedCodesAreRejected(t *testing.T) {
	bad := []string{
		"", "28708", "2870820", " 287082", "287082 ", "28708a", "+87082",
		"٢٨٧٠٨٢", // Arabic-Indic digits
	}
	for _, code := range bad {
		if VerifyTOTP(rfcSecret, code, unix(59)) {
			t.Errorf("code %q must be rejected", code)
		}
	}
	if VerifyTOTP("not base32!", "287082", unix(59)) {
		t.Error("an invalid secret must not verify")
	}
}

func TestEnrollmentSecretAndURI(t *testing.T) {
	e, err := NewTOTPEnrollment("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[A-Z2-7]{32}$`).MatchString(e.Secret) {
		t.Errorf("secret %q is not 32 Base32 characters", e.Secret)
	}

	u, err := url.Parse(e.URI)
	if err != nil {
		t.Fatalf("URI does not parse: %v", err)
	}
	q := u.Query()
	if u.Scheme != "otpauth" || u.Host != "totp" {
		t.Errorf("unexpected URI start: %s://%s", u.Scheme, u.Host)
	}
	for key, want := range map[string]string{
		"issuer": "AuthRosetta-Go", "secret": e.Secret,
		"algorithm": "SHA1", "digits": "6", "period": "30",
	} {
		if got := q.Get(key); got != want {
			t.Errorf("URI %s = %q, want %q", key, got, want)
		}
	}
	if !regexp.MustCompile(`alice@example\.com`).MatchString(u.Path) {
		t.Errorf("account name missing from path %q", u.Path)
	}

	// A different secret every time.
	other, _ := NewTOTPEnrollment("alice@example.com")
	if other.Secret == e.Secret {
		t.Error("two enrollments produced the same secret")
	}
}

func TestGeneratedSecretVerifiesItsOwnCode(t *testing.T) {
	e, err := NewTOTPEnrollment("bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	code, err := totp.GenerateCodeCustom(e.Secret, now, totpParams)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyTOTP(e.Secret, code, now) {
		t.Errorf("fresh code %s rejected", code)
	}
}
