package api

import (
	"log"
	"net/http"
)

// shopDashboardResponse is the JSON body of GET /api/shop/dashboard.
type shopDashboardResponse struct {
	OpenOrders        int64 `json:"open_orders"`
	FinishedToday     int64 `json:"finished_today"`
	RevenueMonthCents int64 `json:"revenue_month_cents"`
}

// handleShopDashboard serves GET /api/shop/dashboard. It reports the number of
// still-open orders, the number of orders finished today (UTC) and the revenue
// of the current UTC month, all read from PostgreSQL. The route is wrapped by
// requireSession, so only an authenticated employee reaches this handler
// (SPEC AC-21, AC-28).
func (s *Server) handleShopDashboard(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.Store == nil || s.Store.Pool == nil {
		internalError(w, "Die Datenbank ist nicht konfiguriert.")
		return
	}

	stats, err := s.Store.DashboardStats(r.Context())
	if err != nil {
		log.Printf("dashboard stats: %v", err)
		internalError(w, "Das Dashboard konnte nicht geladen werden.")
		return
	}

	writeJSON(w, http.StatusOK, shopDashboardResponse{
		OpenOrders:        stats.OpenOrders,
		FinishedToday:     stats.FinishedToday,
		RevenueMonthCents: stats.RevenueMonthCents,
	})
}
