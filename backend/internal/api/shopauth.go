package api

import "net/http"

// handleShopLogin serves POST /api/shop/login.
// Owned by "Implement workshop authentication with hashed passwords, sessions
// and rate limit".
func (s *Server) handleShopLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "POST /api/shop/login")
}
