package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

// storageService is the subset of *service.StorageService the HTTP layer
// depends on.
type storageService interface {
	Inventory(ctx context.Context) (domain.StorageInventory, error)
	Cleanup(ctx context.Context, plan domain.CleanupPlan, dryRun bool) (domain.CleanupReport, error)
}

type storageHandlers struct {
	svc storageService
}

type storageItemDTO struct {
	Kind       string            `json:"kind"`
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	SizeBytes  int64             `json:"sizeBytes"`
	SizeKnown  bool              `json:"sizeKnown"`
	CreatedAt  *time.Time        `json:"createdAt,omitempty"`
	LastUsedAt *time.Time        `json:"lastUsedAt,omitempty"`
	State      string            `json:"state,omitempty"`
	Project    string            `json:"project,omitempty"`
	Tags       []string          `json:"tags"`
	Detail     string            `json:"detail,omitempty"`
	UsedBy     []string          `json:"usedBy"`
	Labels     map[string]string `json:"labels,omitempty"`
	Shared     bool              `json:"shared,omitempty"`
	InUse      bool              `json:"inUse"`
	Eligible   bool              `json:"eligible"`
	Reason     string            `json:"reason,omitempty"`
}

type storageInventoryDTO struct {
	GeneratedAt time.Time        `json:"generatedAt"`
	Usage       diskUsageDTO     `json:"usage"`
	Items       []storageItemDTO `json:"items"`
}

type cleanupRequest struct {
	Containers []string `json:"containers"`
	Images     []string `json:"images"`
	Volumes    []string `json:"volumes"`
	BuildCache bool     `json:"buildCache"`
}

type cleanupResultDTO struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	FreedBytes int64  `json:"freedBytes"`
}

type cleanupReportDTO struct {
	DryRun     bool               `json:"dryRun"`
	Results    []cleanupResultDTO `json:"results"`
	FreedBytes int64              `json:"freedBytes"`
}

func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (h *storageHandlers) handleInventory(w http.ResponseWriter, r *http.Request) {
	inv, err := h.s(r).Inventory(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := storageInventoryDTO{
		GeneratedAt: inv.GeneratedAt,
		Usage:       newDiskUsageDTO(inv.Usage),
		Items:       make([]storageItemDTO, 0, len(inv.Items)),
	}
	for _, it := range inv.Items {
		out.Items = append(out.Items, storageItemDTO{
			Kind: string(it.Kind), ID: it.ID, Name: it.Name,
			SizeBytes: it.SizeBytes, SizeKnown: it.SizeKnown,
			CreatedAt: optionalTime(it.CreatedAt), LastUsedAt: optionalTime(it.LastUsedAt),
			State: it.State, Project: it.Project, Tags: nonNil(it.Tags), Detail: it.Detail,
			UsedBy: nonNil(it.UsedBy), Labels: it.Labels, Shared: it.Shared,
			InUse: it.InUse, Eligible: it.Eligible, Reason: it.Reason,
		})
	}
	httpjson.Write(w, http.StatusOK, out)
}

// handlePreview and handleCleanup share a body. The preview is a read: it is
// not rate-limited as a write and works on a read-only instance, so viewers
// can see what a cleanup would do.
func (h *storageHandlers) handlePreview(w http.ResponseWriter, r *http.Request) {
	h.runCleanup(w, r, true)
}

func (h *storageHandlers) handleCleanup(w http.ResponseWriter, r *http.Request) {
	h.runCleanup(w, r, false)
}

func (h *storageHandlers) runCleanup(w http.ResponseWriter, r *http.Request, dryRun bool) {
	var req cleanupRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	for _, id := range req.Containers {
		if !isValidContainerID(id) {
			writeError(w, http.StatusBadRequest, "invalid container id")
			return
		}
	}
	for _, id := range req.Images {
		if !isValidImageRef(id) {
			writeError(w, http.StatusBadRequest, "invalid image id")
			return
		}
	}
	for _, name := range req.Volumes {
		if !isValidResourceName(name) {
			writeError(w, http.StatusBadRequest, "invalid volume name")
			return
		}
	}

	report, err := h.s(r).Cleanup(r.Context(), domain.CleanupPlan{
		Containers: req.Containers, Images: req.Images, Volumes: req.Volumes, BuildCache: req.BuildCache,
	}, dryRun)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := cleanupReportDTO{DryRun: report.DryRun, FreedBytes: report.FreedBytes, Results: make([]cleanupResultDTO, 0, len(report.Results))}
	for _, res := range report.Results {
		out.Results = append(out.Results, cleanupResultDTO{
			Kind: string(res.Kind), ID: res.ID, Name: res.Name,
			Status: string(res.Status), Message: res.Message, FreedBytes: res.FreedBytes,
		})
	}
	httpjson.Write(w, http.StatusOK, out)
}
