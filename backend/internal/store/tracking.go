package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TrackedItem is one recorded labour or parts position of a tracked order.
type TrackedItem struct {
	ID             int64
	Kind           string
	Description    string
	Quantity       float64
	Hours          float64
	UnitPriceCents int64
	AmountCents    int64
}

// TrackedStatusEntry is one recorded status transition of an order.
type TrackedStatusEntry struct {
	At         time.Time
	FromStatus string
	ToStatus   string
}

// TrackedOrder is the order view a customer may see after passing the
// order-number + licence-plate check. It carries the order summary, the
// related customer and vehicle and the recorded positions and status history.
type TrackedOrder struct {
	ID             int64
	OrderNumber    string
	Status         string
	DesiredDate    time.Time
	Problem        string
	CustomerID     int64
	CustomerName   string
	CustomerEmail  string
	CustomerPhone  string
	VehicleID      int64
	VehiclePlate   string
	VehicleMake    string
	VehicleModel   string
	VehicleMileage int
	Items          []TrackedItem
	History        []TrackedStatusEntry
}

// TrackOrder resolves an order by its order number and the licence plate of the
// vehicle it belongs to. It reports found=false when either value does not
// match, so the caller can answer 404 without releasing any order data
// (SPEC AC-11). Every value is a bound parameter (SPEC AC-27).
func (s *Store) TrackOrder(ctx context.Context, orderNumber, plate string) (*TrackedOrder, bool, error) {
	if s == nil || s.Pool == nil {
		return nil, false, fmt.Errorf("store is not configured")
	}

	const query = `
		SELECT o.id, o.order_number, o.status, o.desired_date, o.problem,
		       c.id, c.name, c.email, c.phone,
		       v.id, v.plate, v.make, v.model, v.mileage
		FROM orders o
		JOIN customers c ON c.id = o.customer_id
		JOIN vehicles v ON v.id = o.vehicle_id
		WHERE o.order_number = $1
		  AND upper(trim(v.plate)) = upper(trim($2))`

	order := &TrackedOrder{}
	err := s.Pool.QueryRow(ctx, query, orderNumber, plate).Scan(
		&order.ID,
		&order.OrderNumber,
		&order.Status,
		&order.DesiredDate,
		&order.Problem,
		&order.CustomerID,
		&order.CustomerName,
		&order.CustomerEmail,
		&order.CustomerPhone,
		&order.VehicleID,
		&order.VehiclePlate,
		&order.VehicleMake,
		&order.VehicleModel,
		&order.VehicleMileage,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("load order: %w", err)
	}

	items, err := s.orderItems(ctx, order.ID)
	if err != nil {
		return nil, false, err
	}
	order.Items = items

	history, err := s.orderStatusHistory(ctx, order.ID)
	if err != nil {
		return nil, false, err
	}
	order.History = history

	return order, true, nil
}

func (s *Store) orderItems(ctx context.Context, orderID int64) ([]TrackedItem, error) {
	const query = `
		SELECT id, kind, description, quantity::float8, hours::float8,
		       unit_price_cents, amount_cents
		FROM order_items
		WHERE order_id = $1
		ORDER BY id`

	rows, err := s.Pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("load order items: %w", err)
	}
	defer rows.Close()

	items := make([]TrackedItem, 0)
	for rows.Next() {
		var item TrackedItem
		if err := rows.Scan(
			&item.ID,
			&item.Kind,
			&item.Description,
			&item.Quantity,
			&item.Hours,
			&item.UnitPriceCents,
			&item.AmountCents,
		); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order items: %w", err)
	}
	return items, nil
}

func (s *Store) orderStatusHistory(ctx context.Context, orderID int64) ([]TrackedStatusEntry, error) {
	const query = `
		SELECT at, COALESCE(from_status, ''), to_status
		FROM status_log
		WHERE order_id = $1
		ORDER BY at ASC, id ASC`

	rows, err := s.Pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("load status history: %w", err)
	}
	defer rows.Close()

	history := make([]TrackedStatusEntry, 0)
	for rows.Next() {
		var entry TrackedStatusEntry
		if err := rows.Scan(&entry.At, &entry.FromStatus, &entry.ToStatus); err != nil {
			return nil, fmt.Errorf("scan status entry: %w", err)
		}
		history = append(history, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status history: %w", err)
	}
	return history, nil
}
