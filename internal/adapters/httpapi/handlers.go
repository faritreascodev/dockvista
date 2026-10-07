package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

// containerService is the subset of *service.ContainerService the HTTP
// layer depends on. Declaring it here (consumer-defined interface) keeps
// httpapi decoupled from the concrete service package and easy to test with
// a fake.
type containerService interface {
	Engine(ctx context.Context) (domain.EngineInfo, error)
	ListContainers() []domain.Container
	GetContainer(id string) (domain.Container, error)
	Stats(ctx context.Context, id string) (domain.Stats, error)
	FleetStats(ctx context.Context) []domain.Stats
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Pause(ctx context.Context, id string) error
	Unpause(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
	StreamLogs(ctx context.Context, id string, opts domain.LogStreamOptions) (io.ReadCloser, error)
	LogsMultiplexed(ctx context.Context, id string) (bool, error)
	RefreshOnce(ctx context.Context) error
	CreateExec(ctx context.Context, id string) (string, error)
	AttachExec(ctx context.Context, execID string) (io.ReadWriteCloser, error)
	ResizeExec(ctx context.Context, execID string, rows, cols uint) error
	Create(ctx context.Context, spec domain.ContainerSpec) (id string, started bool, err error)
	Remove(ctx context.Context, id string, force bool) error
	Inspect(ctx context.Context, id string) ([]byte, error)
	Filesystem(ctx context.Context, id string) (domain.ContainerFS, error)
	ListFiles(ctx context.Context, id, path string) (domain.DirListing, error)
	FileStat(ctx context.Context, id, path string) (domain.FSEntry, error)
	FileContent(ctx context.Context, id, path string) (io.ReadCloser, domain.FSEntry, error)
	Changes(ctx context.Context, id string) (domain.FSChangeList, error)
}

type handlers struct {
	svc     containerService
	events  eventSubscriber
	log     *slog.Logger
	streams *streamGate
}

func (h *handlers) handleEngine(w http.ResponseWriter, r *http.Request) {
	info, err := h.c(r).Engine(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, newEngineDTO(info))
}

func (h *handlers) handleListContainers(w http.ResponseWriter, r *http.Request) {
	containers := h.c(r).ListContainers()
	httpjson.Write(w, http.StatusOK, newContainerListDTO(containers))
}

func (h *handlers) handleGetContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}

	c, err := h.c(r).GetContainer(id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, newContainerDTO(c))
}

func (h *handlers) handleContainerStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}

	stats, err := h.c(r).Stats(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, newStatsDTO(stats))
}

// handleFleetStats returns one sample per running container, so the UI makes
// one request per poll instead of one per row.
func (h *handlers) handleFleetStats(w http.ResponseWriter, r *http.Request) {
	samples := h.c(r).FleetStats(r.Context())
	out := make([]statsDTO, 0, len(samples))
	for _, s := range samples {
		out = append(out, newStatsDTO(s))
	}
	httpjson.Write(w, http.StatusOK, out)
}

// containerAction adapts one of the service's lifecycle methods into an
// http.HandlerFunc: validate the ID, run the action, refresh the cache so
// the next list poll already reflects the new state, and reply.
func (h *handlers) containerAction(pick func(containerService, context.Context, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !isValidContainerID(id) {
			writeError(w, http.StatusBadRequest, "invalid container id")
			return
		}

		if err := pick(h.c(r), r.Context(), id); err != nil {
			writeServiceError(w, err)
			return
		}

		if err := h.c(r).RefreshOnce(r.Context()); err != nil {
			h.log.Warn("post-action cache refresh failed", "error", err)
		}

		httpjson.Write(w, http.StatusOK, actionResultDTO{Status: "ok", ID: id})
	}
}
