package api

import (
	"errors"
	"net/http"
	"strings"

	"workshop/backend/internal/store"
)

// handleListOrders serves GET /api/shop/orders. The optional status and plate
// query parameters narrow the list; both are passed to the store as bound
// parameters.
//
// Owned by "Implement the workshop order list with status filter and plate
// search".
func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	if !hasShopSession(r) {
		unauthorized(w, "Anmeldung erforderlich.")
		return
	}

	query := r.URL.Query()
	filter := store.OrderFilter{
		Status: strings.TrimSpace(query.Get("status")),
		Plate:  strings.TrimSpace(query.Get("plate")),
	}

	orders, err := s.Store.ListOrders(r.Context(), filter)
	if err != nil {
		internalError(w, "Aufträge konnten nicht geladen werden.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

// handleGetShopOrder serves GET /api/shop/orders/{order_number} and returns the
// order with its positions and status history.
//
// Owned by "Implement the workshop order list with status filter and plate
// search".
func (s *Server) handleGetShopOrder(w http.ResponseWriter, r *http.Request) {
	if !hasShopSession(r) {
		unauthorized(w, "Anmeldung erforderlich.")
		return
	}

	orderNumber := strings.TrimSpace(r.PathValue("order_number"))
	if orderNumber == "" {
		notFound(w, "Auftrag nicht gefunden.")
		return
	}

	detail, err := s.Store.GetOrder(r.Context(), orderNumber)
	if errors.Is(err, store.ErrOrderNotFound) {
		notFound(w, "Auftrag nicht gefunden.")
		return
	}
	if err != nil {
		internalError(w, "Auftrag konnte nicht geladen werden.")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// hasShopSession reports whether requireSession resolved a valid employee for
// this request. Until the authentication ticket #15 makes the middleware refuse
// a missing token itself, this keeps the shop endpoints from serving data to an
// unauthenticated caller (SPEC AC-28).
func hasShopSession(r *http.Request) bool {
	_, ok := r.Context().Value(ctxEmployeeID).(int64)
	return ok
}
