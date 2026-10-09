package store

import (
	"context"
	"fmt"
)

// DashboardStats holds the three metrics served by GET /api/shop/dashboard.
//
// All three values are whole numbers, and the two time-based ones use the UTC
// calendar: "today" is the current UTC date, and "this month" is the current
// UTC month (SPEC AC-21).
type DashboardStats struct {
	// OpenOrders is the number of orders that have not been picked up yet,
	// i.e. every status except 'picked_up'.
	OpenOrders int64
	// FinishedToday is the number of status-log entries that moved an order to
	// 'done' on the current UTC date.
	FinishedToday int64
	// RevenueMonthCents is the sum of the gross amounts of all invoices created
	// in the current UTC month, in integer cents.
	RevenueMonthCents int64
}

// DashboardStats reads the three dashboard metrics in one transaction so they
// describe the same database snapshot. It takes no user input and computes its
// time boundaries explicitly in UTC, never in the server's session time zone
// (SPEC AC-21).
func (s *Store) DashboardStats(ctx context.Context) (DashboardStats, error) {
	var stats DashboardStats

	if s == nil || s.Pool == nil {
		return stats, fmt.Errorf("store is not configured")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return stats, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const openOrdersQuery = `SELECT count(*) FROM orders WHERE status <> 'picked_up'`
	if err := tx.QueryRow(ctx, openOrdersQuery).Scan(&stats.OpenOrders); err != nil {
		return stats, err
	}

	const finishedTodayQuery = `
		SELECT count(*)
		FROM status_log
		WHERE to_status = 'done'
		  AND (at AT TIME ZONE 'UTC')::date = (now() AT TIME ZONE 'UTC')::date`
	if err := tx.QueryRow(ctx, finishedTodayQuery).Scan(&stats.FinishedToday); err != nil {
		return stats, err
	}

	const revenueQuery = `
		SELECT COALESCE(SUM(gross_cents), 0)
		FROM invoices
		WHERE created_at >= date_trunc('month', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'
		  AND created_at < (date_trunc('month', now() AT TIME ZONE 'UTC') + interval '1 month') AT TIME ZONE 'UTC'`
	if err := tx.QueryRow(ctx, revenueQuery).Scan(&stats.RevenueMonthCents); err != nil {
		return stats, err
	}

	if err := tx.Commit(ctx); err != nil {
		return stats, err
	}
	return stats, nil
}
