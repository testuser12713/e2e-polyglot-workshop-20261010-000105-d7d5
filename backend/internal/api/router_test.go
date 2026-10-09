package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"workshop/backend/internal/config"
)

func newTestServer() *Server {
	return NewServer(nil, &config.Config{
		APIPort:           "0",
		CORSAllowedOrigin: "http://localhost:5173",
	}, nil)
}

// TestServerStartsAndHealthAnswersOK proves the handler can be started and the
// health endpoint answers 200 with {"status":"ok"}.
func TestServerStartsAndHealthAnswersOK(t *testing.T) {
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/health status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode health body: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("health status = %q, want %q", body.Status, "ok")
	}
}

// TestEveryContractRouteIsRegistered checks that each route of the agreed
// contract is wired to a handler. It deliberately asserts only that the route
// exists (not 404/405), never what a placeholder answers.
func TestEveryContractRouteIsRegistered(t *testing.T) {
	handler := newTestServer().Handler()

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/public/customers"},
		{http.MethodGet, "/api/public/customers/1"},
		{http.MethodPost, "/api/public/vehicles"},
		{http.MethodPost, "/api/public/orders"},
		{http.MethodGet, "/api/public/orders/AW-2025-000001"},
		{http.MethodGet, "/api/public/orders/AW-2025-000001/invoice"},
		{http.MethodGet, "/api/health"},
		{http.MethodPost, "/api/shop/login"},
		{http.MethodGet, "/api/shop/orders"},
		{http.MethodGet, "/api/shop/orders/AW-2025-000001"},
		{http.MethodPost, "/api/shop/orders/AW-2025-000001/status"},
		{http.MethodPost, "/api/shop/orders/AW-2025-000001/items"},
		{http.MethodGet, "/api/shop/dashboard"},
	}

	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
			t.Errorf("route %s %s is not registered (status %d)", route.method, route.path, rec.Code)
		}
	}
}
