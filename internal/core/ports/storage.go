package ports

import (
	"context"

	"dockvista/internal/core/domain"
)

// StorageClient lists everything that occupies disk and removes single
// objects. Removal methods never cascade: removing a container keeps its
// volumes, removing an image never touches containers.
type StorageClient interface {
	// StorageInventory returns every container, image, volume, and build
	// cache record with its facts filled in; Eligible and Reason are left
	// for the service to decide.
	StorageInventory(ctx context.Context) (domain.StorageInventory, error)
	RemoveContainer(ctx context.Context, id string, force bool) error
	RemoveImage(ctx context.Context, id string, force bool) error
	RemoveVolume(ctx context.Context, name string, force bool) error
	// PruneBuildCache removes every build cache record not in use.
	PruneBuildCache(ctx context.Context) (spaceReclaimed uint64, err error)
}
