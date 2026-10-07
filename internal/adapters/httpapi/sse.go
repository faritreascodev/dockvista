package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/stdcopy"

	"dockvista/internal/core/domain"
)

const defaultLogTail = "200"
const maxLogDownloadBytes = 50 << 20

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
	q := r.URL.Query()
	tail := q.Get("tail")
	if tail == "" {
		tail = defaultLogTail
	} else if !validTailPattern.MatchString(tail) {
		writeError(w, http.StatusBadRequest, "invalid tail parameter")
		return
	}
	since := q.Get("since")
	if !isValidSince(since) {
		writeError(w, http.StatusBadRequest, "invalid since parameter")
		return
	}
	timestamps := q.Get("timestamps") == "1"
	follow := q.Get("follow") != "0"
	plain := q.Get("format") == "plain" || q.Get("download") == "1"
	if q.Get("download") == "1" {
		follow = false
	}

	release := h.acquireStream(w)
	if release == nil {
		return
	}
	defer release()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	multiplexed, err := h.c(r).LogsMultiplexed(ctx, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	stream, err := h.c(r).StreamLogs(ctx, id, domain.LogStreamOptions{
		Tail:       tail,
		Follow:     follow,
		Timestamps: timestamps,
		Since:      since,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	defer stream.Close()

	lines := make(chan domain.LogLine, 64)
	if multiplexed {
		go demuxLogs(ctx, stream, lines, timestamps)
	} else {
		go scanRaw(ctx, stream, lines, timestamps)
	}

	if plain {
		h.writePlainLogs(w, r, id, lines)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

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

func (h *handlers) writePlainLogs(w http.ResponseWriter, r *http.Request, id string, lines <-chan domain.LogLine) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.log"`, downloadFilename(id)))
	}
	w.WriteHeader(http.StatusOK)
	written := 0
	for line := range lines {
		text := line.Message
		if line.Timestamp != "" {
			text = line.Timestamp + " " + line.Message
		}
		n, err := fmt.Fprintln(w, text)
		written += n
		if err != nil || written > maxLogDownloadBytes {
			return
		}
	}
}

// scanRaw reads a TTY log stream, which is not framed, as plain stdout lines.
func scanRaw(ctx context.Context, r io.Reader, out chan<- domain.LogLine, timestamps bool) {
	defer close(out)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := domain.LogLine{Stream: "stdout", Message: scanner.Text()}
		if timestamps {
			line.Timestamp, line.Message = splitLogTimestamp(line.Message)
		}
		select {
		case out <- line:
		case <-ctx.Done():
			return
		}
	}
}

// demuxLogs splits Docker's multiplexed log stream into tagged lines and
// sends them on out, closing out once the source is exhausted or ctx is
// cancelled. It never blocks forever on a full channel: every send is
// itself cancellable, so a disconnected client can't leak this goroutine.
func demuxLogs(ctx context.Context, src io.Reader, out chan<- domain.LogLine, timestamps bool) {
	defer close(out)

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	var wg sync.WaitGroup
	wg.Add(2)
	go scanInto(ctx, stdoutR, "stdout", out, timestamps, &wg)
	go scanInto(ctx, stderrR, "stderr", out, timestamps, &wg)

	_, copyErr := stdcopy.StdCopy(stdoutW, stderrW, src)
	stdoutW.CloseWithError(copyErr)
	stderrW.CloseWithError(copyErr)

	wg.Wait()
}

func scanInto(ctx context.Context, r *io.PipeReader, streamName string, out chan<- domain.LogLine, timestamps bool, wg *sync.WaitGroup) {
	defer wg.Done()
	defer r.Close()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := domain.LogLine{Stream: streamName, Message: scanner.Text()}
		if timestamps {
			line.Timestamp, line.Message = splitLogTimestamp(line.Message)
		}
		select {
		case out <- line:
		case <-ctx.Done():
			return
		}
	}
}

func splitLogTimestamp(line string) (ts, msg string) {
	i := strings.IndexByte(line, ' ')
	if i < 20 {
		return "", line
	}
	candidate := line[:i]
	if _, err := time.Parse(time.RFC3339Nano, candidate); err == nil {
		return candidate, line[i+1:]
	}
	if _, err := time.Parse(time.RFC3339, candidate); err == nil {
		return candidate, line[i+1:]
	}
	return "", line
}
