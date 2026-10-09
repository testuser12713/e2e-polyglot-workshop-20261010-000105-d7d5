package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"workshop/backend/internal/config"
	"workshop/backend/internal/store"
)

func newTrackingServer(t *testing.T) *Server {
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

	return NewServer(st, &config.Config{
		APIPort:           "0",
		CORSAllowedOrigin: "http://localhost:5173",
	}, nil)
}

// seedAPIOrder inserts the rows the tracking endpoints read. It returns the
// order number, the licence plate, the customer name and e-mail so the tests
// can assert what is (and is not) released.
func seedAPIOrder(t *testing.T, st *store.Store, withInvoice bool) (orderNumber, plate, customerName, customerEmail string) {
	t.Helper()
	ctx := context.Background()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	orderNumber = "AW-API-" + suffix
	plate = "M-AP " + suffix[len(suffix)-4:]
	customerName = "API Tracking Kunde"
	customerEmail = "api-track-" + suffix + "@example.de"

	var (
		customerID int64
		vehicleID  int64
		orderID    int64
	)
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		customerName, customerEmail, "089-123456",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		plate, "Audi", "A3", 55000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
		 VALUES ($1, $2, $3, 'confirmed', $4, $5) RETURNING id`,
		orderNumber, customerID, vehicleID, "2025-06-01", "Motor ruckelt",
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO order_items (order_id, kind, description, quantity, hours, unit_price_cents, amount_cents)
		 VALUES ($1, 'labor', 'Diagnose', 1, 1.0, 9500, 9500)`,
		orderID,
	); err != nil {
		t.Fatalf("insert order item: %v", err)
	}
	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO status_log (order_id, from_status, to_status) VALUES ($1, NULL, 'requested')`, orderID,
	); err != nil {
		t.Fatalf("insert status log: %v", err)
	}
	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO status_log (order_id, from_status, to_status) VALUES ($1, 'requested', 'confirmed')`, orderID,
	); err != nil {
		t.Fatalf("insert status log: %v", err)
	}

	if withInvoice {
		var invoiceID int64
		if err := st.Pool.QueryRow(ctx,
			`INSERT INTO invoices (invoice_number, order_id, net_cents, tax_cents, gross_cents)
			 VALUES ($1, $2, 9500, 1805, 11305) RETURNING id`,
			"RE-"+orderNumber, orderID,
		).Scan(&invoiceID); err != nil {
			t.Fatalf("insert invoice: %v", err)
		}
		if _, err := st.Pool.Exec(ctx,
			`INSERT INTO invoice_items (invoice_id, description, amount_cents) VALUES ($1, 'Diagnose', 9500)`,
			invoiceID,
		); err != nil {
			t.Fatalf("insert invoice item: %v", err)
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

	return orderNumber, plate, customerName, customerEmail
}

func trackingGet(t *testing.T, srv *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

type trackingBody struct {
	Order struct {
		OrderNumber  string `json:"order_number"`
		Status       string `json:"status"`
		DesiredDate  string `json:"desired_date"`
		VehiclePlate string `json:"vehicle_plate"`
		CustomerName string `json:"customer_name"`
		Problem      string `json:"problem"`
		Customer     struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
			Phone string `json:"phone"`
		} `json:"customer"`
		Vehicle struct {
			ID      int64  `json:"id"`
			Plate   string `json:"plate"`
			Make    string `json:"make"`
			Model   string `json:"model"`
			Mileage int    `json:"mileage"`
		} `json:"vehicle"`
		Items []struct {
			ID             int64   `json:"id"`
			Kind           string  `json:"kind"`
			Description    string  `json:"description"`
			Quantity       float64 `json:"quantity"`
			Hours          float64 `json:"hours"`
			UnitPriceCents int64   `json:"unit_price_cents"`
			AmountCents    int64   `json:"amount_cents"`
		} `json:"items"`
		History []struct {
			At         string `json:"at"`
			FromStatus string `json:"from_status"`
			ToStatus   string `json:"to_status"`
		} `json:"history"`
	} `json:"order"`
	Invoice *struct {
		InvoiceNumber string `json:"invoice_number"`
		OrderNumber   string `json:"order_number"`
		NetCents      int64  `json:"net_cents"`
		TaxCents      int64  `json:"tax_cents"`
		GrossCents    int64  `json:"gross_cents"`
		CreatedAt     string `json:"created_at"`
		Items         []struct {
			Description string `json:"description"`
			AmountCents int64  `json:"amount_cents"`
		} `json:"items"`
	} `json:"invoice"`
}

func decodeTrackingBody(t *testing.T, rec *httptest.ResponseRecorder) trackingBody {
	t.Helper()
	var body trackingBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body
}

// TestTrackOrderEndpointReturnsOrderAndInvoice proves the happy path returns
// status, vehicle, positions, history and the invoice amounts (SPEC AC-11,
// AC-12).
func TestTrackOrderEndpointReturnsOrderAndInvoice(t *testing.T) {
	srv := newTrackingServer(t)
	orderNumber, plate, customerName, customerEmail := seedAPIOrder(t, srv.Store, true)

	rec := trackingGet(t, srv, "/api/public/orders/"+orderNumber+"?plate="+url.QueryEscape(plate))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}

	body := decodeTrackingBody(t, rec)
	if body.Order.OrderNumber != orderNumber {
		t.Errorf("order_number = %q, want %q", body.Order.OrderNumber, orderNumber)
	}
	if body.Order.Status != "confirmed" {
		t.Errorf("status = %q, want %q", body.Order.Status, "confirmed")
	}
	if body.Order.VehiclePlate != plate {
		t.Errorf("vehicle_plate = %q, want %q", body.Order.VehiclePlate, plate)
	}
	if body.Order.CustomerName != customerName || body.Order.Customer.Email != customerEmail {
		t.Errorf("customer not released: %+v", body.Order.Customer)
	}
	if len(body.Order.Items) != 1 || body.Order.Items[0].AmountCents != 9500 {
		t.Errorf("items = %+v, want one item of 9500 cents", body.Order.Items)
	}
	if len(body.Order.History) != 2 {
		t.Errorf("history entries = %d, want 2", len(body.Order.History))
	}
	if body.Invoice == nil {
		t.Fatal("invoice = null, want the invoice")
	}
	if body.Invoice.NetCents != 9500 || body.Invoice.TaxCents != 1805 || body.Invoice.GrossCents != 11305 {
		t.Errorf("invoice = %d/%d/%d, want 9500/1805/11305",
			body.Invoice.NetCents, body.Invoice.TaxCents, body.Invoice.GrossCents)
	}
	if len(body.Invoice.Items) != 1 || body.Invoice.Items[0].AmountCents != 9500 {
		t.Errorf("invoice items = %+v, want one item of 9500 cents", body.Invoice.Items)
	}
}

// TestTrackOrderEndpointWrongPlateAnswers404WithoutData proves a wrong plate
// answers 404 in the unified error body and releases no order data
// (SPEC AC-11).
func TestTrackOrderEndpointWrongPlateAnswers404WithoutData(t *testing.T) {
	srv := newTrackingServer(t)
	orderNumber, _, customerName, customerEmail := seedAPIOrder(t, srv.Store, true)

	rec := trackingGet(t, srv, "/api/public/orders/"+orderNumber+"?plate=B-XX%200000")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body.String())
	}

	var errBody apiErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	if errBody.Error.Code != "not_found" {
		t.Errorf("error code = %q, want %q", errBody.Error.Code, "not_found")
	}

	raw := rec.Body.String()
	for _, secret := range []string{customerName, customerEmail, orderNumber} {
		if strings.Contains(raw, secret) {
			t.Errorf("404 response leaks %q: %s", secret, raw)
		}
	}
}

// TestTrackOrderEndpointUnknownNumberAnswers404 proves an unknown order number
// is a plain 404.
func TestTrackOrderEndpointUnknownNumberAnswers404(t *testing.T) {
	srv := newTrackingServer(t)
	_, plate, _, _ := seedAPIOrder(t, srv.Store, true)

	rec := trackingGet(t, srv, "/api/public/orders/AW-DOES-NOT-EXIST?plate="+url.QueryEscape(plate))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body.String())
	}
}

// TestTrackOrderEndpointWithoutInvoiceReturnsNullInvoice proves the order is
// still returned when no invoice exists, with invoice = null.
func TestTrackOrderEndpointWithoutInvoiceReturnsNullInvoice(t *testing.T) {
	srv := newTrackingServer(t)
	orderNumber, plate, _, _ := seedAPIOrder(t, srv.Store, false)

	rec := trackingGet(t, srv, "/api/public/orders/"+orderNumber+"?plate="+url.QueryEscape(plate))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := decodeTrackingBody(t, rec); body.Invoice != nil {
		t.Errorf("invoice = %+v, want null without an invoice", body.Invoice)
	}
}

// TestInvoiceEndpointReturnsAmounts proves the dedicated invoice route returns
// positions, net, tax and gross for a matching pair (SPEC AC-12).
func TestInvoiceEndpointReturnsAmounts(t *testing.T) {
	srv := newTrackingServer(t)
	orderNumber, plate, _, _ := seedAPIOrder(t, srv.Store, true)

	rec := trackingGet(t, srv, "/api/public/orders/"+orderNumber+"/invoice?plate="+url.QueryEscape(plate))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}

	var invoice struct {
		InvoiceNumber string `json:"invoice_number"`
		OrderNumber   string `json:"order_number"`
		NetCents      int64  `json:"net_cents"`
		TaxCents      int64  `json:"tax_cents"`
		GrossCents    int64  `json:"gross_cents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &invoice); err != nil {
		t.Fatalf("decode invoice %q: %v", rec.Body.String(), err)
	}
	if invoice.InvoiceNumber != "RE-"+orderNumber || invoice.OrderNumber != orderNumber {
		t.Errorf("invoice numbers = %q / %q, want RE-%s / %s",
			invoice.InvoiceNumber, invoice.OrderNumber, orderNumber, orderNumber)
	}
	if invoice.NetCents != 9500 || invoice.TaxCents != 1805 || invoice.GrossCents != 11305 {
		t.Errorf("invoice = %d/%d/%d, want 9500/1805/11305",
			invoice.NetCents, invoice.TaxCents, invoice.GrossCents)
	}
}

// TestInvoiceEndpointWrongPlateAnswers404 proves the invoice route refuses a
// mismatching plate.
func TestInvoiceEndpointWrongPlateAnswers404(t *testing.T) {
	srv := newTrackingServer(t)
	orderNumber, _, _, _ := seedAPIOrder(t, srv.Store, true)

	rec := trackingGet(t, srv, "/api/public/orders/"+orderNumber+"/invoice?plate=B-XX%200000")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body.String())
	}
}

// TestInvoiceEndpointWithoutInvoiceAnswers404 proves the invoice route answers
// 404 while no invoice exists yet.
func TestInvoiceEndpointWithoutInvoiceAnswers404(t *testing.T) {
	srv := newTrackingServer(t)
	orderNumber, plate, _, _ := seedAPIOrder(t, srv.Store, false)

	rec := trackingGet(t, srv, "/api/public/orders/"+orderNumber+"/invoice?plate="+url.QueryEscape(plate))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body.String())
	}
}

// TestTrackOrderEndpointMissingPlateAnswers422 proves the plate is a required
// part of the tracking check and its absence is a validation error, not a
// lookup miss.
func TestTrackOrderEndpointMissingPlateAnswers422(t *testing.T) {
	srv := newTrackingServer(t)
	orderNumber, _, _, _ := seedAPIOrder(t, srv.Store, true)

	rec := trackingGet(t, srv, "/api/public/orders/"+orderNumber)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %s", rec.Code, rec.Body.String())
	}
}
