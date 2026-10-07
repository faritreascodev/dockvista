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
	case errors.Is(err, domain.ErrNotRunning):
		writeError(w, http.StatusConflict, "container is not running")
	case errors.Is(err, domain.ErrNoShell):
		writeError(w, http.StatusConflict, "this image has no shell to list files; use Changes or a known path")
	case errors.Is(err, domain.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "file exceeds the 32 MB download limit")
	case errors.Is(err, domain.ErrIsDirectory):
		writeError(w, http.StatusBadRequest, "path is a directory")
	case errors.Is(err, domain.ErrEngineTimeout):
		writeError(w, http.StatusGatewayTimeout, "docker engine timed out")
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, domain.ErrUserExists):
		writeError(w, http.StatusConflict, "username already exists")
	case errors.Is(err, domain.ErrLastAdmin):
		writeError(w, http.StatusConflict, "cannot remove or demote the last admin")
	case errors.Is(err, domain.ErrInviteInvalid):
		writeError(w, http.StatusNotFound, "invite is invalid or expired")
	case errors.Is(err, domain.ErrSSHUnsupported):
		writeError(w, http.StatusBadRequest, "SSH endpoints are not available yet")
	case errors.Is(err, domain.ErrLocalProtected):
		writeError(w, http.StatusConflict, "the local environment cannot be removed")
	case errors.Is(err, domain.ErrBuildUnsupported):
		writeError(w, http.StatusBadRequest, "compose build is not supported; set image: on every service")
	case errors.Is(err, domain.ErrBindOutsideStack):
		writeError(w, http.StatusBadRequest, "bind mounts must stay inside the stack workspace")
	default:
		writeError(w, http.StatusBadGateway, "docker engine error")
	}
}
