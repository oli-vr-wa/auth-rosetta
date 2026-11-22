package api

import (
	"fmt"
	"net/http"
)

func NewRouter(svc AuthService, frontendOrigin string) http.Handler {
	h := &handlers{svc: svc}
	baseUrl := "/api/auth"

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc(fmt.Sprintf("POST %s/register", baseUrl), h.register)
	mux.HandleFunc(fmt.Sprintf("POST %s/2fa/confirm", baseUrl), h.confirm)
	mux.HandleFunc(fmt.Sprintf("POST %s/login", baseUrl), h.login)
	mux.HandleFunc(fmt.Sprintf("GET %s/me", baseUrl), h.me)
	mux.HandleFunc(fmt.Sprintf("POST %s/logout", baseUrl), h.logout)
	mux.HandleFunc("/", handleNotFound)
	return newHandler(mux, frontendOrigin)
}

// newHandler wraps the provided ServeMux with middleware for CORS, recovery, and body size limiting.
func newHandler(mux *http.ServeMux, frontendOrigin string) http.Handler {
	return chain(mux, cors(frontendOrigin), recoverer, bodyLimit)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
