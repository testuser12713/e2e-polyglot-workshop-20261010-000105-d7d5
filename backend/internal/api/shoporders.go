package api

import "net/http"

// handleListOrders serves GET /api/shop/orders.
// Owned by "Implement the workshop order list with status filter and plate
// search".
func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "GET /api/shop/orders")
}

// handleGetShopOrder serves GET /api/shop/orders/{order_number}.
// Owned by "Implement the workshop order list with status filter and plate
// search".
func (s *Server) handleGetShopOrder(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "GET /api/shop/orders/{order_number}")
}
