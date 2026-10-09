package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"workshop/backend/internal/store"
)

// trackCustomer is the nested customer object of an OrderDetail response.
type trackCustomer struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// trackVehicle is the nested vehicle object of an OrderDetail response.
type trackVehicle struct {
	ID      int64  `json:"id"`
	Plate   string `json:"plate"`
	Make    string `json:"make"`
	Model   string `json:"model"`
	Mileage int    `json:"mileage"`
}

// trackItem is one position of an OrderDetail response.
type trackItem struct {
	ID             int64   `json:"id"`
	Kind           string  `json:"kind"`
	Description    string  `json:"description"`
	Quantity       float64 `json:"quantity"`
	Hours          float64 `json:"hours"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	AmountCents    int64   `json:"amount_cents"`
}

// trackStatusEntry is one history entry of an OrderDetail response.
type trackStatusEntry struct {
	At         string `json:"at"`
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
}

// trackOrderDetail is the OrderDetail object of the tracking response.
type trackOrderDetail struct {
	OrderNumber  string             `json:"order_number"`
	Status       string             `json:"status"`
	DesiredDate  string             `json:"desired_date"`
	VehiclePlate string             `json:"vehicle_plate"`
	Make         string             `json:"make"`
	Model        string             `json:"model"`
	CustomerName string             `json:"customer_name"`
	Problem      string             `json:"problem"`
	Customer     trackCustomer      `json:"customer"`
	Vehicle      trackVehicle       `json:"vehicle"`
	Items        []trackItem        `json:"items"`
	History      []trackStatusEntry `json:"history"`
}

// trackOrderResponse is the body of GET /api/public/orders/{order_number}.
type trackOrderResponse struct {
	Order   trackOrderDetail  `json:"order"`
	Invoice *trackInvoiceBody `json:"invoice"`
}

// trackInvoiceItem is one position of an invoice.
type trackInvoiceItem struct {
	Description string `json:"description"`
	AmountCents int64  `json:"amount_cents"`
}

// trackInvoiceBody is the Invoice object of the tracking and invoice
// responses.
type trackInvoiceBody struct {
	InvoiceNumber string             `json:"invoice_number"`
	OrderNumber   string             `json:"order_number"`
	Items         []trackInvoiceItem `json:"items"`
	NetCents      int64              `json:"net_cents"`
	TaxCents      int64              `json:"tax_cents"`
	GrossCents    int64              `json:"gross_cents"`
	CreatedAt     string             `json:"created_at"`
}

// handleTrackOrder serves GET /api/public/orders/{order_number}.
// Owned by "Implement public order tracking and invoice retrieval".
//
// The order number alone is not enough: the caller must also present the
// licence plate of the order's vehicle. A missing, wrong or unknown pair
// answers 404 with the unified error body and releases no order data
// (SPEC AC-11).
func (s *Server) handleTrackOrder(w http.ResponseWriter, r *http.Request) {
	orderNumber := strings.TrimSpace(r.PathValue("order_number"))
	plate := strings.TrimSpace(r.URL.Query().Get("plate"))

	if orderNumber == "" {
		notFound(w, "Auftrag nicht gefunden.")
		return
	}
	if plate == "" {
		unprocessable(w, "Bitte geben Sie das Kennzeichen an.")
		return
	}
	if s == nil || s.Store == nil {
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}

	order, found, err := s.Store.TrackOrder(r.Context(), orderNumber, plate)
	if err != nil {
		log.Printf("track order %s: %v", orderNumber, err)
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}
	if !found {
		notFound(w, "Auftrag nicht gefunden.")
		return
	}

	resp := trackOrderResponse{Order: toTrackOrderDetail(order)}

	invoice, hasInvoice, err := s.Store.InvoiceByOrder(r.Context(), orderNumber, plate)
	if err != nil {
		log.Printf("track order %s: load invoice: %v", orderNumber, err)
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}
	if hasInvoice {
		resp.Invoice = toTrackInvoice(invoice)
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGetInvoice serves GET /api/public/orders/{order_number}/invoice.
// Owned by "Implement public order tracking and invoice retrieval".
//
// It returns the invoice only for a matching order-number + plate pair and
// answers 404 both for a mismatch and when no invoice exists yet (SPEC AC-12).
func (s *Server) handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	orderNumber := strings.TrimSpace(r.PathValue("order_number"))
	plate := strings.TrimSpace(r.URL.Query().Get("plate"))

	if orderNumber == "" {
		notFound(w, "Rechnung nicht gefunden.")
		return
	}
	if plate == "" {
		unprocessable(w, "Bitte geben Sie das Kennzeichen an.")
		return
	}
	if s == nil || s.Store == nil {
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}

	invoice, found, err := s.Store.InvoiceByOrder(r.Context(), orderNumber, plate)
	if err != nil {
		log.Printf("invoice for order %s: %v", orderNumber, err)
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}
	if !found {
		notFound(w, "Rechnung nicht gefunden.")
		return
	}

	writeJSON(w, http.StatusOK, toTrackInvoice(invoice))
}

func toTrackOrderDetail(order *store.TrackedOrder) trackOrderDetail {
	items := make([]trackItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, trackItem{
			ID:             item.ID,
			Kind:           item.Kind,
			Description:    item.Description,
			Quantity:       item.Quantity,
			Hours:          item.Hours,
			UnitPriceCents: item.UnitPriceCents,
			AmountCents:    item.AmountCents,
		})
	}

	history := make([]trackStatusEntry, 0, len(order.History))
	for _, entry := range order.History {
		history = append(history, trackStatusEntry{
			At:         entry.At.UTC().Format(time.RFC3339),
			FromStatus: entry.FromStatus,
			ToStatus:   entry.ToStatus,
		})
	}

	return trackOrderDetail{
		OrderNumber:  order.OrderNumber,
		Status:       order.Status,
		DesiredDate:  order.DesiredDate.Format("2006-01-02"),
		VehiclePlate: order.VehiclePlate,
		Make:         order.VehicleMake,
		Model:        order.VehicleModel,
		CustomerName: order.CustomerName,
		Problem:      order.Problem,
		Customer: trackCustomer{
			ID:    order.CustomerID,
			Name:  order.CustomerName,
			Email: order.CustomerEmail,
			Phone: order.CustomerPhone,
		},
		Vehicle: trackVehicle{
			ID:      order.VehicleID,
			Plate:   order.VehiclePlate,
			Make:    order.VehicleMake,
			Model:   order.VehicleModel,
			Mileage: order.VehicleMileage,
		},
		Items:   items,
		History: history,
	}
}

func toTrackInvoice(invoice *store.TrackedInvoice) *trackInvoiceBody {
	items := make([]trackInvoiceItem, 0, len(invoice.Items))
	for _, item := range invoice.Items {
		items = append(items, trackInvoiceItem{
			Description: item.Description,
			AmountCents: item.AmountCents,
		})
	}
	return &trackInvoiceBody{
		InvoiceNumber: invoice.InvoiceNumber,
		OrderNumber:   invoice.OrderNumber,
		Items:         items,
		NetCents:      invoice.NetCents,
		TaxCents:      invoice.TaxCents,
		GrossCents:    invoice.GrossCents,
		CreatedAt:     invoice.CreatedAt.UTC().Format(time.RFC3339),
	}
}
