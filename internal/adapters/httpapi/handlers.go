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
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Pause(ctx context.Context, id string) error
	Unpause(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
	StreamLogs(ctx context.Context, id, tail string) (io.ReadCloser, error)
	LogsMultiplexed(ctx context.Context, id string) (bool, error)
	RefreshOnce(ctx context.Context) error
	CreateExec(ctx context.Context, id string) (string, error)
	AttachExec(ctx context.Context, execID string) (io.ReadWriteCloser, error)
	ResizeExec(ctx context.Context, execID string, rows, cols uint) error
	Create(ctx context.Context, spec domain.ContainerSpec) (id string, started bool, err error)
	Remove(ctx context.Context, id string, force bool) error
	Inspect(ctx context.Context, id string) ([]byte, error)
}

type handlers struct {
	svc     containerService
	events  eventSubscriber
	log     *slog.Logger
	streams *streamGate
}

func (h *handlers) handleEngine(w http.ResponseWriter, r *http.Request) {
	info, err := h.svc.Engine(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, newEngineDTO(info))
}

func (h *handlers) handleListContainers(w http.ResponseWriter, r *http.Request) {
	containers := h.svc.ListContainers()
	httpjson.Write(w, http.StatusOK, newContainerListDTO(containers))
}

func (h *handlers) handleGetContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}

	c, err := h.svc.GetContainer(id)
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

	stats, err := h.svc.Stats(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, newStatsDTO(stats))
}

// containerAction adapts one of the service's lifecycle methods into an
// http.HandlerFunc: validate the ID, run the action, refresh the cache so
// the next list poll already reflects the new state, and reply.
func (h *handlers) containerAction(action func(ctx context.Context, id string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !isValidContainerID(id) {
			writeError(w, http.StatusBadRequest, "invalid container id")
			return
		}

		if err := action(r.Context(), id); err != nil {
			writeServiceError(w, err)
			return
		}

		if err := h.svc.RefreshOnce(r.Context()); err != nil {
			h.log.Warn("post-action cache refresh failed", "error", err)
		}

		httpjson.Write(w, http.StatusOK, actionResultDTO{Status: "ok", ID: id})
	}
}
