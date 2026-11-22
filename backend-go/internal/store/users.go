package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"authrosetta/internal/auth"
)

const (
	uniqueViolation       = "23505" // PostgreSQL error code for unique constraint violation
	emailUniqueConstraint = "uq_users_email"
)

type Users struct {
	pool *pgxpool.Pool
}

// NewUsers creates a new Users store with the given database connection pool.
// Returns a pointer to the Users store.
func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

// Add inserts a new user into the database.
// Returns true if the user was added successfully.
// Returns false if the email already exists.
// Returns an error for any other failure.
func (s *Users) Add(ctx context.Context, u auth.User) (bool, error) {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, totp_secret, two_factor_enabled, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		u.ID, u.Email, u.PasswordHash, u.TOTPSecret, u.TwoFactorEnabled, u.CreatedAt)
	if err == nil {
		return true, nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == emailUniqueConstraint {
		return false, nil
	}
	return false, fmt.Errorf("inserting user: %w", err)
}

// FindByEmail retrieves a user from the database by their email.
// Returns the user if found.
// Returns auth.ErrNotFound if no user with the given email exists.
// Returns an error for any other failure.
func (s *Users) FindByEmail(ctx context.Context, email string) (auth.User, error) {
	var u auth.User
	err := s.pool.QueryRow(ctx,
		`SELECT id::text, email, password_hash, totp_secret, two_factor_enabled, created_at
		FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.TOTPSecret, &u.TwoFactorEnabled, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("finding user: %w", err)
	}
	return u, nil
}

// EnableTwoFactor enables two-factor authentication for the user with the given ID.
// Returns auth.ErrNotFound if no user with the given ID exists.
// Returns an error for any other failure.
func (s *Users) EnableTwoFactor(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET two_factor_enabled = true WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("enabling two factor: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	return nil
}
