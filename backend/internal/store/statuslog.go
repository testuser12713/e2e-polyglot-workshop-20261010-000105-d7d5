package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// StatusLogEntry is one recorded status change of an order. FromStatus is nil
// for the initial entry, which has no predecessor.
type StatusLogEntry struct {
	At         time.Time
	FromStatus *string
	ToStatus   string
}

// insertStatusLog appends one row to status_log inside the caller's
// transaction. The from_status of the initial entry is NULL.
func insertStatusLog(ctx context.Context, tx pgx.Tx, orderID int64, fromStatus *string, toStatus string) error {
	const query = `
		INSERT INTO status_log (order_id, from_status, to_status)
		VALUES ($1, $2, $3)`
	if _, err := tx.Exec(ctx, query, orderID, fromStatus, toStatus); err != nil {
		return fmt.Errorf("insert status log: %w", err)
	}
	return nil
}

// ListStatusLog returns the status history of one order in chronological order.
// The query uses a bound parameter (SPEC AC-27).
func (s *Store) ListStatusLog(ctx context.Context, orderID int64) ([]StatusLogEntry, error) {
	if s == nil || s.Pool == nil {
		return nil, fmt.Errorf("store is not configured")
	}
	const query = `
		SELECT at, from_status, to_status
		FROM status_log
		WHERE order_id = $1
		ORDER BY at ASC, id ASC`
	rows, err := s.Pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("list status log: %w", err)
	}
	defer rows.Close()

	var entries []StatusLogEntry
	for rows.Next() {
		var entry StatusLogEntry
		if err := rows.Scan(&entry.At, &entry.FromStatus, &entry.ToStatus); err != nil {
			return nil, fmt.Errorf("scan status log: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status log: %w", err)
	}
	return entries, nil
}
