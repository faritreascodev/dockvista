package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

type envDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Host      string `json:"host"`
	Reachable bool   `json:"reachable"`
	Version   string `json:"version,omitempty"`
	Active    bool   `json:"active"`
	Local     bool   `json:"local"`
}

type createEnvRequest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Host    string `json:"host"`
	TLSCA   string `json:"tlsCa"`
	TLSCert string `json:"tlsCert"`
	TLSKey  string `json:"tlsKey"`
}

func (h *envHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	if h.res == nil {
		httpjson.Write(w, http.StatusOK, []envDTO{localOnlyDTO(true)})
		return
	}
	list, err := h.res.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	active := envIDFrom(r)
	out := make([]envDTO, 0, len(list))
	for _, e := range list {
		out = append(out, envDTO{
			ID:        e.ID,
			Name:      e.Name,
			Kind:      string(e.Kind),
			Host:      e.Host,
			Reachable: e.Reachable,
			Version:   e.Version,
			Active:    e.ID == active,
			Local:     e.ID == domain.LocalEnvironmentID,
		})
	}
	httpjson.Write(w, http.StatusOK, out)
}

func (h *envHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	if h.res == nil {
		writeError(w, http.StatusNotImplemented, "environments are not available")
		return
	}
	var req createEnvRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	env, err := h.res.Create(r.Context(), domain.EnvironmentSpec{
		Name:    req.Name,
		Kind:    domain.EnvironmentKind(req.Kind),
		Host:    req.Host,
		TLSCA:   req.TLSCA,
		TLSCert: req.TLSCert,
		TLSKey:  req.TLSKey,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusCreated, envDTO{
		ID: env.ID, Name: env.Name, Kind: string(env.Kind), Host: env.Host,
		Reachable: env.Reachable, Version: env.Version,
	})
}

func (h *envHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	if h.res == nil {
		writeError(w, http.StatusNotImplemented, "environments are not available")
		return
	}
	id := r.PathValue("id")
	if err := h.res.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	if envIDFrom(r) == id {
		http.SetCookie(w, envCookie(domain.LocalEnvironmentID, r, h.cookieSecure))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *envHandlers) handleSelect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.res != nil {
		if _, err := h.res.Resolve(r.Context(), id); err != nil {
			writeServiceError(w, err)
			return
		}
	} else if id != domain.LocalEnvironmentID {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}
	http.SetCookie(w, envCookie(id, r, h.cookieSecure))
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok", "id": id})
}

func envCookie(id string, r *http.Request, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     envCookieName,
		Value:    id,
		Path:     "/",
		Expires:  time.Now().Add(365 * 24 * time.Hour),
		HttpOnly: true,
		Secure:   secure || r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	}
}

func localOnlyDTO(active bool) envDTO {
	return envDTO{ID: domain.LocalEnvironmentID, Name: "Local", Kind: "local", Host: "local socket", Active: active, Local: true}
}

type envHandlers struct {
	res          EngineResolver
	cookieSecure bool
}
