package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"authrosetta/internal/auth"
	"authrosetta/internal/security"
	"authrosetta/internal/validation"
)

// ---- a fake service (like the fakes in the Python tests) ----

type fakeService struct {
	calls                             int // how often the service was reached
	lastEmail, lastPassword, lastCode string
	loggedOutJTI                      string
	registerErr, confirmErr, loginErr error
	authErr, logoutErr                error
}

func newFake() *fakeService { return &fakeService{} }

func (f *fakeService) Register(_ context.Context, email, password string) (auth.RegisterResult, error) {
	f.calls++
	f.lastEmail, f.lastPassword = email, password
	if f.registerErr != nil {
		return auth.RegisterResult{}, f.registerErr
	}
	return auth.RegisterResult{OtpauthURI: "otpauth://totp/test", Secret: "SECRET"}, nil
}

func (f *fakeService) ConfirmTwoFactor(_ context.Context, email, code string) error {
	f.calls++
	f.lastEmail, f.lastCode = email, code
	return f.confirmErr
}

func (f *fakeService) Login(_ context.Context, email, password, code string) (string, error) {
	f.calls++
	f.lastEmail, f.lastPassword, f.lastCode = email, password, code
	if f.loginErr != nil {
		return "", f.loginErr
	}
	return "jwt-token", nil
}

func (f *fakeService) Authenticate(_ context.Context, token string) (security.TokenClaims, error) {
	f.calls++
	if f.authErr != nil {
		return security.TokenClaims{}, f.authErr
	}
	if token != "good-token" {
		return security.TokenClaims{}, auth.ErrUnauthorized
	}
	return security.TokenClaims{UserID: "u1", Email: "a@b.com", JTI: "j1", ExpiresAt: time.Unix(2000000000, 0)}, nil
}

func (f *fakeService) Logout(_ context.Context, claims security.TokenClaims) error {
	f.calls++
	f.loggedOutJTI = claims.JTI
	return f.logoutErr
}

// ---- helpers ----

const (
	registerPath = "/api/auth/register"
	confirmPath  = "/api/auth/2fa/confirm"
	loginPath    = "/api/auth/login"
	mePath       = "/api/auth/me"
	logoutPath   = "/api/auth/logout"

	validRegister = `{"email":"a@b.com","password":"password123"}`
	validConfirm  = `{"email":"a@b.com","code":"123456"}`
	validLogin    = `{"email":"a@b.com","password":"password123","code":"123456"}`
)

func request(f *fakeService, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Origin", testOrigin)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	}
	return serve(NewRouter(f, testOrigin), req)
}

func findCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName {
			return c
		}
	}
	return nil
}

// objectWith builds a JSON object where every field is "x", except that
// `field` gets the raw JSON text `raw` (or is left out when raw is "").
func objectWith(fields []string, field, raw string) string {
	var members []string
	for _, f := range fields {
		value := `"x"`
		if f == field {
			if raw == "" {
				continue
			}
			value = raw
		}
		members = append(members, `"`+f+`":`+value)
	}
	return "{" + strings.Join(members, ",") + "}"
}

func malformedBodies(fields ...string) []string {
	bodies := []string{`{bad`, ``, `{}`, `null`, `[]`, `"text"`}
	for _, f := range fields {
		for _, raw := range []string{"", "null", "123", "true", `["x"]`} {
			bodies = append(bodies, objectWith(fields, f, raw))
		}
	}
	return bodies
}

// unrepresentableBodies put text into each field that strict JSON decoding
// refuses: lone surrogates and invalid UTF-8 bytes.
func unrepresentableBodies(fields ...string) []string {
	var bodies []string
	for _, bad := range []string{`"\ud800"`, `"\udc00"`, "\"\xff\xfe\""} {
		for _, f := range fields {
			bodies = append(bodies, objectWith(fields, f, bad))
		}
	}
	return bodies
}

func shorten(s string) string {
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}

// ---- register ----

func TestRegisterSuccess(t *testing.T) {
	f := newFake()
	rec := request(f, http.MethodPost, registerPath, validRegister, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 2 || body["otpauthUri"] == "" || body["secret"] != "SECRET" {
		t.Errorf("body = %v, want exactly otpauthUri and secret", body)
	}
	if f.lastEmail != "a@b.com" || f.lastPassword != "password123" {
		t.Errorf("service got (%q, %q)", f.lastEmail, f.lastPassword)
	}
}

func TestRegisterMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"validation", &validation.ValidationError{Message: "Email is required."}, 400, "Email is required."},
		{"email taken", auth.ErrEmailTaken, 409, MsgEmailTaken},
		{"wrapped email taken", fmt.Errorf("registering: %w", auth.ErrEmailTaken), 409, MsgEmailTaken},
		{"unexpected failure", errors.New("dial 10.0.0.5: password=hunter2"), 500, MsgInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			f.registerErr = tc.err
			rec := request(f, http.MethodPost, registerPath, validRegister, "")
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			assertErrorBody(t, rec, tc.message)
			if strings.Contains(rec.Body.String(), "hunter2") {
				t.Error("an internal error message leaked to the client")
			}
		})
	}
}

// ---- malformed and odd bodies ----

func TestMalformedBodiesAre400AndNeverReachTheService(t *testing.T) {
	endpoints := []struct {
		path   string
		fields []string
	}{
		{registerPath, []string{"email", "password"}},
		{confirmPath, []string{"email", "code"}},
		{loginPath, []string{"email", "password", "code"}},
	}
	for _, ep := range endpoints {
		bodies := malformedBodies(ep.fields...)
		bodies = append(bodies, `{"email":"`+strings.Repeat("a", MaxBodyBytes)+`"}`) // over 8 KiB

		for _, body := range bodies {
			f := newFake()
			rec := request(f, http.MethodPost, ep.path, body, "")
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s %q: status = %d, want 400", ep.path, shorten(body), rec.Code)
				continue
			}
			assertErrorBody(t, rec, MsgInvalidRequest)
			if f.calls != 0 {
				t.Errorf("%s %q: the service was called", ep.path, shorten(body))
			}
		}
	}
}

func TestLoginWithUnrepresentableTextIsAGeneric401(t *testing.T) {
	for _, body := range unrepresentableBodies("email", "password", "code") {
		f := newFake()
		rec := request(f, http.MethodPost, loginPath, body, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%q: status = %d, want 401", body, rec.Code)
			continue
		}
		assertErrorBody(t, rec, MsgInvalidCredentials)
		if f.calls != 0 || findCookie(rec) != nil {
			t.Errorf("%q: the service was reached or a cookie was set", body)
		}
	}
}

func TestConfirmWithUnrepresentableTextIsInvalidEmailOrCode(t *testing.T) {
	for _, body := range unrepresentableBodies("email", "code") {
		f := newFake()
		rec := request(f, http.MethodPost, confirmPath, body, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want 400", body, rec.Code)
			continue
		}
		assertErrorBody(t, rec, MsgInvalidEmailOrCode)
		if f.calls != 0 {
			t.Errorf("%q: the service was called", body)
		}
	}
}

func TestRegisterWithUnrepresentableTextIsInvalidRequest(t *testing.T) {
	for _, body := range unrepresentableBodies("email", "password") {
		f := newFake()
		rec := request(f, http.MethodPost, registerPath, body, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want 400", body, rec.Code)
			continue
		}
		assertErrorBody(t, rec, MsgInvalidRequest)
		if f.calls != 0 {
			t.Errorf("%q: the service was called", body)
		}
	}
}

func TestBadTextDoesNotHideOtherProblems(t *testing.T) {
	for _, body := range []string{
		`{"password":"\ud800","code":"1"}`,                         // email missing
		`{"email":123,"password":"\ud800","code":"1"}`,             // wrong type
		`{"email":"a","password":"\ud800","code":"1"`,              // broken JSON
		`{"email":"a","email":"b","password":"\ud800","code":"1"}`, // duplicate names
	} {
		f := newFake()
		rec := request(f, http.MethodPost, loginPath, body, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want 400", body, rec.Code)
			continue
		}
		assertErrorBody(t, rec, MsgInvalidRequest)
	}
}

func TestNULReachesTheServiceIntact(t *testing.T) {
	// NUL is valid JSON and valid UTF-8, so the handler passes it on whole;
	// the service decides what it means.
	f := newFake()
	f.loginErr = auth.ErrInvalidCredentials
	body := `{"email":"a\u0000b@c.com","password":"p\u0000q","code":"12\u0000"}`

	rec := request(f, http.MethodPost, loginPath, body, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if f.calls != 1 || f.lastEmail != "a\x00b@c.com" || f.lastPassword != "p\x00q" {
		t.Errorf("service saw (%q, %q), calls = %d", f.lastEmail, f.lastPassword, f.calls)
	}
}

// ---- confirm ----

func TestConfirm(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rec := request(newFake(), http.MethodPost, confirmPath, validConfirm, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != `{"message":"Two-factor authentication enabled"}` {
			t.Errorf("body = %s", got)
		}
	})
	t.Run("wrong code", func(t *testing.T) {
		f := newFake()
		f.confirmErr = auth.ErrInvalidCode
		rec := request(f, http.MethodPost, confirmPath, validConfirm, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		assertErrorBody(t, rec, MsgInvalidEmailOrCode)
	})
	t.Run("unexpected failure", func(t *testing.T) {
		f := newFake()
		f.confirmErr = errors.New("db down")
		rec := request(f, http.MethodPost, confirmPath, validConfirm, "")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		assertErrorBody(t, rec, MsgInternal)
	})
}

// ---- login ----

func TestLoginSuccessSetsTheCookie(t *testing.T) {
	rec := request(newFake(), http.MethodPost, loginPath, validLogin, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"message":"Login successful"}` {
		t.Errorf("body = %s", got)
	}
	c := findCookie(rec)
	if c == nil {
		t.Fatal("no access_token cookie")
	}
	if c.Value != "jwt-token" || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode ||
		c.MaxAge != 3600 || c.Path != "/" {
		t.Errorf("cookie = %+v", c)
	}
}

func TestLoginFailures(t *testing.T) {
	t.Run("bad credentials", func(t *testing.T) {
		f := newFake()
		f.loginErr = auth.ErrInvalidCredentials
		rec := request(f, http.MethodPost, loginPath, validLogin, "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		assertErrorBody(t, rec, MsgInvalidCredentials)
		assertCORS(t, rec) // errors from real routes carry CORS headers too
		if findCookie(rec) != nil {
			t.Error("a failed login set a cookie")
		}
	})
	t.Run("unexpected failure is a 500, not a 401", func(t *testing.T) {
		f := newFake()
		f.loginErr = errors.New("db down")
		rec := request(f, http.MethodPost, loginPath, validLogin, "")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		assertErrorBody(t, rec, MsgInternal)
		if findCookie(rec) != nil {
			t.Error("a failed login set a cookie")
		}
	})
}

// ---- me and logout ----

func TestMe(t *testing.T) {
	t.Run("no cookie", func(t *testing.T) {
		f := newFake()
		rec := request(f, http.MethodGet, mePath, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		assertErrorBody(t, rec, MsgUnauthorized)
		if f.calls != 0 {
			t.Error("the service was called without a token")
		}
	})
	t.Run("bad token", func(t *testing.T) {
		rec := request(newFake(), http.MethodGet, mePath, "", "garbage")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		assertErrorBody(t, rec, MsgUnauthorized)
	})
	t.Run("good token", func(t *testing.T) {
		rec := request(newFake(), http.MethodGet, mePath, "", "good-token")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != `{"email":"a@b.com"}` {
			t.Errorf("body = %s", got)
		}
	})
	t.Run("denylist failure is a 500, not a 401", func(t *testing.T) {
		f := newFake()
		f.authErr = errors.New("db down")
		rec := request(f, http.MethodGet, mePath, "", "good-token")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		assertErrorBody(t, rec, MsgInternal)
	})
}

func TestLogout(t *testing.T) {
	t.Run("requires a token", func(t *testing.T) {
		rec := request(newFake(), http.MethodPost, logoutPath, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		assertErrorBody(t, rec, MsgUnauthorized)
	})
	t.Run("success clears the cookie", func(t *testing.T) {
		f := newFake()
		rec := request(f, http.MethodPost, logoutPath, "", "good-token")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("a 204 must have no body, got %q", rec.Body.String())
		}
		if f.loggedOutJTI != "j1" {
			t.Errorf("service logged out %q, want j1", f.loggedOutJTI)
		}
		if raw := rec.Header().Get("Set-Cookie"); !strings.Contains(raw, "Max-Age=0") {
			t.Errorf("the cookie is not cleared: %s", raw)
		}
	})
	t.Run("failure keeps the cookie and is a 500", func(t *testing.T) {
		f := newFake()
		f.logoutErr = errors.New("db down")
		rec := request(f, http.MethodPost, logoutPath, "", "good-token")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		assertErrorBody(t, rec, MsgInternal)
		if findCookie(rec) != nil {
			t.Error("the cookie was cleared although the token was not revoked")
		}
	})
}

func TestWrongMethodOnARealRouteIsAJSONError(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, loginPath},
		{http.MethodGet, registerPath},
		{http.MethodPost, mePath},
	} {
		rec := request(newFake(), tc.method, tc.path, "", "")
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status = %d, want 404 or 405", tc.method, tc.path, rec.Code)
			continue
		}
		assertErrorBody(t, rec, "")
	}
}
