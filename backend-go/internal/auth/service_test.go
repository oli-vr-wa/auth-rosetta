package auth

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"authrosetta/internal/security"
	"authrosetta/internal/validation"
)

const (
	rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	rfcCode   = "081804" // the RFC 6238 code at the pinned time below
	testKey   = "0123456789abcdef0123456789abcdef"
	testPass  = "correct horse battery"
)

var pinned = time.Unix(1111111109, 0)

// ---- fakes (the Go version of fakes.py) ----

type fakeUsers struct {
	byEmail  map[string]User
	failWith error // makes every call fail
}

func newFakeUsers() *fakeUsers { return &fakeUsers{byEmail: map[string]User{}} }

func (f *fakeUsers) Add(_ context.Context, u User) (bool, error) {
	if f.failWith != nil {
		return false, f.failWith
	}
	if _, exists := f.byEmail[u.Email]; exists {
		return false, nil
	}
	f.byEmail[u.Email] = u
	return true, nil
}

func (f *fakeUsers) FindByEmail(_ context.Context, email string) (User, error) {
	if f.failWith != nil {
		return User{}, f.failWith
	}
	u, ok := f.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeUsers) EnableTwoFactor(_ context.Context, id string) error {
	for email, u := range f.byEmail {
		if u.ID == id {
			u.TwoFactorEnabled = true
			f.byEmail[email] = u
			return nil
		}
	}
	return ErrNotFound
}

// racyUsers says "not found" on lookup but the email is already there when
// Add runs: a registration that lost a race.
type racyUsers struct{ *fakeUsers }

func (racyUsers) FindByEmail(context.Context, string) (User, error) {
	return User{}, ErrNotFound
}

type fakeRevoked struct {
	jtis     map[string]time.Time
	failWith error
}

func newFakeRevoked() *fakeRevoked { return &fakeRevoked{jtis: map[string]time.Time{}} }

func (f *fakeRevoked) Revoke(_ context.Context, jti string, expiresAt time.Time) error {
	if f.failWith != nil {
		return f.failWith
	}
	f.jtis[jti] = expiresAt
	return nil
}

func (f *fakeRevoked) IsRevoked(_ context.Context, jti string) (bool, error) {
	if f.failWith != nil {
		return false, f.failWith
	}
	_, revoked := f.jtis[jti]
	return revoked, nil
}

// ---- helpers ----

func fastPasswords(t *testing.T) *security.Passwords {
	t.Helper()
	p, err := security.NewPasswords(bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newService(t *testing.T, users UserStore, revoked RevokedStore) *Service {
	t.Helper()
	clock := func() time.Time { return pinned }
	tokens, err := security.NewTokens(testKey, clock)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(users, revoked, fastPasswords(t), tokens, clock)
}

// seed puts a user straight into the fake, with the RFC secret so the code
// at the pinned time is known.
func seed(t *testing.T, users *fakeUsers, email, password string, twoFactor bool) User {
	t.Helper()
	hash, err := fastPasswords(t).Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	u := User{
		ID: "user-" + email, Email: email, PasswordHash: hash,
		TOTPSecret: rfcSecret, TwoFactorEnabled: twoFactor, CreatedAt: pinned,
	}
	users.byEmail[email] = u
	return u
}

var ctx = context.Background()

// ---- register ----

func TestRegisterStoresANormalizedUserAndReturnsEnrollment(t *testing.T) {
	users := newFakeUsers()
	svc := newService(t, users, newFakeRevoked())

	res, err := svc.Register(ctx, "  Alice@Example.COM ", testPass)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(res.Secret) != 32 || !strings.HasPrefix(res.OtpauthURI, "otpauth://totp/") {
		t.Errorf("unexpected enrollment: %+v", res)
	}
	u, ok := users.byEmail["alice@example.com"]
	if !ok {
		t.Fatalf("user not stored under the normalized email: %v", users.byEmail)
	}
	if u.PasswordHash == testPass || !strings.HasPrefix(u.PasswordHash, "$2") {
		t.Errorf("password not hashed: %q", u.PasswordHash)
	}
	if u.TwoFactorEnabled {
		t.Error("2FA must stay off until confirmed")
	}
	if u.TOTPSecret != res.Secret {
		t.Error("stored secret differs from the returned one")
	}
}

func TestRegisterDuplicatesInAnyCaseOrSpacing(t *testing.T) {
	users := newFakeUsers()
	svc := newService(t, users, newFakeRevoked())
	if _, err := svc.Register(ctx, "alice@example.com", testPass); err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"alice@example.com", "ALICE@example.com", "  alice@example.com  "} {
		if _, err := svc.Register(ctx, email, testPass); !errors.Is(err, ErrEmailTaken) {
			t.Errorf("%q: err = %v, want ErrEmailTaken", email, err)
		}
	}
}

func TestRegisterRejectsBadInputAndStoresNothing(t *testing.T) {
	bad := []struct{ name, email, password string }{
		{"empty email", "", testPass},
		{"short password", "a@b.com", "short"},
		{"73-byte password", "a@b.com", strings.Repeat("a", 73)},
		{"NUL in email", "a\x00@b.com", testPass},
		{"invalid UTF-8 password", "a@b.com", "\xff\xfe\xfd\xfc\xfb\xfa\xf9\xf8"},
	}
	for _, tc := range bad {
		users := newFakeUsers()
		svc := newService(t, users, newFakeRevoked())
		_, err := svc.Register(ctx, tc.email, tc.password)
		var ve *validation.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a ValidationError", tc.name, err)
		}
		if len(users.byEmail) != 0 {
			t.Errorf("%s: a user was stored", tc.name)
		}
	}
}

func TestRegisterLostRaceIsEmailTaken(t *testing.T) {
	users := newFakeUsers()
	seed(t, users, "alice@example.com", testPass, false)
	svc := newService(t, racyUsers{users}, newFakeRevoked())

	_, err := svc.Register(ctx, "alice@example.com", testPass)
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("err = %v, want ErrEmailTaken", err)
	}
}

func TestRegisterStoreFailureIsNotEmailTaken(t *testing.T) {
	users := newFakeUsers()
	users.failWith = errors.New("database down")
	svc := newService(t, users, newFakeRevoked())

	_, err := svc.Register(ctx, "alice@example.com", testPass)
	if err == nil || errors.Is(err, ErrEmailTaken) {
		t.Errorf("err = %v, want a plain failure", err)
	}
}

// ---- confirm ----

func TestConfirmFailuresAreIndistinguishable(t *testing.T) {
	users := newFakeUsers()
	seed(t, users, "alice@example.com", testPass, false)
	svc := newService(t, users, newFakeRevoked())

	cases := []struct{ name, email, code string }{
		{"wrong code", "alice@example.com", "000000"},
		{"too short", "alice@example.com", "08180"},
		{"too long", "alice@example.com", "0818044"},
		{"non-ASCII digits", "alice@example.com", "٠٨١٨٠٤"},
		{"unknown email", "nobody@example.com", rfcCode},
		{"NUL in email", "alice\x00@example.com", rfcCode},
	}
	for _, tc := range cases {
		if err := svc.ConfirmTwoFactor(ctx, tc.email, tc.code); err != ErrInvalidCode {
			t.Errorf("%s: err = %v, want exactly ErrInvalidCode", tc.name, err)
		}
	}
	if users.byEmail["alice@example.com"].TwoFactorEnabled {
		t.Error("a failed confirm enabled 2FA")
	}
}

func TestConfirmWithTheRightCodeEnablesTwoFactor(t *testing.T) {
	users := newFakeUsers()
	seed(t, users, "alice@example.com", testPass, false)
	svc := newService(t, users, newFakeRevoked())

	if err := svc.ConfirmTwoFactor(ctx, "  ALICE@example.com ", rfcCode); err != nil {
		t.Fatalf("ConfirmTwoFactor: %v", err)
	}
	if !users.byEmail["alice@example.com"].TwoFactorEnabled {
		t.Error("2FA not enabled")
	}
}

// ---- login ----

func TestLoginSucceedsAndTheTokenAuthenticates(t *testing.T) {
	users := newFakeUsers()
	seed(t, users, "alice@example.com", testPass, true)
	svc := newService(t, users, newFakeRevoked())

	token, err := svc.Login(ctx, "  ALICE@example.com ", testPass, rfcCode)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	claims, err := svc.Authenticate(ctx, token)
	if err != nil || claims.Email != "alice@example.com" {
		t.Errorf("Authenticate = %+v, %v", claims, err)
	}
}

func TestEveryLoginFailureIsTheSameError(t *testing.T) {
	users := newFakeUsers()
	seed(t, users, "alice@example.com", testPass, true)
	seed(t, users, "pending@example.com", testPass, false) // never confirmed 2FA
	svc := newService(t, users, newFakeRevoked())

	cases := []struct{ name, email, password, code string }{
		{"unknown email", "nobody@example.com", testPass, rfcCode},
		{"wrong password", "alice@example.com", "wrong password", rfcCode},
		{"wrong code", "alice@example.com", testPass, "000000"},
		{"2FA not confirmed", "pending@example.com", testPass, rfcCode},
		{"NUL in email", "alice\x00@example.com", testPass, rfcCode},
		{"73-byte password", "alice@example.com", strings.Repeat("a", 73), rfcCode},
		{"invalid UTF-8 password", "alice@example.com", "\xff\xfe\xfd\xfc", rfcCode},
		{"everything empty", "", "", ""},
	}
	for _, tc := range cases {
		token, err := svc.Login(ctx, tc.email, tc.password, tc.code)
		if err != ErrInvalidCredentials || token != "" {
			t.Errorf("%s: got (%q, %v), want (\"\", ErrInvalidCredentials)", tc.name, token, err)
		}
	}
}

func TestLoginStoreFailureIsNotAnAuthFailure(t *testing.T) {
	users := newFakeUsers()
	users.failWith = errors.New("database down")
	svc := newService(t, users, newFakeRevoked())

	_, err := svc.Login(ctx, "alice@example.com", testPass, rfcCode)
	if err == nil || errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v; a database failure must not look like bad credentials", err)
	}
}

func TestLoginLogsNoSecrets(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	users := newFakeUsers()
	seed(t, users, "alice@example.com", testPass, true)
	svc := newService(t, users, newFakeRevoked())
	svc.Login(ctx, "alice@example.com", testPass, rfcCode)     // success
	svc.Login(ctx, "alice@example.com", "wrong password", "1") // failure

	out := logs.String()
	if out == "" {
		t.Fatal("no log output was captured, so this test proves nothing")
	}
	for _, secret := range []string{"alice@example.com", testPass, "wrong password", rfcCode, rfcSecret} {
		if strings.Contains(out, secret) {
			t.Errorf("logs contain %q:\n%s", secret, out)
		}
	}
}

// ---- authenticate and logout ----

func TestLogoutRevokesTheToken(t *testing.T) {
	users := newFakeUsers()
	seed(t, users, "alice@example.com", testPass, true)
	svc := newService(t, users, newFakeRevoked())

	token, _ := svc.Login(ctx, "alice@example.com", testPass, rfcCode)
	claims, err := svc.Authenticate(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // logging out twice is fine
		if err := svc.Logout(ctx, claims); err != nil {
			t.Fatalf("Logout #%d: %v", i+1, err)
		}
	}
	if _, err := svc.Authenticate(ctx, token); err != ErrUnauthorized {
		t.Errorf("replayed token: err = %v, want ErrUnauthorized", err)
	}
}

func TestAuthenticateRejectsGarbage(t *testing.T) {
	svc := newService(t, newFakeUsers(), newFakeRevoked())
	for _, token := range []string{"", "garbage", "a.b.c"} {
		if _, err := svc.Authenticate(ctx, token); err != ErrUnauthorized {
			t.Errorf("%q: err = %v, want ErrUnauthorized", token, err)
		}
	}
}

func TestAuthenticateFailsClosedWhenTheDenylistIsDown(t *testing.T) {
	users := newFakeUsers()
	seed(t, users, "alice@example.com", testPass, true)
	revoked := newFakeRevoked()
	svc := newService(t, users, revoked)
	token, _ := svc.Login(ctx, "alice@example.com", testPass, rfcCode)

	revoked.failWith = errors.New("database down")
	claims, err := svc.Authenticate(ctx, token)
	if err == nil || errors.Is(err, ErrUnauthorized) || claims.JTI != "" {
		t.Errorf("got (%+v, %v); want an error that is not ErrUnauthorized and no claims", claims, err)
	}
}
