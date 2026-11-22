package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"authrosetta/internal/auth"
	"authrosetta/internal/security"
	"authrosetta/internal/validation"
)

type AuthService interface {
	Register(ctx context.Context, email, pasword string) (auth.RegisterResult, error)
	ConfirmTwoFactor(ctx context.Context, email, code string) error
	Login(ctx context.Context, email, password, code string) (string, error)
	Authenticate(ctx context.Context, token string) (security.TokenClaims, error)
	Logout(ctx context.Context, claims security.TokenClaims) error
}

var _ AuthService = (*auth.Service)(nil)

type handlers struct {
	svc AuthService
}

type registerRequest struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
}

type confirmRequest struct {
	Email *string `json:"email"`
	Code  *string `json:"code"`
}

type loginRequest struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Code     *string `json:"code"`
}

type registerResponse struct {
	OtpauthURI string `json:"otpauthUri"`
	Secret     string `json:"secret"`
}

type messageBody struct {
	Message string `json:"message"`
}

type meResponse struct {
	Email string `json:"email"`
}

func internalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, MsgInternal)
}

func (h *handlers) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r.Body, &req); err != nil || req.Email == nil || req.Password == nil {
		writeError(w, http.StatusBadRequest, MsgInvalidRequest)
		return
	}

	result, err := h.svc.Register(r.Context(), *req.Email, *req.Password)
	var invalid *validation.ValidationError
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, registerResponse{OtpauthURI: result.OtpauthURI, Secret: result.Secret})
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, invalid.Message)
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, http.StatusConflict, MsgEmailTaken)
	default:
		internalError(w, r, err)
	}
}

func (h *handlers) confirm(w http.ResponseWriter, r *http.Request) {
	req, badText, err := decodeAuthBody[confirmRequest](r.Body)
	if err != nil || req.Email == nil || req.Code == nil {
		writeError(w, http.StatusBadRequest, MsgInvalidRequest)
		return
	}
	if badText {
		writeError(w, http.StatusBadRequest, MsgInvalidEmailOrCode)
		return
	}

	err = h.svc.ConfirmTwoFactor(r.Context(), *req.Email, *req.Code)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, messageBody{Message: "Two-factor authentication enabled"})
	case errors.Is(err, auth.ErrInvalidCode):
		writeError(w, http.StatusBadRequest, MsgInvalidEmailOrCode)
	default:
		internalError(w, r, err)
	}
}

func (h *handlers) login(w http.ResponseWriter, r *http.Request) {
	req, badText, err := decodeAuthBody[loginRequest](r.Body)
	if err != nil || req.Email == nil || req.Password == nil || req.Code == nil {
		writeError(w, http.StatusBadRequest, MsgInvalidRequest)
		return
	}
	if badText {
		writeError(w, http.StatusUnauthorized, MsgInvalidCredentials)
		return
	}

	token, err := h.svc.Login(r.Context(), *req.Email, *req.Password, *req.Code)
	switch {
	case err == nil:
		setTokenCookie(w, r, token) // headers first, then the body
		writeJSON(w, http.StatusOK, messageBody{Message: "Login successful"})
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, MsgInvalidCredentials)
	default:
		internalError(w, r, err)
	}
}

func (h *handlers) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, meResponse{Email: claims.Email})
}

func (h *handlers) logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	if err := h.svc.Logout(r.Context(), claims); err != nil {
		internalError(w, r, err)
		return
	}
	clearTokenCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// authenticate reads the cookie and checks the token. It has already written
// the error response when it returns false.
func (h *handlers) authenticate(w http.ResponseWriter, r *http.Request) (security.TokenClaims, bool) {
	token, ok := tokenFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, MsgUnauthorized)
		return security.TokenClaims{}, false
	}
	claims, err := h.svc.Authenticate(r.Context(), token)
	switch {
	case err == nil:
		return claims, true
	case errors.Is(err, auth.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, MsgUnauthorized)
	default:
		internalError(w, r, err)
	}
	return security.TokenClaims{}, false
}
