package docker

import (
	"context"
	"fmt"
	"strconv"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"

	"dockvista/internal/core/domain"
)

func (c *Client) SwarmSnapshot(ctx context.Context) (domain.SwarmSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	info, err := c.sdk.Info(ctx)
	if err != nil {
		return domain.SwarmSnapshot{}, wrapErr("swarm info", err)
	}
	si := info.Swarm
	snap := domain.SwarmSnapshot{
		State:            string(si.LocalNodeState),
		ControlAvailable: si.ControlAvailable,
		NodeID:           si.NodeID,
		NodeAddr:         si.NodeAddr,
		Error:            si.Error,
		Nodes:            []domain.SwarmNode{},
		Services:         []domain.SwarmService{},
	}
	if si.Cluster != nil {
		snap.ClusterID = si.Cluster.ID
	}
	if si.LocalNodeState != swarm.LocalNodeStateActive || !si.ControlAvailable {
		return snap, nil
	}

	nodes, err := c.sdk.NodeList(ctx, dockertypes.NodeListOptions{})
	if err != nil {
		return domain.SwarmSnapshot{}, wrapErr("swarm nodes", err)
	}
	snap.Nodes = mapSwarmNodes(nodes)

	svcs, err := c.sdk.ServiceList(ctx, dockertypes.ServiceListOptions{Status: true})
	if err != nil {
		return domain.SwarmSnapshot{}, wrapErr("swarm services", err)
	}
	snap.Services = mapSwarmServices(svcs)
	return snap, nil
}

func mapSwarmNodes(in []swarm.Node) []domain.SwarmNode {
	out := make([]domain.SwarmNode, 0, len(in))
	for _, n := range in {
		item := domain.SwarmNode{
			ID:            n.ID,
			Hostname:      n.Description.Hostname,
			Role:          string(n.Spec.Role),
			Availability:  string(n.Spec.Availability),
			Status:        string(n.Status.State),
			Addr:          n.Status.Addr,
			EngineVersion: n.Description.Engine.EngineVersion,
			Manager:       n.ManagerStatus != nil,
		}
		if n.ManagerStatus != nil {
			item.Leader = n.ManagerStatus.Leader
		}
		out = append(out, item)
	}
	return out
}

func mapSwarmServices(in []swarm.Service) []domain.SwarmService {
	out := make([]domain.SwarmService, 0, len(in))
	for _, s := range in {
		item := domain.SwarmService{
			ID:    s.ID,
			Name:  s.Spec.Name,
			Image: "",
			Mode:  swarmMode(s.Spec.Mode),
			Ports: swarmPorts(s.Endpoint.Ports),
		}
		if s.Spec.TaskTemplate.ContainerSpec != nil {
			item.Image = s.Spec.TaskTemplate.ContainerSpec.Image
		}
		if s.Spec.Mode.Replicated != nil && s.Spec.Mode.Replicated.Replicas != nil {
			item.Replicas = int(*s.Spec.Mode.Replicated.Replicas)
		}
		if s.ServiceStatus != nil {
			item.Running = int(s.ServiceStatus.RunningTasks)
			if item.Replicas == 0 {
				item.Replicas = int(s.ServiceStatus.DesiredTasks)
			}
		}
		out = append(out, item)
	}
	return out
}

func swarmMode(m swarm.ServiceMode) string {
	switch {
	case m.Global != nil:
		return "global"
	case m.ReplicatedJob != nil:
		return "replicated-job"
	case m.GlobalJob != nil:
		return "global-job"
	default:
		return "replicated"
	}
}

func swarmPorts(ports []swarm.PortConfig) []string {
	if len(ports) == 0 {
		return nil
	}
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		proto := string(p.Protocol)
		if proto == "" {
			proto = "tcp"
		}
		out = append(out, fmt.Sprintf("%s:%s/%s", strconv.FormatUint(uint64(p.PublishedPort), 10), strconv.FormatUint(uint64(p.TargetPort), 10), proto))
	}
	return out
}
