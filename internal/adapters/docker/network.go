package docker

import (
	"context"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	"dockvista/internal/core/domain"
)

func toDomainNetwork(n dockertypes.NetworkResource) domain.Network {
	containers := make(map[string]string, len(n.Containers))
	for id, ep := range n.Containers {
		containers[id] = ep.Name
	}
	return domain.Network{
		ID:         n.ID,
		Name:       n.Name,
		Driver:     n.Driver,
		Scope:      n.Scope,
		Internal:   n.Internal,
		Attachable: n.Attachable,
		Created:    n.Created.UTC(),
		Labels:     n.Labels,
		Containers: containers,
	}
}

func (c *Client) ListNetworks(ctx context.Context) ([]domain.Network, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	raw, err := c.sdk.NetworkList(ctx, dockertypes.NetworkListOptions{})
	if err != nil {
		return nil, wrapErr("list networks", err)
	}

	out := make([]domain.Network, 0, len(raw))
	for _, n := range raw {
		out = append(out, toDomainNetwork(n))
	}
	return out, nil
}

func (c *Client) CreateNetwork(ctx context.Context, name, driver string, labels map[string]string) (domain.Network, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	resp, err := c.sdk.NetworkCreate(ctx, name, dockertypes.NetworkCreate{Driver: driver, Labels: labels})
	if err != nil {
		return domain.Network{}, wrapErr("create network", err)
	}
	// NetworkCreate's response has no Created timestamp; approximate with
	// "now" rather than leaving the domain.Network zero-valued (which would
	// otherwise serialize as a year-1 Unix epoch to the frontend).
	return domain.Network{ID: resp.ID, Name: name, Driver: driver, Labels: labels, Created: time.Now().UTC()}, nil
}

func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	if err := c.sdk.NetworkRemove(ctx, id); err != nil {
		return wrapErr("remove network", err)
	}
	return nil
}

func (c *Client) ConnectNetwork(ctx context.Context, networkID, containerID string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	if err := c.sdk.NetworkConnect(ctx, networkID, containerID, nil); err != nil {
		return wrapErr("connect network", err)
	}
	return nil
}

func (c *Client) DisconnectNetwork(ctx context.Context, networkID, containerID string, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	if err := c.sdk.NetworkDisconnect(ctx, networkID, containerID, force); err != nil {
		return wrapErr("disconnect network", err)
	}
	return nil
}

func (c *Client) PruneNetworks(ctx context.Context) (deleted int, err error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	report, err := c.sdk.NetworksPrune(ctx, filters.Args{})
	if err != nil {
		return 0, wrapErr("prune networks", err)
	}
	return len(report.NetworksDeleted), nil
}
