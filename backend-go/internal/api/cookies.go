package api

import (
	"authrosetta/internal/security"
	"net/http"
)

const cookieName = "access_token"

// setTokenCookie sets the access token cookie with the specified token value.
func setTokenCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, tokenCookie(r, token, int(security.TokenLifetime.Seconds())))
}

// clearTokenCookie clears the access token cookie by setting it with an expired max age.
func clearTokenCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, tokenCookie(r, "", -1))
}

// tokenCookie creates an http.Cookie with the specified value and max age, configured for security and same-site strict mode.
func tokenCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge, // an int, so the header is never "3600.0"
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	}
}

// tokenFromRequest retrieves the access token from the request cookie, if present. It returns the token and a boolean indicating success.
func tokenFromRequest(r *http.Request) (string, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}
