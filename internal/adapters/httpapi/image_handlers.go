package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

// imageService is the subset of *service.ImageService the HTTP layer
// depends on (consumer-defined interface, same decoupling pattern as
// containerService/authService).
type imageService interface {
	List(ctx context.Context) ([]domain.Image, error)
	Remove(ctx context.Context, id string, force bool) error
	Pull(ctx context.Context, ref string) (io.ReadCloser, error)
	Prune(ctx context.Context) (deleted int, spaceReclaimed uint64, err error)
}

type imageHandlers struct {
	svc imageService
	log *slog.Logger
}

func (h *imageHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	images, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, newImageListDTO(images))
}

func (h *imageHandlers) handleRemove(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if !isValidImageRef(id) {
		writeError(w, http.StatusBadRequest, "invalid or missing image id")
		return
	}
	force := r.URL.Query().Get("force") == "true"

	if err := h.svc.Remove(r.Context(), id, force); err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, actionResultDTO{Status: "ok", ID: id})
}

func (h *imageHandlers) handlePrune(w http.ResponseWriter, r *http.Request) {
	deleted, reclaimed, err := h.svc.Prune(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, pruneResultDTO{Deleted: deleted, SpaceReclaimed: reclaimed})
}

type pullRequest struct {
	Reference string `json:"reference"`
}

// handlePull streams the daemon's own newline-delimited pull progress
// straight through as SSE, one "data:" frame per line — same flush pattern
// as handleContainerLogs, just without reshaping the payload.
func (h *imageHandlers) handlePull(w http.ResponseWriter, r *http.Request) {
	var req pullRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !isValidImageRef(req.Reference) {
		writeError(w, http.StatusBadRequest, "invalid image reference")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	stream, err := h.svc.Pull(ctx, req.Reference)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", scanner.Bytes()); err != nil {
			return
		}
		flusher.Flush()
	}
	if err := scanner.Err(); err != nil {
		h.log.Warn("image pull stream ended with error", "error", err)
	}
}
