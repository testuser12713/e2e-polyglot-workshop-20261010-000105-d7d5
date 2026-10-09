package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ctxKey is an unexported context key type so handlers only read the values
// this package sets.
type ctxKey int

const (
	ctxEmployeeID ctxKey = iota
	ctxEmployeeEmail
	ctxEmployeeName
)

// corsMiddleware applies the browser access policy (SPEC AC-30). The allowed
// origin is a single configured value; with credentials enabled the API never
// answers "*". A request carrying a different Origin is refused.
func corsMiddleware(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		originAllowed := origin == "" || origin == allowedOrigin

		if origin != "" && allowedOrigin != "" {
			w.Header().Add("Vary", "Origin")
		}
		if origin != "" && origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
		}

		if r.Method == http.MethodOptions {
			if !originAllowed {
				writeError(w, http.StatusForbidden, "origin_not_allowed", "Dieser Origin ist nicht erlaubt.")
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs one technical line per request. It never logs the
// query string or a request body, so no personal data (name, e-mail, phone,
// licence plate) can reach the log (SPEC AC-33).
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s -> %d (%s)", r.Method, safePath(r.URL.Path), rec.status, time.Since(start).Round(time.Millisecond))
	})
}

// recoveringMiddleware turns a panic into the unified 500 body. It sits inside
// the CORS middleware, so the error response still carries the CORS headers the
// browser needs to read it.
func recoveringMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic handling %s %s: %v", r.Method, safePath(r.URL.Path), rec)
				writeError(w, http.StatusInternalServerError, "internal_error", "Interner Fehler. Bitte später erneut versuchen.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// safePath strips any query string from a path before it is logged.
func safePath(path string) string {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		return path[:i]
	}
	return path
}

// requireSession enforces the workshop bearer token on the /api/shop/* routes
// (everything except login). A missing, unknown or expired token answers 401
// with the unified body, server-side, regardless of what the web app shows
// (SPEC AC-28).
func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			unauthorized(w, "Anmeldung erforderlich.")
			return
		}
		id, email, name, err := s.lookupSession(r.Context(), token)
		if err != nil {
			unauthorized(w, "Anmeldung erforderlich.")
			return
		}
		ctx := context.WithValue(r.Context(), ctxEmployeeID, id)
		ctx = context.WithValue(ctx, ctxEmployeeEmail, email)
		ctx = context.WithValue(ctx, ctxEmployeeName, name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// lookupSession resolves a bearer token to an employee. The query uses a bound
// parameter (SPEC AC-27).
func (s *Server) lookupSession(ctx context.Context, token string) (int64, string, string, error) {
	if s == nil || s.Store == nil || s.Store.Pool == nil {
		return 0, "", "", fmt.Errorf("store is not configured")
	}
	employee, err := s.Store.EmployeeBySessionToken(ctx, token)
	if err != nil {
		return 0, "", "", err
	}
	return employee.ID, employee.Email, employee.Name, nil
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
