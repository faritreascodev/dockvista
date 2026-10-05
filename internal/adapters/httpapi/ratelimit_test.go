package httpapi

import (
	"fmt"
	"testing"
	"time"
)

func TestRateLimiter_AllowsUpToLimitPerKey(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !rl.allow("a") {
			t.Fatalf("request %d for key a denied, want allowed", i+1)
		}
	}
	if rl.allow("a") {
		t.Fatal("fourth request for key a allowed, want denied")
	}
	if !rl.allow("b") {
		t.Fatal("key b was affected by key a's limit")
	}
}

func TestRateLimiter_SweepDropsIdleKeys(t *testing.T) {
	window := 50 * time.Millisecond
	rl := newRateLimiter(10, window)
	for i := 0; i < 300; i++ {
		rl.allow(fmt.Sprintf("client-%d", i))
	}
	time.Sleep(2 * window)

	// The first call after a window has elapsed triggers the sweep.
	rl.allow("fresh")

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if n := len(rl.visitors); n != 1 {
		t.Fatalf("after sweep %d keys remain, want 1 (only the fresh client)", n)
	}
}
