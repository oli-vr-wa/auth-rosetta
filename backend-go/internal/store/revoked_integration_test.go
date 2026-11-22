//go:build integration

package store

import (
	"context"
	"sync"
	"testing"
	"time"
	"uuid"
)

func newTestRevoked(t *testing.T) *RevokedTokens {
	t.Helper()
	return NewRevokedTokens(testPool(t))
}

func trackJTI(t *testing.T, s *RevokedTokens, jti string) {
	t.Cleanup(func() {
		s.pool.Exec(context.Background(), "DELETE FROM revoked_tokens WHERE jti = $1", jti)
	})
}

func TestRevokeThenIsRevoked(t *testing.T) {
	s := newTestRevoked(t)
	ctx := context.Background()
	jti := uuid.New().String()
	trackJTI(t, s, jti)

	if err := s.Revoke(ctx, jti, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	revoked, err := s.IsRevoked(ctx, jti)
	if err != nil || !revoked {
		t.Errorf("IsRevoked = %v, %v; want true, nil", revoked, err)
	}
}

func TestUnknownTokenIsNotRevoked(t *testing.T) {
	s := newTestRevoked(t)
	revoked, err := s.IsRevoked(context.Background(), uuid.New().String())
	if err != nil || revoked {
		t.Errorf("IsRevoked = %v, %v; want false, nil", revoked, err)
	}
}

func TestRevokingTwiceIsFine(t *testing.T) {
	s := newTestRevoked(t)
	ctx := context.Background()
	jti := uuid.New().String()
	trackJTI(t, s, jti)

	for i := 0; i < 2; i++ {
		if err := s.Revoke(ctx, jti, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("Revoke #%d: %v", i+1, err)
		}
	}
}

func TestConcurrentRevokesAllSucceed(t *testing.T) {
	// Two logouts racing with the same token must not produce an error.
	s := newTestRevoked(t)
	ctx := context.Background()
	jti := uuid.New().String()
	trackJTI(t, s, jti)

	const attempts = 8
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = s.Revoke(ctx, jti, time.Now().Add(time.Hour))
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("attempt %d: %v", i, err)
		}
	}
}
