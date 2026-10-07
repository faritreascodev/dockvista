package docker

import (
	"context"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"

	"dockvista/internal/core/domain"
)

// buildCachePruneTimeout is generous: pruning gigabytes of BuildKit cache
// is disk-bound and can take well over a minute.
const buildCachePruneTimeout = 3 * time.Minute

const composeProjectLabel = "com.docker.compose.project"

func (c *Client) StorageInventory(ctx context.Context) (domain.StorageInventory, error) {
	ctx, cancel := context.WithTimeout(ctx, diskUsageTimeout)
	defer cancel()

	du, err := c.sdk.DiskUsage(ctx, dockertypes.DiskUsageOptions{})
	if err != nil {
		return domain.StorageInventory{}, wrapErr("disk usage", err)
	}
	return mapInventory(du), nil
}

func mapInventory(du dockertypes.DiskUsage) domain.StorageInventory {
	inv := domain.StorageInventory{Usage: mapDiskUsage(du)}

	imageUsers := map[string][]string{}
	volumeUsers := map[string][]string{}
	for _, ctr := range du.Containers {
		if ctr == nil {
			continue
		}
		imageUsers[ctr.ImageID] = append(imageUsers[ctr.ImageID], ctr.ID)
		for _, m := range ctr.Mounts {
			if m.Type == mount.TypeVolume && m.Name != "" {
				volumeUsers[m.Name] = append(volumeUsers[m.Name], ctr.ID)
			}
		}
		name := ctr.ID
		if len(ctr.Names) > 0 {
			name = strings.TrimPrefix(ctr.Names[0], "/")
		}
		inv.Items = append(inv.Items, domain.StorageItem{
			Kind:      domain.StorageContainer,
			ID:        ctr.ID,
			Name:      name,
			SizeBytes: max(ctr.SizeRw, 0),
			SizeKnown: true,
			CreatedAt: time.Unix(ctr.Created, 0).UTC(),
			State:     ctr.State,
			Labels:    ctr.Labels,
			Project:   ctr.Labels[composeProjectLabel],
			Detail:    ctr.Image,
			InUse:     ctr.State == "running" || ctr.State == "paused" || ctr.State == "restarting",
		})
	}

	for _, img := range du.Images {
		if img == nil {
			continue
		}
		tags := realTags(img.RepoTags)
		name := shortImageID(img.ID)
		if len(tags) > 0 {
			name = tags[0]
		}
		unique := img.Size
		if img.SharedSize > 0 {
			unique -= img.SharedSize
		}
		users := imageUsers[img.ID]
		inv.Items = append(inv.Items, domain.StorageItem{
			Kind:      domain.StorageImage,
			ID:        img.ID,
			Name:      name,
			SizeBytes: max(unique, 0),
			SizeKnown: true,
			CreatedAt: time.Unix(img.Created, 0).UTC(),
			Labels:    img.Labels,
			Tags:      tags,
			UsedBy:    users,
			InUse:     img.Containers > 0 || len(users) > 0,
		})
	}

	for _, vol := range du.Volumes {
		if vol == nil {
			continue
		}
		item := domain.StorageItem{
			Kind:    domain.StorageVolume,
			ID:      vol.Name,
			Name:    vol.Name,
			Labels:  vol.Labels,
			Project: vol.Labels[composeProjectLabel],
			Detail:  vol.Driver,
			UsedBy:  volumeUsers[vol.Name],
		}
		if t, err := time.Parse(time.RFC3339, vol.CreatedAt); err == nil {
			item.CreatedAt = t.UTC()
		}
		// UsageData is nil, or Size is -1, when the driver cannot size it.
		if vol.UsageData != nil {
			if vol.UsageData.Size >= 0 {
				item.SizeBytes = vol.UsageData.Size
				item.SizeKnown = true
			}
			item.InUse = vol.UsageData.RefCount > 0
		}
		item.InUse = item.InUse || len(item.UsedBy) > 0
		inv.Items = append(inv.Items, item)
	}

	for _, bc := range du.BuildCache {
		if bc == nil {
			continue
		}
		item := domain.StorageItem{
			Kind:      domain.StorageBuildCache,
			ID:        bc.ID,
			Name:      truncate(bc.Description, 80),
			SizeBytes: max(bc.Size, 0),
			SizeKnown: true,
			CreatedAt: bc.CreatedAt.UTC(),
			Detail:    bc.Type,
			Shared:    bc.Shared,
			InUse:     bc.InUse,
		}
		if bc.LastUsedAt != nil {
			item.LastUsedAt = bc.LastUsedAt.UTC()
		}
		inv.Items = append(inv.Items, item)
	}

	return inv
}

func (c *Client) PruneBuildCache(ctx context.Context) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, buildCachePruneTimeout)
	defer cancel()

	// All: true matches `docker builder prune -a`: every record not in use,
	// not just dangling ones. In-use records are never removed.
	report, err := c.sdk.BuildCachePrune(ctx, dockertypes.BuildCachePruneOptions{All: true})
	if err != nil {
		return 0, wrapErr("prune build cache", err)
	}
	return report.SpaceReclaimed, nil
}

// realTags drops the "<none>:<none>" placeholder the daemon reports for
// dangling images.
func realTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != "" && t != "<none>:<none>" {
			out = append(out, t)
		}
	}
	return out
}

func shortImageID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
