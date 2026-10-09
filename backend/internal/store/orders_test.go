package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// openTestStore connects to the real PostgreSQL instance the office provides
// (SPEC AC-25) and returns a shared context. It is skipped when DATABASE_URL is
// not set. The schema is applied by Open, so the test provisions the tables it
// uses. Both this file and the auth tests use this single helper.
func openTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	st, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(st.Close)
	return st, ctx
}

// cleanupOrder removes the rows one CreateOrder call created, in foreign-key
// order. It touches only the tables this slice owns plus the customer/vehicle
// rows the test itself created (unique per test run).
func cleanupOrder(t *testing.T, st *Store, created *CreatedOrder) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := st.Pool.Exec(ctx, `DELETE FROM status_log WHERE order_id = $1`, created.ID); err != nil {
		t.Logf("cleanup status_log: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM order_items WHERE order_id = $1`, created.ID); err != nil {
		t.Logf("cleanup order_items: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, created.ID); err != nil {
		t.Logf("cleanup orders: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, created.VehicleID); err != nil {
		t.Logf("cleanup vehicles: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, created.CustomerID); err != nil {
		t.Logf("cleanup customers: %v", err)
	}
}

// TestCreateOrderPersistsOrderItemsAndStatus proves the whole create flow:
// customer and vehicle are resolved or created, the order starts as
// "requested", both positions are stored with their amounts and exactly one
// status-log entry exists (SPEC AC-03, AC-05).
func TestCreateOrderPersistsOrderItemsAndStatus(t *testing.T) {
	st, ctx := openTestStore(t)
	unique := time.Now().UnixNano()

	params := CreateOrderParams{
		Customer: CustomerInput{
			Name:  "Anna Beispiel",
			Email: fmt.Sprintf("store-order-%d@example.de", unique),
			Phone: "0151 1234567",
		},
		Vehicle: VehicleInput{
			Plate:   fmt.Sprintf("STORE-%d", unique),
			Make:    "VW",
			Model:   "Golf",
			Mileage: 123456,
		},
		DesiredDate: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
		Problem:     "Die Bremsen quietschen bei Nässe.",
		Items: []OrderItemInput{
			{Kind: "labor", Description: "Bremsen prüfen", Hours: 1.5, UnitPriceCents: 9500},
			{Kind: "part", Description: "Bremsbelag", Quantity: 2, UnitPriceCents: 4500},
		},
		HourlyRateCents: 9500,
	}

	created, err := st.CreateOrder(ctx, params)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if created == nil {
		t.Fatal("CreateOrder returned nil")
	}
	t.Cleanup(func() { cleanupOrder(t, st, created) })

	if created.Status != "requested" {
		t.Errorf("created status = %q, want %q", created.Status, "requested")
	}
	if created.OrderNumber == "" {
		t.Fatal("order number is empty")
	}
	wantNumber := fmt.Sprintf("AW-%d-%06d", created.CreatedAt.UTC().Year(), created.ID)
	if created.OrderNumber != wantNumber {
		t.Errorf("order number = %q, want %q", created.OrderNumber, wantNumber)
	}

	var (
		status      string
		problem     string
		customerID  int64
		vehicleID   int64
		desiredDate time.Time
	)
	const orderQuery = `SELECT status, problem, customer_id, vehicle_id, desired_date FROM orders WHERE id = $1`
	if err := st.Pool.QueryRow(ctx, orderQuery, created.ID).
		Scan(&status, &problem, &customerID, &vehicleID, &desiredDate); err != nil {
		t.Fatalf("read order: %v", err)
	}
	if status != "requested" {
		t.Errorf("stored status = %q, want requested", status)
	}
	if problem != params.Problem {
		t.Errorf("stored problem = %q, want %q", problem, params.Problem)
	}
	if customerID != created.CustomerID || vehicleID != created.VehicleID {
		t.Errorf("stored ids = (%d,%d), want (%d,%d)", customerID, vehicleID, created.CustomerID, created.VehicleID)
	}
	if got := desiredDate.UTC().Format("2006-01-02"); got != "2026-11-03" {
		t.Errorf("stored desired_date = %q, want 2026-11-03", got)
	}

	type itemRow struct {
		kind   string
		amount int64
	}
	rows, err := st.Pool.Query(ctx, `SELECT kind, amount_cents FROM order_items WHERE order_id = $1 ORDER BY id`, created.ID)
	if err != nil {
		t.Fatalf("read order items: %v", err)
	}
	var items []itemRow
	for rows.Next() {
		var r itemRow
		if err := rows.Scan(&r.kind, &r.amount); err != nil {
			t.Fatalf("scan order item: %v", err)
		}
		items = append(items, r)
	}
	rows.Close()
	if len(items) != 2 {
		t.Fatalf("stored %d items, want 2", len(items))
	}
	if items[0].kind != "labor" || items[0].amount != 14250 {
		t.Errorf("labour item = %+v, want kind labor amount 14250", items[0])
	}
	if items[1].kind != "part" || items[1].amount != 9000 {
		t.Errorf("part item = %+v, want kind part amount 9000", items[1])
	}

	history, err := st.ListStatusLog(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListStatusLog: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("status log has %d entries, want 1", len(history))
	}
	if history[0].FromStatus != nil {
		t.Errorf("initial status from_status = %v, want nil", *history[0].FromStatus)
	}
	if history[0].ToStatus != "requested" {
		t.Errorf("initial status to_status = %q, want requested", history[0].ToStatus)
	}
}

// TestCreateOrderResolvesExistingCustomerAndVehicle proves that a second order
// with the same e-mail and plate reuses the existing rows instead of creating
// duplicates.
func TestCreateOrderResolvesExistingCustomerAndVehicle(t *testing.T) {
	st, ctx := openTestStore(t)
	unique := time.Now().UnixNano()
	email := fmt.Sprintf("store-reuse-%d@example.de", unique)
	plate := fmt.Sprintf("REUSE-%d", unique)
	date := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)

	first, err := st.CreateOrder(ctx, CreateOrderParams{
		Customer:    CustomerInput{Name: "Erster Name", Email: email, Phone: "1"},
		Vehicle:     VehicleInput{Plate: plate, Make: "Audi", Model: "A3", Mileage: 1000},
		DesiredDate: date,
		Items:       nil,
	})
	if err != nil {
		t.Fatalf("first CreateOrder: %v", err)
	}
	second, err := st.CreateOrder(ctx, CreateOrderParams{
		Customer:    CustomerInput{Name: "Zweiter Name", Email: email, Phone: "2"},
		Vehicle:     VehicleInput{Plate: plate, Make: "Audi", Model: "A3", Mileage: 2000},
		DesiredDate: date,
		Items:       nil,
	})
	if err != nil {
		t.Fatalf("second CreateOrder: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, order := range []*CreatedOrder{first, second} {
			_, _ = st.Pool.Exec(ctx, `DELETE FROM status_log WHERE order_id = $1`, order.ID)
			_, _ = st.Pool.Exec(ctx, `DELETE FROM order_items WHERE order_id = $1`, order.ID)
			_, _ = st.Pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, order.ID)
		}
		_, _ = st.Pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, first.VehicleID)
		_, _ = st.Pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, first.CustomerID)
	})

	if first.CustomerID != second.CustomerID {
		t.Errorf("customer ids differ: %d vs %d", first.CustomerID, second.CustomerID)
	}
	if first.VehicleID != second.VehicleID {
		t.Errorf("vehicle ids differ: %d vs %d", first.VehicleID, second.VehicleID)
	}
	if first.OrderNumber == second.OrderNumber {
		t.Errorf("two orders share the order number %q", first.OrderNumber)
	}
}

// TestAmountCents checks the position amount rules, including the hourly-rate
// fallback for labour.
func TestAmountCents(t *testing.T) {
	cases := []struct {
		name string
		item OrderItemInput
		rate int64
		want int64
	}{
		{"labour uses position price", OrderItemInput{Kind: "labor", Hours: 2, UnitPriceCents: 9500}, 8000, 19000},
		{"labour falls back to rate", OrderItemInput{Kind: "labor", Hours: 1.5, UnitPriceCents: 0}, 8000, 12000},
		{"part multiplies quantity", OrderItemInput{Kind: "part", Quantity: 3, UnitPriceCents: 4500}, 8000, 13500},
		{"part rounds to nearest cent", OrderItemInput{Kind: "part", Quantity: 0.5, UnitPriceCents: 999}, 0, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := amountCents(tc.item, tc.rate); got != tc.want {
				t.Errorf("amountCents(%+v, %d) = %d, want %d", tc.item, tc.rate, got, tc.want)
			}
		})
	}
}
