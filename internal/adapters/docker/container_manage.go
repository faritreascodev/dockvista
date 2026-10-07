package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"

	"dockvista/internal/core/domain"
)

func toRestartPolicy(policy string) container.RestartPolicy {
	switch policy {
	case string(container.RestartPolicyAlways), string(container.RestartPolicyOnFailure), string(container.RestartPolicyUnlessStopped):
		return container.RestartPolicy{Name: container.RestartPolicyMode(policy)}
	default:
		return container.RestartPolicy{Name: container.RestartPolicyDisabled}
	}
}

func toPortBindings(ports []domain.PortBinding) (nat.PortSet, nat.PortMap, error) {
	exposed := make(nat.PortSet, len(ports))
	bindings := make(nat.PortMap, len(ports))
	for _, p := range ports {
		port, err := nat.NewPort(p.Protocol, p.ContainerPort)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid port %q/%s: %w", p.ContainerPort, p.Protocol, err)
		}
		exposed[port] = struct{}{}
		bindings[port] = append(bindings[port], nat.PortBinding{HostPort: p.HostPort})
	}
	return exposed, bindings, nil
}

// CreateContainer creates and starts a new container. No path/mount
// sandboxing is applied to spec.Binds — anyone who can reach this API
// already has full Docker socket access, which is host-root-equivalent
// regardless of what validation happens here (see README's security
// section for the full reasoning).
func (c *Client) CreateContainer(ctx context.Context, spec domain.ContainerSpec) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	exposedPorts, portBindings, err := toPortBindings(spec.Ports)
	if err != nil {
		return "", false, fmt.Errorf("docker: %w: %w", domain.ErrInvalidInput, err)
	}

	hostCfg := &container.HostConfig{
		PortBindings:  portBindings,
		Binds:         spec.Binds,
		RestartPolicy: toRestartPolicy(spec.RestartPolicy),
	}
	hostCfg.Memory = spec.MemoryBytes
	if spec.Network != "" {
		hostCfg.NetworkMode = container.NetworkMode(spec.Network)
	}
	resp, err := c.sdk.ContainerCreate(ctx,
		&container.Config{
			Image:        spec.Image,
			Env:          spec.Env,
			Cmd:          spec.Cmd,
			Labels:       spec.Labels,
			ExposedPorts: exposedPorts,
		},
		hostCfg,
		nil, nil, spec.Name,
	)
	if err != nil {
		return "", false, wrapErr("create container", err)
	}

	if err := c.sdk.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return resp.ID, false, wrapErr("start created container", err)
	}
	return resp.ID, true, nil
}

func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	if err := c.sdk.ContainerRemove(ctx, id, container.RemoveOptions{Force: force, RemoveVolumes: false}); err != nil {
		return wrapErr("remove container", err)
	}
	return nil
}

func (c *Client) InspectContainer(ctx context.Context, id string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	_, raw, err := c.sdk.ContainerInspectWithRaw(ctx, id, false)
	if err != nil {
		return nil, wrapErr("inspect container", err)
	}
	return raw, nil
}
