package domain

import "time"

// Volume is the engine-agnostic representation of a Docker volume.
type Volume struct {
	Name       string
	Driver     string
	Mountpoint string
	CreatedAt  time.Time
	Labels     map[string]string
}
