package domain

import "time"

// Event is a single occurrence reported by the Docker daemon's own event
// stream (container start/die, image pull, volume/network changes, ...).
// It's forwarded to the frontend near-instantly over SSE so the UI can
// refetch the affected resource instead of waiting for the next poll tick.
type Event struct {
	Type       string            `json:"type"`
	Action     string            `json:"action"`
	ActorID    string            `json:"actorId"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Time       time.Time         `json:"time"`
}
