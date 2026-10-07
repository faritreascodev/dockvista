package docker

import (
	"context"
	"io"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"

	"dockvista/internal/core/domain"
)

func (c *Client) ListImages(ctx context.Context) ([]domain.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	raw, err := c.sdk.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, wrapErr("list images", err)
	}

	out := make([]domain.Image, 0, len(raw))
	for _, img := range raw {
		out = append(out, domain.Image{
			ID:          img.ID,
			RepoTags:    img.RepoTags,
			RepoDigests: img.RepoDigests,
			Size:        img.Size,
			Containers:  img.Containers,
			Created:     time.Unix(img.Created, 0).UTC(),
			Labels:      img.Labels,
		})
	}
	return out, nil
}

func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	if _, err := c.sdk.ImageRemove(ctx, id, image.RemoveOptions{Force: force, PruneChildren: true}); err != nil {
		return wrapErr("remove image", err)
	}
	return nil
}

// PullImage is not bounded by defaultCallTimeout — a large image can
// legitimately take minutes; the caller's context (tied to the client's SSE
// connection) is what ends it.
func (c *Client) PullImage(ctx context.Context, ref, registryAuth string) (io.ReadCloser, error) {
	out, err := c.sdk.ImagePull(ctx, ref, image.PullOptions{RegistryAuth: registryAuth})
	if err != nil {
		return nil, wrapErr("pull image", err)
	}
	return out, nil
}

func (c *Client) ImageHistory(ctx context.Context, id string) ([]domain.ImageLayer, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()
	raw, err := c.sdk.ImageHistory(ctx, id)
	if err != nil {
		return nil, wrapErr("image history", err)
	}
	out := make([]domain.ImageLayer, 0, len(raw))
	for _, row := range raw {
		out = append(out, domain.ImageLayer{
			ID:        row.ID,
			Created:   time.Unix(row.Created, 0).UTC(),
			CreatedBy: row.CreatedBy,
			Size:      row.Size,
			Tags:      row.Tags,
			Comment:   row.Comment,
		})
	}
	return out, nil
}

func (c *Client) PruneImages(ctx context.Context) (deleted int, spaceReclaimed uint64, err error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	report, err := c.sdk.ImagesPrune(ctx, filters.Args{})
	if err != nil {
		return 0, 0, wrapErr("prune images", err)
	}
	return len(report.ImagesDeleted), report.SpaceReclaimed, nil
}
