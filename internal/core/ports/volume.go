package ports

import (
	"context"

	"dockvista/internal/core/domain"
)

// VolumeClient is the boundary between the volume service and the Docker
// engine's volume API.
type VolumeClient interface {
	ListVolumes(ctx context.Context) ([]domain.Volume, error)
	CreateVolume(ctx context.Context, name, driver string, labels map[string]string) (domain.Volume, error)
	RemoveVolume(ctx context.Context, name string, force bool) error
	PruneVolumes(ctx context.Context) (deleted int, spaceReclaimed uint64, err error)
}
