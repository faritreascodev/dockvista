package httpapi

import (
	"context"
	"net/http"
	"time"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

// systemService is the subset of *service.SystemService the HTTP layer
// depends on.
type systemService interface {
	Info(ctx context.Context) (domain.SystemInfo, error)
	DiskUsage(ctx context.Context) (domain.DiskUsage, error)
	InvalidateDiskUsage()
	Ping(ctx context.Context) error
}

type systemHandlers struct {
	svc systemService
}

func (h *systemHandlers) handleInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.s(r).Info(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, newSystemInfoDTO(info))
}

func (h *systemHandlers) handleReady(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("unavailable"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.svc.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("docker unavailable"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *systemHandlers) handleDiskUsage(w http.ResponseWriter, r *http.Request) {
	usage, err := h.s(r).DiskUsage(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, newDiskUsageDTO(usage))
}

// invalidatesDiskUsage drops the cached disk usage after a request that
// changed what is on disk succeeds, so the overview does not show space a
// prune already freed.
func invalidatesDiskUsage(fallback systemService) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			if rec.status < http.StatusMultipleChoices {
				svc := fallback
				if e, ok := boundFrom(r.Context()); ok && e.System != nil {
					svc = e.System
				}
				if svc != nil {
					svc.InvalidateDiskUsage()
				}
			}
		})
	}
}
