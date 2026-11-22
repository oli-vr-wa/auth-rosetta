package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"authrosetta/internal/security"
	"authrosetta/internal/validation"
)

var (
	ErrEmailTaken         = errors.New("email is already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidCode        = errors.New("invalid email or code")
	ErrUnauthorized       = errors.New("unauthorized")
)

type UserStore interface {
	Add(ctx context.Context, u User) (bool, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	EnableTwoFactor(ctx context.Context, id string) error
}

type RevokedStore interface {
	Revoke(ctx context.Context, jti string, expiresAt time.Time) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

type RegisterResult struct {
	OtpauthURI string
	Secret     string
}

type Service struct {
	users     UserStore
	revoked   RevokedStore
	passwords *security.Passwords
	tokens    *security.Tokens
	now       func() time.Time
}

// NewService creates a new instance of the authentication service. It requires user and revoked token stores,
// password and token security utilities, and a function to get the current time.
// It returns a pointer to the newly created Service instance.
func NewService(users UserStore, revoked RevokedStore, passwords *security.Passwords, tokens *security.Tokens, now func() time.Time) *Service {
	return &Service{
		users:     users,
		revoked:   revoked,
		passwords: passwords,
		tokens:    tokens,
		now:       now,
	}
}

// Register registers a new user with the given email and password.
// It returns a RegisterResult containing the TOTP enrollment details if successful.
// If the email is already taken or any error occurs during registration, it returns an appropriate error.
func (s *Service) Register(ctx context.Context, email, password string) (RegisterResult, error) {
	email = validation.NormalizeEmail(email)
	if err := validation.ValidateRegister(email, password); err != nil {
		return RegisterResult{}, err
	}

	_, found, err := s.findUser(ctx, email)
	if err != nil {
		return RegisterResult{}, err
	}
	if found {
		return RegisterResult{}, ErrEmailTaken
	}

	enrollment, err := security.NewTOTPEnrollment(email)
	if err != nil {
		return RegisterResult{}, err
	}
	hash, err := s.passwords.Hash(password)
	if err != nil {
		return RegisterResult{}, err
	}

	added, err := s.users.Add(ctx, User{
		ID:               uuid.New().String(),
		Email:            email,
		PasswordHash:     hash,
		TOTPSecret:       enrollment.Secret,
		TwoFactorEnabled: false,
		CreatedAt:        s.now().UTC(),
	})
	if err != nil {
		return RegisterResult{}, err
	}
	if !added {
		return RegisterResult{}, ErrEmailTaken
	}

	return RegisterResult{
		OtpauthURI: enrollment.URI,
		Secret:     enrollment.Secret,
	}, nil
}

// ConfirmTwoFactor confirms the two-factor authentication setup for a user.
// It takes the user's email and the TOTP code as input.
// If the code is valid, it enables two-factor authentication for the user.
// It returns an error if the user is not found, the code is invalid, or any other error occurs during the process.
func (s *Service) ConfirmTwoFactor(ctx context.Context, email, code string) error {
	user, found, err := s.findUser(ctx, validation.NormalizeEmail(email))
	if err != nil {
		return err
	}
	if !found || !security.VerifyTOTP(user.TOTPSecret, code, s.now()) {
		return ErrInvalidCode
	}
	return s.users.EnableTwoFactor(ctx, user.ID)
}

// Login authenticates a user with the given email, password, and TOTP code.
// It returns a JWT token if the authentication is successful.
// It returns an error if the user is not found, the credentials are invalid, or any other error occurs during the process.
func (s *Service) Login(ctx context.Context, email, password, code string) (string, error) {
	user, found, err := s.findUser(ctx, validation.NormalizeEmail(email))
	if err != nil {
		return "", err
	}

	var passwordOK bool
	if found {
		passwordOK = s.passwords.Verify(password, user.PasswordHash)
	} else {
		passwordOK = s.passwords.VerifyAgainstDummy()
	}
	enabled := found && user.TwoFactorEnabled
	codeOK := found && security.VerifyTOTP(user.TOTPSecret, code, s.now())

	slog.Debug("login attempt",
		"userFound", found,
		"passwordOK", passwordOK,
		"twoFactorEnabled", enabled,
		"codeOK", codeOK,
	)

	if !(found && passwordOK && enabled && codeOK) {
		return "", ErrInvalidCredentials
	}

	return s.tokens.Create(user.ID, user.Email)
}

// Authenticate validates a JWT token and checks if it has been revoked.
// It returns the token claims if the token is valid and not revoked.
// It returns an error if the token is invalid, revoked, or any other error occurs during the process.
func (s *Service) Authenticate(ctx context.Context, token string) (security.TokenClaims, error) {
	claims, ok := s.tokens.Decode(token)
	if !ok {
		return security.TokenClaims{}, ErrUnauthorized
	}
	revoked, err := s.revoked.IsRevoked(ctx, claims.JTI)
	if err != nil {
		return security.TokenClaims{}, err
	}
	if revoked {
		return security.TokenClaims{}, ErrUnauthorized
	}
	return claims, nil
}

// Logout revokes the JWT token represented by the given claims.
func (s *Service) Logout(ctx context.Context, claims security.TokenClaims) error {
	return s.revoked.Revoke(ctx, claims.JTI, claims.ExpiresAt)
}

// findUser looks up a user by email. It returns the user, a boolean indicating if the user was found, and an error if any occurred during the lookup.
// It returns false for the boolean if the email is not storable or the user is not found.
// If an error occurs during the lookup, it returns the error and false for the boolean.
// If the user is found, it returns the user, true for the boolean, and nil for the error.
func (s *Service) findUser(ctx context.Context, email string) (User, bool, error) {
	if !validation.IsStorable(email) {
		return User{}, false, nil
	}
	user, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("looking up user: %w", err)
	}
	return user, true, nil
}
