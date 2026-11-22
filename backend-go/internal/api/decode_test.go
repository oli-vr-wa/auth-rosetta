package api

import (
	"strings"
	"testing"
)

type probe struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
}

type outcome int

const (
	mustFail outcome = iota // decoding must return an error
	unusable                // an error, or every field left nil: never a usable request
	usable                  // decodes, with the given values
)

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		want     outcome
		email    string
		password string
	}{
		{"valid", `{"email":"a@b.com","password":"pw"}`, usable, "a@b.com", "pw"},
		{"extra members ignored", `{"email":"a@b.com","password":"pw","admin":true}`, usable, "a@b.com", "pw"},
		{"surrogate pair (emoji)", `{"email":"a@b.com","password":"\ud83d\ude00"}`, usable, "a@b.com", "😀"},
		{"NUL escape is valid JSON", `{"email":"a@b.com","password":"a\u0000b"}`, usable, "a@b.com", "a\x00b"},

		{"empty object", `{}`, unusable, "", ""},
		{"null members", `{"email":null,"password":null}`, unusable, "", ""},
		{"null body", `null`, unusable, "", ""},
		{"array body", `[]`, unusable, "", ""},
		{"string body", `"x"`, unusable, "", ""},
		{"empty body", ``, unusable, "", ""},
		{"wrong-case names", `{"Email":"a@b.com","PASSWORD":"pw"}`, unusable, "", ""},

		{"broken JSON", `{bad`, mustFail, "", ""},
		{"number instead of string", `{"email":123,"password":"x"}`, mustFail, "", ""},
		{"trailing junk", `{"email":"a@b.com","password":"pw"} xyz`, mustFail, "", ""},
		{"second object appended", `{"email":"a@b.com","password":"pw"}{"email":"c"}`, mustFail, "", ""},
		{"duplicate names", `{"email":"a@b.com","email":"c@d.com","password":"pw"}`, mustFail, "", ""},
		{"invalid UTF-8 bytes", "{\"email\":\"a@b.com\",\"password\":\"\xff\xfe\"}", mustFail, "", ""},
		{"lone high surrogate", `{"email":"a@b.com","password":"\ud800"}`, mustFail, "", ""},
		{"lone low surrogate", `{"email":"a@b.com","password":"\udc00"}`, mustFail, "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var p probe
			err := decodeJSON(strings.NewReader(tc.body), &p)

			switch tc.want {
			case mustFail:
				if err == nil {
					t.Errorf("accepted %q as %s", tc.body, show(p))
				}
			case unusable:
				if err == nil && (p.Email != nil || p.Password != nil) {
					t.Errorf("accepted %q as %s", tc.body, show(p))
				}
			case usable:
				if err != nil {
					t.Fatalf("rejected %q: %v", tc.body, err)
				}
				if p.Email == nil || *p.Email != tc.email || p.Password == nil || *p.Password != tc.password {
					t.Errorf("decoded %q as %s", tc.body, show(p))
				}
			}
		})
	}
}

func TestDecodeAuthBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		badText bool
		wantErr bool
	}{
		{"valid", `{"email":"a","password":"b"}`, false, false},
		{"lone surrogate", `{"email":"a","password":"\ud800"}`, true, false},
		{"lone low surrogate", `{"email":"a","password":"\udc00"}`, true, false},
		{"invalid UTF-8 bytes", "{\"email\":\"\xff\",\"password\":\"b\"}", true, false},
		{"bad text and a missing field", `{"password":"\ud800"}`, true, false},
		{"bad text but broken JSON", `{"password":"\ud800"`, false, true},
		{"bad text but wrong type", `{"email":123,"password":"\ud800"}`, false, true},
		{"bad text but duplicate names", `{"email":"a","email":"b","password":"\ud800"}`, false, true},
		{"plain broken JSON", `{bad`, false, true},
		{"empty body", ``, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, badText, err := decodeAuthBody[probe](strings.NewReader(tc.body))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && badText != tc.badText {
				t.Errorf("badText = %v, want %v", badText, tc.badText)
			}
		})
	}
}

// show prints the values behind the pointers (a bare %v would print addresses).
func show(p probe) string {
	str := func(s *string) string {
		if s == nil {
			return "nil"
		}
		return "\"" + *s + "\""
	}
	return "{email: " + str(p.Email) + ", password: " + str(p.Password) + "}"
}
