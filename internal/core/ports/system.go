package ports

import (
	"context"

	"dockvista/internal/core/domain"
)

// SystemClient is the boundary between the system service and the daemon's
// info and disk-usage endpoints.
type SystemClient interface {
	SystemInfo(ctx context.Context) (domain.SystemInfo, error)
	// DiskUsage walks every image layer, container layer, volume, and build
	// cache record. It is the most expensive read the daemon offers.
	DiskUsage(ctx context.Context) (domain.DiskUsage, error)
	// Ping is the cheap daemon reachability check used by /readyz.
	Ping(ctx context.Context) (domain.EngineInfo, error)
}
