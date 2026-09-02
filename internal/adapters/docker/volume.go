package docker

import (
	"context"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"

	"dockvista/internal/core/domain"
)

func toDomainVolume(v volume.Volume) domain.Volume {
	created, _ := time.Parse(time.RFC3339, v.CreatedAt)
	return domain.Volume{
		Name:       v.Name,
		Driver:     v.Driver,
		Mountpoint: v.Mountpoint,
		CreatedAt:  created.UTC(),
		Labels:     v.Labels,
	}
}

func (c *Client) ListVolumes(ctx context.Context) ([]domain.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	resp, err := c.sdk.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, wrapErr("list volumes", err)
	}

	out := make([]domain.Volume, 0, len(resp.Volumes))
	for _, v := range resp.Volumes {
		if v == nil {
			continue
		}
		out = append(out, toDomainVolume(*v))
	}
	return out, nil
}

func (c *Client) CreateVolume(ctx context.Context, name, driver string, labels map[string]string) (domain.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	v, err := c.sdk.VolumeCreate(ctx, volume.CreateOptions{Name: name, Driver: driver, Labels: labels})
	if err != nil {
		return domain.Volume{}, wrapErr("create volume", err)
	}
	return toDomainVolume(v), nil
}

func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	if err := c.sdk.VolumeRemove(ctx, name, force); err != nil {
		return wrapErr("remove volume", err)
	}
	return nil
}

func (c *Client) PruneVolumes(ctx context.Context) (deleted int, spaceReclaimed uint64, err error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	report, err := c.sdk.VolumesPrune(ctx, filters.Args{})
	if err != nil {
		return 0, 0, wrapErr("prune volumes", err)
	}
	return len(report.VolumesDeleted), report.SpaceReclaimed, nil
}
