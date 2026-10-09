package api

import "net/http"

// handleAddOrderItem serves POST /api/shop/orders/{order_number}/items.
// Owned by "Implement workshop position capture with cent amounts".
func (s *Server) handleAddOrderItem(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "POST /api/shop/orders/{order_number}/items")
}
