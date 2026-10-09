package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// dashboardTestLockKey serializes the dashboard integration tests across the
// store and api packages, which `go test ./...` runs in parallel against the
// same PostgreSQL instance. The api test uses the same key.
const dashboardTestLockKey int64 = 882374100123

// openDashboardStore connects to the real PostgreSQL instance and acquires the
// cross-package advisory lock. It skips when DATABASE_URL is not provided.
func openDashboardStore(t *testing.T) *Store {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	ctx := context.Background()
	st, err := Open(ctx, databaseURL)
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

// seedDashboardRows inserts one customer, one vehicle and a fixed set of orders,
// status-log entries and invoices, then removes them again in t.Cleanup.
func seedDashboardRows(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()
	tag := fmt.Sprintf("%d", time.Now().UnixNano())

	var customerID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Dashboard Store Test", "dashboard-store-"+tag+"@example.test", "0000000001",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var vehicleID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		"DS-"+tag, "Test", "Model", 2000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}

	orderIDs := make([]int64, 0, 6)
	insertOrder := func(suffix, status string) int64 {
		var id int64
		if err := st.Pool.QueryRow(ctx,
			`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
			 VALUES ($1, $2, $3, $4, current_date, '') RETURNING id`,
			"DS-"+tag+"-"+suffix, customerID, vehicleID, status,
		).Scan(&id); err != nil {
			t.Fatalf("insert order %s: %v", suffix, err)
		}
		orderIDs = append(orderIDs, id)
		return id
	}

	now := time.Now().UTC()

	insertOrder("01-requested", "requested")
	insertOrder("02-confirmed", "confirmed")
	insertOrder("03-inprogress", "in_progress")
	doneToday := insertOrder("04-donetoday", "done")
	doneInvoice := insertOrder("05-doneinvoice", "done")
	pickedUp := insertOrder("06-pickedup", "picked_up")

	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO status_log (order_id, at, to_status) VALUES ($1, $2, 'done'), ($3, $4, 'done'), ($5, $6, 'done')`,
		doneToday, now, doneInvoice, now, pickedUp, now.Add(-48*time.Hour),
	); err != nil {
		t.Fatalf("insert status logs: %v", err)
	}

	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO invoices (invoice_number, order_id, net_cents, tax_cents, gross_cents, created_at)
		 VALUES ($1, $2, $3, 0, $3, $4), ($5, $6, $7, 0, $7, $8), ($9, $10, $11, 0, $11, $12)`,
		"DS-RE-"+tag+"-1", doneInvoice, int64(10000), now,
		"DS-RE-"+tag+"-2", pickedUp, int64(5000), now,
		"DS-RE-"+tag+"-3", doneToday, int64(9999), now.Add(-40*24*time.Hour),
	); err != nil {
		t.Fatalf("insert invoices: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM invoices WHERE order_id = ANY($1)`, orderIDs)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM status_log WHERE order_id = ANY($1)`, orderIDs)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM order_items WHERE order_id = ANY($1)`, orderIDs)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM orders WHERE id = ANY($1)`, orderIDs)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM vehicles WHERE id = $1`, vehicleID)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM customers WHERE id = $1`, customerID)
	})
}

// TestDashboardStatsCountsSeededRows runs the three dashboard queries against
// real PostgreSQL and checks each metric against the seeded rows as a delta.
func TestDashboardStatsCountsSeededRows(t *testing.T) {
	st := openDashboardStore(t)
	ctx := context.Background()

	before, err := st.DashboardStats(ctx)
	if err != nil {
		t.Fatalf("baseline DashboardStats: %v", err)
	}

	seedDashboardRows(t, st)

	got, err := st.DashboardStats(ctx)
	if err != nil {
		t.Fatalf("DashboardStats: %v", err)
	}

	if want := before.OpenOrders + 5; got.OpenOrders != want {
		t.Errorf("OpenOrders = %d, want %d (baseline %d + 5)", got.OpenOrders, want, before.OpenOrders)
	}
	if want := before.FinishedToday + 2; got.FinishedToday != want {
		t.Errorf("FinishedToday = %d, want %d (baseline %d + 2)", got.FinishedToday, want, before.FinishedToday)
	}
	if want := before.RevenueMonthCents + 15000; got.RevenueMonthCents != want {
		t.Errorf("RevenueMonthCents = %d, want %d (baseline %d + 15000)", got.RevenueMonthCents, want, before.RevenueMonthCents)
	}
}

// TestDashboardStatsWithoutPool proves a Store without a pool reports an error
// instead of panicking.
func TestDashboardStatsWithoutPool(t *testing.T) {
	var st Store
	if _, err := st.DashboardStats(context.Background()); err == nil {
		t.Fatal("DashboardStats on an unconfigured store should fail, got nil")
	}
}
