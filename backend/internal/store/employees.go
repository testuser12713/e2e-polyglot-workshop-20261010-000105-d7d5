package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound signals that a lookup matched no row. Handlers turn it into a 404
// or, for the login e-mail, a generic 401.
var ErrNotFound = errors.New("not found")

// Employee is a workshop employee as returned to clients. It deliberately
// carries no password hash: the hash never leaves this package except through
// LookupEmployeeCredentials.
type Employee struct {
	ID    int64
	Email string
	Name  string
}

// BootstrapEmployee creates the first employee from configuration. It is
// idempotent: when an employee with the same e-mail already exists the row is
// left completely untouched, so a restart creates no duplicate and never
// overwrites a password changed by hand (SPEC AC-16). It reports whether a new
// row was inserted.
func (s *Store) BootstrapEmployee(ctx context.Context, email, name, password string) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, fmt.Errorf("store is not configured")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return false, fmt.Errorf("bootstrap employee e-mail and password must both be set")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return false, fmt.Errorf("hash bootstrap password: %w", err)
	}

	const query = `
		INSERT INTO employees (email, password_hash, name)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO NOTHING`
	tag, err := s.Pool.Exec(ctx, query, email, hash, name)
	if err != nil {
		return false, fmt.Errorf("insert bootstrap employee: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// LookupEmployeeCredentials returns the employee and its stored password hash
// for the login check. Unknown e-mails report ErrNotFound; the caller answers
// with the same generic body as a wrong password so the response never reveals
// which part was wrong (SPEC AC-15).
func (s *Store) LookupEmployeeCredentials(ctx context.Context, email string) (Employee, string, error) {
	if s == nil || s.Pool == nil {
		return Employee{}, "", fmt.Errorf("store is not configured")
	}

	email = strings.ToLower(strings.TrimSpace(email))
	const query = `SELECT id, email, name, password_hash FROM employees WHERE email = $1`
	var (
		employee Employee
		hash     string
	)
	err := s.Pool.QueryRow(ctx, query, email).Scan(&employee.ID, &employee.Email, &employee.Name, &hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Employee{}, "", ErrNotFound
		}
		return Employee{}, "", fmt.Errorf("lookup employee: %w", err)
	}
	return employee, hash, nil
}

// EmployeeByID returns one employee by primary key. It exists for callers that
// already resolved a session and want the current record.
func (s *Store) EmployeeByID(ctx context.Context, id int64) (Employee, error) {
	if s == nil || s.Pool == nil {
		return Employee{}, fmt.Errorf("store is not configured")
	}

	const query = `SELECT id, email, name FROM employees WHERE id = $1`
	var employee Employee
	err := s.Pool.QueryRow(ctx, query, id).Scan(&employee.ID, &employee.Email, &employee.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Employee{}, ErrNotFound
		}
		return Employee{}, fmt.Errorf("lookup employee by id: %w", err)
	}
	return employee, nil
}
