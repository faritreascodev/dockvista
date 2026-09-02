package domain

import "time"

// Network is the engine-agnostic representation of a Docker network.
type Network struct {
	ID         string
	Name       string
	Driver     string
	Scope      string
	Internal   bool
	Attachable bool
	Created    time.Time
	Labels     map[string]string
	// Containers currently attached to this network, keyed by container ID.
	Containers map[string]string // container ID -> container name
}
