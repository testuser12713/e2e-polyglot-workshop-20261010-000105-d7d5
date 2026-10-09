package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/backend/internal/config"
	"workshop/backend/internal/store"
)

var (
	integrationOnce  sync.Once
	integrationStore *store.Store
	integrationErr   error
)

// newCustomerVehicleIntegrationServer builds a Server on a real PostgreSQL
// instance and wraps it in an httptest server (SPEC AC-25). The test skips when
// DATABASE_URL is not set, so it never falls back to SQLite or an in-memory
// stand-in (SPEC AC-24).
func newCustomerVehicleIntegrationServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()

	st := sharedIntegrationStore(t)
	srv := httptest.NewServer(NewServer(st, &config.Config{
		CORSAllowedOrigin: "http://localhost:5173",
	}, nil).Handler())
	t.Cleanup(srv.Close)

	return srv, st
}

// sharedIntegrationStore opens the PostgreSQL store once per test binary. It
// does not race the skeleton's own store test on an empty database: applying
// schema.sql concurrently from two test binaries can hit PostgreSQL's
// pg_class duplicate-key error, so this helper waits for whichever process
// applies it first and only creates the schema itself when nobody else does.
func sharedIntegrationStore(t *testing.T) *store.Store {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	integrationOnce.Do(func() {
		integrationStore, integrationErr = openIntegrationStore(databaseURL)
	})
	if integrationErr != nil {
		t.Fatalf("open integration store: %v", integrationErr)
	}
	return integrationStore
}

// openIntegrationStore waits up to schemaWaitTimeout for another test binary to
// apply the schema and otherwise applies it itself. store.Open applies the
// whole schema in one statement, so once customers exists every table does.
func openIntegrationStore(databaseURL string) (*store.Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(schemaWaitTimeout)
	for {
		var table *string
		err := pool.QueryRow(ctx, "SELECT to_regclass('public.customers')::text").Scan(&table)
		if err == nil && table != nil && *table != "" {
			return &store.Store{Pool: pool}, nil
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Nobody applied the schema within the wait: do it ourselves.
	pool.Close()
	return store.Open(ctx, databaseURL)
}

// schemaWaitTimeout bounds how long a test binary waits for the schema another
// test binary applies on an empty database.
const schemaWaitTimeout = 5 * time.Second

// cvUniqueSuffix returns a per-test suffix so parallel runs never collide on the
// unique e-mail or plate.
func cvUniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// cvRequest performs an HTTP request with an optional JSON body and returns the
// response and its raw body.
func cvRequest(t *testing.T, method, url, body string) (*http.Response, []byte) {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return resp, raw
}

// TestCreateCustomerReturns201WithID checks that a valid customer is created
// and returned with a unique id (SPEC AC-01).
func TestCreateCustomerReturns201WithID(t *testing.T) {
	srv, st := newCustomerVehicleIntegrationServer(t)

	email := "customer-" + cvUniqueSuffix() + "@example.de"
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), "DELETE FROM customers WHERE email = $1", email)
	})

	body := fmt.Sprintf(`{"name":"Anna Beispiel","email":%q,"phone":"0151 2345678"}`, email)
	resp, raw := cvRequest(t, http.MethodPost, srv.URL+"/api/public/customers", body)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", resp.StatusCode, raw)
	}

	var created store.Customer
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("decode customer: %v (body %s)", err, raw)
	}
	if created.ID <= 0 {
		t.Errorf("customer id = %d, want a positive id", created.ID)
	}
	if created.Name != "Anna Beispiel" || created.Email != email || created.Phone != "0151 2345678" {
		t.Errorf("customer = %+v, want the submitted fields", created)
	}
}

// TestCreateCustomerDuplicateEmailReturns409 checks that a second customer with
// the same e-mail is rejected with 409 in the unified error body (SPEC AC-01).
func TestCreateCustomerDuplicateEmailReturns409(t *testing.T) {
	srv, st := newCustomerVehicleIntegrationServer(t)

	email := "duplicate-" + cvUniqueSuffix() + "@example.de"
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), "DELETE FROM customers WHERE email = $1", email)
	})

	body := fmt.Sprintf(`{"name":"Anna Beispiel","email":%q,"phone":"0151 2345678"}`, email)

	first, raw := cvRequest(t, http.MethodPost, srv.URL+"/api/public/customers", body)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first create status = %d, want 201 (body %s)", first.StatusCode, raw)
	}

	second, raw := cvRequest(t, http.MethodPost, srv.URL+"/api/public/customers", body)
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second create status = %d, want 409 (body %s)", second.StatusCode, raw)
	}

	var errBody struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode error body: %v (body %s)", err, raw)
	}
	if errBody.Error.Code != "conflict" || errBody.Error.Message == "" {
		t.Errorf("error body = %+v, want a conflict code and a message", errBody.Error)
	}
}

// TestCreateCustomerInvalidReturns422 checks that a missing or invalid field is
// rejected with 422 and the unified error body.
func TestCreateCustomerInvalidReturns422(t *testing.T) {
	srv, _ := newCustomerVehicleIntegrationServer(t)

	cases := map[string]string{
		"missing name":  `{"name":"","email":"a@b.de","phone":"123"}`,
		"invalid email": `{"name":"Anna","email":"not-an-email","phone":"123"}`,
		"missing phone": `{"name":"Anna","email":"a@b.de","phone":""}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			resp, raw := cvRequest(t, http.MethodPost, srv.URL+"/api/public/customers", body)
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422 (body %s)", resp.StatusCode, raw)
			}
		})
	}
}

// TestGetCustomerReturnsCreatedCustomer checks that the customer created via
// POST can be read back by id (SPEC AC-01).
func TestGetCustomerReturnsCreatedCustomer(t *testing.T) {
	srv, st := newCustomerVehicleIntegrationServer(t)

	email := "get-" + cvUniqueSuffix() + "@example.de"
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), "DELETE FROM customers WHERE email = $1", email)
	})

	body := fmt.Sprintf(`{"name":"Anna Beispiel","email":%q,"phone":"0151 2345678"}`, email)
	resp, raw := cvRequest(t, http.MethodPost, srv.URL+"/api/public/customers", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body %s)", resp.StatusCode, raw)
	}
	var created store.Customer
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("decode customer: %v (body %s)", err, raw)
	}

	getResp, getRaw := cvRequest(t, http.MethodGet, fmt.Sprintf("%s/api/public/customers/%d", srv.URL, created.ID), "")
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200 (body %s)", getResp.StatusCode, getRaw)
	}
	var fetched store.Customer
	if err := json.Unmarshal(getRaw, &fetched); err != nil {
		t.Fatalf("decode fetched customer: %v (body %s)", err, getRaw)
	}
	if fetched != created {
		t.Errorf("fetched customer = %+v, want %+v", fetched, created)
	}
}

// TestGetUnknownCustomerReturns404 checks that an unknown id answers 404 in the
// unified error body.
func TestGetUnknownCustomerReturns404(t *testing.T) {
	srv, _ := newCustomerVehicleIntegrationServer(t)

	resp, raw := cvRequest(t, http.MethodGet, srv.URL+"/api/public/customers/999999999", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", resp.StatusCode, raw)
	}
}
