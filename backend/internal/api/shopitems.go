package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"workshop/backend/internal/store"
)

// handleAddOrderItem serves POST /api/shop/orders/{order_number}/items. It
// captures one labour or part position on an order, computes its amount in
// whole cents and answers 201 with the stored position (SPEC AC-19).
//
// A labour position is described by hours and is billed at the configured
// hourly rate; a part position is described by quantity and unit price. The
// client never sends amount_cents.
//
// Owned by "Implement workshop position capture with cent amounts".
func (s *Server) handleAddOrderItem(w http.ResponseWriter, r *http.Request) {
	if !hasShopSession(r) {
		unauthorized(w, "Anmeldung erforderlich.")
		return
	}
	if s.Store == nil || s.Config == nil {
		internalError(w, "Die Position konnte nicht gespeichert werden.")
		return
	}

	orderNumber := strings.TrimSpace(r.PathValue("order_number"))
	if orderNumber == "" {
		notFound(w, "Auftrag nicht gefunden.")
		return
	}

	var in store.ItemInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		unprocessable(w, "Die Anfrage ist kein gültiges JSON.")
		return
	}
	in.Kind = strings.TrimSpace(in.Kind)
	in.Description = strings.TrimSpace(in.Description)

	if message := validateItemInput(in); message != "" {
		unprocessable(w, message)
		return
	}

	item, err := s.Store.AddOrderItem(r.Context(), orderNumber, in, int64(s.Config.WorkshopHourlyRateCents))
	if errors.Is(err, store.ErrOrderNotFound) {
		notFound(w, "Auftrag nicht gefunden.")
		return
	}
	if err != nil {
		logStoreFailure("add order item", err)
		internalError(w, "Die Position konnte nicht gespeichert werden.")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// validateItemInput returns a German message for the first missing or invalid
// field, or an empty string when the position is valid. amount_cents is
// deliberately not read from the request.
func validateItemInput(in store.ItemInput) string {
	switch in.Kind {
	case "labor":
		if in.Description == "" {
			return "Bitte eine Beschreibung angeben."
		}
		if in.Hours <= 0 {
			return "Bitte die Arbeitszeit in Stunden angeben."
		}
	case "part":
		if in.Description == "" {
			return "Bitte eine Beschreibung angeben."
		}
		if in.Quantity <= 0 {
			return "Bitte die Menge angeben."
		}
		if in.UnitPriceCents <= 0 {
			return "Bitte einen Stückpreis angeben."
		}
	default:
		return "Bitte die Positionsart mit 'labor' oder 'part' angeben."
	}
	return ""
}
