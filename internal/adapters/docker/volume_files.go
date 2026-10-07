package docker

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"

	"dockvista/internal/core/domain"
)

func (c *Client) FindVolumeMount(ctx context.Context, volumeName string) (containerID, dest string, err error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	list, err := c.sdk.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("volume", volumeName), filters.Arg("status", "running")),
	})
	if err != nil {
		return "", "", wrapErr("list containers for volume", err)
	}
	if len(list) == 0 {
		return "", "", domain.ErrNotRunning
	}
	info, err := c.sdk.ContainerInspect(ctx, list[0].ID)
	if err != nil {
		return "", "", wrapErr("inspect volume mount", err)
	}
	for _, m := range info.Mounts {
		if string(m.Type) == "volume" && m.Name == volumeName {
			return info.ID, m.Destination, nil
		}
	}
	return "", "", domain.ErrNotFound
}

func (c *Client) ListVolumeDir(ctx context.Context, volumeName, relPath string) (domain.DirListing, error) {
	id, dest, err := c.FindVolumeMount(ctx, volumeName)
	if errors.Is(err, domain.ErrNotRunning) {
		return domain.DirListing{Path: relPath, Reason: domain.ListReasonNotRunning}, nil
	}
	if err != nil {
		return domain.DirListing{}, err
	}
	return c.ListContainerDir(ctx, id, joinMount(dest, relPath))
}

func (c *Client) StatVolumePath(ctx context.Context, volumeName, relPath string) (domain.FSEntry, error) {
	id, dest, err := c.FindVolumeMount(ctx, volumeName)
	if err != nil {
		return domain.FSEntry{}, err
	}
	return c.StatContainerPath(ctx, id, joinMount(dest, relPath))
}

func (c *Client) CopyVolumeFile(ctx context.Context, volumeName, relPath string) (io.ReadCloser, domain.FSEntry, error) {
	id, dest, err := c.FindVolumeMount(ctx, volumeName)
	if err != nil {
		return nil, domain.FSEntry{}, err
	}
	return c.CopyContainerFile(ctx, id, joinMount(dest, relPath))
}

func joinMount(dest, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "/" {
		return dest
	}
	return path.Join(dest, strings.TrimPrefix(rel, "/"))
}
