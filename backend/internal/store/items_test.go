package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestComputeItemAmountCents proves the pure cent computation for labour and
// part positions, including half-up rounding.
func TestComputeItemAmountCents(t *testing.T) {
	cases := []struct {
		name     string
		in       ItemInput
		rate     int64
		expected int64
	}{
		{"labour whole hours", ItemInput{Kind: "labor", Hours: 2}, 9500, 19000},
		{"labour fractional hours", ItemInput{Kind: "labor", Hours: 1.5}, 9500, 14250},
		{"labour quarter hour", ItemInput{Kind: "labor", Hours: 0.25}, 8900, 2225},
		{"part whole quantity", ItemInput{Kind: "part", Quantity: 2, UnitPriceCents: 4500}, 9500, 9000},
		{"part fractional quantity", ItemInput{Kind: "part", Quantity: 1.5, UnitPriceCents: 999}, 9500, 1499},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComputeItemAmountCents(tc.in, tc.rate); got != tc.expected {
				t.Errorf("ComputeItemAmountCents(%+v, %d) = %d, want %d", tc.in, tc.rate, got, tc.expected)
			}
		})
	}
}

// seedItemTestOrder inserts one customer, vehicle and order without any
// positions and removes exactly those rows when the test ends.
func seedItemTestOrder(t *testing.T, st *Store, suffix string) (orderNumber string, orderID int64) {
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
		"ITM-"+suffix, "Audi", "A4", 50000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}

	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orderNumber, customerID, vehicleID, "confirmed", "2025-05-05", "Kupplung rutscht",
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = st.Pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, orderID)
		_, _ = st.Pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, vehicleID)
		_, _ = st.Pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, customerID)
	})

	return orderNumber, orderID
}

// TestAddOrderItemStoresAmounts proves a labour and a part position are stored
// with the amounts computed from hours and quantity (SPEC AC-19).
func TestAddOrderItemStoresAmounts(t *testing.T) {
	st := openListTestStore(t)
	suffix := uniqueSuffix(t)
	orderNumber, _ := seedItemTestOrder(t, st, suffix)

	const rate = int64(9500)

	labor, err := st.AddOrderItem(context.Background(), orderNumber,
		ItemInput{Kind: "labor", Description: "Arbeitszeit", Hours: 1.5}, rate)
	if err != nil {
		t.Fatalf("AddOrderItem(labor): %v", err)
	}
	if labor.AmountCents != 14250 {
		t.Errorf("labour amount = %d, want 14250", labor.AmountCents)
	}
	if labor.Hours != 1.5 {
		t.Errorf("labour hours = %v, want 1.5", labor.Hours)
	}

	part, err := st.AddOrderItem(context.Background(), orderNumber,
		ItemInput{Kind: "part", Description: "Bremsbelag", Quantity: 2, UnitPriceCents: 4500}, rate)
	if err != nil {
		t.Fatalf("AddOrderItem(part): %v", err)
	}
	if part.AmountCents != 9000 {
		t.Errorf("part amount = %d, want 9000", part.AmountCents)
	}
	if part.Quantity != 2 || part.UnitPriceCents != 4500 {
		t.Errorf("part quantity/unit price = %v/%d, want 2/4500", part.Quantity, part.UnitPriceCents)
	}

	detail, err := st.GetOrder(context.Background(), orderNumber)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	amounts := map[string]int64{}
	for _, item := range detail.Items {
		amounts[item.Description] = item.AmountCents
	}
	if amounts["Arbeitszeit"] != 14250 {
		t.Errorf("stored labour amount = %d, want 14250", amounts["Arbeitszeit"])
	}
	if amounts["Bremsbelag"] != 9000 {
		t.Errorf("stored part amount = %d, want 9000", amounts["Bremsbelag"])
	}
}

// TestAddOrderItemUnknownOrder proves an unknown order number reports
// ErrOrderNotFound instead of writing an orphan position.
func TestAddOrderItemUnknownOrder(t *testing.T) {
	st := openListTestStore(t)

	_, err := st.AddOrderItem(context.Background(), "AW-does-not-exist-"+uniqueSuffix(t),
		ItemInput{Kind: "labor", Description: "Arbeitszeit", Hours: 1}, 9500)
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("AddOrderItem error = %v, want ErrOrderNotFound", err)
	}
}
