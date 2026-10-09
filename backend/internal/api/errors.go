package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// apiErrorBody is the single error envelope every failed API response uses:
//
//	{"error":{"code":"snake_case","message":"German user text"}}
type apiErrorBody struct {
	Error apiErrorDetail `json:"error"`
}

type apiErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

// writeError writes the unified error body.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiErrorBody{Error: apiErrorDetail{Code: code, Message: message}})
}

// Convenience constructors used by the feature handlers. The messages are
// German, human-readable text the web app can show directly (SPEC AC-22).

func badRequest(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, "bad_request", message)
}

func unprocessable(w http.ResponseWriter, message string) {
	writeError(w, http.StatusUnprocessableEntity, "validation_error", message)
}

func unauthorized(w http.ResponseWriter, message string) {
	writeError(w, http.StatusUnauthorized, "unauthorized", message)
}

func notFound(w http.ResponseWriter, message string) {
	writeError(w, http.StatusNotFound, "not_found", message)
}

func conflict(w http.ResponseWriter, message string) {
	writeError(w, http.StatusConflict, "conflict", message)
}

func tooManyRequests(w http.ResponseWriter, message string) {
	writeError(w, http.StatusTooManyRequests, "rate_limited", message)
}

func internalError(w http.ResponseWriter, message string) {
	writeError(w, http.StatusInternalServerError, "internal_error", message)
}

// notImplemented marks a route whose owning ticket has not landed yet. It
// answers 501, never 500, so a mid-sprint build stays readable.
func notImplemented(w http.ResponseWriter, feature string) {
	writeError(w, http.StatusNotImplemented, "not_implemented", feature+" ist noch nicht implementiert.")
}
