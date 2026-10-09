package api

import (
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

// openShopOrdersStore connects to the real PostgreSQL instance the office runs
// (SPEC AC-25), applying the schema. Skipped when DATABASE_URL is not set.
func openShopOrdersStore(t *testing.T) *store.Store {
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
	return st
}

func shopOrdersSuffix(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), t.Name())
}

// seedShopSession inserts one employee and one valid session and returns the
// bearer token. Both rows are removed when the test ends.
func seedShopSession(t *testing.T, st *store.Store) string {
	t.Helper()

	ctx := context.Background()
	suffix := shopOrdersSuffix(t)
	token := "session-" + suffix

	hash, err := store.HashPassword("secret-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	var employeeID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO employees (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`,
		"tester-"+suffix+"@example.de", hash, "Test Meister",
	).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO sessions (token, employee_id, expires_at) VALUES ($1, $2, now() + interval '1 hour')`,
		token, employeeID,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = st.Pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
		_, _ = st.Pool.Exec(ctx, `DELETE FROM employees WHERE id = $1`, employeeID)
	})

	return token
}

// seedShopOrder inserts one order with a position and an initial status entry
// and returns its identifiers. The rows are removed when the test ends.
func seedShopOrder(t *testing.T, st *store.Store, suffix, status, plate string) (orderNumber string) {
	t.Helper()

	ctx := context.Background()
	orderNumber = "AW-" + suffix

	var customerID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Testkunde "+suffix, "kunde-"+suffix+"@example.de", "0170 0000000",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var vehicleID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		plate, "BMW", "320d", 90000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}

	var orderID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orderNumber, customerID, vehicleID, status, "2025-04-02", "Bremsen quietschen",
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO status_log (order_id, from_status, to_status) VALUES ($1, NULL, $2)`,
		orderID, status,
	); err != nil {
		t.Fatalf("insert status log: %v", err)
	}

	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO order_items (order_id, kind, description, quantity, hours, unit_price_cents, amount_cents)
		 VALUES ($1, 'labor', 'Arbeitszeit', 1, 1.5, 9500, 14250)`,
		orderID,
	); err != nil {
		t.Fatalf("insert order item: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = st.Pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, orderID)
		_, _ = st.Pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, vehicleID)
		_, _ = st.Pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, customerID)
	})

	return orderNumber
}

func newShopOrdersServer(t *testing.T) (*Server, *store.Store, string) {
	t.Helper()
	st := openShopOrdersStore(t)
	token := seedShopSession(t, st)
	cfg := &config.Config{APIPort: "0", CORSAllowedOrigin: "http://localhost:5173"}
	return NewServer(st, cfg, nil), st, token
}

func shopOrdersGET(t *testing.T, srv *Server, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// TestShopOrdersUnfilteredList proves the endpoint returns every order with
// status and plate.
func TestShopOrdersUnfilteredList(t *testing.T) {
	srv, st, token := newShopOrdersServer(t)
	suffix := shopOrdersSuffix(t)

	first := seedShopOrder(t, st, suffix+"-A", "confirmed", "TST-"+suffix+"-A")
	second := seedShopOrder(t, st, suffix+"-B", "requested", "TST-"+suffix+"-B")

	rec := shopOrdersGET(t, srv, "/api/shop/orders", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Orders []store.OrderSummary `json:"orders"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	byNumber := map[string]store.OrderSummary{}
	for _, o := range body.Orders {
		byNumber[o.OrderNumber] = o
	}
	for _, want := range []string{first, second} {
		got, ok := byNumber[want]
		if !ok {
			t.Fatalf("response is missing order %q", want)
		}
		if got.Status == "" || got.VehiclePlate == "" {
			t.Errorf("order %q is missing status or plate: %+v", want, got)
		}
	}
}

// TestShopOrdersStatusFilter proves the status parameter narrows the list.
func TestShopOrdersStatusFilter(t *testing.T) {
	srv, st, token := newShopOrdersServer(t)
	suffix := shopOrdersSuffix(t)

	confirmed := seedShopOrder(t, st, suffix+"-C", "confirmed", "TST-"+suffix+"-C")
	requested := seedShopOrder(t, st, suffix+"-D", "requested", "TST-"+suffix+"-D")

	rec := shopOrdersGET(t, srv, "/api/shop/orders?status=confirmed", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Orders []store.OrderSummary `json:"orders"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	found := map[string]bool{}
	for _, o := range body.Orders {
		if o.Status != "confirmed" {
			t.Errorf("filter returned order %q with status %q", o.OrderNumber, o.Status)
		}
		found[o.OrderNumber] = true
	}
	if !found[confirmed] {
		t.Errorf("filter is missing confirmed order %q", confirmed)
	}
	if found[requested] {
		t.Errorf("filter returned requested order %q", requested)
	}
}

// TestShopOrdersPlateSearch proves the plate parameter returns only the
// matching vehicle's orders.
func TestShopOrdersPlateSearch(t *testing.T) {
	srv, st, token := newShopOrdersServer(t)
	suffix := shopOrdersSuffix(t)

	alpha := seedShopOrder(t, st, suffix+"-E", "requested", "TST-"+suffix+"-E")
	beta := seedShopOrder(t, st, suffix+"-F", "requested", "TST-"+suffix+"-F")

	search := "TST-" + suffix + "-E"
	rec := shopOrdersGET(t, srv, "/api/shop/orders?plate="+search, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Orders []store.OrderSummary `json:"orders"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if len(body.Orders) == 0 {
		t.Fatalf("plate search returned no order for %q", search)
	}
	alphaFound := false
	for _, o := range body.Orders {
		if o.OrderNumber == beta {
			t.Errorf("plate search returned the other vehicle's order %q", beta)
		}
		if o.OrderNumber == alpha {
			alphaFound = true
		}
	}
	if !alphaFound {
		t.Errorf("plate search is missing order %q", alpha)
	}
}

// TestShopOrderDetail proves the detail route returns one order with positions
// and status history.
func TestShopOrderDetail(t *testing.T) {
	srv, st, token := newShopOrdersServer(t)
	suffix := shopOrdersSuffix(t)

	orderNumber := seedShopOrder(t, st, suffix+"-G", "in_progress", "TST-"+suffix+"-G")

	rec := shopOrdersGET(t, srv, "/api/shop/orders/"+orderNumber, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var detail store.OrderDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if detail.OrderNumber != orderNumber {
		t.Errorf("order_number = %q, want %q", detail.OrderNumber, orderNumber)
	}
	if detail.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", detail.Status)
	}
	if detail.Problem == "" || detail.Customer.Name == "" || detail.Vehicle.Plate == "" {
		t.Errorf("detail is missing problem, customer or vehicle: %+v", detail)
	}
	if len(detail.Items) != 1 {
		t.Errorf("items = %d, want 1", len(detail.Items))
	}
	if len(detail.History) != 1 {
		t.Errorf("history = %d, want 1", len(detail.History))
	}
}

// TestShopOrderDetailUnknown proves an unknown order number answers 404 in the
// unified error body.
func TestShopOrderDetailUnknown(t *testing.T) {
	srv, _, token := newShopOrdersServer(t)

	rec := shopOrdersGET(t, srv, "/api/shop/orders/AW-unknown-"+shopOrdersSuffix(t), token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code == "" || body.Error.Message == "" {
		t.Errorf("error body is not the unified shape: %s", rec.Body.String())
	}
}

// TestShopOrdersRequireSession proves the shop order routes answer 401 without
// a valid bearer token (SPEC AC-28).
func TestShopOrdersRequireSession(t *testing.T) {
	srv, _, _ := newShopOrdersServer(t)

	for _, path := range []string{"/api/shop/orders", "/api/shop/orders/AW-1"} {
		rec := shopOrdersGET(t, srv, path, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without token status = %d, want 401", path, rec.Code)
		}
	}
}
