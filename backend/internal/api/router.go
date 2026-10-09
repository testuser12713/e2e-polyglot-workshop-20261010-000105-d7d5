// Package api wires the HTTP surface of the workshop API. It registers every
// route of the agreed contract exactly once and routes each request to its own
// feature handler.
package api

import (
	"net/http"

	"workshop/backend/internal/config"
	"workshop/backend/internal/queue"
	"workshop/backend/internal/store"
)

// Server carries the dependencies every handler needs.
type Server struct {
	Config *config.Config
	Store  *store.Store
	Queue  *queue.Queue

	// loginLimiter is the per-process failed-login counter used by
	// POST /api/shop/login (SPEC AC-29).
	loginLimiter *loginLimiter
}

// NewServer builds a Server from its dependencies. Store and Queue may be nil
// for tests that only exercise routing and the health endpoint.
func NewServer(st *store.Store, cfg *config.Config, q *queue.Queue) *Server {
	return &Server{Config: cfg, Store: st, Queue: q, loginLimiter: newLoginLimiter()}
}

// Handler returns the fully wired HTTP handler: CORS (outermost, so error
// responses keep their headers) -> panic recovery -> logging -> routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	return corsMiddleware(s.corsOrigin(), recoveringMiddleware(loggingMiddleware(mux)))
}

func (s *Server) corsOrigin() string {
	if s != nil && s.Config != nil && s.Config.CORSAllowedOrigin != "" {
		return s.Config.CORSAllowedOrigin
	}
	return config.DefaultCORSAllowedOrigin
}

// registerRoutes declares the complete public, health and shop contract. Every
// path matches the agreed spelling character for character; no trailing-slash
// alias is registered.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Public customer area.
	mux.HandleFunc("POST /api/public/customers", s.handleCreateCustomer)
	mux.HandleFunc("GET /api/public/customers/{id}", s.handleGetCustomer)
	mux.HandleFunc("POST /api/public/vehicles", s.handleCreateVehicle)
	mux.HandleFunc("POST /api/public/orders", s.handleCreateOrder)
	mux.HandleFunc("GET /api/public/orders/{order_number}", s.handleTrackOrder)
	mux.HandleFunc("GET /api/public/orders/{order_number}/invoice", s.handleGetInvoice)

	// Health.
	mux.HandleFunc("GET /api/health", s.handleHealth)

	// Workshop area. Login is public; every other shop route requires a bearer
	// token.
	mux.HandleFunc("POST /api/shop/login", s.handleShopLogin)
	mux.Handle("GET /api/shop/orders", s.requireSession(http.HandlerFunc(s.handleListOrders)))
	mux.Handle("GET /api/shop/orders/{order_number}", s.requireSession(http.HandlerFunc(s.handleGetShopOrder)))
	mux.Handle("POST /api/shop/orders/{order_number}/status", s.requireSession(http.HandlerFunc(s.handleSetOrderStatus)))
	mux.Handle("POST /api/shop/orders/{order_number}/items", s.requireSession(http.HandlerFunc(s.handleAddOrderItem)))
	mux.Handle("GET /api/shop/dashboard", s.requireSession(http.HandlerFunc(s.handleShopDashboard)))
}

// handleHealth reports that the process is serving.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
