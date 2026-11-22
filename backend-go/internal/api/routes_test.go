package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testOrigin = "http://localhost:5173"

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// assertErrorBody checks the contract shape: a JSON object whose only
// member is a string called "error".
func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder, wantMessage string) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %q", rec.Body.String())
	}
	msg, ok := body["error"].(string)
	if !ok || len(body) != 1 {
		t.Fatalf("body = %v, want exactly one string member \"error\"", body)
	}
	if wantMessage != "" && msg != wantMessage {
		t.Errorf("error = %q, want %q", msg, wantMessage)
	}
}

func TestHealth(t *testing.T) {
	rec := serve(NewRouter(nil, testOrigin), httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestUnknownRouteIsAJSON404(t *testing.T) {
	rec := serve(NewRouter(nil, testOrigin), httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	assertErrorBody(t, rec, "")
}

func TestWrongMethodIsAJSON404Or405(t *testing.T) {
	rec := serve(NewRouter(nil, testOrigin), httptest.NewRequest(http.MethodPost, "/health", nil))
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 404 or 405", rec.Code)
	}
	assertErrorBody(t, rec, "")
}

func TestTrailingSlashIsNotRedirected(t *testing.T) {
	rec := serve(NewRouter(nil, testOrigin), httptest.NewRequest(http.MethodGet, "/health/", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (no slash redirects)", rec.Code)
	}
	assertErrorBody(t, rec, "")
}
