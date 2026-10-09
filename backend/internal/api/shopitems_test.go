package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"workshop/backend/internal/config"
	"workshop/backend/internal/store"
)

// testHourlyRateCents is the labour rate the handler tests configure. It is a
// test-local non-secret value; the running product reads
// WORKSHOP_HOURLY_RATE_CENTS from the environment (SPEC AC-26).
const testHourlyRateCents = 9500

// seedItemsOrder inserts one order without positions and removes exactly its
// rows when the test ends. It reuses the PostgreSQL connection helpers of the
// order-list tests.
func seedItemsOrder(t *testing.T, st *store.Store, suffix string) (orderNumber string, orderID int64) {
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
		"ITM-"+suffix, "Seat", "Leon", 40000,
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

func newShopItemsServer(t *testing.T) (*Server, *store.Store, string) {
	t.Helper()
	st := openShopOrdersStore(t)
	token := seedShopSession(t, st)
	cfg := &config.Config{
		APIPort:                 "0",
		CORSAllowedOrigin:       "http://localhost:5173",
		WorkshopHourlyRateCents: testHourlyRateCents,
	}
	return NewServer(st, cfg, nil), st, token
}

func postShopItem(t *testing.T, srv *Server, orderNumber, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/shop/orders/"+orderNumber+"/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// TestAddOrderItemsAppearInDetail captures a labour and a part position and
// proves both appear in the order detail with their computed cent amounts
// (SPEC AC-19).
func TestAddOrderItemsAppearInDetail(t *testing.T) {
	srv, st, token := newShopItemsServer(t)
	orderNumber, _ := seedItemsOrder(t, st, shopOrdersSuffix(t))

	laborRec := postShopItem(t, srv, orderNumber, token,
		`{"kind":"labor","description":"Arbeitszeit","quantity":0,"hours":1.5,"unit_price_cents":0}`)
	if laborRec.Code != http.StatusCreated {
		t.Fatalf("labour status = %d, want 201 (body: %s)", laborRec.Code, laborRec.Body.String())
	}
	var labor store.OrderItem
	if err := json.Unmarshal(laborRec.Body.Bytes(), &labor); err != nil {
		t.Fatalf("decode labour: %v", err)
	}
	if labor.Kind != "labor" || labor.AmountCents != 14250 {
		t.Errorf("labour = %+v, want kind labor and amount 14250", labor)
	}

	partRec := postShopItem(t, srv, orderNumber, token,
		`{"kind":"part","description":"Bremsbelag","quantity":2,"hours":0,"unit_price_cents":4500}`)
	if partRec.Code != http.StatusCreated {
		t.Fatalf("part status = %d, want 201 (body: %s)", partRec.Code, partRec.Body.String())
	}
	var part store.OrderItem
	if err := json.Unmarshal(partRec.Body.Bytes(), &part); err != nil {
		t.Fatalf("decode part: %v", err)
	}
	if part.Kind != "part" || part.AmountCents != 9000 {
		t.Errorf("part = %+v, want kind part and amount 9000", part)
	}

	detailRec := shopOrdersGET(t, srv, "/api/shop/orders/"+orderNumber, token)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200 (body: %s)", detailRec.Code, detailRec.Body.String())
	}
	var detail store.OrderDetail
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	amounts := map[string]int64{}
	for _, item := range detail.Items {
		amounts[item.Description] = item.AmountCents
	}
	if amounts["Arbeitszeit"] != 14250 {
		t.Errorf("detail labour amount = %d, want 14250", amounts["Arbeitszeit"])
	}
	if amounts["Bremsbelag"] != 9000 {
		t.Errorf("detail part amount = %d, want 9000", amounts["Bremsbelag"])
	}
}

// TestAddOrderItemInvalid proves a malformed or incomplete position answers 422
// in the unified error body.
func TestAddOrderItemInvalid(t *testing.T) {
	srv, st, token := newShopItemsServer(t)
	orderNumber, _ := seedItemsOrder(t, st, shopOrdersSuffix(t))

	cases := []struct {
		name string
		body string
	}{
		{"malformed json", `{"kind":`},
		{"unknown kind", `{"kind":"material","description":"Öl","quantity":1,"unit_price_cents":500}`},
		{"labour without hours", `{"kind":"labor","description":"Arbeitszeit","hours":0}`},
		{"labour without description", `{"kind":"labor","description":"","hours":1}`},
		{"part without quantity", `{"kind":"part","description":"Öl","quantity":0,"unit_price_cents":500}`},
		{"part without unit price", `{"kind":"part","description":"Öl","quantity":1,"unit_price_cents":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postShopItem(t, srv, orderNumber, token, tc.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422 (body: %s)", rec.Code, rec.Body.String())
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
		})
	}
}

// TestAddOrderItemUnknownOrder proves an unknown order number answers 404.
func TestAddOrderItemUnknownOrder(t *testing.T) {
	srv, _, token := newShopItemsServer(t)

	rec := postShopItem(t, srv, "AW-unknown-"+shopOrdersSuffix(t), token,
		`{"kind":"labor","description":"Arbeitszeit","hours":1}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestAddOrderItemRequiresSession proves the route refuses an unauthenticated
// caller with 401 (SPEC AC-28).
func TestAddOrderItemRequiresSession(t *testing.T) {
	srv, st, _ := newShopItemsServer(t)
	orderNumber, _ := seedItemsOrder(t, st, shopOrdersSuffix(t))

	rec := postShopItem(t, srv, orderNumber, "",
		`{"kind":"labor","description":"Arbeitszeit","hours":1}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}
