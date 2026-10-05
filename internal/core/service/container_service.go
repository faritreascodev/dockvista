// Package service implements the core application logic: it orchestrates
// the DockerClient port and the ContainerStore cache, and is the only layer
// that knows about both. HTTP handlers call this, never the Docker client
// directly.
package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

// statsCacheTTL collapses repeated samples for the same container. The UI
// polls every few seconds per row; identical in-flight calls share one
// daemon request via statsFlight, and a hit inside the window does not
// call the daemon again.
const statsCacheTTL = time.Second

// ContainerService is the use-case layer for container inspection and
// lifecycle control.
type ContainerService struct {
	docker ports.DockerClient
	store  ports.ContainerStore
	log    *slog.Logger

	listFlight  flight
	statsFlight flight

	statsMu    sync.Mutex
	statsCache map[string]cachedStats
}

type cachedStats struct {
	stats domain.Stats
	until time.Time
}

// New builds a ContainerService. logger may be nil, in which case a no-op
// logger is used.
func New(docker ports.DockerClient, store ports.ContainerStore, logger *slog.Logger) *ContainerService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ContainerService{
		docker:     docker,
		store:      store,
		log:        logger,
		statsCache: map[string]cachedStats{},
	}
}

// Engine reports Docker daemon reachability and version.
func (s *ContainerService) Engine(ctx context.Context) (domain.EngineInfo, error) {
	info, err := s.docker.Ping(ctx)
	if err != nil {
		return domain.EngineInfo{}, fmt.Errorf("service: engine status: %w", err)
	}
	// The count comes from the cache. Ping used to list every container
	// just to measure len, which is a full daemon read on a health check.
	info.Containers = len(s.store.List())
	return info, nil
}

// ListContainers returns the cached container list. It never hits the
// Docker daemon directly; freshness is maintained by RunCollector.
func (s *ContainerService) ListContainers() []domain.Container {
	return s.store.List()
}

// GetContainer returns a single cached container, or domain.ErrNotFound if
// the ID isn't known.
func (s *ContainerService) GetContainer(id string) (domain.Container, error) {
	c, ok := s.store.Get(id)
	if !ok {
		return domain.Container{}, fmt.Errorf("service: get container %q: %w", id, domain.ErrNotFound)
	}
	return c, nil
}

// Stats takes a live one-shot resource usage sample for a container. Unlike
// ListContainers this always hits the daemon, since CPU/memory usage is
// only meaningful in near-real-time.
func (s *ContainerService) Stats(ctx context.Context, id string) (domain.Stats, error) {
	if stats, ok := s.cachedStats(id); ok {
		return stats, nil
	}

	v, err := s.statsFlight.Do(id, func() (any, error) {
		if stats, ok := s.cachedStats(id); ok {
			return stats, nil
		}
		stats, err := s.docker.ContainerStats(ctx, id)
		if err != nil {
			return domain.Stats{}, err
		}
		s.statsMu.Lock()
		s.statsCache[id] = cachedStats{stats: stats, until: time.Now().Add(statsCacheTTL)}
		s.statsMu.Unlock()
		return stats, nil
	})
	if err != nil {
		return domain.Stats{}, fmt.Errorf("service: container stats: %w", err)
	}
	stats, _ := v.(domain.Stats)
	return stats, nil
}

func (s *ContainerService) cachedStats(id string) (domain.Stats, bool) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	entry, ok := s.statsCache[id]
	if !ok || time.Now().After(entry.until) {
		return domain.Stats{}, false
	}
	return entry.stats, true
}

func (s *ContainerService) Start(ctx context.Context, id string) error {
	if err := s.docker.StartContainer(ctx, id); err != nil {
		return fmt.Errorf("service: start container: %w", err)
	}
	return nil
}

func (s *ContainerService) Stop(ctx context.Context, id string) error {
	if err := s.docker.StopContainer(ctx, id); err != nil {
		return fmt.Errorf("service: stop container: %w", err)
	}
	return nil
}

func (s *ContainerService) Pause(ctx context.Context, id string) error {
	if err := s.docker.PauseContainer(ctx, id); err != nil {
		return fmt.Errorf("service: pause container: %w", err)
	}
	return nil
}

func (s *ContainerService) Unpause(ctx context.Context, id string) error {
	if err := s.docker.UnpauseContainer(ctx, id); err != nil {
		return fmt.Errorf("service: unpause container: %w", err)
	}
	return nil
}

func (s *ContainerService) Restart(ctx context.Context, id string) error {
	if err := s.docker.RestartContainer(ctx, id); err != nil {
		return fmt.Errorf("service: restart container: %w", err)
	}
	return nil
}

// StreamLogs proxies a live log stream for the given container.
func (s *ContainerService) StreamLogs(ctx context.Context, id, tail string) (io.ReadCloser, error) {
	r, err := s.docker.StreamLogs(ctx, id, tail)
	if err != nil {
		return nil, fmt.Errorf("service: stream logs: %w", err)
	}
	return r, nil
}

// LogsMultiplexed reports whether the log stream is Docker's framed
// stdout/stderr multiplex. False means a raw TTY stream.
func (s *ContainerService) LogsMultiplexed(ctx context.Context, id string) (bool, error) {
	multiplexed, err := s.docker.LogsMultiplexed(ctx, id)
	if err != nil {
		return false, fmt.Errorf("service: log mode: %w", err)
	}
	return multiplexed, nil
}

func (s *ContainerService) CreateExec(ctx context.Context, id string) (string, error) {
	execID, err := s.docker.CreateExec(ctx, id)
	if err != nil {
		return "", fmt.Errorf("service: create exec: %w", err)
	}
	return execID, nil
}

func (s *ContainerService) AttachExec(ctx context.Context, execID string) (io.ReadWriteCloser, error) {
	conn, err := s.docker.AttachExec(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("service: attach exec: %w", err)
	}
	return conn, nil
}

// Create makes a new container and immediately refreshes the cache so it
// shows up without waiting for the next poll or daemon event.
func (s *ContainerService) Create(ctx context.Context, spec domain.ContainerSpec) (string, bool, error) {
	id, started, err := s.docker.CreateContainer(ctx, spec)
	// Refresh whenever the daemon actually created a container, including
	// the case where start failed and the caller still needs the id.
	if id != "" {
		if refreshErr := s.RefreshOnce(ctx); refreshErr != nil {
			s.log.Warn("post-create cache refresh failed", "error", refreshErr)
		}
	}
	if err != nil {
		return id, started, fmt.Errorf("service: create container: %w", err)
	}
	return id, true, nil
}

func (s *ContainerService) Remove(ctx context.Context, id string, force bool) error {
	if err := s.docker.RemoveContainer(ctx, id, force); err != nil {
		return fmt.Errorf("service: remove container: %w", err)
	}
	if refreshErr := s.RefreshOnce(ctx); refreshErr != nil {
		s.log.Warn("post-remove cache refresh failed", "error", refreshErr)
	}
	return nil
}

func (s *ContainerService) Inspect(ctx context.Context, id string) ([]byte, error) {
	raw, err := s.docker.InspectContainer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service: inspect container: %w", err)
	}
	return raw, nil
}

func (s *ContainerService) ResizeExec(ctx context.Context, execID string, rows, cols uint) error {
	if err := s.docker.ResizeExec(ctx, execID, rows, cols); err != nil {
		return fmt.Errorf("service: resize exec: %w", err)
	}
	return nil
}

// RefreshOnce fetches the current container list from Docker and updates
// the cache. Concurrent callers share one in-flight list: a burst of daemon
// events must not become a burst of ContainerList calls.
func (s *ContainerService) RefreshOnce(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := s.listFlight.Do("containers", func() (any, error) {
		// Detach from the caller's context. The first caller is often an
		// HTTP request; cancelling that must not fail the shared refresh
		// everyone else is waiting on.
		cctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		containers, err := s.docker.ListContainers(cctx)
		if err != nil {
			return nil, fmt.Errorf("service: refresh containers: %w", err)
		}
		s.store.Replace(containers)
		return nil, nil
	})
	return err
}

// RunCollector polls the Docker daemon on a fixed interval and keeps the
// store up to date until ctx is cancelled. It's meant to run in its own
// goroutine for the lifetime of the process.
func (s *ContainerService) RunCollector(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Populate the cache immediately instead of waiting for the first tick.
	if err := s.RefreshOnce(ctx); err != nil {
		s.log.Error("initial container list refresh failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.RefreshOnce(ctx); err != nil {
				s.log.Error("container list refresh failed", "error", err)
			}
		}
	}
}
