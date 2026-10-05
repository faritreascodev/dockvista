package httpapi

import "net/http"

// streamGate caps concurrent long-lived responses (logs, events, exec).
// A single authenticated session can otherwise open one daemon stream per
// request until the process runs out of file descriptors.
type streamGate struct {
	sem chan struct{}
}

func newStreamGate(n int) *streamGate {
	return &streamGate{sem: make(chan struct{}, n)}
}

func (g *streamGate) acquire() bool {
	select {
	case g.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (g *streamGate) release() {
	<-g.sem
}

func (h *handlers) acquireStream(w http.ResponseWriter) func() {
	if h.streams == nil {
		return func() {}
	}
	if !h.streams.acquire() {
		writeError(w, http.StatusTooManyRequests, "too many open streams")
		return nil
	}
	return h.streams.release
}
