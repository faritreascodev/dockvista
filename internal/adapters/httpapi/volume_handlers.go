package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

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
	ListFiles(ctx context.Context, name, relPath string) (domain.DirListing, error)
	FileStat(ctx context.Context, name, relPath string) (domain.FSEntry, error)
	FileContent(ctx context.Context, name, relPath string) (io.ReadCloser, domain.FSEntry, error)
}

type volumeHandlers struct {
	svc volumeService
}

func (h *volumeHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	volumes, err := h.s(r).List(r.Context())
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

	v, err := h.s(r).Create(r.Context(), req.Name, req.Driver, req.Labels)
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

	if err := h.s(r).Remove(r.Context(), name, force); err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, actionResultDTO{Status: "ok", ID: name})
}

func (h *volumeHandlers) handlePrune(w http.ResponseWriter, r *http.Request) {
	deleted, reclaimed, err := h.s(r).Prune(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, pruneResultDTO{Deleted: deleted, SpaceReclaimed: reclaimed})
}

func (h *volumeHandlers) handleFiles(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !isValidResourceName(name) {
		writeError(w, http.StatusBadRequest, "invalid volume name")
		return
	}
	listing, err := h.s(r).ListFiles(r.Context(), name, r.URL.Query().Get("path"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, listing)
}

func (h *volumeHandlers) handleFileContent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !isValidResourceName(name) {
		writeError(w, http.StatusBadRequest, "invalid volume name")
		return
	}
	body, entry, err := h.s(r).FileContent(r.Context(), name, r.URL.Query().Get("path"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadFilename(entry.Name)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if entry.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(entry.SizeBytes, 10))
	}
	_, _ = io.Copy(w, body)
}
