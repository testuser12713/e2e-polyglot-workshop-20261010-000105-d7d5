package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func uniqueEmail(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("employee-%d@example.test", time.Now().UnixNano())
}

// deleteEmployee removes only the rows this test created. The sessions table
// cascades on delete, so no sibling table is touched.
func deleteEmployee(t *testing.T, st *Store, email string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := st.Pool.Exec(ctx, `DELETE FROM employees WHERE email = $1`, email); err != nil {
		t.Fatalf("cleanup employee: %v", err)
	}
}

// TestBootstrapEmployeeIsIdempotentAndKeepsChangedPassword proves AC-16: the
// first employee is created once, a second bootstrap does not duplicate it and
// does not overwrite a password that was changed afterwards.
func TestBootstrapEmployeeIsIdempotentAndKeepsChangedPassword(t *testing.T) {
	st, ctx := openTestStore(t)

	email := uniqueEmail(t)
	t.Cleanup(func() { deleteEmployee(t, st, email) })

	created, err := st.BootstrapEmployee(ctx, email, "Werkstatt-Team", "initial-pass")
	if err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	if !created {
		t.Fatal("first bootstrap should have created the employee")
	}

	// A restart with the same configuration must not create a second row.
	created, err = st.BootstrapEmployee(ctx, email, "Werkstatt-Team", "initial-pass")
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if created {
		t.Fatal("second bootstrap created a duplicate employee")
	}

	var count int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM employees WHERE email = $1`, email).Scan(&count); err != nil {
		t.Fatalf("count employees: %v", err)
	}
	if count != 1 {
		t.Fatalf("employee count = %d, want 1", count)
	}

	// Simulate a password changed by hand, then bootstrap again with the old one.
	changedHash, err := HashPassword("changed-by-hand")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE employees SET password_hash = $2 WHERE email = $1`, email, changedHash); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if _, err := st.BootstrapEmployee(ctx, email, "Werkstatt-Team", "initial-pass"); err != nil {
		t.Fatalf("third bootstrap: %v", err)
	}

	employee, hash, err := st.LookupEmployeeCredentials(ctx, email)
	if err != nil {
		t.Fatalf("LookupEmployeeCredentials: %v", err)
	}
	if employee.Email != email {
		t.Errorf("employee e-mail = %q, want %q", employee.Email, email)
	}
	if employee.Name != "Werkstatt-Team" {
		t.Errorf("employee name = %q, want %q", employee.Name, "Werkstatt-Team")
	}
	if !VerifyPassword(hash, "changed-by-hand") {
		t.Error("changed password was overwritten by the bootstrap")
	}
	if VerifyPassword(hash, "initial-pass") {
		t.Error("the old bootstrap password still works after a manual change")
	}
}

// TestLookupEmployeeUnknownEmail proves an unknown e-mail reports ErrNotFound,
// the signal the login handler turns into the generic 401.
func TestLookupEmployeeUnknownEmail(t *testing.T) {
	st, ctx := openTestStore(t)

	_, _, err := st.LookupEmployeeCredentials(ctx, uniqueEmail(t))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown e-mail error = %v, want ErrNotFound", err)
	}
}

// TestSessionRoundTrip proves a created session resolves to its employee and
// that an unknown token does not.
func TestSessionRoundTrip(t *testing.T) {
	st, ctx := openTestStore(t)

	email := uniqueEmail(t)
	t.Cleanup(func() { deleteEmployee(t, st, email) })

	if _, err := st.BootstrapEmployee(ctx, email, "Werkstatt-Team", "session-pass"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	employee, _, err := st.LookupEmployeeCredentials(ctx, email)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}

	token, expiresAt, err := st.CreateSession(ctx, employee.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if token == "" {
		t.Fatal("CreateSession returned an empty token")
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("session expiry %v is not in the future", expiresAt)
	}

	resolved, err := st.EmployeeBySessionToken(ctx, token)
	if err != nil {
		t.Fatalf("EmployeeBySessionToken: %v", err)
	}
	if resolved.ID != employee.ID {
		t.Errorf("resolved employee id = %d, want %d", resolved.ID, employee.ID)
	}

	if _, err := st.EmployeeBySessionToken(ctx, "not-a-real-token"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown token error = %v, want ErrNotFound", err)
	}
}
