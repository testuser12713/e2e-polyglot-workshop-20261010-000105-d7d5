package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SessionTTL is how long a session token stays valid. Sessions are validated
// server-side on every workshop request; an expired row no longer resolves
// (SPEC AC-28).
const SessionTTL = 24 * time.Hour

// CreateSession opens a server-side session row for the employee and returns
// the opaque bearer token together with its expiry. The token is 32 random
// bytes, so it cannot be guessed.
func (s *Store) CreateSession(ctx context.Context, employeeID int64) (string, time.Time, error) {
	if s == nil || s.Pool == nil {
		return "", time.Time{}, fmt.Errorf("store is not configured")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generate session token: %w", err)
	}
	token := hex.EncodeToString(raw)
	expiresAt := time.Now().UTC().Add(SessionTTL)

	const query = `INSERT INTO sessions (token, employee_id, expires_at) VALUES ($1, $2, $3)`
	if _, err := s.Pool.Exec(ctx, query, token, employeeID, expiresAt); err != nil {
		return "", time.Time{}, fmt.Errorf("insert session: %w", err)
	}
	return token, expiresAt, nil
}

// EmployeeBySessionToken resolves a bearer token to the employee behind it.
// Only an unexpired session resolves; unknown or expired tokens report
// ErrNotFound.
func (s *Store) EmployeeBySessionToken(ctx context.Context, token string) (Employee, error) {
	if s == nil || s.Pool == nil {
		return Employee{}, fmt.Errorf("store is not configured")
	}

	const query = `
		SELECT e.id, e.email, e.name
		FROM sessions s
		JOIN employees e ON e.id = s.employee_id
		WHERE s.token = $1 AND s.expires_at > now()`
	var employee Employee
	err := s.Pool.QueryRow(ctx, query, token).Scan(&employee.ID, &employee.Email, &employee.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Employee{}, ErrNotFound
		}
		return Employee{}, fmt.Errorf("lookup session: %w", err)
	}
	return employee, nil
}

// DeleteSession removes a session row. It is exposed for tests and for a future
// logout, and is safe to call for an unknown token.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("store is not configured")
	}
	if _, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
