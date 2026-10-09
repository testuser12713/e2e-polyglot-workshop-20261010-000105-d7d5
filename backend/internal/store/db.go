// Package store owns the PostgreSQL access layer. It opens one pgx connection
// pool per process, applies the embedded schema at startup and exposes it to
// the feature files (customers, vehicles, orders, ...) of the same package.
package store

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// Store wraps the pgx connection pool. Feature files add methods on *Store.
type Store struct {
	Pool *pgxpool.Pool
}

// Open connects to PostgreSQL, verifies the connection and applies the schema.
// There is deliberately no SQLite or in-memory fallback: when the database is
// unavailable the caller gets an error and the process reports it instead of
// starting against a substitute (SPEC AC-24).
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set; declare it in RUN.json and export it before starting the API")
	}

	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("PostgreSQL is not reachable: %w", err)
	}

	if err := applySchema(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &Store{Pool: pool}, nil
}

// schemaLockKey is an application-wide advisory lock key that serializes schema
// creation across processes. Concurrent CREATE TABLE IF NOT EXISTS statements
// can still collide on pg_class (SQLSTATE 23505), so the schema is applied
// while holding a transaction-scoped advisory lock.
const schemaLockKey int64 = 0x776f726b73686f70 // "workshop" as ASCII bytes

// applySchema runs the embedded schema in one transaction guarded by an
// advisory lock, so several callers opening the database at the same time
// (parallel test packages, multiple API replicas) never race on DDL.
func applySchema(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("apply database schema: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, schemaLockKey); err != nil {
		return fmt.Errorf("apply database schema: lock: %w", err)
	}
	if _, err := tx.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply database schema: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("apply database schema: commit: %w", err)
	}
	return nil
}

// Ping verifies that the pool can still reach PostgreSQL.
func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("store is not configured")
	}
	return s.Pool.Ping(ctx)
}

// Close releases the connection pool.
func (s *Store) Close() {
	if s != nil && s.Pool != nil {
		s.Pool.Close()
	}
}
