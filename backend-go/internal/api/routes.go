package api

import "net/http"

func NewRouter(frontendOrigin string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
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
