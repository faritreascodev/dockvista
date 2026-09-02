package ports

import (
	"context"

	"dockvista/internal/core/domain"
)

// NetworkClient is the boundary between the network service and the Docker
// engine's network API.
type NetworkClient interface {
	ListNetworks(ctx context.Context) ([]domain.Network, error)
	CreateNetwork(ctx context.Context, name, driver string, labels map[string]string) (domain.Network, error)
	RemoveNetwork(ctx context.Context, id string) error
	ConnectNetwork(ctx context.Context, networkID, containerID string) error
	DisconnectNetwork(ctx context.Context, networkID, containerID string, force bool) error
	PruneNetworks(ctx context.Context) (deleted int, err error)
}
