package httpapi

import (
	"sync"
	"time"
)

const (
	loginFailLimit = 8
	loginLockFor   = 15 * time.Minute
)

type loginGuard struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

type loginAttempt struct {
	fails int
	until time.Time
}

func newLoginGuard() *loginGuard {
	return &loginGuard{attempts: make(map[string]*loginAttempt)}
}

func (g *loginGuard) key(ip, username string) string {
	return ip + "\x00" + username
}

func (g *loginGuard) blocked(ip, username string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	a := g.attempts[g.key(ip, username)]
	if a == nil {
		return false
	}
	if time.Now().Before(a.until) {
		return true
	}
	if !a.until.IsZero() {
		delete(g.attempts, g.key(ip, username))
	}
	return false
}

// fail records a bad password. It returns true when this attempt trips
// the lockout so the caller can audit once.
func (g *loginGuard) fail(ip, username string) (locked bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	k := g.key(ip, username)
	a := g.attempts[k]
	if a == nil {
		a = &loginAttempt{}
		g.attempts[k] = a
	}
	a.fails++
	if a.fails >= loginFailLimit {
		a.until = time.Now().Add(loginLockFor)
		return true
	}
	return false
}

func (g *loginGuard) success(ip, username string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.attempts, g.key(ip, username))
}
