package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"workshop/backend/internal/store"
)

// Order status workflow (SPEC AC-04, AC-05, AC-06, AC-18).
//
// POST /api/shop/orders/{order_number}/status {status} accepts exactly the
// linear workflow
//
//	requested -> confirmed -> in_progress -> done -> picked_up
//
// Every other transition answers 409 and leaves the order unchanged. An accepted
// change is persisted with a UTC status_log entry; reaching "done" also pushes
// exactly one JSON message onto the Valkey completed-orders list.

// orderStatusChain maps each status to its single allowed successor.
var orderStatusChain = map[string]string{
	"requested":   "confirmed",
	"confirmed":   "in_progress",
	"in_progress": "done",
	"done":        "picked_up",
}

// knownOrderStatuses is the closed set of statuses an order may carry.
var knownOrderStatuses = map[string]bool{
	"requested":   true,
	"confirmed":   true,
	"in_progress": true,
	"done":        true,
	"picked_up":   true,
}

// setOrderStatusRequest is the request body of the status endpoint.
type setOrderStatusRequest struct {
	Status string `json:"status"`
}

// setOrderStatusResponse is the response body of the status endpoint.
type setOrderStatusResponse struct {
	OrderNumber string `json:"order_number"`
	Status      string `json:"status"`
}

// handleSetOrderStatus serves POST /api/shop/orders/{order_number}/status.
// Like every other shop endpoint it refuses an unauthenticated caller with the
// unified 401 body (SPEC AC-28, shared contract).
func (s *Server) handleSetOrderStatus(w http.ResponseWriter, r *http.Request) {
	if !hasShopSession(r) {
		unauthorized(w, "Anmeldung erforderlich.")
		return
	}

	orderNumber := strings.TrimSpace(r.PathValue("order_number"))
	if orderNumber == "" {
		notFound(w, "Auftrag nicht gefunden.")
		return
	}

	var req setOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		unprocessable(w, "Ungültige Anfrage: erwartet wird ein Status.")
		return
	}
	target := strings.TrimSpace(req.Status)
	if !knownOrderStatuses[target] {
		unprocessable(w, "Unbekannter Status.")
		return
	}

	if s == nil || s.Store == nil {
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}

	current, err := s.Store.GetOrderStatus(r.Context(), orderNumber)
	if err != nil {
		if errors.Is(err, store.ErrOrderNotFound) {
			notFound(w, "Auftrag nicht gefunden.")
			return
		}
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}

	next, ok := orderStatusChain[current.Status]
	if !ok || next != target {
		conflict(w, "Dieser Statuswechsel ist nicht erlaubt.")
		return
	}

	orderID, err := s.Store.TransitionOrderStatus(r.Context(), orderNumber, current.Status, target)
	if err != nil {
		if errors.Is(err, store.ErrStatusConflict) || errors.Is(err, store.ErrOrderNotFound) {
			conflict(w, "Dieser Statuswechsel ist nicht erlaubt.")
			return
		}
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}

	if target == "done" {
		if s.Queue == nil {
			internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
			return
		}
		if err := s.Queue.Enqueue(r.Context(), orderID, orderNumber); err != nil {
			internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
			return
		}
	}

	writeJSON(w, http.StatusOK, setOrderStatusResponse{OrderNumber: orderNumber, Status: target})
}
