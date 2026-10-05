package httpapi

import (
	"errors"
	"net/http"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

func writeError(w http.ResponseWriter, status int, message string) {
	httpjson.WriteError(w, status, message)
}

// writeServiceError maps a service-layer error to an HTTP status by
// checking it against domain sentinels, instead of guessing from the error
// string. Falls back to 500 for anything unrecognized.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid input")
	case errors.Is(err, domain.ErrEngineTimeout):
		writeError(w, http.StatusGatewayTimeout, "docker engine timed out")
	default:
		writeError(w, http.StatusBadGateway, "docker engine error")
	}
}
