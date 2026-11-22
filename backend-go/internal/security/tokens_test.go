package security

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testKey = "0123456789abcdef0123456789abcdef" // 32 bytes

var t0 = time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)

func tokensAt(t *testing.T, key string, at time.Time) *Tokens {
	t.Helper()
	tk, err := NewTokens(key, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

// signRaw signs arbitrary claims, bypassing our Create, to build bad tokens.
func signRaw(t *testing.T, method jwt.SigningMethod, key string, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validMap() jwt.MapClaims {
	return jwt.MapClaims{
		"sub": "user-1", "email": "a@b.com", "jti": "jti-1",
		"iss": "AuthRosetta-Go", "aud": "AuthRosetta",
		"exp": t0.Add(time.Hour).Unix(),
	}
}

func without(m jwt.MapClaims, key string) jwt.MapClaims {
	delete(m, key)
	return m
}

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func TestCreateThenDecode(t *testing.T) {
	tk := tokensAt(t, testKey, t0)
	token, err := tk.Create("user-1", "a@b.com")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := tk.Decode(token)
	if !ok {
		t.Fatal("a fresh token was rejected")
	}
	if got.UserID != "user-1" || got.Email != "a@b.com" || got.JTI == "" {
		t.Errorf("claims = %+v", got)
	}
	if want := t0.Add(60 * time.Minute); !got.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, want)
	}
}

func TestPayloadMatchesTheContract(t *testing.T) {
	token, _ := tokensAt(t, testKey, t0).Create("user-1", "a@b.com")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts", len(parts))
	}

	var header map[string]any
	headerJSON, _ := base64.RawURLEncoding.DecodeString(parts[0])
	json.Unmarshal(headerJSON, &header)
	if header["alg"] != "HS256" {
		t.Errorf("alg = %v, want HS256", header["alg"])
	}

	var payload map[string]any
	payloadJSON, _ := base64.RawURLEncoding.DecodeString(parts[1])
	json.Unmarshal(payloadJSON, &payload)

	if len(payload) != 6 {
		t.Errorf("payload has %d claims, want exactly 6: %v", len(payload), payload)
	}
	for key, want := range map[string]any{
		"sub": "user-1", "email": "a@b.com",
		"iss": "AuthRosetta-Go", "aud": "AuthRosetta", // aud must be a string
	} {
		if payload[key] != want {
			t.Errorf("%s = %v (%T), want %v", key, payload[key], payload[key], want)
		}
	}
	if exp, _ := payload["exp"].(float64); int64(exp) != t0.Add(time.Hour).Unix() {
		t.Errorf("exp = %v", payload["exp"])
	}
	if jti, _ := payload["jti"].(string); jti == "" {
		t.Error("jti missing")
	}
}

func TestEveryTokenHasAUniqueJTI(t *testing.T) {
	tk := tokensAt(t, testKey, t0)
	a, _ := tk.Create("user-1", "a@b.com")
	b, _ := tk.Create("user-1", "a@b.com")
	ca, _ := tk.Decode(a)
	cb, _ := tk.Decode(b)
	if ca.JTI == cb.JTI {
		t.Error("two tokens share a jti")
	}
}

func TestExpiryWithThirtySecondLeeway(t *testing.T) {
	token, _ := tokensAt(t, testKey, t0).Create("user-1", "a@b.com")
	expiry := t0.Add(TokenLifetime)

	for _, tc := range []struct {
		name string
		at   time.Time
		want bool
	}{
		{"just created", t0, true},
		{"one second before expiry", expiry.Add(-time.Second), true},
		{"29 s after expiry (inside leeway)", expiry.Add(29 * time.Second), true},
		{"31 s after expiry (outside leeway)", expiry.Add(31 * time.Second), false},
	} {
		_, ok := tokensAt(t, testKey, tc.at).Decode(token)
		if ok != tc.want {
			t.Errorf("%s: valid = %v, want %v", tc.name, ok, tc.want)
		}
	}
}

func TestForgedAndBrokenTokensAreRejected(t *testing.T) {
	tk := tokensAt(t, testKey, t0)
	good, _ := tk.Create("user-1", "a@b.com")
	parts := strings.Split(good, ".")

	otherKey := "ffffffffffffffffffffffffffffffff"
	noneToken := b64(`{"alg":"none","typ":"JWT"}`) + "." +
		b64(`{"sub":"user-1","email":"a@b.com","jti":"j","iss":"AuthRosetta-Go","aud":"AuthRosetta","exp":9999999999}`) + "."
	tampered := parts[0] + "." +
		b64(`{"sub":"admin","email":"evil@b.com","jti":"j","iss":"AuthRosetta-Go","aud":"AuthRosetta","exp":9999999999}`) +
		"." + parts[2]

	tests := map[string]string{
		"alg none":                 noneToken,
		"wrong key":                signRaw(t, jwt.SigningMethodHS256, otherKey, validMap()),
		"HS384 with the right key": signRaw(t, jwt.SigningMethodHS384, testKey, validMap()),
		"tampered payload":         tampered,
		"wrong issuer":             signRaw(t, jwt.SigningMethodHS256, testKey, func() jwt.MapClaims { m := validMap(); m["iss"] = "Other"; return m }()),
		"wrong audience":           signRaw(t, jwt.SigningMethodHS256, testKey, func() jwt.MapClaims { m := validMap(); m["aud"] = "Other"; return m }()),
		"no exp":                   signRaw(t, jwt.SigningMethodHS256, testKey, without(validMap(), "exp")),
		"no jti":                   signRaw(t, jwt.SigningMethodHS256, testKey, without(validMap(), "jti")),
		"no sub":                   signRaw(t, jwt.SigningMethodHS256, testKey, without(validMap(), "sub")),
		"no email":                 signRaw(t, jwt.SigningMethodHS256, testKey, without(validMap(), "email")),
		"sub is a number":          signRaw(t, jwt.SigningMethodHS256, testKey, func() jwt.MapClaims { m := validMap(); m["sub"] = 42; return m }()),
		"empty string":             "",
		"not a token":              "hello",
		"two parts":                "a.b",
		"empty parts":              "..",
		"garbage parts":            "a.b.c",
		"100000 dots":              strings.Repeat(".", 100000),
	}
	for name, token := range tests {
		if _, ok := tk.Decode(token); ok {
			t.Errorf("%s: token was accepted", name)
		}
	}
}

func TestSanityOfTheBadTokenBuilder(t *testing.T) {
	// If signRaw/validMap were wrong, every rejection above would pass for the
	// wrong reason. A correctly built token must be accepted.
	token := signRaw(t, jwt.SigningMethodHS256, testKey, validMap())
	if _, ok := tokensAt(t, testKey, t0).Decode(token); !ok {
		t.Error("the helper's valid token was rejected, so the rejection tests prove nothing")
	}
}

func TestShortKeyIsRefused(t *testing.T) {
	if _, err := NewTokens("too-short", time.Now); err == nil {
		t.Error("a short key must be refused")
	}
}
