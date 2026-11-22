package security

import (
	"errors"
	"log/slog"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenIssuer   = "AuthRosetta-Go"
	tokenAudience = "AuthRosetta"
	TokenLifetime = 60 * time.Minute
	tokenLeeway   = 30 * time.Second
	minKeyBytes   = 32
)

func init() {
	// Configure JWT to not marshal single string claims as arrays
	// By default the library writes "aud" as a one-element array even if it contains a single string
	jwt.MarshalSingleStringAsArray = false
}

// TokenClaims is what the rest of the app sees after a token is verified.
type TokenClaims struct {
	UserID    string
	Email     string
	JTI       string
	ExpiresAt time.Time
}

// jwtClaims is the payload as it appears on the wire. Embedding
// jwt.RegisteredClaims promotes sub, jti, iss, aud and exp into it.
type jwtClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type Tokens struct {
	key []byte
	now func() time.Time
}

// NewTokens creates a new Tokens instance with the given signing key and time function.
// The key must be at least 32 bytes long.
func NewTokens(key string, now func() time.Time) (*Tokens, error) {
	if len(key) < minKeyBytes {
		return nil, errors.New("signing key must be at least 32 bytes")
	}
	return &Tokens{
		key: []byte(key),
		now: now,
	}, nil
}

// Create generates a new JWT for the given user ID and email.
// The token will be signed using the instance's key and will include standard claims such as sub, jti, iss, aud, and exp.
func (t *Tokens) Create(userID, email string) (string, error) {
	claims := jwtClaims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        uuid.New().String(),
			Issuer:    tokenIssuer,
			Audience:  jwt.ClaimStrings{tokenAudience},
			ExpiresAt: jwt.NewNumericDate(t.now().Add(TokenLifetime)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.key)
}

func (t *Tokens) Decode(tokenString string) (TokenClaims, bool) {
	var claims jwtClaims
	token, err := jwt.ParseWithClaims(tokenString, &claims,
		func(*jwt.Token) (any, error) { return t.key, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithAudience(tokenAudience),
		jwt.WithLeeway(tokenLeeway),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil || !token.Valid {
		slog.Debug("token rejected", "reason", err)
		return TokenClaims{}, false
	}
	if claims.Subject == "" || claims.Email == "" || claims.ID == "" || claims.ExpiresAt == nil {
		slog.Debug("token rejectred", "reason", "missing required claims")
		return TokenClaims{}, false
	}

	return TokenClaims{
		UserID:    claims.Subject,
		Email:     claims.Email,
		JTI:       claims.ID,
		ExpiresAt: claims.ExpiresAt.Time,
	}, true
}
