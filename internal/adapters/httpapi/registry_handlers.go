package httpapi

import (
	"encoding/json"
	"net/http"

	"dockvista/internal/adapters/regstore"
	"dockvista/pkg/httpjson"
)

type registryHandlers struct {
	store *regstore.Store
}

type registryDTO struct {
	ID       string `json:"id"`
	Host     string `json:"host"`
	Username string `json:"username"`
}

type upsertRegistryRequest struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *registryHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		httpjson.Write(w, http.StatusOK, []registryDTO{})
		return
	}
	list, err := h.store.List()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]registryDTO, 0, len(list))
	for _, it := range list {
		out = append(out, registryDTO{ID: it.ID, Host: it.Host, Username: it.Username})
	}
	httpjson.Write(w, http.StatusOK, out)
}

func (h *registryHandlers) handleUpsert(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusNotImplemented, "registries are not available")
		return
	}
	var req upsertRegistryRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	reg, err := h.store.Upsert(req.Host, req.Username, req.Password)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, registryDTO{ID: reg.ID, Host: reg.Host, Username: reg.Username})
}

func (h *registryHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusNotFound, "registry not found")
		return
	}
	if err := h.store.Delete(r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
