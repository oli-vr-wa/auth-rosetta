package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeJSON writes the given body as a JSON response with the specified status code.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("writing response", "err", err)
	}
}
