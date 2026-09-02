// Package config loads DockVista's runtime configuration from environment
// variables, with sane defaults so the binary runs with zero setup against
// a local Docker daemon.
package config

import (
	"os"
	"time"
)

// Config holds every environment-tunable setting.
type Config struct {
	// Addr is the address the HTTP server listens on, e.g. ":8080".
	Addr string
	// PollInterval is how often the background collector refreshes the
	// container list cache as a safety net; the daemon's own event stream
	// (see internal/adapters/broker) drives near-instant refreshes on top
	// of this, so the default only needs to catch a missed event.
	PollInterval time.Duration
	// ShutdownTimeout bounds how long graceful shutdown waits for
	// in-flight requests (like a log stream) to finish.
	ShutdownTimeout time.Duration
	// DataDir is where the admin credentials and session signing key are
	// persisted, so login survives a process restart.
	DataDir string
}

// Load reads configuration from the environment, falling back to defaults
// for anything unset or invalid.
func Load() Config {
	return Config{
		Addr:            envOr("DOCKVISTA_ADDR", ":8080"),
		PollInterval:    envDurationOr("DOCKVISTA_POLL_INTERVAL", 30*time.Second),
		ShutdownTimeout: envDurationOr("DOCKVISTA_SHUTDOWN_TIMEOUT", 10*time.Second),
		DataDir:         envOr("DOCKVISTA_DATA_DIR", "./data"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
