package store

import (
	"context"
	"os"
	"testing"
)

// packageIntegrationLockKey serializes the store and api test binaries against
// each other. `go test ./...` runs the two packages in parallel against the
// same PostgreSQL instance, and the dashboard metric tests measure global
// aggregates over the orders, status_log and invoices tables. While one package
// holds this session-level advisory lock, the other waits, so an order-mutating
// test of one package (create the rows, then clean them up) can never overlap
// another package's dashboard measurement. The api package uses the same key.
const packageIntegrationLockKey int64 = 882374100999

// TestMain holds the cross-package advisory lock for the whole package run and
// releases it before the process exits. It stays transparent when DATABASE_URL
// is not set: the individual tests then skip themselves.
func TestMain(m *testing.M) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		os.Exit(m.Run())
	}

	ctx := context.Background()
	st, err := Open(ctx, databaseURL)
	if err != nil {
		// Let the individual tests report the connection failure themselves.
		os.Exit(m.Run())
	}

	conn, err := st.Pool.Acquire(ctx)
	if err != nil {
		st.Close()
		os.Exit(m.Run())
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, packageIntegrationLockKey); err != nil {
		conn.Release()
		st.Close()
		os.Exit(m.Run())
	}

	code := m.Run()

	_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, packageIntegrationLockKey)
	conn.Release()
	st.Close()
	os.Exit(code)
}
