// Package docker adapts the official Docker SDK to the ports.DockerClient
// interface consumed by the core service. It is the only package in the
// module allowed to import github.com/docker/docker.
package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

// defaultCallTimeout bounds any single Docker daemon call that isn't
// already governed by a caller-supplied deadline (e.g. background polling).
const defaultCallTimeout = 10 * time.Second

// stopGraceSeconds is how long we ask the daemon to wait for a clean exit
// before it sends SIGKILL. The call's own context timeout must exceed this,
// or Go's deadline can fire right as the daemon-side stop is still in its
// grace period, misreporting a successful stop as ErrEngineTimeout.
const stopGraceSeconds = 10

// Client wraps the official Docker SDK client.
type Client struct {
	sdk *dockerclient.Client
}

// New connects to the Docker daemon using the standard environment
// variables (DOCKER_HOST, DOCKER_CERT_PATH, ...), negotiating the API
// version so the client works across daemon versions.
func New() (*Client, error) {
	sdk, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker: connect to daemon: %w", err)
	}
	return &Client{sdk: sdk}, nil
}

// Close releases the underlying HTTP transport.
func (c *Client) Close() error {
	if err := c.sdk.Close(); err != nil {
		return fmt.Errorf("docker: close client: %w", err)
	}
	return nil
}

// Ping reports whether the daemon is reachable and its version.
func (c *Client) Ping(ctx context.Context) (domain.EngineInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	pong, err := c.sdk.Ping(ctx)
	if err != nil {
		return domain.EngineInfo{Reachable: false}, wrapErr("ping engine", err)
	}

	return domain.EngineInfo{
		Version:   pong.APIVersion,
		Reachable: true,
	}, nil
}

// ListContainers returns every container known to the daemon, running or not.
func (c *Client) ListContainers(ctx context.Context) ([]domain.Container, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	raw, err := c.sdk.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, wrapErr("list containers", err)
	}

	out := make([]domain.Container, 0, len(raw))
	for _, c := range raw {
		out = append(out, toDomainContainer(c))
	}
	return out, nil
}

// ContainerStats takes a single, non-streaming resource usage sample.
func (c *Client) ContainerStats(ctx context.Context, id string) (domain.Stats, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	resp, err := c.sdk.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return domain.Stats{}, wrapErr("read container stats", err)
	}
	defer resp.Body.Close()

	var raw dockertypes.StatsJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return domain.Stats{}, fmt.Errorf("docker: decode stats payload: %w", err)
	}

	usage := memoryUsage(raw)
	return domain.Stats{
		ContainerID:   id,
		CPUPercent:    cpuPercent(raw),
		MemoryUsage:   usage,
		MemoryLimit:   raw.MemoryStats.Limit,
		MemoryPercent: memoryPercent(usage, raw.MemoryStats.Limit),
		SampledAt:     time.Now().UTC(),
	}, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	if err := c.sdk.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return wrapErr("start container", err)
	}
	return nil
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, (stopGraceSeconds+5)*time.Second)
	defer cancel()

	timeout := stopGraceSeconds
	if err := c.sdk.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout}); err != nil {
		return wrapErr("stop container", err)
	}
	return nil
}

func (c *Client) PauseContainer(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	if err := c.sdk.ContainerPause(ctx, id); err != nil {
		return wrapErr("pause container", err)
	}
	return nil
}

func (c *Client) UnpauseContainer(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	if err := c.sdk.ContainerUnpause(ctx, id); err != nil {
		return wrapErr("unpause container", err)
	}
	return nil
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, (stopGraceSeconds+5)*time.Second)
	defer cancel()

	timeout := stopGraceSeconds
	if err := c.sdk.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout}); err != nil {
		return wrapErr("restart container", err)
	}
	return nil
}

// StreamLogs opens a live stdout/stderr stream. The caller's context governs
// how long the stream stays open; closing it (e.g. on client disconnect)
// stops the underlying read promptly instead of leaking a goroutine.
func (c *Client) StreamLogs(ctx context.Context, id string, tail string) (io.ReadCloser, error) {
	out, err := c.sdk.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       tail,
		Timestamps: false,
	})
	if err != nil {
		return nil, wrapErr("stream container logs", err)
	}
	return out, nil
}

// LogsMultiplexed reports whether this container's log stream is framed.
// TTY containers write a raw PTY stream; stdcopy on that stream drops bytes.
func (c *Client) LogsMultiplexed(ctx context.Context, id string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	info, err := c.sdk.ContainerInspect(ctx, id)
	if err != nil {
		return false, wrapErr("inspect container logs", err)
	}
	tty := info.Config != nil && info.Config.Tty
	return !tty, nil
}

// wrapErr classifies a raw SDK error against domain sentinels so the HTTP
// layer can map it to the right status code without string matching.
func wrapErr(action string, err error) error {
	switch {
	case dockerclient.IsErrNotFound(err):
		return fmt.Errorf("docker: %s: %w: %w", action, domain.ErrNotFound, err)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("docker: %s: %w: %w", action, domain.ErrEngineTimeout, err)
	default:
		return fmt.Errorf("docker: %s: %w: %w", action, domain.ErrEngine, err)
	}
}

var _ ports.DockerClient = (*Client)(nil)
