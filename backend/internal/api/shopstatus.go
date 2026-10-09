package api

import "net/http"

// handleSetOrderStatus serves POST /api/shop/orders/{order_number}/status.
// Owned by "Implement the order status workflow with transition rules and
// completion message".
func (s *Server) handleSetOrderStatus(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "POST /api/shop/orders/{order_number}/status")
}
