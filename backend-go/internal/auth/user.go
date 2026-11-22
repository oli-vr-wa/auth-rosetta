package auth

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type User struct {
	ID               string
	Email            string
	PasswordHash     string
	TOTPSecret       string
	TwoFactorEnabled bool
	CreatedAt        time.Time
}
