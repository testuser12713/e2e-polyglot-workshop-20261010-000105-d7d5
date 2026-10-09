package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Order status workflow (SPEC AC-04, AC-05, AC-18).
//
// The only accepted transitions are
//
//	requested -> confirmed -> in_progress -> done -> picked_up
//
// and every accepted change appends one status_log row carrying the instant in
// UTC, the previous status and the new status. These queries belong to the
// "Implement the order status workflow" ticket.

// ErrStatusConflict is returned when the current status no longer matches the
// status the caller expected, so the transition is refused and nothing changes.
// It covers both a concurrent change and any illegal jump.
var ErrStatusConflict = errors.New("order status conflict")

// OrderStatusRecord is the minimal order identity the status workflow needs.
type OrderStatusRecord struct {
	ID          int64
	OrderNumber string
	Status      string
}

// OrderStatusEntry is one persisted status change: the UTC instant, the previous
// status (empty for the very first entry) and the new status. It is kept apart
// from the list/detail StatusEntry in list.go, which carries JSON-tagged string
// fields for the order detail response.
type OrderStatusEntry struct {
	At         time.Time
	FromStatus string
	ToStatus   string
}

// GetOrderStatus loads the current status of an order by its order number. It
// returns ErrOrderNotFound when the order does not exist.
func (s *Store) GetOrderStatus(ctx context.Context, orderNumber string) (OrderStatusRecord, error) {
	if s == nil || s.Pool == nil {
		return OrderStatusRecord{}, fmt.Errorf("store is not configured")
	}
	const query = `SELECT id, order_number, status FROM orders WHERE order_number = $1`
	var rec OrderStatusRecord
	if err := s.Pool.QueryRow(ctx, query, orderNumber).Scan(&rec.ID, &rec.OrderNumber, &rec.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrderStatusRecord{}, ErrOrderNotFound
		}
		return OrderStatusRecord{}, fmt.Errorf("load order %s: %w", orderNumber, err)
	}
	return rec, nil
}

// TransitionOrderStatus atomically moves an order from fromStatus to toStatus
// and appends the status_log entry with the change instant in UTC. It locks the
// order row for the duration of the transaction, so a concurrent change cannot
// slip in between the check and the update. It returns ErrOrderNotFound when the
// order does not exist and ErrStatusConflict when the order's current status is
// not fromStatus; in both cases the order is left unchanged. The order id is
// returned so the caller can enqueue a completion message.
func (s *Store) TransitionOrderStatus(ctx context.Context, orderNumber, fromStatus, toStatus string) (int64, error) {
	if s == nil || s.Pool == nil {
		return 0, fmt.Errorf("store is not configured")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin status transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orderID int64
	var current string
	const selectQuery = `SELECT id, status FROM orders WHERE order_number = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, selectQuery, orderNumber).Scan(&orderID, &current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrOrderNotFound
		}
		return 0, fmt.Errorf("load order %s for status change: %w", orderNumber, err)
	}
	if current != fromStatus {
		return 0, ErrStatusConflict
	}

	const updateQuery = `UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`
	if _, err := tx.Exec(ctx, updateQuery, toStatus, orderID); err != nil {
		return 0, fmt.Errorf("update order %s status: %w", orderNumber, err)
	}

	const logQuery = `INSERT INTO status_log (order_id, at, from_status, to_status) VALUES ($1, $2, $3, $4)`
	if _, err := tx.Exec(ctx, logQuery, orderID, time.Now().UTC(), fromStatus, toStatus); err != nil {
		return 0, fmt.Errorf("write status log for order %s: %w", orderNumber, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit status change for order %s: %w", orderNumber, err)
	}
	return orderID, nil
}

// OrderStatusHistory returns the persisted status changes of one order, oldest
// first (SPEC AC-05). A NULL from_status, which a row created at order creation
// may carry, is returned as the empty string.
func (s *Store) OrderStatusHistory(ctx context.Context, orderID int64) ([]OrderStatusEntry, error) {
	if s == nil || s.Pool == nil {
		return nil, fmt.Errorf("store is not configured")
	}
	const query = `
		SELECT at, COALESCE(from_status, ''), to_status
		FROM status_log
		WHERE order_id = $1
		ORDER BY at, id`
	rows, err := s.Pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("load status history for order %d: %w", orderID, err)
	}
	defer rows.Close()

	var entries []OrderStatusEntry
	for rows.Next() {
		var entry OrderStatusEntry
		if err := rows.Scan(&entry.At, &entry.FromStatus, &entry.ToStatus); err != nil {
			return nil, fmt.Errorf("scan status history for order %d: %w", orderID, err)
		}
		entry.At = entry.At.UTC()
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status history for order %d: %w", orderID, err)
	}
	return entries, nil
}
