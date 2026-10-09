package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestOpenAppliesSchema runs against a real PostgreSQL instance (SPEC AC-25)
// and checks that Open connects and creates every table the spec needs. It is
// skipped where DATABASE_URL is not provided.
func TestOpenAppliesSchema(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	st, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	tables := []string{
		"customers", "vehicles", "orders", "order_items", "status_log",
		"invoices", "invoice_items", "employees", "sessions", "outbox",
	}
	for _, table := range tables {
		var exists bool
		const query = `SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`
		if err := st.Pool.QueryRow(ctx, query, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %q was not created by the schema", table)
		}
	}
}

// TestOpenRequiresDatabaseURL proves there is no fallback: a missing
// DATABASE_URL is an error.
func TestOpenRequiresDatabaseURL(t *testing.T) {
	if _, err := Open(context.Background(), ""); err == nil {
		t.Fatal("Open with an empty DATABASE_URL should fail, got nil")
	}
}

// TestOpenRejectsUnreachableDatabase proves that an unreachable PostgreSQL
// reports an error instead of silently using a substitute (SPEC AC-24).
func TestOpenRejectsUnreachableDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	_, err := Open(ctx, "postgres://workshop:workshop@127.0.0.1:59999/workshop?sslmode=disable&connect_timeout=3")
	if err == nil {
		t.Fatal("Open against an unreachable database should fail, got nil")
	}
}
