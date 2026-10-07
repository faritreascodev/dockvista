package httpapi

import (
	"encoding/json"
	"net/http"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

type portBindingRequest struct {
	HostPort      string `json:"hostPort"`
	ContainerPort string `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

type createContainerRequest struct {
	Image         string               `json:"image"`
	Name          string               `json:"name"`
	Env           []string             `json:"env"`
	Ports         []portBindingRequest `json:"ports"`
	Binds         []string             `json:"binds"`
	RestartPolicy string               `json:"restartPolicy"`
	Command       string               `json:"command"`
	Network       string               `json:"network"`
	MemoryBytes   int64                `json:"memoryBytes"`
}

func (h *handlers) handleCreateContainer(w http.ResponseWriter, r *http.Request) {
	var req createContainerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !isValidImageRef(req.Image) {
		writeError(w, http.StatusBadRequest, "invalid image reference")
		return
	}
	if req.Name != "" && !isValidContainerID(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid container name")
		return
	}
	if req.RestartPolicy == "" {
		req.RestartPolicy = "no"
	}
	if !isValidRestartPolicy(req.RestartPolicy) {
		writeError(w, http.StatusBadRequest, "invalid restart policy")
		return
	}

	ports := make([]domain.PortBinding, 0, len(req.Ports))
	for _, p := range req.Ports {
		if !isValidPortNumber(p.HostPort) || !isValidPortNumber(p.ContainerPort) {
			writeError(w, http.StatusBadRequest, "invalid port number (must be 1-65535)")
			return
		}
		if !isValidProtocol(p.Protocol) {
			writeError(w, http.StatusBadRequest, "invalid protocol (must be tcp or udp)")
			return
		}
		ports = append(ports, domain.PortBinding{
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol,
		})
	}

	var cmd []string
	if req.Command != "" {
		cmd = []string{"/bin/sh", "-c", req.Command}
	}
	id, started, err := h.c(r).Create(r.Context(), domain.ContainerSpec{
		Image:         req.Image,
		Name:          req.Name,
		Env:           req.Env,
		Ports:         ports,
		Binds:         req.Binds,
		RestartPolicy: req.RestartPolicy,
		Cmd:           cmd,
		Network:       req.Network,
		MemoryBytes:   req.MemoryBytes,
	})
	if err != nil {
		if id != "" {
			httpjson.Write(w, http.StatusBadGateway, createContainerResult{
				Status:  "created",
				ID:      id,
				Started: false,
				Error:   "container was created but did not start",
			})
			return
		}
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusCreated, createContainerResult{Status: "ok", ID: id, Started: started})
}

func (h *handlers) handleRemoveContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}
	force := r.URL.Query().Get("force") == "true"

	if err := h.c(r).Remove(r.Context(), id, force); err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, actionResultDTO{Status: "ok", ID: id})
}

// handleInspectContainer passes the daemon's raw inspect JSON straight
// through — it's read-only display data for the frontend's Inspect tab, not
// something that benefits from being re-typed through our own DTO.
func (h *handlers) handleInspectContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}

	raw, err := h.c(r).Inspect(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// Confirm it's actually well-formed JSON before writing it through
	// as-is — cheap insurance against ever forwarding something a browser
	// could misinterpret, on top of the explicit content type and the
	// nosniff header below.
	if !json.Valid(raw) {
		writeError(w, http.StatusBadGateway, "docker engine returned an invalid inspect payload")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
