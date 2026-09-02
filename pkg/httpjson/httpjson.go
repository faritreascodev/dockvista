// Package httpjson provides tiny, dependency-free helpers for writing JSON
// HTTP responses. It intentionally has no dependency on anything under
// internal/, so it could be extracted into its own module if useful
// elsewhere.
package httpjson

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Write encodes v as JSON with the given status code. Encoding errors are
// logged (the response is already committed by then, so there's nothing
// more useful to do with them).
func Write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("httpjson: encode response", "error", err)
	}
}

// ErrorResponse is the JSON shape returned for every error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteError writes a JSON error body with the given status code.
func WriteError(w http.ResponseWriter, status int, message string) {
	Write(w, status, ErrorResponse{Error: message})
}
