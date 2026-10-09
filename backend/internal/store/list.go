package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrOrderNotFound reports that no order carries the requested order number.
// Handlers translate it into the unified 404 body.
var ErrOrderNotFound = errors.New("order not found")

// OrderSummary is one row of the workshop order list. It carries the order
// metadata the workshop area shows without loading the full detail.
type OrderSummary struct {
	OrderNumber  string `json:"order_number"`
	Status       string `json:"status"`
	DesiredDate  string `json:"desired_date"`
	VehiclePlate string `json:"vehicle_plate"`
	Make         string `json:"make"`
	Model        string `json:"model"`
	CustomerName string `json:"customer_name"`
}

// OrderItem is one captured position (labour or part) of an order.
type OrderItem struct {
	ID             int64   `json:"id"`
	Kind           string  `json:"kind"`
	Description    string  `json:"description"`
	Quantity       float64 `json:"quantity"`
	Hours          float64 `json:"hours"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	AmountCents    int64   `json:"amount_cents"`
}

// StatusEntry is one recorded status transition. FromStatus is nil for the
// initial transition into "requested".
type StatusEntry struct {
	At         string  `json:"at"`
	FromStatus *string `json:"from_status"`
	ToStatus   string  `json:"to_status"`
}

// OrderDetail is the full order: the summary plus problem, customer, vehicle,
// positions and status history.
type OrderDetail struct {
	OrderSummary
	Problem  string        `json:"problem"`
	Customer Customer      `json:"customer"`
	Vehicle  Vehicle       `json:"vehicle"`
	Items    []OrderItem   `json:"items"`
	History  []StatusEntry `json:"history"`
}

// OrderFilter narrows the order list. An empty field means "no restriction";
// Plate matches case-insensitively as a substring of the stored plate.
type OrderFilter struct {
	Status string
	Plate  string
}

// ListOrders returns every order matching the filter, newest first. Both
// filter values are passed as bound parameters (SPEC AC-27).
func (s *Store) ListOrders(ctx context.Context, filter OrderFilter) ([]OrderSummary, error) {
	if s == nil || s.Pool == nil {
		return nil, fmt.Errorf("store is not configured")
	}

	const query = `
		SELECT o.order_number, o.status, o.desired_date::text,
		       v.plate, v.make, v.model, c.name
		FROM orders o
		JOIN vehicles v ON v.id = o.vehicle_id
		JOIN customers c ON c.id = o.customer_id
		WHERE ($1 = '' OR o.status = $1)
		  AND ($2 = '' OR v.plate ILIKE '%' || $2 || '%' ESCAPE '\')
		ORDER BY o.created_at DESC, o.id DESC`

	rows, err := s.Pool.Query(ctx, query, strings.TrimSpace(filter.Status), escapeLike(strings.TrimSpace(filter.Plate)))
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]OrderSummary, 0)
	for rows.Next() {
		var summary OrderSummary
		if err := rows.Scan(
			&summary.OrderNumber,
			&summary.Status,
			&summary.DesiredDate,
			&summary.VehiclePlate,
			&summary.Make,
			&summary.Model,
			&summary.CustomerName,
		); err != nil {
			return nil, fmt.Errorf("scan order summary: %w", err)
		}
		orders = append(orders, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}
	return orders, nil
}

// GetOrder loads one order with its positions and status history. It returns
// ErrOrderNotFound when the order number is unknown.
func (s *Store) GetOrder(ctx context.Context, orderNumber string) (*OrderDetail, error) {
	if s == nil || s.Pool == nil {
		return nil, fmt.Errorf("store is not configured")
	}

	const orderQuery = `
		SELECT o.id, o.order_number, o.status, o.desired_date::text, o.problem,
		       v.id, v.plate, v.make, v.model, v.mileage,
		       c.id, c.name, c.email, c.phone
		FROM orders o
		JOIN vehicles v ON v.id = o.vehicle_id
		JOIN customers c ON c.id = o.customer_id
		WHERE o.order_number = $1`

	var (
		orderID int64
		detail  OrderDetail
	)
	err := s.Pool.QueryRow(ctx, orderQuery, orderNumber).Scan(
		&orderID,
		&detail.OrderNumber,
		&detail.Status,
		&detail.DesiredDate,
		&detail.Problem,
		&detail.Vehicle.ID,
		&detail.Vehicle.Plate,
		&detail.Vehicle.Make,
		&detail.Vehicle.Model,
		&detail.Vehicle.Mileage,
		&detail.Customer.ID,
		&detail.Customer.Name,
		&detail.Customer.Email,
		&detail.Customer.Phone,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load order: %w", err)
	}

	// Mirror the summary columns from the joined vehicle/customer so the
	// embedded OrderSummary is fully populated.
	detail.VehiclePlate = detail.Vehicle.Plate
	detail.Make = detail.Vehicle.Make
	detail.Model = detail.Vehicle.Model
	detail.CustomerName = detail.Customer.Name

	items, err := s.listOrderItems(ctx, orderID)
	if err != nil {
		return nil, err
	}
	history, err := s.listStatusHistory(ctx, orderID)
	if err != nil {
		return nil, err
	}
	detail.Items = items
	detail.History = history
	return &detail, nil
}

func (s *Store) listOrderItems(ctx context.Context, orderID int64) ([]OrderItem, error) {
	const query = `
		SELECT id, kind, description, quantity::float8, hours::float8,
		       unit_price_cents, amount_cents
		FROM order_items
		WHERE order_id = $1
		ORDER BY id`

	rows, err := s.Pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	defer rows.Close()

	items := make([]OrderItem, 0)
	for rows.Next() {
		var item OrderItem
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

func (s *Store) listStatusHistory(ctx context.Context, orderID int64) ([]StatusEntry, error) {
	const query = `
		SELECT at, from_status, to_status
		FROM status_log
		WHERE order_id = $1
		ORDER BY at ASC, id ASC`

	rows, err := s.Pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("list status history: %w", err)
	}
	defer rows.Close()

	history := make([]StatusEntry, 0)
	for rows.Next() {
		var (
			at    time.Time
			from  *string
			to    string
			entry StatusEntry
		)
		if err := rows.Scan(&at, &from, &to); err != nil {
			return nil, fmt.Errorf("scan status entry: %w", err)
		}
		entry.At = at.UTC().Format(time.RFC3339)
		entry.FromStatus = from
		entry.ToStatus = to
		history = append(history, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status history: %w", err)
	}
	return history, nil
}

// escapeLike neutralises the LIKE wildcards in user input so a plate search
// matches literally (SPEC AC-27 keeps the value a bound parameter).
func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
