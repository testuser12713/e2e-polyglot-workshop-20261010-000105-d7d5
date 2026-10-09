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

// dashboardTestLockKey serializes the dashboard integration tests across the
// api and store packages, which `go test ./...` runs in parallel against the
// same PostgreSQL instance. The store test uses the same key.
const dashboardTestLockKey int64 = 882374100123

// openDashboardTestStore connects to the real PostgreSQL instance, acquires the
// cross-package advisory lock and registers cleanup. It skips when DATABASE_URL
// is not provided.
func openDashboardTestStore(t *testing.T) *store.Store {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	conn, err := st.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection for advisory lock: %v", err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, dashboardTestLockKey); err != nil {
		conn.Release()
		t.Fatalf("acquire advisory lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, dashboardTestLockKey)
		conn.Release()
	})

	return st
}

// seedDashboardData inserts a fixed set of orders, status-log entries, invoices
// and one employee session. It returns the session token and, via t.Cleanup,
// removes every row it created. The expected metric deltas are:
//
//	open_orders        +5  (#1..#5, everything except the picked-up #6)
//	finished_today     +2  (#4 and #5 logged 'done' today; #6 logged two days ago)
//	revenue_month_cents +15000 (invoice #5 10000 + #6 5000 this month; #4's
//	                           invoice is 40 days old and therefore last month)
func seedDashboardData(t *testing.T, st *store.Store) string {
	t.Helper()
	ctx := context.Background()
	tag := fmt.Sprintf("%d", time.Now().UnixNano())

	var customerID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Dashboard Test", "dashboard-"+tag+"@example.test", "0000000000",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var vehicleID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		"DT-"+tag, "Test", "Model", 1000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}

	orderNumbers := make([]int64, 0, 8)
	insertOrder := func(suffix, status string) int64 {
		var id int64
		if err := st.Pool.QueryRow(ctx,
			`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
			 VALUES ($1, $2, $3, $4, current_date, '') RETURNING id`,
			"DT-"+tag+"-"+suffix, customerID, vehicleID, status,
		).Scan(&id); err != nil {
			t.Fatalf("insert order %s: %v", suffix, err)
		}
		orderNumbers = append(orderNumbers, id)
		return id
	}

	insertLog := func(orderID int64, to string, at time.Time) {
		if _, err := st.Pool.Exec(ctx,
			`INSERT INTO status_log (order_id, at, to_status) VALUES ($1, $2, $3)`,
			orderID, at, to,
		); err != nil {
			t.Fatalf("insert status log: %v", err)
		}
	}

	insertInvoice := func(orderID int64, number string, gross int64, createdAt time.Time) {
		if _, err := st.Pool.Exec(ctx,
			`INSERT INTO invoices (invoice_number, order_id, net_cents, tax_cents, gross_cents, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			number, orderID, gross, 0, gross, createdAt,
		); err != nil {
			t.Fatalf("insert invoice: %v", err)
		}
	}

	insertOrder("01-requested", "requested")
	insertOrder("02-confirmed", "confirmed")
	insertOrder("03-inprogress", "in_progress")
	doneToday := insertOrder("04-donetoday", "done")
	doneInvoice := insertOrder("05-doneinvoice", "done")
	pickedUp := insertOrder("06-pickedup", "picked_up")

	now := time.Now().UTC()
	insertLog(doneToday, "done", now)
	insertLog(doneInvoice, "done", now)
	insertLog(pickedUp, "done", now.Add(-48*time.Hour))

	insertInvoice(doneInvoice, "RE-"+tag+"-1", 10000, now)
	insertInvoice(pickedUp, "RE-"+tag+"-2", 5000, now)
	insertInvoice(doneToday, "RE-"+tag+"-3", 9999, now.Add(-40*24*time.Hour))

	var employeeID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO employees (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`,
		"dashboard-"+tag+"@example.test", "x", "Dashboard Tester",
	).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}

	token := "dashboard-session-" + tag
	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO sessions (token, employee_id, expires_at) VALUES ($1, $2, now() + interval '1 hour')`,
		token, employeeID,
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM invoices WHERE order_id = ANY($1)`, orderNumbers)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM status_log WHERE order_id = ANY($1)`, orderNumbers)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM order_items WHERE order_id = ANY($1)`, orderNumbers)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM orders WHERE id = ANY($1)`, orderNumbers)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM sessions WHERE employee_id = $1`, employeeID)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM employees WHERE id = $1`, employeeID)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM vehicles WHERE id = $1`, vehicleID)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM customers WHERE id = $1`, customerID)
	})

	return token
}

// TestShopDashboardReturnsSeededMetrics drives GET /api/shop/dashboard through
// the real router with a valid session and checks the three metrics against the
// seeded rows, measured as deltas so the test is independent of any other data.
func TestShopDashboardReturnsSeededMetrics(t *testing.T) {
	st := openDashboardTestStore(t)

	before, err := st.DashboardStats(context.Background())
	if err != nil {
		t.Fatalf("baseline DashboardStats: %v", err)
	}

	token := seedDashboardData(t, st)

	srv := NewServer(st, &config.Config{CORSAllowedOrigin: "http://localhost:5173"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/shop/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/shop/dashboard status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var got struct {
		OpenOrders        int64 `json:"open_orders"`
		FinishedToday     int64 `json:"finished_today"`
		RevenueMonthCents int64 `json:"revenue_month_cents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode dashboard body: %v", err)
	}

	if want := before.OpenOrders + 5; got.OpenOrders != want {
		t.Errorf("open_orders = %d, want %d (baseline %d + 5 seeded)", got.OpenOrders, want, before.OpenOrders)
	}
	if want := before.FinishedToday + 2; got.FinishedToday != want {
		t.Errorf("finished_today = %d, want %d (baseline %d + 2 seeded)", got.FinishedToday, want, before.FinishedToday)
	}
	if want := before.RevenueMonthCents + 15000; got.RevenueMonthCents != want {
		t.Errorf("revenue_month_cents = %d, want %d (baseline %d + 15000 seeded)", got.RevenueMonthCents, want, before.RevenueMonthCents)
	}
}

// TestShopDashboardRejectsMissingSession checks that a caller without an
// Authorization header gets 401, not the workshop metrics. It needs no database:
// the handler must refuse the request before it touches the store (SPEC AC-28).
func TestShopDashboardRejectsMissingSession(t *testing.T) {
	srv := NewServer(nil, &config.Config{CORSAllowedOrigin: "http://localhost:5173"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/shop/dashboard", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/shop/dashboard without token status = %d, want 401; body: %s", rec.Code, rec.Body.String())
	}
}
