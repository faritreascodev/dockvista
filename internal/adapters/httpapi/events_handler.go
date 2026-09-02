package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"dockvista/internal/core/domain"
)

// eventSubscriber is the subset of *broker.Broker[domain.Event] the HTTP
// layer depends on (consumer-defined interface, same decoupling pattern as
// containerService/authService).
type eventSubscriber interface {
	Subscribe() (<-chan domain.Event, func())
}

// handleEvents streams the daemon's event feed to the browser over SSE, one
// broker subscription per connected tab. Same flush-per-message pattern as
// handleContainerLogs.
func (h *handlers) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	events, unsubscribe := h.events.Subscribe()
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, open := <-events:
			if !open {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
