package api

import "net/http"

// handleShopDashboard serves GET /api/shop/dashboard.
// Owned by "Implement the workshop dashboard metrics endpoint".
func (s *Server) handleShopDashboard(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "GET /api/shop/dashboard")
}
