// Package domain holds the core business types for DockVista. It has zero
// dependencies on the Docker SDK or any transport framework so it can be
// tested and reasoned about in isolation.
package domain

import "time"

// ContainerState mirrors the finite set of states the Docker engine reports.
type ContainerState string

const (
	StateCreated    ContainerState = "created"
	StateRunning    ContainerState = "running"
	StatePaused     ContainerState = "paused"
	StateRestarting ContainerState = "restarting"
	StateRemoving   ContainerState = "removing"
	StateExited     ContainerState = "exited"
	StateDead       ContainerState = "dead"
)

// Port describes a single published container port mapping.
type Port struct {
	PrivatePort uint16 `json:"privatePort"`
	PublicPort  uint16 `json:"publicPort,omitempty"`
	Type        string `json:"type"`
}

// Container is the engine-agnostic representation of a Docker container.
type Container struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	State   ContainerState    `json:"state"`
	Status  string            `json:"status"`
	Created time.Time         `json:"created"`
	Ports   []Port            `json:"ports"`
	Labels  map[string]string `json:"labels"`
}

// PortBinding maps one container port to a host port for container
// creation (as opposed to Port, which describes an already-running
// container's actual published mapping).
type PortBinding struct {
	HostPort      string
	ContainerPort string
	Protocol      string // "tcp" or "udp"
}

// ContainerSpec is what the caller supplies to create a new container.
type ContainerSpec struct {
	Image         string
	Name          string
	Env           []string
	Ports         []PortBinding
	Binds         []string // "hostPath:containerPath[:ro]", Docker's own format
	RestartPolicy string   // "no", "always", "on-failure", "unless-stopped"
}

// Stats holds a single point-in-time resource usage sample for a container.
type Stats struct {
	ContainerID   string    `json:"containerId"`
	CPUPercent    float64   `json:"cpuPercent"`
	MemoryUsage   uint64    `json:"memoryUsageBytes"`
	MemoryLimit   uint64    `json:"memoryLimitBytes"`
	MemoryPercent float64   `json:"memoryPercent"`
	SampledAt     time.Time `json:"sampledAt"`
}

// LogLine is a single line emitted by a container's stdout/stderr stream.
type LogLine struct {
	Stream  string `json:"stream"` // "stdout" or "stderr"
	Message string `json:"message"`
}

// EngineInfo describes the Docker daemon DockVista is connected to.
type EngineInfo struct {
	Version    string `json:"version"`
	Reachable  bool   `json:"reachable"`
	Containers int    `json:"containers"`
}
