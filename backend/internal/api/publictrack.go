package api

import "net/http"

// handleTrackOrder serves GET /api/public/orders/{order_number}.
// Owned by "Implement public order tracking and invoice retrieval".
func (s *Server) handleTrackOrder(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "GET /api/public/orders/{order_number}")
}

// handleGetInvoice serves GET /api/public/orders/{order_number}/invoice.
// Owned by "Implement public order tracking and invoice retrieval".
func (s *Server) handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "GET /api/public/orders/{order_number}/invoice")
}
