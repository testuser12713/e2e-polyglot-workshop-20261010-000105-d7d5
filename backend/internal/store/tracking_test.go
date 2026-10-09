package store

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

// newTrackingStore opens a real PostgreSQL store for the tracking tests and
// skips when DATABASE_URL is not provided (SPEC AC-25).
func newTrackingStore(t *testing.T) *Store {
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

// seedTrackedOrder inserts one customer, vehicle, order with two positions, a
// status history and (optionally) an invoice. It returns the order number and
// licence plate used. Every row is removed again in t.Cleanup.
func seedTrackedOrder(t *testing.T, st *Store, withInvoice bool) (orderNumber, plate string) {
	t.Helper()
	ctx := context.Background()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	orderNumber = "AW-TRACK-" + suffix
	plate = "B-TR " + suffix[len(suffix)-4:]

	var (
		customerID int64
		vehicleID  int64
		orderID    int64
	)
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Tracking Kunde", "track-"+suffix+"@example.de", "030-123456",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		plate, "VW", "Golf", 123456,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
		 VALUES ($1, $2, $3, 'confirmed', $4, $5) RETURNING id`,
		orderNumber, customerID, vehicleID, "2025-06-01", "Bremsen quietschen",
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO order_items (order_id, kind, description, quantity, hours, unit_price_cents, amount_cents)
		 VALUES ($1, 'labor', 'Bremsen wechseln', 1, 2.5, 9500, 23750),
		        ($1, 'part', 'Bremsscheibe', 2, 0, 4500, 9000)`,
		orderID,
	); err != nil {
		t.Fatalf("insert order items: %v", err)
	}

	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO status_log (order_id, from_status, to_status) VALUES ($1, NULL, 'requested')`, orderID,
	); err != nil {
		t.Fatalf("insert first status log: %v", err)
	}
	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO status_log (order_id, from_status, to_status) VALUES ($1, 'requested', 'confirmed')`, orderID,
	); err != nil {
		t.Fatalf("insert second status log: %v", err)
	}

	if withInvoice {
		var invoiceID int64
		if err := st.Pool.QueryRow(ctx,
			`INSERT INTO invoices (invoice_number, order_id, net_cents, tax_cents, gross_cents)
			 VALUES ($1, $2, 32750, 6223, 38973) RETURNING id`,
			"RE-"+orderNumber, orderID,
		).Scan(&invoiceID); err != nil {
			t.Fatalf("insert invoice: %v", err)
		}
		if _, err := st.Pool.Exec(ctx,
			`INSERT INTO invoice_items (invoice_id, description, amount_cents)
			 VALUES ($1, 'Bremsen wechseln', 23750), ($1, 'Bremsscheibe', 9000)`,
			invoiceID,
		); err != nil {
			t.Fatalf("insert invoice items: %v", err)
		}
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM invoice_items WHERE invoice_id IN (SELECT id FROM invoices WHERE order_id = $1)`, orderID)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM invoices WHERE order_id = $1`, orderID)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM status_log WHERE order_id = $1`, orderID)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM order_items WHERE order_id = $1`, orderID)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM orders WHERE id = $1`, orderID)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM customers WHERE id = $1`, customerID)
		_, _ = st.Pool.Exec(cleanupCtx, `DELETE FROM vehicles WHERE id = $1`, vehicleID)
	})

	return orderNumber, plate
}

// TestTrackOrderReturnsStatusPositionsAndHistory proves the happy path: the
// right order number with the right plate releases the full order view.
func TestTrackOrderReturnsStatusPositionsAndHistory(t *testing.T) {
	st := newTrackingStore(t)
	orderNumber, plate := seedTrackedOrder(t, st, true)

	order, found, err := st.TrackOrder(context.Background(), orderNumber, plate)
	if err != nil {
		t.Fatalf("TrackOrder: %v", err)
	}
	if !found {
		t.Fatal("TrackOrder found = false, want true")
	}
	if order.OrderNumber != orderNumber {
		t.Errorf("order number = %q, want %q", order.OrderNumber, orderNumber)
	}
	if order.Status != "confirmed" {
		t.Errorf("status = %q, want %q", order.Status, "confirmed")
	}
	if order.VehiclePlate != plate {
		t.Errorf("vehicle plate = %q, want %q", order.VehiclePlate, plate)
	}
	if order.CustomerName != "Tracking Kunde" {
		t.Errorf("customer name = %q, want %q", order.CustomerName, "Tracking Kunde")
	}
	if len(order.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(order.Items))
	}
	if len(order.History) != 2 {
		t.Fatalf("history = %d, want 2", len(order.History))
	}
	if order.History[0].ToStatus != "requested" || order.History[1].ToStatus != "confirmed" {
		t.Errorf("history statuses = %q, %q; want requested, confirmed",
			order.History[0].ToStatus, order.History[1].ToStatus)
	}
	if order.History[1].FromStatus != "requested" {
		t.Errorf("history from_status = %q, want %q", order.History[1].FromStatus, "requested")
	}
}

// TestTrackOrderRejectsWrongPlate proves a wrong plate releases no order data.
func TestTrackOrderRejectsWrongPlate(t *testing.T) {
	st := newTrackingStore(t)
	orderNumber, _ := seedTrackedOrder(t, st, true)

	_, found, err := st.TrackOrder(context.Background(), orderNumber, "B-XX 0000")
	if err != nil {
		t.Fatalf("TrackOrder: %v", err)
	}
	if found {
		t.Fatal("TrackOrder found = true for a wrong plate, want false")
	}
}

// TestTrackOrderRejectsUnknownNumber proves an unknown order number is not
// found, even with a known plate.
func TestTrackOrderRejectsUnknownNumber(t *testing.T) {
	st := newTrackingStore(t)
	_, plate := seedTrackedOrder(t, st, true)

	_, found, err := st.TrackOrder(context.Background(), "AW-DOES-NOT-EXIST", plate)
	if err != nil {
		t.Fatalf("TrackOrder: %v", err)
	}
	if found {
		t.Fatal("TrackOrder found = true for an unknown order number, want false")
	}
}

// TestInvoiceByOrderReturnsAmounts proves the invoice of a matching pair is
// returned with its positions and cent amounts.
func TestInvoiceByOrderReturnsAmounts(t *testing.T) {
	st := newTrackingStore(t)
	orderNumber, plate := seedTrackedOrder(t, st, true)

	invoice, found, err := st.InvoiceByOrder(context.Background(), orderNumber, plate)
	if err != nil {
		t.Fatalf("InvoiceByOrder: %v", err)
	}
	if !found {
		t.Fatal("InvoiceByOrder found = false, want true")
	}
	if invoice.InvoiceNumber != "RE-"+orderNumber {
		t.Errorf("invoice number = %q, want %q", invoice.InvoiceNumber, "RE-"+orderNumber)
	}
	if invoice.NetCents != 32750 || invoice.TaxCents != 6223 || invoice.GrossCents != 38973 {
		t.Errorf("invoice amounts = %d/%d/%d, want 32750/6223/38973",
			invoice.NetCents, invoice.TaxCents, invoice.GrossCents)
	}
	if len(invoice.Items) != 2 {
		t.Fatalf("invoice items = %d, want 2", len(invoice.Items))
	}
}

// TestInvoiceByOrderRejectsWrongPlate proves the invoice is not released for a
// wrong plate.
func TestInvoiceByOrderRejectsWrongPlate(t *testing.T) {
	st := newTrackingStore(t)
	orderNumber, _ := seedTrackedOrder(t, st, true)

	_, found, err := st.InvoiceByOrder(context.Background(), orderNumber, "B-XX 0000")
	if err != nil {
		t.Fatalf("InvoiceByOrder: %v", err)
	}
	if found {
		t.Fatal("InvoiceByOrder found = true for a wrong plate, want false")
	}
}

// TestInvoiceByOrderWithoutInvoice proves an order without an invoice reports
// found=false (the handler then answers 404).
func TestInvoiceByOrderWithoutInvoice(t *testing.T) {
	st := newTrackingStore(t)
	orderNumber, plate := seedTrackedOrder(t, st, false)

	_, found, err := st.InvoiceByOrder(context.Background(), orderNumber, plate)
	if err != nil {
		t.Fatalf("InvoiceByOrder: %v", err)
	}
	if found {
		t.Fatal("InvoiceByOrder found = true without an invoice, want false")
	}
}
