package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

// networkService is the subset of *service.NetworkService the HTTP layer
// depends on.
type networkService interface {
	List(ctx context.Context) ([]domain.Network, error)
	Create(ctx context.Context, name, driver string, labels map[string]string) (domain.Network, error)
	Remove(ctx context.Context, id string) error
	Connect(ctx context.Context, networkID, containerID string) error
	Disconnect(ctx context.Context, networkID, containerID string, force bool) error
	Prune(ctx context.Context) (deleted int, err error)
}

type networkHandlers struct {
	svc networkService
}

func (h *networkHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	networks, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, newNetworkListDTO(networks))
}

type createNetworkRequest struct {
	Name   string            `json:"name"`
	Driver string            `json:"driver"`
	Labels map[string]string `json:"labels"`
}

func (h *networkHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createNetworkRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !isValidResourceName(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid network name")
		return
	}

	n, err := h.svc.Create(r.Context(), req.Name, req.Driver, req.Labels)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusCreated, newNetworkDTO(n))
}

func (h *networkHandlers) handleRemove(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if !isValidResourceName(id) {
		writeError(w, http.StatusBadRequest, "invalid or missing network id")
		return
	}

	if err := h.svc.Remove(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, actionResultDTO{Status: "ok", ID: id})
}

type networkAttachRequest struct {
	NetworkID   string `json:"networkId"`
	ContainerID string `json:"containerId"`
	Force       bool   `json:"force"`
}

func (h *networkHandlers) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req networkAttachRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !isValidResourceName(req.NetworkID) || !isValidContainerID(req.ContainerID) {
		writeError(w, http.StatusBadRequest, "invalid network or container id")
		return
	}

	if err := h.svc.Connect(r.Context(), req.NetworkID, req.ContainerID); err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, actionResultDTO{Status: "ok", ID: req.NetworkID})
}

func (h *networkHandlers) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	var req networkAttachRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !isValidResourceName(req.NetworkID) || !isValidContainerID(req.ContainerID) {
		writeError(w, http.StatusBadRequest, "invalid network or container id")
		return
	}

	if err := h.svc.Disconnect(r.Context(), req.NetworkID, req.ContainerID, req.Force); err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, actionResultDTO{Status: "ok", ID: req.NetworkID})
}

func (h *networkHandlers) handlePrune(w http.ResponseWriter, r *http.Request) {
	deleted, err := h.svc.Prune(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, pruneResultDTO{Deleted: deleted})
}
