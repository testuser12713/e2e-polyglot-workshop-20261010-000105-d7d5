// Command api is the Kfz-Werkstatt HTTP service. It reads its configuration
// from the environment, connects to PostgreSQL and Valkey, applies the database
// schema and serves the workshop API.
//
// It binds the port given in API_PORT (set by RUN.json) and prints the port it
// listens on so the start contract can be read from the log.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"workshop/backend/internal/api"
	"workshop/backend/internal/config"
	"workshop/backend/internal/queue"
	"workshop/backend/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connectCtx, cancelConnect := context.WithTimeout(ctx, 15*time.Second)
	defer cancelConnect()

	// PostgreSQL is required. Open reports the error instead of falling back to
	// SQLite or an in-memory substitute (SPEC AC-24).
	st, err := store.Open(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	q, err := queue.New(cfg.ValkeyURL)
	if err != nil {
		return err
	}
	defer q.Close()
	if err := q.Ping(connectCtx); err != nil {
		return fmt.Errorf("Valkey is not reachable: %w", err)
	}

	server := api.NewServer(st, cfg, q)
	httpServer := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("workshop API listening on port %s", cfg.APIPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Print("shutdown signal received")
	case err := <-serveErr:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
