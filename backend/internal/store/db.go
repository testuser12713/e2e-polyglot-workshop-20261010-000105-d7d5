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

	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply database schema: %w", err)
	}

	return &Store{Pool: pool}, nil
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
