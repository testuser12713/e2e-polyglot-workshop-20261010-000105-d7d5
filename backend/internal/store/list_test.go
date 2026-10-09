package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// openListTestStore connects to the real PostgreSQL instance the office runs
// (SPEC AC-25) and applies the schema. It is skipped where DATABASE_URL is not
// provided so the suite still runs on a machine without a database.
func openListTestStore(t *testing.T) *Store {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	st, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(st.Close)
	return st
}

type seededOrder struct {
	orderID     int64
	orderNumber string
	plate       string
	customerID  int64
	vehicleID   int64
}

// seedOrder inserts one customer, vehicle, order, initial status entry and two
// positions (labour and part) with unique identifiers, and removes exactly
// those rows when the test ends.
func seedOrder(t *testing.T, st *Store, suffix, status, plate string) seededOrder {
	t.Helper()

	ctx := context.Background()
	seeded := seededOrder{
		orderNumber: "AW-" + suffix,
		plate:       plate,
	}

	email := fmt.Sprintf("kunde-%s@example.de", suffix)
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Testkunde "+suffix, email, "0170 0000000",
	).Scan(&seeded.customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		plate, "VW", "Golf", 120000,
	).Scan(&seeded.vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}

	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		seeded.orderNumber, seeded.customerID, seeded.vehicleID, status, "2025-03-07", "Motor ruckelt",
	).Scan(&seeded.orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO status_log (order_id, from_status, to_status) VALUES ($1, NULL, $2)`,
		seeded.orderID, status,
	); err != nil {
		t.Fatalf("insert status log: %v", err)
	}

	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO order_items (order_id, kind, description, quantity, hours, unit_price_cents, amount_cents)
		 VALUES ($1, 'labor', 'Arbeitszeit', 1, 2.5, 9500, 23750),
		        ($1, 'part', 'Bremsbelag', 2, 0, 4500, 9000)`,
		seeded.orderID,
	); err != nil {
		t.Fatalf("insert order items: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = st.Pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, seeded.orderID)
		_, _ = st.Pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, seeded.vehicleID)
		_, _ = st.Pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, seeded.customerID)
	})

	return seeded
}

func uniqueSuffix(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), t.Name())
}

// TestListOrdersUnfiltered proves the unfiltered list contains every seeded
// order with its status and plate.
func TestListOrdersUnfiltered(t *testing.T) {
	st := openListTestStore(t)
	suffix := uniqueSuffix(t)

	confirmed := seedOrder(t, st, suffix+"-A", "confirmed", "TST-"+suffix+"-A")
	requested := seedOrder(t, st, suffix+"-B", "requested", "TST-"+suffix+"-B")

	orders, err := st.ListOrders(context.Background(), OrderFilter{})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}

	byNumber := map[string]OrderSummary{}
	for _, o := range orders {
		byNumber[o.OrderNumber] = o
	}

	for _, want := range []seededOrder{confirmed, requested} {
		got, ok := byNumber[want.orderNumber]
		if !ok {
			t.Fatalf("unfiltered list is missing order %q", want.orderNumber)
		}
		if got.VehiclePlate != want.plate {
			t.Errorf("order %q plate = %q, want %q", want.orderNumber, got.VehiclePlate, want.plate)
		}
		if got.Status == "" {
			t.Errorf("order %q has an empty status", want.orderNumber)
		}
		if got.Make != "VW" || got.Model != "Golf" {
			t.Errorf("order %q vehicle = %q %q, want VW Golf", want.orderNumber, got.Make, got.Model)
		}
		if got.DesiredDate != "2025-03-07" {
			t.Errorf("order %q desired_date = %q, want 2025-03-07", want.orderNumber, got.DesiredDate)
		}
	}
}

// TestListOrdersStatusFilter proves that only orders of the requested status
// are returned.
func TestListOrdersStatusFilter(t *testing.T) {
	st := openListTestStore(t)
	suffix := uniqueSuffix(t)

	confirmed := seedOrder(t, st, suffix+"-C", "confirmed", "TST-"+suffix+"-C")
	requested := seedOrder(t, st, suffix+"-D", "requested", "TST-"+suffix+"-D")

	orders, err := st.ListOrders(context.Background(), OrderFilter{Status: "confirmed"})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}

	found := map[string]bool{}
	for _, o := range orders {
		if o.Status != "confirmed" {
			t.Errorf("status filter returned order %q with status %q", o.OrderNumber, o.Status)
		}
		found[o.OrderNumber] = true
	}
	if !found[confirmed.orderNumber] {
		t.Errorf("status filter is missing order %q", confirmed.orderNumber)
	}
	if found[requested.orderNumber] {
		t.Errorf("status filter returned requested order %q", requested.orderNumber)
	}
}

// TestListOrdersPlateSearch proves that the plate search returns only the
// orders of the matching vehicle.
func TestListOrdersPlateSearch(t *testing.T) {
	st := openListTestStore(t)
	suffix := uniqueSuffix(t)

	alpha := seedOrder(t, st, suffix+"-E", "requested", "TST-"+suffix+"-E")
	beta := seedOrder(t, st, suffix+"-F", "requested", "TST-"+suffix+"-F")

	orders, err := st.ListOrders(context.Background(), OrderFilter{Plate: "TST-" + suffix + "-E"})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}

	if len(orders) == 0 {
		t.Fatalf("plate search returned no order for %q", alpha.plate)
	}
	alphaFound := false
	for _, o := range orders {
		if o.OrderNumber == beta.orderNumber {
			t.Errorf("plate search returned the other vehicle's order %q", beta.orderNumber)
		}
		if o.OrderNumber == alpha.orderNumber {
			alphaFound = true
		}
	}
	if !alphaFound {
		t.Errorf("plate search is missing order %q", alpha.orderNumber)
	}
}

// TestGetOrderReturnsPositionsAndHistory proves the detail call returns one
// order with its captured positions and its status history.
func TestGetOrderReturnsPositionsAndHistory(t *testing.T) {
	st := openListTestStore(t)
	suffix := uniqueSuffix(t)
	seeded := seedOrder(t, st, suffix+"-G", "in_progress", "TST-"+suffix+"-G")

	detail, err := st.GetOrder(context.Background(), seeded.orderNumber)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if detail.OrderNumber != seeded.orderNumber {
		t.Errorf("order_number = %q, want %q", detail.OrderNumber, seeded.orderNumber)
	}
	if detail.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", detail.Status)
	}
	if detail.VehiclePlate != seeded.plate {
		t.Errorf("vehicle_plate = %q, want %q", detail.VehiclePlate, seeded.plate)
	}
	if detail.CustomerName == "" {
		t.Error("customer_name is empty")
	}
	if len(detail.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(detail.Items))
	}
	if len(detail.History) != 1 {
		t.Fatalf("history = %d, want 1", len(detail.History))
	}
	if detail.History[0].FromStatus != nil {
		t.Errorf("first history entry from_status = %v, want nil", *detail.History[0].FromStatus)
	}
	if detail.History[0].ToStatus != "in_progress" {
		t.Errorf("first history entry to_status = %q, want in_progress", detail.History[0].ToStatus)
	}
	if _, err := time.Parse(time.RFC3339, detail.History[0].At); err != nil {
		t.Errorf("history at %q is not RFC3339: %v", detail.History[0].At, err)
	}
}

// TestGetOrderUnknownNumber proves an unknown order number reports
// ErrOrderNotFound.
func TestGetOrderUnknownNumber(t *testing.T) {
	st := openListTestStore(t)

	_, err := st.GetOrder(context.Background(), "AW-does-not-exist-"+uniqueSuffix(t))
	if err != ErrOrderNotFound {
		t.Fatalf("GetOrder error = %v, want ErrOrderNotFound", err)
	}
}
