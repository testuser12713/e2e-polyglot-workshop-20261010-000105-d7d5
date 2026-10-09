package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"workshop/backend/internal/config"
	"workshop/backend/internal/store"
)

// newOrderTestServer wires a Server around a real PostgreSQL store (SPEC AC-25).
// It skips when DATABASE_URL is not set. The store applies the schema on Open.
func newOrderTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	st, err := store.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(st.Close)

	cfg := &config.Config{
		WorkshopHourlyRateCents: 9500,
		CORSAllowedOrigin:       "http://localhost:5173",
	}
	return NewServer(st, cfg, nil), st
}

// cleanupOrderByNumber removes the rows created by one order request, touching
// only the tables of this slice plus the customer/vehicle rows the test created.
func cleanupOrderByNumber(t *testing.T, st *store.Store, orderNumber string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var orderID, customerID, vehicleID int64
	const find = `SELECT id, customer_id, vehicle_id FROM orders WHERE order_number = $1`
	if err := st.Pool.QueryRow(ctx, find, orderNumber).Scan(&orderID, &customerID, &vehicleID); err != nil {
		return
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM status_log WHERE order_id = $1`, orderID); err != nil {
		t.Logf("cleanup status_log: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM order_items WHERE order_id = $1`, orderID); err != nil {
		t.Logf("cleanup order_items: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, orderID); err != nil {
		t.Logf("cleanup orders: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, vehicleID); err != nil {
		t.Logf("cleanup vehicles: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, customerID); err != nil {
		t.Logf("cleanup customers: %v", err)
	}
}

// TestCreateOrderEndpointReturns201 posts a complete order request and expects
// 201 with a fresh order number and status "requested", with the order, its two
// positions and one status-log entry persisted (SPEC AC-03, AC-05).
func TestCreateOrderEndpointReturns201(t *testing.T) {
	srv, st := newOrderTestServer(t)
	unique := time.Now().UnixNano()

	body := fmt.Sprintf(`{
		"customer": {"name": "Anna Beispiel", "email": "api-order-%d@example.de", "phone": "0151 1234567"},
		"vehicle": {"plate": "API-%d", "make": "VW", "model": "Golf", "mileage": 123456},
		"desired_date": "2026-11-03",
		"problem": "Die Bremsen quietschen bei Nässe.",
		"items": [
			{"kind": "labor", "description": "Bremsen prüfen", "quantity": 0, "hours": 1.5, "unit_price_cents": 9500},
			{"kind": "part", "description": "Bremsbelag", "quantity": 2, "hours": 0, "unit_price_cents": 4500}
		]
	}`, unique, unique)

	req := httptest.NewRequest(http.MethodPost, "/api/public/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		OrderNumber string `json:"order_number"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "requested" {
		t.Errorf("status = %q, want requested", resp.Status)
	}
	if resp.OrderNumber == "" {
		t.Fatal("order number is empty")
	}
	t.Cleanup(func() { cleanupOrderByNumber(t, st, resp.OrderNumber) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var (
		orderID int64
		status  string
	)
	if err := st.Pool.QueryRow(ctx, `SELECT id, status FROM orders WHERE order_number = $1`, resp.OrderNumber).
		Scan(&orderID, &status); err != nil {
		t.Fatalf("read stored order: %v", err)
	}
	if status != "requested" {
		t.Errorf("stored status = %q, want requested", status)
	}

	var itemCount int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM order_items WHERE order_id = $1`, orderID).Scan(&itemCount); err != nil {
		t.Fatalf("count order items: %v", err)
	}
	if itemCount != 2 {
		t.Errorf("stored item count = %d, want 2", itemCount)
	}

	var logCount int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM status_log WHERE order_id = $1`, orderID).Scan(&logCount); err != nil {
		t.Fatalf("count status log: %v", err)
	}
	if logCount != 1 {
		t.Errorf("stored status log count = %d, want 1", logCount)
	}
}

// TestCreateOrderEndpointRejectsInvalidBody checks malformed and incomplete
// requests answer 422 in the unified error envelope.
func TestCreateOrderEndpointRejectsInvalidBody(t *testing.T) {
	srv, _ := newOrderTestServer(t)

	cases := map[string]string{
		"malformed json": `{"customer":`,
		"missing fields": `{}`,
		"bad email": `{"customer":{"name":"A","email":"nope","phone":"1"},
			"vehicle":{"plate":"X","make":"Y","model":"Z"},"desired_date":"2026-11-03"}`,
		"bad date": `{"customer":{"name":"A","email":"a@example.de","phone":"1"},
			"vehicle":{"plate":"X","make":"Y","model":"Z"},"desired_date":"morgen"}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/public/orders", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body = %s", rec.Code, rec.Body.String())
			}
			var errBody apiErrorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if errBody.Error.Code == "" || errBody.Error.Message == "" {
				t.Errorf("error envelope is incomplete: %+v", errBody)
			}
		})
	}
}

// TestCreateOrderEndpointLogsNoPersonalData proves AC-33 for this route: no log
// line carries the customer name, e-mail, phone or the licence plate.
func TestCreateOrderEndpointLogsNoPersonalData(t *testing.T) {
	srv, st := newOrderTestServer(t)
	unique := time.Now().UnixNano()

	name := "Logtest Person"
	email := fmt.Sprintf("logtest-%d@example.de", unique)
	phone := "0170 555444"
	plate := fmt.Sprintf("LOG-%d", unique)

	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(previous)

	body := fmt.Sprintf(`{
		"customer": {"name": %q, "email": %q, "phone": %q},
		"vehicle": {"plate": %q, "make": "VW", "model": "Golf", "mileage": 10},
		"desired_date": "2026-11-03",
		"problem": "Leerlauf unrund.",
		"items": []
	}`, name, email, phone, plate)

	req := httptest.NewRequest(http.MethodPost, "/api/public/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		OrderNumber string `json:"order_number"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	t.Cleanup(func() { cleanupOrderByNumber(t, st, resp.OrderNumber) })

	logged := buf.String()
	for _, secret := range []string{name, email, phone, plate} {
		if strings.Contains(logged, secret) {
			t.Errorf("log output contains personal data %q:\n%s", secret, logged)
		}
	}
}
