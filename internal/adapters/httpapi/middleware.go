package httpapi

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type middleware func(http.Handler) http.Handler

// contentSecurityPolicy allows scripts only from this origin, so an injected
// <script> or inline handler never runs next to a session that can open a
// shell. style-src keeps 'unsafe-inline' because xterm.js writes <style>
// elements at runtime; styles cannot read the cookie or call the API.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// securityHeaders sets the headers that apply to both the API and the
// embedded UI.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// denyWhenReadOnly blocks a route outright in read-only mode. It wraps the
// handler itself rather than checking methods, so a GET that opens a shell
// (exec) is covered the same way as a DELETE.
func denyWhenReadOnly(readOnly bool) middleware {
	return func(next http.Handler) http.Handler {
		if !readOnly {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusForbidden, "this DockVista instance is read-only")
		})
	}
}

// denyWhenViewer blocks Docker-mutating routes (and exec) for the Viewer
// role. Combined with denyWhenReadOnly so a process-wide flag still wins.
func denyWhenViewer() middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !principalFrom(r.Context()).Role.CanMutate() {
				writeError(w, http.StatusForbidden, "your role cannot change this instance")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func denyUnlessAdmin() middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !principalFrom(r.Context()).Role.CanManageUsers() {
				writeError(w, http.StatusForbidden, "only an admin can manage users")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func chain(h http.Handler, mws ...middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// recoverMiddleware turns a panic in any handler into a 500 response instead
// of crashing the whole server — a single bad request must never take down
// the process.
func recoverMiddleware(log *slog.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered", "panic", rec, "path", r.URL.Path)
					writeError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Flush lets streaming handlers (SSE) keep using http.Flusher through the
// wrapped ResponseWriter.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack lets the WebSocket handler (exec) take over the raw connection
// through the wrapped ResponseWriter — without this, wrapping every request
// in statusRecorder for logging silently breaks every Upgrade: websocket
// request with a 501, since the library can no longer find a Hijacker.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return hj.Hijack()
}

// Unwrap lets http.ResponseController (the mechanism newer stdlib-adjacent
// code, including some WebSocket libraries, uses to reach through wrapper
// ResponseWriters) find the real one directly.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// loggingMiddleware logs one line per request with method, path, status and
// latency, plus a per-request ID for correlating logs with client reports.
func loggingMiddleware(log *slog.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := uuid.NewString()
			ctx := withRequestID(r.Context(), reqID)

			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r.WithContext(ctx))

			log.Info("request",
				"id", reqID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
