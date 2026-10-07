package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

type userDTO struct {
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type inviteDTO struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	CreatedBy string    `json:"createdBy"`
	ExpiresAt time.Time `json:"expiresAt"`
	Used      bool      `json:"used"`
}

type createdInviteDTO struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expiresAt"`
	Token     string    `json:"token"`
	Path      string    `json:"path"`
}

type createInviteRequest struct {
	Role string `json:"role"`
}

type acceptInviteRequest struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type setRoleRequest struct {
	Role string `json:"role"`
}

func toUserDTO(u domain.User) userDTO {
	role := u.Role
	if role == "" {
		role = domain.RoleAdmin
	}
	return userDTO{Username: u.Username, Role: string(role), CreatedAt: u.CreatedAt}
}

func toInviteDTO(inv domain.Invite) inviteDTO {
	return inviteDTO{
		ID:        inv.ID,
		Role:      string(inv.Role),
		CreatedBy: inv.CreatedBy,
		ExpiresAt: inv.ExpiresAt,
		Used:      inv.Used(),
	}
}

func (h *authHandlers) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list users")
		return
	}
	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		out = append(out, toUserDTO(u))
	}
	httpjson.Write(w, http.StatusOK, out)
}

func (h *authHandlers) handleListInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := h.svc.ListInvites()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list invites")
		return
	}
	out := make([]inviteDTO, 0, len(invites))
	for _, inv := range invites {
		out = append(out, toInviteDTO(inv))
	}
	httpjson.Write(w, http.StatusOK, out)
}

func (h *authHandlers) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	var req createInviteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	role, err := domain.ParseRole(req.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, "role must be admin, operator, or viewer")
		return
	}
	inv, token, err := h.svc.CreateInvite(principalFrom(r.Context()), role)
	if err != nil {
		writeUserError(w, err)
		return
	}
	h.record("auth.invite", r, map[string]string{"role": string(role), "invite": inv.ID})
	httpjson.Write(w, http.StatusCreated, createdInviteDTO{
		ID:        inv.ID,
		Role:      string(inv.Role),
		ExpiresAt: inv.ExpiresAt,
		Token:     token,
		Path:      "/invite?token=" + token,
	})
}

func (h *authHandlers) handlePeekInvite(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	inv, err := h.svc.PeekInvite(token)
	if err != nil {
		writeUserError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]any{
		"role":      string(inv.Role),
		"expiresAt": inv.ExpiresAt,
	})
}

func (h *authHandlers) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req acceptInviteRequest
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
	if err := h.svc.AcceptInvite(req.Token, req.Username, req.Password); err != nil {
		writeUserError(w, err)
		return
	}
	h.record("auth.accept", r, map[string]string{"user": req.Username})
	httpjson.Write(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *authHandlers) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if err := h.svc.DeleteUser(principalFrom(r.Context()), username); err != nil {
		writeUserError(w, err)
		return
	}
	h.record("auth.delete_user", r, map[string]string{"target": username})
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandlers) handleSetRole(w http.ResponseWriter, r *http.Request) {
	var req setRoleRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	role, err := domain.ParseRole(req.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, "role must be admin, operator, or viewer")
		return
	}
	target := r.PathValue("username")
	if err := h.svc.SetRole(principalFrom(r.Context()), target, role); err != nil {
		writeUserError(w, err)
		return
	}
	h.record("auth.role", r, map[string]string{"target": target, "role": string(role)})
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, domain.ErrUserExists):
		writeError(w, http.StatusConflict, "username already exists")
	case errors.Is(err, domain.ErrLastAdmin):
		writeError(w, http.StatusConflict, "cannot remove or demote the last admin")
	case errors.Is(err, domain.ErrInviteInvalid):
		writeError(w, http.StatusNotFound, "invite is invalid or expired")
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid input")
	case errors.Is(err, domain.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "could not complete request")
	}
}
