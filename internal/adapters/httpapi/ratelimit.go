package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// rateLimiter is a per-key sliding-window counter. It exists to blunt
// brute-force and scripted-abuse traffic against a single-instance local
// tool — not to replace a real gateway-level limiter in a multi-tenant
// deployment.
type rateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	visitors map[string][]time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:    limit,
		window:   window,
		visitors: make(map[string][]time.Time),
	}
}

func (rl *rateLimiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-rl.window)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	hits := rl.visitors[key]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(rl.visitors, key)
	}
	if len(kept) >= rl.limit {
		rl.visitors[key] = kept
		rl.sweep(cutoff)
		return false
	}
	rl.visitors[key] = append(kept, now)
	rl.sweep(cutoff)
	return true
}

// sweep drops keys whose window has expired once the map is large enough
// that a scan is cheaper than unbounded growth from one-off client addresses.
func (rl *rateLimiter) sweep(cutoff time.Time) {
	if len(rl.visitors) < 256 {
		return
	}
	for key, hits := range rl.visitors {
		live := false
		for _, t := range hits {
			if t.After(cutoff) {
				live = true
				break
			}
		}
		if !live {
			delete(rl.visitors, key)
		}
	}
}

func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimitMiddleware rejects requests over limit/window per client IP with
// 429. Intended for mutating routes (login, lifecycle actions, resource
// create/delete) rather than read-only GETs.
func rateLimitMiddleware(rl *rateLimiter) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(clientKey(r)) {
				writeError(w, http.StatusTooManyRequests, "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
