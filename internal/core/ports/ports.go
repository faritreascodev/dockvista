// Package ports defines the interfaces the core service depends on. Concrete
// implementations live in internal/adapters/*; the core never imports them,
// only the other way around (dependency inversion).
package ports

import (
	"context"
	"io"

	"dockvista/internal/core/domain"
)

// DockerClient is the boundary between the core service and the Docker
// engine. internal/adapters/docker implements this against the real SDK;
// tests can implement it with a fake.
type DockerClient interface {
	// Ping reports daemon reachability and version. It must not list
	// containers — the service fills the count from its own cache.
	Ping(ctx context.Context) (domain.EngineInfo, error)
	ListContainers(ctx context.Context) ([]domain.Container, error)
	ContainerStats(ctx context.Context, id string) (domain.Stats, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	PauseContainer(ctx context.Context, id string) error
	UnpauseContainer(ctx context.Context, id string) error
	RestartContainer(ctx context.Context, id string) error
	// StreamLogs returns a live stream of the container's stdout/stderr.
	// The caller owns the returned ReadCloser and must Close it; the
	// implementation must stop producing data promptly once ctx is done.
	StreamLogs(ctx context.Context, id string, tail string) (io.ReadCloser, error)
	// LogsMultiplexed reports whether StreamLogs returns Docker's 8-byte
	// framed stdout/stderr stream. A TTY container emits a raw byte stream
	// instead, and framing that corrupts the log.
	LogsMultiplexed(ctx context.Context, id string) (bool, error)
	// Events subscribes to the daemon's own event feed. Both channels close
	// once ctx is cancelled or the underlying connection ends.
	Events(ctx context.Context) (<-chan domain.Event, <-chan error)

	// CreateExec starts a new /bin/sh exec session in the container and
	// returns its ID. AttachExec then hijacks the connection for bytes to
	// flow both ways; the caller owns the returned connection and must
	// close it. ResizeExec adjusts the pty size for an attached session.
	CreateExec(ctx context.Context, containerID string) (execID string, err error)
	AttachExec(ctx context.Context, execID string) (io.ReadWriteCloser, error)
	ResizeExec(ctx context.Context, execID string, rows, cols uint) error

	// CreateContainer creates the container and tries to start it.
	// When create succeeds and start fails, id is the new container and
	// started is false — the caller must not drop that id.
	CreateContainer(ctx context.Context, spec domain.ContainerSpec) (id string, started bool, err error)
	RemoveContainer(ctx context.Context, id string, force bool) error
	// InspectContainer returns the daemon's raw JSON for the container,
	// passed straight through to the frontend's Inspect tab.
	InspectContainer(ctx context.Context, id string) ([]byte, error)

	Close() error
}

// ContainerStore is a read-through cache of the last known container list,
// refreshed by the background collector and served to HTTP requests without
// hitting the Docker daemon on every call.
type ContainerStore interface {
	Replace(containers []domain.Container)
	List() []domain.Container
	Get(id string) (domain.Container, bool)
}
