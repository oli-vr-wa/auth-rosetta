package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RevokedTokens struct {
	pool *pgxpool.Pool
}

// NewRevokedTokens creates a new instance of RevokedTokens with the given database pool.
// It returns a pointer to the created RevokedTokens instance.
func NewRevokedTokens(pool *pgxpool.Pool) *RevokedTokens {
	return &RevokedTokens{pool: pool}
}

// Revoke adds the given token identifier (jti) to the revoked tokens list with its expiration time.
// If the token is already revoked, it does nothing.
// It returns an error if there is an issue with the database operation.
func (s *RevokedTokens) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO revoked_tokens (jti, expires_at) VALUES ($1, $2)
		ON CONFLICT (jti) DO NOTHING`,
		jti, expiresAt)
	if err != nil {
		return fmt.Errorf("revoking token: %w", err)
	}
	return nil
}

// IsRevoked checks if the given token identifier (jti) is in the revoked tokens list.
// It returns true if the token is revoked, false otherwise.
// It returns an error if there is an issue with the database operation.
func (s *RevokedTokens) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var revoked bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM revoked_tokens WHERE jti = $1)`,
		jti).Scan(&revoked)
	if err != nil {
		return false, fmt.Errorf("checking revoked token: %w", err)
	}
	return revoked, nil
}
