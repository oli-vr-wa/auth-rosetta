package security

import (
	"crypto/rand"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	WorkFactor        = 12
	MaxPasswordLength = 72
)

type Passwords struct {
	cost  int
	dummy []byte // dummy hash for timing attacks
}

// NewPasswords creates a new Passwords instance with the given cost and a dummy hash for timing attacks.
// It returns the Passwords instance or an error if the dummy hash creation fails.
func NewPasswords(cost int) (*Passwords, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), cost)
	if err != nil {
		return nil, fmt.Errorf("creating dummy hash: %w", err)
	}
	return &Passwords{cost: cost, dummy: dummy}, nil
}

// Hash hashes the given password using bcrypt with the configured cost.
// It returns the hashed password as a string or an error if hashing fails.
func (p *Passwords) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), p.cost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}

// Verify checks if the given password matches the provided hash.
// It returns true if the password is correct, false otherwise.
func (p *Passwords) Verify(password, hash string) bool {
	if len(password) > MaxPasswordLength || !utf8.ValidString(password) {
		return p.VerifyAgainstDummy() // early return for invalid password
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// VerifyAgainstDummy performs a dummy verification against the precomputed dummy hash.
// It always returns false but ensures consistent timing to mitigate timing attacks.
func (p *Passwords) VerifyAgainstDummy() bool {
	bcrypt.CompareHashAndPassword(p.dummy, []byte("x"))
	return false
}
