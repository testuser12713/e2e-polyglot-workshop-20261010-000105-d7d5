package store

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

// CustomerInput is the customer data of a public order request. It identifies an
// existing customer by e-mail and creates a new one when the e-mail is unknown.
type CustomerInput struct {
	Name  string
	Email string
	Phone string
}

// VehicleInput is the vehicle data of a public order request. It identifies an
// existing vehicle by licence plate and creates a new one when unknown.
type VehicleInput struct {
	Plate   string
	Make    string
	Model   string
	Mileage int
}

// OrderItemInput is one position of a new order. Hours apply to labour
// positions, quantity to parts. Amounts are whole cents.
type OrderItemInput struct {
	Kind           string
	Description    string
	Quantity       float64
	Hours          float64
	UnitPriceCents int64
}

// CreateOrderParams carries everything needed to create one order, its
// positions and the initial status-log entry.
type CreateOrderParams struct {
	Customer        CustomerInput
	Vehicle         VehicleInput
	DesiredDate     time.Time
	Problem         string
	Items           []OrderItemInput
	HourlyRateCents int64
}

// CreatedOrder is the result of a successful CreateOrder call.
type CreatedOrder struct {
	ID          int64
	OrderNumber string
	Status      string
	CustomerID  int64
	VehicleID   int64
	CreatedAt   time.Time
}

// CreateOrder resolves or creates the customer and the vehicle, then inserts the
// order, its positions and the first status-log entry in a single transaction.
// The order number is derived from the generated order id, so it is unique and
// stable. The order always starts in status "requested" (SPEC AC-03, AC-05).
//
// Every statement uses bound parameters; no SQL is assembled from user input
// (SPEC AC-27).
func (s *Store) CreateOrder(ctx context.Context, p CreateOrderParams) (*CreatedOrder, error) {
	if s == nil || s.Pool == nil {
		return nil, fmt.Errorf("store is not configured")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin order transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	customerID, err := upsertCustomer(ctx, tx, p.Customer)
	if err != nil {
		return nil, err
	}

	vehicleID, err := upsertVehicle(ctx, tx, p.Vehicle)
	if err != nil {
		return nil, err
	}

	const insertOrder = `
		INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
		VALUES (gen_random_uuid()::text, $1, $2, 'requested', $3, $4)
		RETURNING id, created_at`
	var (
		orderID   int64
		createdAt time.Time
	)
	if err := tx.QueryRow(ctx, insertOrder, customerID, vehicleID, p.DesiredDate, p.Problem).
		Scan(&orderID, &createdAt); err != nil {
		return nil, fmt.Errorf("insert order: %w", err)
	}

	orderNumber := formatOrderNumber(orderID, createdAt)
	const updateOrderNumber = `UPDATE orders SET order_number = $1, updated_at = now() WHERE id = $2`
	if _, err := tx.Exec(ctx, updateOrderNumber, orderNumber, orderID); err != nil {
		return nil, fmt.Errorf("assign order number: %w", err)
	}

	const insertItem = `
		INSERT INTO order_items
			(order_id, kind, description, quantity, hours, unit_price_cents, amount_cents)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	for _, item := range p.Items {
		amount := amountCents(item, p.HourlyRateCents)
		if _, err := tx.Exec(ctx, insertItem,
			orderID, item.Kind, item.Description,
			item.Quantity, item.Hours, item.UnitPriceCents, amount,
		); err != nil {
			return nil, fmt.Errorf("insert order item: %w", err)
		}
	}

	if err := insertStatusLog(ctx, tx, orderID, nil, "requested"); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit order transaction: %w", err)
	}

	return &CreatedOrder{
		ID:          orderID,
		OrderNumber: orderNumber,
		Status:      "requested",
		CustomerID:  customerID,
		VehicleID:   vehicleID,
		CreatedAt:   createdAt,
	}, nil
}

// upsertCustomer returns the id of the customer with the given e-mail, creating
// one when the e-mail is unknown. The no-op update lets the conflicting row be
// returned without overwriting the stored data.
func upsertCustomer(ctx context.Context, tx pgx.Tx, in CustomerInput) (int64, error) {
	const query = `
		INSERT INTO customers (name, email, phone)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`
	var id int64
	if err := tx.QueryRow(ctx, query, in.Name, in.Email, in.Phone).Scan(&id); err != nil {
		return 0, fmt.Errorf("resolve or create customer: %w", err)
	}
	return id, nil
}

// upsertVehicle returns the id of the vehicle with the given plate, creating one
// when the plate is unknown.
func upsertVehicle(ctx context.Context, tx pgx.Tx, in VehicleInput) (int64, error) {
	const query = `
		INSERT INTO vehicles (plate, make, model, mileage)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (plate) DO UPDATE SET plate = EXCLUDED.plate
		RETURNING id`
	var id int64
	if err := tx.QueryRow(ctx, query, in.Plate, in.Make, in.Model, in.Mileage).Scan(&id); err != nil {
		return 0, fmt.Errorf("resolve or create vehicle: %w", err)
	}
	return id, nil
}

// formatOrderNumber builds the human-readable order number from the generated
// order id and the year the order was created.
func formatOrderNumber(orderID int64, createdAt time.Time) string {
	return fmt.Sprintf("AW-%d-%06d", createdAt.UTC().Year(), orderID)
}

// amountCents computes the amount of one position. Labour is hours times the
// position price, falling back to the configured hourly rate when no price was
// given; a part is quantity times its unit price. Fractions of a cent round to
// the nearest cent.
func amountCents(item OrderItemInput, hourlyRateCents int64) int64 {
	if item.Kind == "labor" {
		rate := item.UnitPriceCents
		if rate <= 0 {
			rate = hourlyRateCents
		}
		return int64(math.Round(item.Hours * float64(rate)))
	}
	return int64(math.Round(item.Quantity * float64(item.UnitPriceCents)))
}
