package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"workshop/backend/internal/store"
)

// customerInput is the JSON body of POST /api/public/customers.
type customerInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// handleCreateCustomer serves POST /api/public/customers. It creates a customer
// and answers 201 with the new id; an already used e-mail answers 409 and an
// invalid body answers 422 (SPEC AC-01).
func (s *Server) handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		internalError(w, "Der Kunde konnte nicht angelegt werden.")
		return
	}

	var in customerInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		unprocessable(w, "Die Anfrage ist kein gültiges JSON.")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)
	in.Phone = strings.TrimSpace(in.Phone)

	if message := validateCustomerInput(in); message != "" {
		unprocessable(w, message)
		return
	}

	customer, err := s.Store.CreateCustomer(r.Context(), in.Name, in.Email, in.Phone)
	if err != nil {
		if errors.Is(err, store.ErrCustomerEmailTaken) {
			conflict(w, "Diese E-Mail-Adresse ist bereits registriert.")
			return
		}
		logStoreFailure("create customer", err)
		internalError(w, "Der Kunde konnte nicht angelegt werden.")
		return
	}
	writeJSON(w, http.StatusCreated, customer)
}

// handleGetCustomer serves GET /api/public/customers/{id}. It returns the
// customer or 404 when no such customer exists (SPEC AC-01).
func (s *Server) handleGetCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		notFound(w, "Kunde nicht gefunden.")
		return
	}
	if s.Store == nil {
		internalError(w, "Der Kunde konnte nicht geladen werden.")
		return
	}

	customer, err := s.Store.GetCustomerByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrCustomerNotFound) {
			notFound(w, "Kunde nicht gefunden.")
			return
		}
		logStoreFailure("get customer", err)
		internalError(w, "Der Kunde konnte nicht geladen werden.")
		return
	}
	writeJSON(w, http.StatusOK, customer)
}

// validateCustomerInput returns a German message for the first missing or
// invalid field, or an empty string when the input is valid. An empty form is
// an error here because the API is the last line of defence, not the UI.
func validateCustomerInput(in customerInput) string {
	if in.Name == "" {
		return "Bitte einen Namen angeben."
	}
	if in.Email == "" {
		return "Bitte eine E-Mail-Adresse angeben."
	}
	if !validCustomerEmail(in.Email) {
		return "Bitte eine gültige E-Mail-Adresse angeben."
	}
	if in.Phone == "" {
		return "Bitte eine Telefonnummer angeben."
	}
	return ""
}

// validCustomerEmail reports whether s is a bare, parseable e-mail address.
func validCustomerEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Address == s
}

// logStoreFailure logs a failed database operation by its operation name and,
// for a pgx error, its SQLSTATE code only. It never logs field values or the
// raw error, so no personal data (name, e-mail, phone, plate) can reach the log
// (SPEC AC-33).
func logStoreFailure(operation string, err error) {
	code := fmt.Sprintf("%T", err)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		code = pgErr.Code
	}
	log.Printf("store: %s failed (db code %s)", operation, code)
}
