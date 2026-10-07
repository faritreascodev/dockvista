// Package config loads DockVista's runtime configuration from environment
// variables, with sane defaults so the binary runs with zero setup against
// a local Docker daemon.
package config

import (
	"os"
	"strings"
	"time"
)

// Config holds every environment-tunable setting.
type Config struct {
	// Addr is the address the HTTP server listens on. The default is
	// loopback-only: reaching DockVista is reaching the Docker socket, so
	// exposing it to the network has to be an explicit choice.
	Addr string
	// PollInterval is how often the background collector refreshes the
	// container list cache as a safety net; the daemon's own event stream
	// (see internal/adapters/broker) drives near-instant refreshes on top
	// of this, so the default only needs to catch a missed event.
	PollInterval time.Duration
	// ShutdownTimeout bounds how long graceful shutdown waits for
	// in-flight requests (like a log stream) to finish.
	ShutdownTimeout time.Duration
	// DataDir is where accounts, invites, the session signing key, and
	// audit.log are persisted, so login survives a process restart.
	DataDir string
	// SetupToken, when set, is the token required by POST /api/auth/setup.
	// Empty means the process generates one and logs it once.
	SetupToken string
	// CookieSecure marks the session cookie Secure even when this process
	// itself is plain HTTP because TLS terminates at a proxy in front.
	CookieSecure bool
	// ReadOnly rejects every request that changes daemon state or opens a
	// shell, so an instance can be shown without handing out the socket.
	ReadOnly bool
	// IdleTimeout signs a session out after this much quiet. Zero disables
	// idle expiry; the cookie's absolute 8-hour lifetime still applies.
	IdleTimeout time.Duration
}

// Load reads configuration from the environment, falling back to defaults
// for anything unset or invalid.
func Load() Config {
	return Config{
		Addr:            envOr("DOCKVISTA_ADDR", "127.0.0.1:8080"),
		PollInterval:    envDurationOr("DOCKVISTA_POLL_INTERVAL", 30*time.Second),
		ShutdownTimeout: envDurationOr("DOCKVISTA_SHUTDOWN_TIMEOUT", 10*time.Second),
		DataDir:         envOr("DOCKVISTA_DATA_DIR", "./data"),
		SetupToken:      os.Getenv("DOCKVISTA_SETUP_TOKEN"),
		CookieSecure:    envBool("DOCKVISTA_COOKIE_SECURE"),
		ReadOnly:        envBool("DOCKVISTA_READ_ONLY"),
		IdleTimeout:     envDurationOr("DOCKVISTA_IDLE_TIMEOUT", 30*time.Minute),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
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
