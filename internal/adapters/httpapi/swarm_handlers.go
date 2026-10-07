package httpapi

import (
	"context"
	"net/http"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

type swarmService interface {
	Snapshot(ctx context.Context) (domain.SwarmSnapshot, error)
}

type swarmHandlers struct{}

type swarmNodeDTO struct {
	ID            string `json:"id"`
	Hostname      string `json:"hostname"`
	Role          string `json:"role"`
	Availability  string `json:"availability"`
	Status        string `json:"status"`
	Addr          string `json:"addr,omitempty"`
	EngineVersion string `json:"engineVersion,omitempty"`
	Manager       bool   `json:"manager"`
	Leader        bool   `json:"leader"`
}

type swarmServiceDTO struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Image    string   `json:"image"`
	Mode     string   `json:"mode"`
	Replicas int      `json:"replicas"`
	Running  int      `json:"running"`
	Ports    []string `json:"ports,omitempty"`
}

type swarmSnapshotDTO struct {
	State            string             `json:"state"`
	ControlAvailable bool               `json:"controlAvailable"`
	NodeID           string             `json:"nodeId,omitempty"`
	NodeAddr         string             `json:"nodeAddr,omitempty"`
	ClusterID        string             `json:"clusterId,omitempty"`
	Error            string             `json:"error,omitempty"`
	Nodes            []swarmNodeDTO     `json:"nodes"`
	Services         []swarmServiceDTO  `json:"services"`
}

func toSwarmDTO(s domain.SwarmSnapshot) swarmSnapshotDTO {
	nodes := make([]swarmNodeDTO, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		nodes = append(nodes, swarmNodeDTO{
			ID: n.ID, Hostname: n.Hostname, Role: n.Role, Availability: n.Availability,
			Status: n.Status, Addr: n.Addr, EngineVersion: n.EngineVersion, Manager: n.Manager, Leader: n.Leader,
		})
	}
	svcs := make([]swarmServiceDTO, 0, len(s.Services))
	for _, svc := range s.Services {
		svcs = append(svcs, swarmServiceDTO{
			ID: svc.ID, Name: svc.Name, Image: svc.Image, Mode: svc.Mode,
			Replicas: svc.Replicas, Running: svc.Running, Ports: svc.Ports,
		})
	}
	return swarmSnapshotDTO{
		State: s.State, ControlAvailable: s.ControlAvailable, NodeID: s.NodeID,
		NodeAddr: s.NodeAddr, ClusterID: s.ClusterID, Error: s.Error,
		Nodes: nodes, Services: svcs,
	}
}

func swarmOf(r *http.Request) swarmService {
	if e, ok := boundFrom(r.Context()); ok {
		return e.Swarm
	}
	return nil
}

func (h *swarmHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	svc := swarmOf(r)
	if svc == nil {
		httpjson.Write(w, http.StatusOK, toSwarmDTO(domain.SwarmSnapshot{State: "inactive"}))
		return
	}
	snap, err := svc.Snapshot(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, toSwarmDTO(snap))
}
