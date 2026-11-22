package security

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func fastPasswords(t *testing.T) *Passwords {
	t.Helper()
	p, err := NewPasswords(bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRealWorkFactorIsTwelve(t *testing.T) {
	p, err := NewPasswords(WorkFactor)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := p.Hash("password123")
	if err != nil {
		t.Fatal(err)
	}
	if cost, _ := bcrypt.Cost([]byte(hash)); cost != 12 {
		t.Errorf("hash cost = %d, want 12", cost)
	}
	if cost, _ := bcrypt.Cost(p.dummy); cost != 12 {
		t.Errorf("dummy hash cost = %d, want 12 (timing rule)", cost)
	}
}

func TestHashAndVerify(t *testing.T) {
	p := fastPasswords(t)
	hash, err := p.Hash("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Verify("correct horse", hash) {
		t.Error("right password rejected")
	}
	if p.Verify("wrong horse", hash) {
		t.Error("wrong password accepted")
	}
}

func TestSeventyTwoBytesIsTheLimit(t *testing.T) {
	p := fastPasswords(t)
	ok72 := strings.Repeat("a", 72)

	hash, err := p.Hash(ok72)
	if err != nil {
		t.Fatalf("72 bytes should hash: %v", err)
	}
	if !p.Verify(ok72, hash) {
		t.Error("72-byte password rejected")
	}
	// Never truncate: the 73-byte version must not match the 72-byte hash.
	if p.Verify(ok72+"b", hash) {
		t.Error("73-byte password matched a 72-byte hash: truncation!")
	}
	// Documents the library's behaviour: it refuses, it does not truncate.
	if _, err := p.Hash(ok72 + "b"); err == nil {
		t.Error("hashing 73 bytes should fail")
	}
}

func TestNULDoesNotTruncatePasswords(t *testing.T) {
	p := fastPasswords(t)
	hash, err := p.Hash("secret\x00one")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Verify("secret\x00one", hash) {
		t.Error("the exact password should verify")
	}
	if p.Verify("secret", hash) {
		t.Error("prefix before the NUL matched: truncated at NUL")
	}
	if p.Verify("secret\x00two", hash) {
		t.Error("different tail after the NUL matched")
	}
}

func TestVerifyNeverPanicsOnOddInput(t *testing.T) {
	p := fastPasswords(t)
	hash, _ := p.Hash("password123")

	if p.Verify("\xff\xfe\xfd", hash) {
		t.Error("invalid UTF-8 accepted")
	}
	if p.Verify("password123", "not-a-bcrypt-hash") {
		t.Error("malformed hash accepted")
	}
	if p.Verify("password123", "") {
		t.Error("empty hash accepted")
	}
}

func TestVerifyAgainstDummyIsAlwaysFalse(t *testing.T) {
	if fastPasswords(t).VerifyAgainstDummy() {
		t.Error("dummy verification returned true")
	}
}
