package ports

import (
	"context"

	"dockvista/internal/core/domain"
)

// SwarmClient is the daemon's swarm inventory. Init/join/scale stay off
// this interface until the read view is honest on a non-swarm engine.
type SwarmClient interface {
	SwarmSnapshot(ctx context.Context) (domain.SwarmSnapshot, error)
}
