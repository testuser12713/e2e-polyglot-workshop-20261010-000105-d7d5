package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"workshop/backend/internal/config"
	"workshop/backend/internal/store"
)

// openShopServer connects to the real PostgreSQL instance the API tests run
// against (SPEC AC-25) and returns a fully wired Server. It is skipped when
// DATABASE_URL is not set.
func openShopServer(t *testing.T, rateLimit int) (*Server, *store.Store) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	st, err := store.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(st.Close)

	cfg := &config.Config{
		CORSAllowedOrigin:       "http://localhost:5173",
		LoginRateLimitPerMinute: rateLimit,
	}
	return NewServer(st, cfg, nil), st
}

func shopTestEmail(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("shop-%d@example.test", time.Now().UnixNano())
}

func cleanupEmployee(t *testing.T, st *store.Store, email string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := st.Pool.Exec(ctx, `DELETE FROM employees WHERE email = $1`, email); err != nil {
		t.Fatalf("cleanup employee: %v", err)
	}
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func doLogin(handler http.Handler, remoteAddr, email, password string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/shop/login", bytes.NewReader(body))
	req.RemoteAddr = remoteAddr
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// TestLoginReturnsTokenThatOpensWorkshopRoute proves the whole happy path: the
// configured bootstrap credentials log in, the token opens a workshop route,
// and a missing or unknown token answers 401 with the unified body (AC-15,
// AC-28).
func TestLoginReturnsTokenThatOpensWorkshopRoute(t *testing.T) {
	server, st := openShopServer(t, 10)
	server.loginLimiter.reset()
	handler := server.Handler()

	email := shopTestEmail(t)
	t.Cleanup(func() { cleanupEmployee(t, st, email) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := st.BootstrapEmployee(ctx, email, "Werkstatt-Team", "correct-horse"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	rec := doLogin(handler, "203.0.113.10:1111", email, "correct-horse")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Token    string `json:"token"`
		Employee struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"employee"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode login body: %v", err)
	}
	if body.Token == "" {
		t.Fatal("login returned an empty token")
	}
	if body.Employee.Email != email {
		t.Errorf("employee e-mail = %q, want %q", body.Employee.Email, email)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("password")) {
		t.Error("login response leaked a password field")
	}

	// With the token the workshop route is reachable (not 401). The concrete
	// order-list body is another ticket's; only the session gate is ours.
	authorized := httptest.NewRequest(http.MethodGet, "/api/shop/orders", nil)
	authorized.Header.Set("Authorization", "Bearer "+body.Token)
	authorized.RemoteAddr = "203.0.113.10:1111"
	authorizedRec := httptest.NewRecorder()
	handler.ServeHTTP(authorizedRec, authorized)
	if authorizedRec.Code == http.StatusUnauthorized {
		t.Fatalf("valid token was rejected with 401 (body %s)", authorizedRec.Body.String())
	}
	if authorizedRec.Code == http.StatusNotFound {
		t.Fatal("GET /api/shop/orders is not registered")
	}

	// No token: server-side 401 with the unified body.
	anonymous := httptest.NewRequest(http.MethodGet, "/api/shop/orders", nil)
	anonymousRec := httptest.NewRecorder()
	handler.ServeHTTP(anonymousRec, anonymous)
	if anonymousRec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", anonymousRec.Code)
	}
	var env errorEnvelope
	if err := json.Unmarshal(anonymousRec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if env.Error.Code != "unauthorized" || env.Error.Message == "" {
		t.Errorf("unified error body = %+v, want code unauthorized and a message", env.Error)
	}

	// Unknown token: same 401.
	unknown := httptest.NewRequest(http.MethodGet, "/api/shop/orders", nil)
	unknown.Header.Set("Authorization", "Bearer not-a-real-token")
	unknownRec := httptest.NewRecorder()
	handler.ServeHTTP(unknownRec, unknown)
	if unknownRec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown token status = %d, want 401", unknownRec.Code)
	}
}

// TestLoginWrongPasswordAndUnknownEmailShareOne401 proves a failed login never
// reveals which part was wrong (AC-15).
func TestLoginWrongPasswordAndUnknownEmailShareOne401(t *testing.T) {
	server, st := openShopServer(t, 10)
	server.loginLimiter.reset()
	handler := server.Handler()

	email := shopTestEmail(t)
	t.Cleanup(func() { cleanupEmployee(t, st, email) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := st.BootstrapEmployee(ctx, email, "Werkstatt-Team", "correct-horse"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	wrong := doLogin(handler, "203.0.113.11:1111", email, "wrong-password")
	unknown := doLogin(handler, "203.0.113.12:1111", shopTestEmail(t), "whatever")

	for name, rec := range map[string]*httptest.ResponseRecorder{"wrong password": wrong, "unknown email": unknown} {
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", name, rec.Code)
		}
		var env errorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("%s: decode error body: %v", name, err)
		}
		if env.Error.Code != "unauthorized" {
			t.Errorf("%s: code = %q, want unauthorized", name, env.Error.Code)
		}
	}
	if wrong.Body.String() != unknown.Body.String() {
		t.Errorf("failed logins differ and hint at the wrong part:\n wrong:   %s\n unknown: %s",
			wrong.Body.String(), unknown.Body.String())
	}
	if bytes.Contains(wrong.Body.Bytes(), []byte("token")) {
		t.Error("a failed login issued a token")
	}
}

// TestLoginRateLimitIsPerClient proves repeated failures per client answer 429
// while a different client is unaffected (AC-29).
func TestLoginRateLimitIsPerClient(t *testing.T) {
	server, _ := openShopServer(t, 3)
	server.loginLimiter.reset()
	handler := server.Handler()

	const clientA = "203.0.113.20:1111"
	const clientB = "203.0.113.21:2222"
	email := shopTestEmail(t)

	for i := 1; i <= 3; i++ {
		rec := doLogin(handler, clientA, email, "wrong")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401", i, rec.Code)
		}
	}
	rec := doLogin(handler, clientA, email, "wrong")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked attempt status = %d, want 429 (body %s)", rec.Code, rec.Body.String())
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode 429 body: %v", err)
	}
	if env.Error.Code != "rate_limited" {
		t.Errorf("429 code = %q, want rate_limited", env.Error.Code)
	}

	// Another client has its own budget.
	if other := doLogin(handler, clientB, email, "wrong"); other.Code != http.StatusUnauthorized {
		t.Fatalf("second client status = %d, want 401 (limit is per client)", other.Code)
	}
}
