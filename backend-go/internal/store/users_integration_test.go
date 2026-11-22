//go:build integration

package store

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"authrosetta/internal/auth"
	"authrosetta/internal/config"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// Tests run inside internal/store, so the repo-root .env is three levels up.
	settings, err := config.Load("../../../.env")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := NewPool(ctx, settings.DatabaseURL())
	if err != nil {
		t.Fatalf("database not reachable (is Postgres running, and have you run the migrations?): %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newTestUsers(t *testing.T) *Users {
	return NewUsers(testPool(t))
}

func uniqueEmail() string {
	return "store-test-" + uuid.New().String() + "@example.com"
}

func newUser(email string) auth.User {
	return auth.User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: "hash-" + uuid.New().String(),
		TOTPSecret:   "SECRET",
		CreatedAt:    time.Now().UTC(),
	}
}

// track makes sure the test's rows are deleted afterwards.
func track(t *testing.T, s *Users, email string) {
	t.Cleanup(func() {
		s.pool.Exec(context.Background(), "DELETE FROM users WHERE email = $1", email)
	})
}

func TestAddThenFind(t *testing.T) {
	s := newTestUsers(t)
	ctx := context.Background()
	email := uniqueEmail()
	track(t, s, email)

	u := newUser(email)
	ok, err := s.Add(ctx, u)
	if err != nil || !ok {
		t.Fatalf("Add = %v, %v; want true, nil", ok, err)
	}

	got, err := s.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if got.ID != u.ID || got.PasswordHash != u.PasswordHash || got.TwoFactorEnabled {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt was not stored")
	}
}

func TestDuplicateEmailReturnsFalse(t *testing.T) {
	s := newTestUsers(t)
	ctx := context.Background()
	email := uniqueEmail()
	track(t, s, email)

	if ok, err := s.Add(ctx, newUser(email)); err != nil || !ok {
		t.Fatalf("first Add = %v, %v", ok, err)
	}
	ok, err := s.Add(ctx, newUser(email))
	if err != nil || ok {
		t.Errorf("second Add = %v, %v; want false, nil", ok, err)
	}
}

func TestDuplicateIDIsARealError(t *testing.T) {
	// A primary key clash is a bug, not "email taken": it must not return false.
	s := newTestUsers(t)
	ctx := context.Background()
	first, second := uniqueEmail(), uniqueEmail()
	track(t, s, first)
	track(t, s, second)

	u := newUser(first)
	if ok, err := s.Add(ctx, u); err != nil || !ok {
		t.Fatalf("first Add = %v, %v", ok, err)
	}
	u.Email = second // same ID, different email
	ok, err := s.Add(ctx, u)
	if err == nil || ok {
		t.Errorf("Add = %v, %v; want false and an error", ok, err)
	}
}

func TestFindUnknownEmail(t *testing.T) {
	s := newTestUsers(t)
	_, err := s.FindByEmail(context.Background(), uniqueEmail())
	if !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("err = %v, want auth.ErrNotFound", err)
	}
}

func TestEnableTwoFactor(t *testing.T) {
	s := newTestUsers(t)
	ctx := context.Background()
	email := uniqueEmail()
	track(t, s, email)

	u := newUser(email)
	if _, err := s.Add(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := s.EnableTwoFactor(ctx, u.ID); err != nil {
		t.Fatalf("EnableTwoFactor: %v", err)
	}
	got, _ := s.FindByEmail(ctx, email)
	if !got.TwoFactorEnabled {
		t.Error("two_factor_enabled is still false")
	}

	err := s.EnableTwoFactor(ctx, uuid.New().String())
	if !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("unknown id: err = %v, want auth.ErrNotFound", err)
	}
}

func TestConcurrentRegistrationsExactlyOneWins(t *testing.T) {
	s := newTestUsers(t)
	ctx := context.Background()
	email := uniqueEmail()
	track(t, s, email)

	const attempts = 8
	oks := make([]bool, attempts)
	errs := make([]error, attempts)

	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			oks[i], errs[i] = s.Add(ctx, newUser(email))
		}()
	}
	wg.Wait()

	winners := 0
	for i := 0; i < attempts; i++ {
		if errs[i] != nil {
			t.Errorf("attempt %d: unexpected error %v", i, errs[i])
		}
		if oks[i] {
			winners++
		}
	}
	if winners != 1 {
		t.Errorf("%d attempts won, want exactly 1", winners)
	}
}

func TestErrorsDoNotLeakParameters(t *testing.T) {
	// An email over 256 characters makes Postgres fail for a different reason.
	s := newTestUsers(t)
	u := newUser(strings.Repeat("a", 300))

	ok, err := s.Add(context.Background(), u)
	if err == nil || ok {
		t.Fatalf("Add = %v, %v; want false and an error", ok, err)
	}
	if strings.Contains(err.Error(), u.PasswordHash) || strings.Contains(err.Error(), u.TOTPSecret) {
		t.Errorf("error leaks parameter values: %v", err)
	}
}
