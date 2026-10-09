package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"workshop/backend/internal/config"
	"workshop/backend/internal/store"
)

// loginRequest is the body of POST /api/shop/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginResponse is the successful login body: an opaque bearer token and the
// employee it belongs to.
type loginResponse struct {
	Token    string          `json:"token"`
	Employee employeePayload `json:"employee"`
}

// employeePayload mirrors the shared Employee type without the password hash.
type employeePayload struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// invalidCredentialsMessage is deliberately identical for an unknown e-mail and
// a wrong password, so a failed login never reveals which part was wrong
// (SPEC AC-15).
const invalidCredentialsMessage = "E-Mail oder Passwort ist falsch."

// loginLimiter counts failed login attempts per client over a sliding one-minute
// window. Once the configured number of failures is reached the client gets 429
// for the rest of the window (SPEC AC-29).
type loginLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{failures: make(map[string][]time.Time)}
}

// blocked reports whether the client already reached the allowed failure count
// in the last minute. It also prunes attempts that fell out of the window.
func (l *loginLimiter) blocked(client string, limit int, now time.Time) bool {
	if l == nil || limit <= 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-time.Minute)
	kept := l.failures[client][:0]
	for _, at := range l.failures[client] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(l.failures, client)
	} else {
		l.failures[client] = kept
	}
	return len(kept) >= limit
}

// recordFailure notes one failed attempt for the client.
func (l *loginLimiter) recordFailure(client string, now time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[client] = append(l.failures[client], now)
}

// reset drops all recorded failures. Used by tests to isolate the process-wide
// limiter state.
func (l *loginLimiter) reset() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures = make(map[string][]time.Time)
}

// handleShopLogin serves POST /api/shop/login: it verifies the stored password
// hash, opens a server-side session and returns a bearer token. Wrong
// credentials answer 401 without a hint; too many failures answer 429. Owned by
// "Implement workshop authentication with hashed passwords, sessions and rate
// limit".
func (s *Server) handleShopLogin(w http.ResponseWriter, r *http.Request) {
	client := clientKey(r)
	limit := s.loginRateLimit()

	if s != nil && s.loginLimiter != nil && s.loginLimiter.blocked(client, limit, time.Now()) {
		tooManyRequests(w, "Zu viele Anmeldeversuche. Bitte später erneut versuchen.")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		badRequest(w, "Ungültige Anfrage.")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", invalidCredentialsMessage)
		return
	}

	if s == nil || s.Store == nil {
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}

	employee, hash, err := s.Store.LookupEmployeeCredentials(r.Context(), email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.recordLoginFailure(client)
			unauthorized(w, invalidCredentialsMessage)
			return
		}
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}

	if !store.VerifyPassword(hash, req.Password) {
		s.recordLoginFailure(client)
		unauthorized(w, invalidCredentialsMessage)
		return
	}

	token, _, err := s.Store.CreateSession(r.Context(), employee.ID)
	if err != nil {
		internalError(w, "Interner Fehler. Bitte später erneut versuchen.")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		Token: token,
		Employee: employeePayload{
			ID:    employee.ID,
			Email: employee.Email,
			Name:  employee.Name,
		},
	})
}

func (s *Server) recordLoginFailure(client string) {
	if s != nil && s.loginLimiter != nil {
		s.loginLimiter.recordFailure(client, time.Now())
	}
}

// loginRateLimit returns the configured cap, falling back to the default so the
// endpoint always has a working bound.
func (s *Server) loginRateLimit() int {
	if s != nil && s.Config != nil && s.Config.LoginRateLimitPerMinute > 0 {
		return s.Config.LoginRateLimitPerMinute
	}
	return config.DefaultLoginRateLimitPerMinute
}

// clientKey identifies the requesting client for the rate limit. It uses the
// connecting host only, never a client-supplied header, so a forged X-Forwarded
// value cannot reset the counter.
func clientKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
