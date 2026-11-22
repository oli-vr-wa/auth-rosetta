package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestSetTokenCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	rec := httptest.NewRecorder()
	setTokenCookie(rec, req, "tok.en.value")
	raw := rec.Header().Get("Set-Cookie")

	// The regex must be exact: a plain "Max-Age=3600" substring check would
	// also pass for "Max-Age=3600.0", which browsers discard.
	if !regexp.MustCompile(`Max-Age=3600(;|$)`).MatchString(raw) {
		t.Errorf("Max-Age is not a whole 3600: %s", raw)
	}
	for _, want := range []string{"access_token=tok.en.value", "Path=/", "HttpOnly", "SameSite=Strict"} {
		if !strings.Contains(raw, want) {
			t.Errorf("Set-Cookie lacks %q: %s", want, raw)
		}
	}
	if strings.Contains(raw, "Secure") {
		t.Errorf("a plain-HTTP response must not set Secure: %s", raw)
	}
}

func TestSecureFlagFollowsTheScheme(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	setTokenCookie(rec, req, "tok")
	if raw := rec.Header().Get("Set-Cookie"); !strings.Contains(raw, "Secure") {
		t.Errorf("an HTTPS response must set Secure: %s", raw)
	}
}

func TestClearTokenCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()
	clearTokenCookie(rec, req)
	raw := rec.Header().Get("Set-Cookie")

	if !regexp.MustCompile(`^access_token=;`).MatchString(raw) {
		t.Errorf("the value must be empty: %s", raw)
	}
	if !regexp.MustCompile(`Max-Age=0(;|$)`).MatchString(raw) {
		t.Errorf("clearing needs Max-Age=0: %s", raw)
	}
	for _, want := range []string{"Path=/", "HttpOnly", "SameSite=Strict"} {
		if !strings.Contains(raw, want) {
			t.Errorf("the clearing cookie lacks %q: %s", want, raw)
		}
	}
}

func TestTokenFromRequest(t *testing.T) {
	plain := httptest.NewRequest(http.MethodGet, "/me", nil)
	if token, ok := tokenFromRequest(plain); ok || token != "" {
		t.Errorf("no cookie: got (%q, %v)", token, ok)
	}

	withCookie := httptest.NewRequest(http.MethodGet, "/me", nil)
	withCookie.AddCookie(&http.Cookie{Name: cookieName, Value: "abc"})
	if token, ok := tokenFromRequest(withCookie); !ok || token != "abc" {
		t.Errorf("with cookie: got (%q, %v)", token, ok)
	}

	empty := httptest.NewRequest(http.MethodGet, "/me", nil)
	empty.AddCookie(&http.Cookie{Name: cookieName, Value: ""})
	if _, ok := tokenFromRequest(empty); ok {
		t.Error("an empty cookie value must count as no token")
	}
}
