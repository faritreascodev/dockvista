package ports

import (
	"context"
	"io"

	"dockvista/internal/core/domain"
)

// ImageClient is the boundary between the image service and the Docker
// engine's image API. internal/adapters/docker implements this against the
// real SDK, on the same *Client that implements DockerClient.
type ImageClient interface {
	ListImages(ctx context.Context) ([]domain.Image, error)
	RemoveImage(ctx context.Context, id string, force bool) error
	// PullImage streams the daemon's newline-delimited pull progress. The
	// caller owns the returned ReadCloser and must Close it.
	// registryAuth is Docker's X-Registry-Auth payload, or empty.
	PullImage(ctx context.Context, ref, registryAuth string) (io.ReadCloser, error)
	ImageHistory(ctx context.Context, id string) ([]domain.ImageLayer, error)
	PruneImages(ctx context.Context) (deleted int, spaceReclaimed uint64, err error)
}
