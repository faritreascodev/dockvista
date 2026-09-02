package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

const sessionCookieName = "dockvista_session"

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

// authService is the subset of *service.AuthService the HTTP layer depends
// on (consumer-defined interface, same decoupling pattern as
// containerService).
type authService interface {
	IsInitialized() (bool, error)
	Setup(username, password string) error
	Login(username, password string) (token string, expiresAt time.Time, err error)
	VerifySession(token string) (username string, err error)
}

type authHandlers struct {
	svc authService
}

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *authHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	initialized, err := h.svc.IsInitialized()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read auth state")
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]bool{"initialized": initialized})
}

func (h *authHandlers) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !usernamePattern.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "username must be 3-32 characters (letters, digits, _.-)")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	if err := h.svc.Setup(req.Username, req.Password); err != nil {
		if errors.Is(err, domain.ErrAlreadyInitialized) {
			writeError(w, http.StatusConflict, "an admin account already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create admin account")
		return
	}
	httpjson.Write(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *authHandlers) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	token, expiresAt, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}

	http.SetCookie(w, sessionCookie(token, expiresAt, r))
	httpjson.Write(w, http.StatusOK, map[string]string{"username": req.Username})
}

func (h *authHandlers) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, sessionCookie("", time.Unix(0, 0), r))
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandlers) handleMe(w http.ResponseWriter, r *http.Request) {
	username, _ := r.Context().Value(usernameCtxKey).(string)
	httpjson.Write(w, http.StatusOK, map[string]string{"username": username})
}

func sessionCookie(token string, expiresAt time.Time, r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	}
}
