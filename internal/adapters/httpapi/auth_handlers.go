package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

const sessionCookieName = "dockvista_session"

// authService is the subset of *service.AuthService the HTTP layer depends
// on (consumer-defined interface, same decoupling pattern as
// containerService).
type authService interface {
	IsInitialized() (bool, error)
	Setup(username, password, setupToken string) error
	Login(username, password string) (token string, expiresAt time.Time, err error)
	VerifySession(token string) (domain.Principal, error)
	RevokeSessions(username string) error
	ChangePassword(username, current, next string) (token string, expiresAt time.Time, err error)
	ListUsers() ([]domain.User, error)
	ListInvites() ([]domain.Invite, error)
	CreateInvite(actor domain.Principal, role domain.Role) (domain.Invite, string, error)
	PeekInvite(token string) (domain.Invite, error)
	AcceptInvite(token, username, password string) error
	DeleteUser(actor domain.Principal, username string) error
	SetRole(actor domain.Principal, username string, role domain.Role) error
}

type authHandlers struct {
	svc          authService
	cookieSecure bool
	readOnly     bool
	guard        *loginGuard
	audit        Auditor
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type meDTO struct {
	Username         string `json:"username"`
	Role             string `json:"role"`
	ReadOnly         bool   `json:"readOnly"`
	InstanceReadOnly bool   `json:"instanceReadOnly"`
}

type credentialsRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	SetupToken string `json:"setupToken"`
}

func (h *authHandlers) toMeDTO(p domain.Principal) meDTO {
	role := p.Role
	if role == "" {
		role = domain.RoleAdmin
	}
	return meDTO{
		Username:         p.Username,
		Role:             string(role),
		ReadOnly:         h.readOnly || !role.CanMutate(),
		InstanceReadOnly: h.readOnly,
	}
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
	if err := domain.ValidateUsername(req.Username); err != nil {
		writeError(w, http.StatusBadRequest, "username must be 3-32 characters (letters, digits, _.-)")
		return
	}
	if err := domain.ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Setup(req.Username, req.Password, req.SetupToken); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "invalid setup token")
			return
		}
		if errors.Is(err, domain.ErrAlreadyInitialized) {
			writeError(w, http.StatusConflict, "an admin account already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create admin account")
		return
	}
	h.record("auth.setup", r, map[string]string{"user": req.Username})
	httpjson.Write(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *authHandlers) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip := clientKey(r)
	if h.guard != nil && h.guard.blocked(ip, req.Username) {
		h.record("auth.lockout", r, map[string]string{"user": req.Username})
		writeError(w, http.StatusTooManyRequests, "too many failed logins, try again later")
		return
	}

	token, expiresAt, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			if h.guard != nil && h.guard.fail(ip, req.Username) {
				h.record("auth.lockout", r, map[string]string{"user": req.Username})
			} else {
				h.record("auth.login.fail", r, map[string]string{"user": req.Username})
			}
			writeError(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	if h.guard != nil {
		h.guard.success(ip, req.Username)
	}

	p, err := h.svc.VerifySession(token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}

	h.record("auth.login.ok", r, map[string]string{"user": p.Username, "role": string(p.Role)})
	http.SetCookie(w, sessionCookie(token, expiresAt, r, h.cookieSecure))
	httpjson.Write(w, http.StatusOK, h.toMeDTO(p))
}

func (h *authHandlers) handleLogout(w http.ResponseWriter, r *http.Request) {
	username, _ := r.Context().Value(usernameCtxKey).(string)
	if err := h.svc.RevokeSessions(username); err != nil {
		writeError(w, http.StatusInternalServerError, "could not revoke session")
		return
	}
	h.record("auth.logout", r, nil)
	http.SetCookie(w, sessionCookie("", time.Unix(0, 0), r, h.cookieSecure))
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandlers) handleMe(w http.ResponseWriter, r *http.Request) {
	httpjson.Write(w, http.StatusOK, h.toMeDTO(principalFrom(r.Context())))
}

func (h *authHandlers) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p := principalFrom(r.Context())

	token, expiresAt, err := h.svc.ChangePassword(p.Username, req.CurrentPassword, req.NewPassword)
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		writeError(w, http.StatusForbidden, "current password is incorrect")
		return
	case errors.Is(err, domain.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not change password")
		return
	}

	h.record("auth.password", r, nil)
	http.SetCookie(w, sessionCookie(token, expiresAt, r, h.cookieSecure))
	httpjson.Write(w, http.StatusOK, h.toMeDTO(p))
}

func sessionCookie(token string, expiresAt time.Time, r *http.Request, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secure || r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	}
}
