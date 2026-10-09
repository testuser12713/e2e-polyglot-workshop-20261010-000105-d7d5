package api

import "net/http"

// handleCreateVehicle serves POST /api/public/vehicles.
// Owned by "Implement customer and vehicle registration with bound SQL".
func (s *Server) handleCreateVehicle(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "POST /api/public/vehicles")
}
