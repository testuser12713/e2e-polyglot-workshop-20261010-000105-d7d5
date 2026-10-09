package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TrackedInvoiceItem is one invoice position (a labour or parts row the invoice
// was computed from).
type TrackedInvoiceItem struct {
	Description string
	AmountCents int64
}

// TrackedInvoice is the invoice view a customer may see after passing the
// order-number + licence-plate check. All amounts are integer cents.
type TrackedInvoice struct {
	ID            int64
	InvoiceNumber string
	OrderNumber   string
	Items         []TrackedInvoiceItem
	NetCents      int64
	TaxCents      int64
	GrossCents    int64
	CreatedAt     time.Time
}

// InvoiceByOrder resolves the invoice of an order by its order number and the
// licence plate of the vehicle it belongs to. It reports found=false when the
// order/plate pair does not match OR when no invoice exists yet, so the caller
// answers 404 in both cases (SPEC AC-12, contract). Values are bound
// parameters (SPEC AC-27).
func (s *Store) InvoiceByOrder(ctx context.Context, orderNumber, plate string) (*TrackedInvoice, bool, error) {
	if s == nil || s.Pool == nil {
		return nil, false, fmt.Errorf("store is not configured")
	}

	const query = `
		SELECT i.id, i.invoice_number, o.order_number,
		       i.net_cents, i.tax_cents, i.gross_cents, i.created_at
		FROM invoices i
		JOIN orders o ON o.id = i.order_id
		JOIN vehicles v ON v.id = o.vehicle_id
		WHERE o.order_number = $1
		  AND upper(trim(v.plate)) = upper(trim($2))`

	invoice := &TrackedInvoice{}
	err := s.Pool.QueryRow(ctx, query, orderNumber, plate).Scan(
		&invoice.ID,
		&invoice.InvoiceNumber,
		&invoice.OrderNumber,
		&invoice.NetCents,
		&invoice.TaxCents,
		&invoice.GrossCents,
		&invoice.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("load invoice: %w", err)
	}

	items, err := s.invoiceItems(ctx, invoice.ID)
	if err != nil {
		return nil, false, err
	}
	invoice.Items = items

	return invoice, true, nil
}

func (s *Store) invoiceItems(ctx context.Context, invoiceID int64) ([]TrackedInvoiceItem, error) {
	const query = `
		SELECT description, amount_cents
		FROM invoice_items
		WHERE invoice_id = $1
		ORDER BY id`

	rows, err := s.Pool.Query(ctx, query, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("load invoice items: %w", err)
	}
	defer rows.Close()

	items := make([]TrackedInvoiceItem, 0)
	for rows.Next() {
		var item TrackedInvoiceItem
		if err := rows.Scan(&item.Description, &item.AmountCents); err != nil {
			return nil, fmt.Errorf("scan invoice item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invoice items: %w", err)
	}
	return items, nil
}
