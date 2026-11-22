package api

import "net/http"

const (
	MsgInvalidRequest     = "Invalid request"
	MsgUnauthorized       = "Unauthorized"
	MsgInvalidCredentials = "Invalid credentials"
	MsgInvalidEmailOrCode = "Invalid email or code"
	MsgEmailTaken         = "Email already registered"
	MsgInternal           = "Internal server error"
)

type errorBody struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorBody{Error: message})
}

// handleNotFound answers every path (and method) that no route claimed.
// The contract allows any message for 404, as long as the shape is {error}.
func handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "Not found")
}
