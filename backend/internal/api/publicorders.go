package api

import "net/http"

// handleCreateOrder serves POST /api/public/orders.
// Owned by "Implement the public order request with order number".
func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "POST /api/public/orders")
}
