package api

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// MaxBodyBytes is the contract's request body limit (8 KiB).
const MaxBodyBytes = 8 * 1024

// chain applies the given middleware functions to the provided http.Handler.
// The middleware are applied in the order they are provided, with the first middleware wrapping the handler first.
func chain(h http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

// cors is a middleware that handles Cross-Origin Resource Sharing (CORS) for the specified allowed origin.
// It sets the appropriate CORS headers and handles preflight requests.
func cors(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Add("Vary", "Origin")

			origin := r.Header.Get("Origin")
			allowed := origin != "" && origin == allowedOrigin
			if allowed {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
			}

			isPreflight := r.Method == http.MethodOptions &&
				r.Header.Get("Access-Control-Request-Method") != ""
			if isPreflight {
				if allowed {
					h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					h.Set("Access-Control-Allow-Headers", "Content-Type")
					h.Set("Access-Control-Max-Age", "600")
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// recoverer is a middleware that recovers from panics in the HTTP handler chain.
// It logs the panic and returns a 500 Internal Server Error response.
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec) // net/http's own signal: let it through
				}
				slog.Error("panic in handler", "panic", rec, "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, MsgInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// bodyLimit is a middleware that enforces a maximum request body size.
// It returns a 400 Bad Request if the request body exceeds the limit.
func bodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > MaxBodyBytes {
			writeError(w, http.StatusBadRequest, MsgInvalidRequest)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
		next.ServeHTTP(w, r)
	})
}
