// Package config reads the API's runtime configuration from the environment.
//
// Values are read lazily when Load runs, never at import time, and every
// required value is validated once so that a misconfigured process refuses to
// start with a message that names the missing variable instead of crashing on a
// bare traceback. Secrets are never shipped with a value: DATABASE_URL and
// VALKEY_URL must come from the environment (see RUN.json).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds every configuration value the API needs to boot and serve.
type Config struct {
	// DatabaseURL is the PostgreSQL connection string. Required.
	DatabaseURL string
	// ValkeyURL is the Valkey connection string used for the completed-orders
	// queue. Required.
	ValkeyURL string
	// APIPort is the TCP port the HTTP server binds. Non-secret default.
	APIPort string
	// CORSAllowedOrigin is the single browser origin allowed to call the API.
	CORSAllowedOrigin string
	// WorkshopHourlyRateCents is the labour rate used by the invoice worker
	// and by the public order request form. Non-secret default.
	WorkshopHourlyRateCents int
	// BootstrapEmployeeEmail is the e-mail of the first employee. Non-secret.
	BootstrapEmployeeEmail string
	// BootstrapEmployeePassword is the first employee's initial password. It is
	// a secret and is rolled per run by RUN.json, never stored here.
	BootstrapEmployeePassword string
	// BootstrapEmployeeName is the display name of the first employee.
	BootstrapEmployeeName string
	// LoginRateLimitPerMinute caps login attempts per client per minute.
	LoginRateLimitPerMinute int
	// WorkerPollIntervalMS is how often the invoice worker polls the queue.
	WorkerPollIntervalMS int
	// ViteAPIBaseURL is the API base URL the web app is built against.
	ViteAPIBaseURL string
}

// Default values for optional, non-secret configuration.
const (
	DefaultAPIPort                 = "8080"
	DefaultCORSAllowedOrigin       = "http://localhost:5173"
	DefaultWorkshopHourlyRateCents = 9500
	DefaultBootstrapEmployeeName   = "Werkstatt-Team"
	DefaultLoginRateLimitPerMinute = 10
	DefaultWorkerPollIntervalMS    = 1000
)

// Load reads the configuration from the environment and validates it. It
// returns an error naming every missing required variable. A value that is
// still an unresolved "${service:...}" placeholder (a sibling service that has
// not been declared yet) counts as unset, so the caller keeps its default.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:               resolveEnv("DATABASE_URL"),
		ValkeyURL:                 resolveEnv("VALKEY_URL"),
		APIPort:                   defaulted("API_PORT", DefaultAPIPort),
		CORSAllowedOrigin:         defaulted("CORS_ALLOWED_ORIGIN", DefaultCORSAllowedOrigin),
		WorkshopHourlyRateCents:   intOrDefault("WORKSHOP_HOURLY_RATE_CENTS", DefaultWorkshopHourlyRateCents),
		BootstrapEmployeeEmail:    strings.TrimSpace(os.Getenv("BOOTSTRAP_EMPLOYEE_EMAIL")),
		BootstrapEmployeePassword: os.Getenv("BOOTSTRAP_EMPLOYEE_PASSWORD"),
		BootstrapEmployeeName:     defaulted("BOOTSTRAP_EMPLOYEE_NAME", DefaultBootstrapEmployeeName),
		LoginRateLimitPerMinute:   intOrDefault("LOGIN_RATE_LIMIT_PER_MINUTE", DefaultLoginRateLimitPerMinute),
		WorkerPollIntervalMS:      intOrDefault("WORKER_POLL_INTERVAL_MS", DefaultWorkerPollIntervalMS),
		ViteAPIBaseURL:            resolveEnv("VITE_API_BASE_URL"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.ValkeyURL == "" {
		missing = append(missing, "VALKEY_URL")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"missing required configuration: %s (set it in the environment; RUN.json declares it)",
			strings.Join(missing, ", "),
		)
	}
	return cfg, nil
}

// resolveEnv returns the environment value, treating an unresolved
// "${service:...}" placeholder as unset.
func resolveEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if strings.HasPrefix(value, "${") {
		return ""
	}
	return value
}

func defaulted(key, fallback string) string {
	if value := resolveEnv(key); value != "" {
		return value
	}
	return fallback
}

func intOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" || strings.HasPrefix(value, "${") {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
