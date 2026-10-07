package docker

import (
	"context"
	"time"

	dockertypes "github.com/docker/docker/api/types"

	"dockvista/internal/core/domain"
)

// diskUsageTimeout is longer than defaultCallTimeout: /system/df sizes every
// layer and volume, and on a large host that takes several seconds.
const diskUsageTimeout = 30 * time.Second

func (c *Client) SystemInfo(ctx context.Context) (domain.SystemInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	info, err := c.sdk.Info(ctx)
	if err != nil {
		return domain.SystemInfo{}, wrapErr("system info", err)
	}
	return domain.SystemInfo{
		Name:              info.Name,
		ServerVersion:     info.ServerVersion,
		OperatingSystem:   info.OperatingSystem,
		OSType:            info.OSType,
		Architecture:      info.Architecture,
		KernelVersion:     info.KernelVersion,
		StorageDriver:     info.Driver,
		CPUs:              info.NCPU,
		MemoryBytes:       info.MemTotal,
		Containers:        info.Containers,
		ContainersRunning: info.ContainersRunning,
		ContainersPaused:  info.ContainersPaused,
		ContainersStopped: info.ContainersStopped,
		Images:            info.Images,
	}, nil
}

func (c *Client) DiskUsage(ctx context.Context) (domain.DiskUsage, error) {
	ctx, cancel := context.WithTimeout(ctx, diskUsageTimeout)
	defer cancel()

	du, err := c.sdk.DiskUsage(ctx, dockertypes.DiskUsageOptions{})
	if err != nil {
		return domain.DiskUsage{}, wrapErr("disk usage", err)
	}
	return mapDiskUsage(du), nil
}

// mapDiskUsage follows the docker CLI's `system df` accounting: an image is
// reclaimable when no container uses it, minus the layers it shares with
// images that are in use.
func mapDiskUsage(du dockertypes.DiskUsage) domain.DiskUsage {
	var out domain.DiskUsage

	out.Images.SizeBytes = du.LayersSize
	for _, img := range du.Images {
		if img == nil {
			continue
		}
		out.Images.Count++
		if img.Containers > 0 {
			out.Images.Active++
			continue
		}
		unique := img.Size
		if img.SharedSize > 0 {
			unique -= img.SharedSize
		}
		if unique > 0 {
			out.Images.ReclaimableBytes += unique
		}
	}

	for _, ctr := range du.Containers {
		if ctr == nil {
			continue
		}
		out.Containers.Count++
		out.Containers.SizeBytes += ctr.SizeRw
		if ctr.State == "running" {
			out.Containers.Active++
		} else {
			out.Containers.ReclaimableBytes += ctr.SizeRw
		}
	}

	for _, vol := range du.Volumes {
		if vol == nil {
			continue
		}
		out.Volumes.Count++
		// UsageData is nil or -1 when the driver cannot size the volume.
		var size int64
		var refs int64
		if vol.UsageData != nil {
			size = max(vol.UsageData.Size, 0)
			refs = vol.UsageData.RefCount
		}
		out.Volumes.SizeBytes += size
		if refs > 0 {
			out.Volumes.Active++
		} else {
			out.Volumes.ReclaimableBytes += size
		}
	}

	for _, bc := range du.BuildCache {
		if bc == nil {
			continue
		}
		out.BuildCache.Count++
		out.BuildCache.SizeBytes += bc.Size
		if bc.InUse {
			out.BuildCache.Active++
		} else if !bc.Shared {
			out.BuildCache.ReclaimableBytes += bc.Size
		}
	}

	return out
}
