package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

// volumeService is the subset of *service.VolumeService the HTTP layer
// depends on.
type volumeService interface {
	List(ctx context.Context) ([]domain.Volume, error)
	Create(ctx context.Context, name, driver string, labels map[string]string) (domain.Volume, error)
	Remove(ctx context.Context, name string, force bool) error
	Prune(ctx context.Context) (deleted int, spaceReclaimed uint64, err error)
}

type volumeHandlers struct {
	svc volumeService
}

func (h *volumeHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	volumes, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, newVolumeListDTO(volumes))
}

type createVolumeRequest struct {
	Name   string            `json:"name"`
	Driver string            `json:"driver"`
	Labels map[string]string `json:"labels"`
}

func (h *volumeHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createVolumeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name != "" && !isValidResourceName(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid volume name")
		return
	}

	v, err := h.svc.Create(r.Context(), req.Name, req.Driver, req.Labels)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusCreated, newVolumeDTO(v))
}

func (h *volumeHandlers) handleRemove(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if !isValidResourceName(name) {
		writeError(w, http.StatusBadRequest, "invalid or missing volume name")
		return
	}
	force := r.URL.Query().Get("force") == "true"

	if err := h.svc.Remove(r.Context(), name, force); err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, actionResultDTO{Status: "ok", ID: name})
}

func (h *volumeHandlers) handlePrune(w http.ResponseWriter, r *http.Request) {
	deleted, reclaimed, err := h.svc.Prune(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, pruneResultDTO{Deleted: deleted, SpaceReclaimed: reclaimed})
}
