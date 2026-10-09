package api

import "net/http"

// handleCreateCustomer serves POST /api/public/customers.
// Owned by "Implement customer and vehicle registration with bound SQL".
func (s *Server) handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "POST /api/public/customers")
}

// handleGetCustomer serves GET /api/public/customers/{id}.
// Owned by "Implement customer and vehicle registration with bound SQL".
func (s *Server) handleGetCustomer(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "GET /api/public/customers/{id}")
}
