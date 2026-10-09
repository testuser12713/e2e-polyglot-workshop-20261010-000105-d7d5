// Package queue publishes completed-order messages to Valkey for the invoice
// worker. The queue is a Valkey list; the message is a JSON object carrying the
// order id (the idempotency key) and the order number.
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	valkey "github.com/valkey-io/valkey-go"
)

// CompletedOrdersList is the Valkey list the invoice worker consumes.
const CompletedOrdersList = "workshop:completed_orders"

// CompletedOrderMessage is the payload pushed when an order reaches "done".
type CompletedOrderMessage struct {
	OrderID     int64  `json:"order_id"`
	OrderNumber string `json:"order_number"`
}

// Queue is a thin, concurrency-safe Valkey client.
type Queue struct {
	client valkey.Client
}

// New parses the Valkey URL and creates a client. VALKEY_URL is required; there
// is no in-memory fallback.
func New(valkeyURL string) (*Queue, error) {
	if valkeyURL == "" {
		return nil, fmt.Errorf("VALKEY_URL is not set; declare it in RUN.json and export it before starting the API")
	}
	opt, err := valkey.ParseURL(valkeyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid VALKEY_URL: %w", err)
	}
	client, err := valkey.NewClient(opt)
	if err != nil {
		return nil, fmt.Errorf("connect to Valkey: %w", err)
	}
	return &Queue{client: client}, nil
}

// Enqueue pushes one completed-order message onto CompletedOrdersList.
func (q *Queue) Enqueue(ctx context.Context, orderID int64, orderNumber string) error {
	if q == nil || q.client == nil {
		return fmt.Errorf("queue is not configured")
	}
	payload, err := json.Marshal(CompletedOrderMessage{OrderID: orderID, OrderNumber: orderNumber})
	if err != nil {
		return fmt.Errorf("encode completed-order message: %w", err)
	}
	cmd := q.client.B().Rpush().Key(CompletedOrdersList).Element(string(payload)).Build()
	if err := q.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("enqueue completed order %d: %w", orderID, err)
	}
	return nil
}

// Ping verifies the Valkey connection.
func (q *Queue) Ping(ctx context.Context) error {
	if q == nil || q.client == nil {
		return fmt.Errorf("queue is not configured")
	}
	return q.client.Do(ctx, q.client.B().Ping().Build()).Error()
}

// Close releases the client and its connections.
func (q *Queue) Close() {
	if q != nil && q.client != nil {
		q.client.Close()
	}
}
