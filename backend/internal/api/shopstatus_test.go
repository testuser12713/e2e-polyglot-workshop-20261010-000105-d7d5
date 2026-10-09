package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	valkey "github.com/valkey-io/valkey-go"

	"workshop/backend/internal/config"
	"workshop/backend/internal/queue"
	"workshop/backend/internal/store"
)

// statusTestEnv wires the real PostgreSQL store and the real Valkey queue into
// an httptest-backed API server. The behaviour of the workflow (allowed step,
// rejected jump, single completion message) can only be proven against the real
// services (SPEC AC-25).
type statusTestEnv struct {
	server       *Server
	store        *store.Store
	queue        *queue.Queue
	client       valkey.Client
	token        string
	orderNumbers []string
	vehicleIDs   []int64
	customerIDs  []int64
}

// newStatusTestEnv skips when DATABASE_URL or VALKEY_URL is absent. Every row it
// creates is removed again in cleanup; it touches no table another ticket owns
// beyond the rows it inserted itself.
func newStatusTestEnv(t *testing.T) *statusTestEnv {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	vkURL := os.Getenv("VALKEY_URL")
	if dbURL == "" || vkURL == "" {
		t.Skip("DATABASE_URL and VALKEY_URL must be set for the status workflow integration test")
	}

	ctx := context.Background()
	st, err := store.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	q, err := queue.New(vkURL)
	if err != nil {
		st.Close()
		t.Fatalf("open queue: %v", err)
	}
	opt, err := valkey.ParseURL(vkURL)
	if err != nil {
		q.Close()
		st.Close()
		t.Fatalf("parse VALKEY_URL: %v", err)
	}
	client, err := valkey.NewClient(opt)
	if err != nil {
		q.Close()
		st.Close()
		t.Fatalf("create valkey inspection client: %v", err)
	}

	env := &statusTestEnv{
		server: NewServer(st, &config.Config{CORSAllowedOrigin: "http://localhost:5173"}, q),
		store:  st,
		queue:  q,
		client: client,
		token:  seedShopSession(t, st),
	}
	t.Cleanup(env.cleanup)
	return env
}

func (e *statusTestEnv) cleanup() {
	ctx := context.Background()
	for _, orderNumber := range e.orderNumbers {
		_, _ = e.store.Pool.Exec(ctx, `DELETE FROM orders WHERE order_number = $1`, orderNumber)
	}
	for _, id := range e.vehicleIDs {
		_, _ = e.store.Pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, id)
	}
	for _, id := range e.customerIDs {
		_, _ = e.store.Pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, id)
	}
	_, _ = e.client.Do(ctx, e.client.B().Del().Key(queue.CompletedOrdersList).Build()).AsInt64()
	e.client.Close()
	e.queue.Close()
	e.store.Close()
}

// createOrder inserts a customer, a vehicle and an order in the given status.
func (e *statusTestEnv) createOrder(t *testing.T, status string) (int64, string) {
	t.Helper()
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	email := fmt.Sprintf("status-test-%d@example.com", stamp)
	plate := fmt.Sprintf("STT-%d", stamp)

	var customerID int64
	if err := e.store.Pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Statustest", email, "+49 30 000000",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	e.customerIDs = append(e.customerIDs, customerID)

	var vehicleID int64
	if err := e.store.Pool.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		plate, "Testmarke", "Testmodell", 10000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}
	e.vehicleIDs = append(e.vehicleIDs, vehicleID)

	orderNumber := fmt.Sprintf("TEST-%d", stamp)
	var orderID int64
	if err := e.store.Pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
		 VALUES ($1, $2, $3, $4, CURRENT_DATE, $5) RETURNING id`,
		orderNumber, customerID, vehicleID, status, "",
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	e.orderNumbers = append(e.orderNumbers, orderNumber)
	return orderID, orderNumber
}

func (e *statusTestEnv) postStatus(t *testing.T, orderNumber, status string) *httptest.ResponseRecorder {
	t.Helper()
	return e.postStatusWithToken(t, orderNumber, status, e.token)
}

// postStatusWithToken posts the status change with the given bearer token; an
// empty token means no Authorization header at all.
func (e *statusTestEnv) postStatusWithToken(t *testing.T, orderNumber, status, token string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		t.Fatalf("encode request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/shop/orders/"+orderNumber+"/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.server.Handler().ServeHTTP(rec, req)
	return rec
}

func (e *statusTestEnv) dbStatus(t *testing.T, orderNumber string) string {
	t.Helper()
	var status string
	if err := e.store.Pool.QueryRow(context.Background(),
		`SELECT status FROM orders WHERE order_number = $1`, orderNumber).Scan(&status); err != nil {
		t.Fatalf("load order status: %v", err)
	}
	return status
}

func (e *statusTestEnv) statusLogCount(t *testing.T, orderNumber string) int {
	t.Helper()
	var count int
	if err := e.store.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM status_log sl
		 JOIN orders o ON o.id = sl.order_id
		 WHERE o.order_number = $1`, orderNumber).Scan(&count); err != nil {
		t.Fatalf("count status log: %v", err)
	}
	return count
}

func (e *statusTestEnv) queuedMessages(t *testing.T) []string {
	t.Helper()
	raw, err := e.client.Do(context.Background(),
		e.client.B().Lrange().Key(queue.CompletedOrdersList).Start(0).Stop(-1).Build()).AsStrSlice()
	if err != nil {
		t.Fatalf("read completion queue: %v", err)
	}
	return raw
}

func decodeStatusResponse(t *testing.T, rec *httptest.ResponseRecorder) setOrderStatusResponse {
	t.Helper()
	var resp setOrderStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return resp
}

// TestOrderStatusAllowedStepIsLoggedAndPersisted covers SPEC AC-18: a workshop
// employee confirms a 'requested' order; it is then 'confirmed' and the change
// is in the status log.
func TestOrderStatusAllowedStepIsLoggedAndPersisted(t *testing.T) {
	env := newStatusTestEnv(t)
	_, orderNumber := env.createOrder(t, "requested")

	rec := env.postStatus(t, orderNumber, "confirmed")
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	resp := decodeStatusResponse(t, rec)
	if resp.OrderNumber != orderNumber || resp.Status != "confirmed" {
		t.Fatalf("response = %+v, want order_number %q status confirmed", resp, orderNumber)
	}

	if got := env.dbStatus(t, orderNumber); got != "confirmed" {
		t.Errorf("persisted status = %q, want confirmed", got)
	}
	if got := env.statusLogCount(t, orderNumber); got != 1 {
		t.Errorf("status log entries = %d, want 1", got)
	}

	// Re-read the log through the shared store query to assert the UTC instant
	// and the from/to status honestly.
	var id int64
	if err := env.store.Pool.QueryRow(context.Background(),
		`SELECT id FROM orders WHERE order_number = $1`, orderNumber).Scan(&id); err != nil {
		t.Fatalf("load order id: %v", err)
	}
	history, err := env.store.OrderStatusHistory(context.Background(), id)
	if err != nil {
		t.Fatalf("load status history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	entry := history[0]
	if entry.FromStatus != "requested" || entry.ToStatus != "confirmed" {
		t.Errorf("status entry = %s->%s, want requested->confirmed", entry.FromStatus, entry.ToStatus)
	}
	if entry.At.Location() != time.UTC {
		t.Errorf("status entry location = %v, want UTC", entry.At.Location())
	}
	if delta := time.Since(entry.At); delta < 0 || delta > time.Minute {
		t.Errorf("status entry timestamp %v is not close to now", entry.At)
	}
}

// TestOrderStatusRejectsIllegalJump covers SPEC AC-04: an illegal transition is
// refused with 409 and the status stays unchanged.
func TestOrderStatusRejectsIllegalJump(t *testing.T) {
	env := newStatusTestEnv(t)
	_, orderNumber := env.createOrder(t, "requested")

	rec := env.postStatus(t, orderNumber, "picked_up")
	if rec.Code != http.StatusConflict {
		t.Fatalf("illegal jump status = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
	if got := env.dbStatus(t, orderNumber); got != "requested" {
		t.Errorf("status after illegal jump = %q, want requested", got)
	}
	if got := env.statusLogCount(t, orderNumber); got != 0 {
		t.Errorf("status log entries after illegal jump = %d, want 0", got)
	}

	// The unified error envelope must be present on a 409.
	var body apiErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	if body.Error.Code == "" || body.Error.Message == "" {
		t.Errorf("error body = %+v, want a code and a message", body.Error)
	}

	// Skipping a step (confirmed -> done) is refused as well.
	_, secondOrder := env.createOrder(t, "confirmed")
	skip := env.postStatus(t, secondOrder, "done")
	if skip.Code != http.StatusConflict {
		t.Fatalf("skipped step status = %d, want 409", skip.Code)
	}
	if got := env.dbStatus(t, secondOrder); got != "confirmed" {
		t.Errorf("status after skipped step = %q, want confirmed", got)
	}
}

// TestOrderStatusDoneQueuesExactlyOneMessage covers SPEC AC-06: reaching "done"
// pushes exactly one JSON message carrying the order id and order number.
func TestOrderStatusDoneQueuesExactlyOneMessage(t *testing.T) {
	env := newStatusTestEnv(t)
	if _, err := env.client.Do(context.Background(), env.client.B().Del().Key(queue.CompletedOrdersList).Build()).AsInt64(); err != nil {
		t.Fatalf("clear completion queue: %v", err)
	}
	orderID, orderNumber := env.createOrder(t, "requested")

	for _, step := range []string{"confirmed", "in_progress", "done"} {
		rec := env.postStatus(t, orderNumber, step)
		if rec.Code != http.StatusOK {
			t.Fatalf("transition to %s status = %d, want 200 (body %s)", step, rec.Code, rec.Body.String())
		}
	}

	messages := env.queuedMessages(t)
	if len(messages) != 1 {
		t.Fatalf("completion queue length = %d, want exactly 1 (%v)", len(messages), messages)
	}
	var payload queue.CompletedOrderMessage
	if err := json.Unmarshal([]byte(messages[0]), &payload); err != nil {
		t.Fatalf("decode completion message %q: %v", messages[0], err)
	}
	if payload.OrderID != orderID {
		t.Errorf("message order_id = %d, want %d", payload.OrderID, orderID)
	}
	if payload.OrderNumber != orderNumber {
		t.Errorf("message order_number = %q, want %q", payload.OrderNumber, orderNumber)
	}

	// A repeated "done" request is rejected and does not enqueue a second
	// message.
	repeat := env.postStatus(t, orderNumber, "done")
	if repeat.Code != http.StatusConflict {
		t.Fatalf("repeated done status = %d, want 409", repeat.Code)
	}
	if got := len(env.queuedMessages(t)); got != 1 {
		t.Errorf("completion queue length after repeated done = %d, want 1", got)
	}
}

// TestOrderStatusRequiresSession covers SPEC AC-28: without a valid bearer token
// the status route answers 401 in the unified body and changes nothing.
func TestOrderStatusRequiresSession(t *testing.T) {
	env := newStatusTestEnv(t)
	if _, err := env.client.Do(context.Background(), env.client.B().Del().Key(queue.CompletedOrdersList).Build()).AsInt64(); err != nil {
		t.Fatalf("clear completion queue: %v", err)
	}
	_, orderNumber := env.createOrder(t, "requested")

	rec := env.postStatusWithToken(t, orderNumber, "confirmed", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST status without token = %d, want 401 (body %s)", rec.Code, rec.Body.String())
	}

	var body apiErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	if body.Error.Code == "" || body.Error.Message == "" {
		t.Errorf("error body = %+v, want a code and a message", body.Error)
	}

	if got := env.dbStatus(t, orderNumber); got != "requested" {
		t.Errorf("status after unauthenticated request = %q, want requested", got)
	}
	if got := env.statusLogCount(t, orderNumber); got != 0 {
		t.Errorf("status log entries after unauthenticated request = %d, want 0", got)
	}
}
