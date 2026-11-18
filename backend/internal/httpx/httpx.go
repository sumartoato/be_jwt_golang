// Package httpx holds the small JSON helpers shared by the API handlers.
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
)

// FieldErrors maps a request field to a message, e.g. {"code": "Code is required"}.
type FieldErrors map[string]string

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// WriteValidation answers 400 with per-field messages.
func WriteValidation(w http.ResponseWriter, errs FieldErrors) {
	WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "errors": errs})
}

// InternalError logs err and answers a generic 500, so details never reach the client.
func InternalError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	WriteError(w, http.StatusInternalServerError, "internal server error")
}

// DecodeJSON reads a JSON body of at most 1 MiB into dst, answering 400 on failure.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}
