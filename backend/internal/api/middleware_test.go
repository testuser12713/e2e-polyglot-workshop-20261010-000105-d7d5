package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCORSAllowsConfiguredOrigin checks that the configured web origin is
// echoed back as Access-Control-Allow-Origin with credentials (SPEC AC-30).
func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	handler := newTestServer().Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/public/customers", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the configured origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want true", got)
	}
	// Error responses (the placeholder answers 501) must keep the CORS headers.
	if rec.Code == http.StatusOK {
		t.Fatal("placeholder route unexpectedly answered 200")
	}
}

// TestCORSRefusesForeignOrigin checks that a different browser origin is never
// granted access and that "*" is never sent (SPEC AC-30).
func TestCORSRefusesForeignOrigin(t *testing.T) {
	handler := newTestServer().Handler()

	// Preflight from a foreign origin is refused.
	preflight := httptest.NewRequest(http.MethodOptions, "/api/shop/orders", nil)
	preflight.Header.Set("Origin", "http://evil.example")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodGet)
	preRec := httptest.NewRecorder()
	handler.ServeHTTP(preRec, preflight)

	if preRec.Code != http.StatusForbidden {
		t.Errorf("foreign preflight status = %d, want 403", preRec.Code)
	}
	if got := preRec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("foreign preflight got Access-Control-Allow-Origin %q, want none", got)
	}

	// A normal request from a foreign origin gets no CORS grant.
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("foreign origin got Access-Control-Allow-Origin %q, want none", got)
	}
}

// TestCORSAllowsPreflightForConfiguredOrigin checks a valid preflight is
// answered with 204 and the CORS headers.
func TestCORSAllowsPreflightForConfiguredOrigin(t *testing.T) {
	handler := newTestServer().Handler()

	req := httptest.NewRequest(http.MethodOptions, "/api/shop/orders", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("preflight Access-Control-Allow-Origin = %q, want the configured origin", got)
	}
}
