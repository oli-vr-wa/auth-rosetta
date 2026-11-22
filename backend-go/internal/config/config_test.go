package config

import (
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeEnv(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func validVars() map[string]string {
	return map[string]string{
		"DB_USER":     "user",
		"DB_PASSWORD": "pw",
		"JWT_KEY":     strings.Repeat("k", 32),
	}
}

func TestDefaults(t *testing.T) {
	s, err := parse(fakeEnv(validVars()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DBHost != "localhost" || s.DBPort != 5432 || s.DBName != "auth_go" {
		t.Errorf("unexpected DB defaults: %v", s)
	}
	if s.FrontendOrigin != DefaultFrontendOrigin {
		t.Errorf("FrontendOrigin = %q", s.FrontendOrigin)
	}
}

func TestEmptyValueCountsAsUnset(t *testing.T) {
	vars := validVars()
	vars["FRONTEND_ORIGIN"] = ""
	s, err := parse(fakeEnv(vars))
	if err != nil || s.FrontendOrigin != DefaultFrontendOrigin {
		t.Errorf("got %q, %v", s.FrontendOrigin, err)
	}
}

func TestMissingRequiredReportsAll(t *testing.T) {
	_, err := parse(fakeEnv(map[string]string{}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"DB_USER", "DB_PASSWORD", "JWT_KEY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s: %v", want, err)
		}
	}
}

func TestShortJWTKeyRejectedWithoutEchoingIt(t *testing.T) {
	vars := validVars()
	vars["JWT_KEY"] = "short-secret-value"
	_, err := parse(fakeEnv(vars))
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "short-secret-value") {
		t.Errorf("error leaks the key: %v", err)
	}
}

func TestFrontendOrigin(t *testing.T) {
	tests := []struct {
		origin  string
		wantErr bool
	}{
		{"http://localhost:5173", false},
		{"https://app.example.com", false},
		{"*", true},
		{"http://localhost:5173/", true},
		{"http://localhost:5173/app", true},
		{"localhost:5173", true},
		{"ftp://example.com", true},
		{"http://*.example.com", true},
	}
	for _, tc := range tests {
		t.Run(tc.origin, func(t *testing.T) {
			vars := validVars()
			vars["FRONTEND_ORIGIN"] = tc.origin
			_, err := parse(fakeEnv(vars))
			if (err != nil) != tc.wantErr {
				t.Errorf("origin %q: err = %v, wantErr %v", tc.origin, err, tc.wantErr)
			}
		})
	}
}

func TestBadPortAndLogLevel(t *testing.T) {
	bad := []struct{ key, value string }{
		{"DB_PORT", "abc"},
		{"DB_PORT", "0"},
		{"DB_PORT", "70000"},
		{"LOG_LEVEL", "info"}, // must be upper case
	}
	for _, tc := range bad {
		vars := validVars()
		vars[tc.key] = tc.value
		if _, err := parse(fakeEnv(vars)); err == nil {
			t.Errorf("%s=%q should be rejected", tc.key, tc.value)
		}
	}
}

func TestDatabaseURLEscapesPassword(t *testing.T) {
	vars := validVars()
	vars["DB_PASSWORD"] = "p@ss/w:rd#?"
	s, err := parse(fakeEnv(vars))
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(s.DatabaseURL())
	if err != nil {
		t.Fatalf("DatabaseURL is not a valid URL: %v", err)
	}
	if pw, _ := u.User.Password(); pw != "p@ss/w:rd#?" {
		t.Errorf("password did not round-trip: %q", pw)
	}
	if u.Hostname() != "localhost" || u.Path != "/auth_go" {
		t.Errorf("unexpected URL: host=%q path=%q", u.Hostname(), u.Path)
	}
}

func TestStringRedactsSecrets(t *testing.T) {
	vars := validVars()
	vars["DB_PASSWORD"] = "super-secret-pw"
	s, err := parse(fakeEnv(vars))
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{fmt.Sprintf("%v", s), fmt.Sprintf("%+v", s)} {
		if strings.Contains(out, "super-secret-pw") || strings.Contains(out, vars["JWT_KEY"]) {
			t.Errorf("secrets leaked: %s", out)
		}
	}
}

func TestParseDotEnv(t *testing.T) {
	content := "\ufeff# comment\r\nDB_USER=alice\r\nDB_PASSWORD=\"quoted value\"\r\n\r\nEMPTY=\nJWT_KEY = spaced \nNOEQUALS\n"
	want := map[string]string{
		"DB_USER":     "alice",
		"DB_PASSWORD": "quoted value",
		"EMPTY":       "",
		"JWT_KEY":     "spaced",
	}
	if got := parseDotEnv(content); !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoadEnvironmentBeatsFile(t *testing.T) {
	// Neutralise anything the machine's shell may have set, like conftest.py does.
	for _, key := range []string{"DB_USER", "DB_PASSWORD", "JWT_KEY", "DB_HOST",
		"DB_PORT", "DB_NAME", "FRONTEND_ORIGIN", "LOG_LEVEL"} {
		t.Setenv(key, "") // restored automatically when the test ends
	}

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	file := "DB_USER=fromfile\nDB_PASSWORD=pw\nJWT_KEY=" + strings.Repeat("k", 32) + "\n"
	if err := os.WriteFile(path, []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Load(path)
	if err != nil || s.DBUser != "fromfile" {
		t.Fatalf("file value not used: %v, %v", s.DBUser, err)
	}

	t.Setenv("DB_USER", "fromenv")
	s, err = Load(path)
	if err != nil || s.DBUser != "fromenv" {
		t.Errorf("environment should win: %v, %v", s.DBUser, err)
	}
}

func TestLoadMissingFileIsFine(t *testing.T) {
	for key, value := range validVars() {
		t.Setenv(key, value)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "does-not-exist.env")); err != nil {
		t.Errorf("a missing .env must be skipped, got %v", err)
	}
}
