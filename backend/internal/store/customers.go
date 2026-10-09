package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgCodeUniqueViolation is the PostgreSQL SQLSTATE code for a unique-constraint
// violation. A create that hits it means the e-mail or plate is already taken.
const pgCodeUniqueViolation = "23505"

// ErrCustomerNotFound reports that no customer row exists for the given id.
var ErrCustomerNotFound = errors.New("customer not found")

// ErrCustomerEmailTaken reports that a customer with the same e-mail already
// exists (SPEC AC-01). The API answers 409 for it.
var ErrCustomerEmailTaken = errors.New("customer e-mail already registered")

// Customer is a workshop customer. It is the shared shape returned by the
// public customer endpoints (Customer{id,name,email,phone}).
type Customer struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// CreateCustomer inserts a customer and returns it with its assigned id. Every
// value travels as a bound parameter ($1..$3); no SQL is built from user input
// (SPEC AC-27). A duplicate e-mail is reported as ErrCustomerEmailTaken.
func (s *Store) CreateCustomer(ctx context.Context, name, email, phone string) (Customer, error) {
	const query = `
		INSERT INTO customers (name, email, phone)
		VALUES ($1, $2, $3)
		RETURNING id, name, email, phone`

	var customer Customer
	err := s.Pool.QueryRow(ctx, query, name, email, phone).
		Scan(&customer.ID, &customer.Name, &customer.Email, &customer.Phone)
	if err != nil {
		if isPgUniqueViolation(err) {
			return Customer{}, ErrCustomerEmailTaken
		}
		return Customer{}, fmt.Errorf("create customer: %w", err)
	}
	return customer, nil
}

// GetCustomerByID loads a customer by its id. A missing row is reported as
// ErrCustomerNotFound; the query uses a bound parameter (SPEC AC-27).
func (s *Store) GetCustomerByID(ctx context.Context, id int64) (Customer, error) {
	const query = `SELECT id, name, email, phone FROM customers WHERE id = $1`

	var customer Customer
	err := s.Pool.QueryRow(ctx, query, id).
		Scan(&customer.ID, &customer.Name, &customer.Email, &customer.Phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Customer{}, ErrCustomerNotFound
		}
		return Customer{}, fmt.Errorf("get customer %d: %w", id, err)
	}
	return customer, nil
}

// isPgUniqueViolation reports whether err is a PostgreSQL unique-constraint
// violation (SQLSTATE 23505), used to turn a duplicate e-mail or plate into a
// 409 instead of a 500.
func isPgUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgCodeUniqueViolation
}
