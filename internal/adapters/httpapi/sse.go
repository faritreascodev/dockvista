package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"

	"github.com/docker/docker/pkg/stdcopy"

	"dockvista/internal/core/domain"
)

const defaultLogTail = "200"

var validTailPattern = regexp.MustCompile(`^(all|[0-9]{1,7})$`)

// handleContainerLogs streams a container's stdout/stderr as Server-Sent
// Events. Using SSE instead of a blocking io.Copy means each line is
// flushed to the client as soon as it's available, and the stream tears
// down as soon as either side disconnects — no lingering goroutine reading
// into a buffer nobody drains anymore.
func (h *handlers) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = defaultLogTail
	} else if !validTailPattern.MatchString(tail) {
		writeError(w, http.StatusBadRequest, "invalid tail parameter")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	stream, err := h.svc.StreamLogs(ctx, id, tail)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	lines := make(chan domain.LogLine, 64)
	go demuxLogs(ctx, stream, lines)

	for {
		select {
		case <-ctx.Done():
			return
		case line, open := <-lines:
			if !open {
				return
			}
			payload, err := json.Marshal(line)
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

// demuxLogs splits Docker's multiplexed log stream into tagged lines and
// sends them on out, closing out once the source is exhausted or ctx is
// cancelled. It never blocks forever on a full channel: every send is
// itself cancellable, so a disconnected client can't leak this goroutine.
func demuxLogs(ctx context.Context, src io.Reader, out chan<- domain.LogLine) {
	defer close(out)

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	var wg sync.WaitGroup
	wg.Add(2)
	go scanInto(ctx, stdoutR, "stdout", out, &wg)
	go scanInto(ctx, stderrR, "stderr", out, &wg)

	_, copyErr := stdcopy.StdCopy(stdoutW, stderrW, src)
	stdoutW.CloseWithError(copyErr)
	stderrW.CloseWithError(copyErr)

	wg.Wait()
}

func scanInto(ctx context.Context, r *io.PipeReader, streamName string, out chan<- domain.LogLine, wg *sync.WaitGroup) {
	defer wg.Done()
	defer r.Close()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case out <- domain.LogLine{Stream: streamName, Message: scanner.Text()}:
		case <-ctx.Done():
			return
		}
	}
}
