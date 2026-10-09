package store

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
)

// ItemInput is the captured position a workshop employee submits for an order.
// A labour position carries the worked hours, a part position carries the
// quantity and the unit price in whole cents. The amount is never accepted from
// the client: it is computed here in integer cents (SPEC AC-19, AC-27).
type ItemInput struct {
	Kind           string  `json:"kind"`
	Description    string  `json:"description"`
	Quantity       float64 `json:"quantity"`
	Hours          float64 `json:"hours"`
	UnitPriceCents int64   `json:"unit_price_cents"`
}

// ComputeItemAmountCents returns one position's amount in whole cents. A labour
// position is hours times the configured hourly rate, a part position is the
// quantity times its unit price. Rounding is half up, so no sub-cent value is
// ever stored. The hourly rate is passed in (never read from the store) because
// it comes from the process configuration (WORKSHOP_HOURLY_RATE_CENTS).
func ComputeItemAmountCents(in ItemInput, hourlyRateCents int64) int64 {
	if in.Kind == "labor" {
		return int64(math.Round(in.Hours * float64(hourlyRateCents)))
	}
	return int64(math.Round(in.Quantity * float64(in.UnitPriceCents)))
}

// AddOrderItem stores one captured position on the order identified by
// orderNumber and returns it with its computed amount. It reports
// ErrOrderNotFound when no order carries that number. Every value is a bound
// parameter (SPEC AC-27).
//
// A labour position stores its hours; its unit price is not stored because the
// invoice computes it from the configured hourly rate. A part position stores
// its quantity and unit price. Both store the amount computed by
// ComputeItemAmountCents.
func (s *Store) AddOrderItem(ctx context.Context, orderNumber string, in ItemInput, hourlyRateCents int64) (OrderItem, error) {
	if s == nil || s.Pool == nil {
		return OrderItem{}, fmt.Errorf("store is not configured")
	}

	// The columns are NUMERIC(12, 2); normalise first so the stored value and
	// the computed amount agree on the same number of decimals.
	in.Hours = roundToCents(in.Hours)
	in.Quantity = roundToCents(in.Quantity)

	var (
		hours          float64
		quantity       float64
		unitPriceCents int64
	)
	if in.Kind == "labor" {
		hours = in.Hours
	} else {
		quantity = in.Quantity
		unitPriceCents = in.UnitPriceCents
	}
	amountCents := ComputeItemAmountCents(in, hourlyRateCents)

	const query = `
		INSERT INTO order_items (order_id, kind, description, quantity, hours, unit_price_cents, amount_cents)
		SELECT o.id, $2, $3, $4, $5, $6, $7
		FROM orders o
		WHERE o.order_number = $1
		RETURNING id, kind, description, quantity::float8, hours::float8, unit_price_cents, amount_cents`

	var item OrderItem
	err := s.Pool.QueryRow(ctx, query,
		orderNumber, in.Kind, in.Description, quantity, hours, unitPriceCents, amountCents,
	).Scan(
		&item.ID,
		&item.Kind,
		&item.Description,
		&item.Quantity,
		&item.Hours,
		&item.UnitPriceCents,
		&item.AmountCents,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrderItem{}, ErrOrderNotFound
	}
	if err != nil {
		return OrderItem{}, fmt.Errorf("add order item: %w", err)
	}
	return item, nil
}

// roundToCents rounds a decimal value to two decimals (half away from zero),
// the precision the NUMERIC(12, 2) columns use.
func roundToCents(value float64) float64 {
	return math.Round(value*100) / 100
}
