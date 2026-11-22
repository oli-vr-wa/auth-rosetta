package api

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testHandler is the real middleware chain around a few test-only routes.
func testHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			writeError(w, http.StatusBadRequest, MsgInvalidRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
	})
	mux.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) {
		panic("secret internal detail")
	})
	mux.HandleFunc("/", handleNotFound)
	return newHandler(mux, testOrigin)
}

func assertCORS(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	h := rec.Header()
	if got := h.Get("Access-Control-Allow-Origin"); got != testOrigin {
		t.Errorf("Allow-Origin = %q, want %q", got, testOrigin)
	}
	if got := h.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true", got)
	}
}

func withOrigin(req *http.Request, origin string) *http.Request {
	req.Header.Set("Origin", origin)
	return req
}

func TestCORSOnSuccessAndOnErrors(t *testing.T) {
	h := testHandler()

	ok := serve(h, withOrigin(httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("{}")), testOrigin))
	if ok.Code != http.StatusOK {
		t.Fatalf("echo status = %d", ok.Code)
	}
	assertCORS(t, ok)

	notFound := serve(h, withOrigin(httptest.NewRequest(http.MethodGet, "/nope", nil), testOrigin))
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", notFound.Code)
	}
	assertCORS(t, notFound)
	if got := notFound.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want it to include Origin", got)
	}
}

func TestCORSIsNotGrantedToOtherOrigins(t *testing.T) {
	h := testHandler()
	for _, origin := range []string{"http://evil.example", "http://localhost:5174", "https://localhost:5173", ""} {
		req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("{}"))
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := serve(h, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q was granted access: %q", origin, got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Errorf("origin %q got credentials: %q", origin, got)
		}
	}
}

func preflight(origin string) *http.Request {
	req := httptest.NewRequest(http.MethodOptions, "/echo", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	return req
}

func TestPreflight(t *testing.T) {
	rec := serve(testHandler(), preflight(testOrigin))
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 200 or 204", rec.Code)
	}
	assertCORS(t, rec)
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("Allow-Methods = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
		t.Errorf("Allow-Headers = %q", got)
	}
}

func TestPreflightFromAForeignOriginGetsNoPermission(t *testing.T) {
	rec := serve(testHandler(), preflight("http://evil.example"))
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want none", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("Allow-Methods = %q, want none", got)
	}
}

func TestPanicBecomesAGeneric500WithCORS(t *testing.T) {
	// The recovered panic is logged with its stack; that output is expected.
	rec := serve(testHandler(), withOrigin(httptest.NewRequest(http.MethodGet, "/boom", nil), testOrigin))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	assertErrorBody(t, rec, MsgInternal)
	assertCORS(t, rec)
	if strings.Contains(rec.Body.String(), "secret internal detail") {
		t.Error("the panic message leaked to the client")
	}
}

func TestBodyLimit(t *testing.T) {
	h := testHandler()
	for _, chunked := range []bool{false, true} {
		for _, tc := range []struct{ size, want int }{
			{MaxBodyBytes, http.StatusOK},
			{MaxBodyBytes + 1, http.StatusBadRequest},
		} {
			t.Run(fmt.Sprintf("chunked=%v/size=%d", chunked, tc.size), func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/echo",
					strings.NewReader(strings.Repeat("a", tc.size)))
				req.Header.Set("Origin", testOrigin)
				if chunked {
					req.ContentLength = -1 // unknown length, like a chunked upload
				}
				rec := serve(h, req)
				if rec.Code != tc.want {
					t.Fatalf("status = %d, want %d", rec.Code, tc.want)
				}
				if tc.want == http.StatusBadRequest {
					assertErrorBody(t, rec, MsgInvalidRequest)
					assertCORS(t, rec) // the contract: errors carry CORS headers
				}
			})
		}
	}
}
