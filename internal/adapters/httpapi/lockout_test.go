package httpapi

import "testing"

func TestLoginGuardLocksAfterLimit(t *testing.T) {
	g := newLoginGuard()
	if g.blocked("127.0.0.1", "admin") {
		t.Fatal("fresh guard is locked")
	}
	locked := false
	for i := 0; i < loginFailLimit; i++ {
		locked = g.fail("127.0.0.1", "admin")
	}
	if !locked {
		t.Fatal("expected lockout on the last failure")
	}
	if !g.blocked("127.0.0.1", "admin") {
		t.Fatal("expected blocked after limit")
	}
	if g.blocked("127.0.0.1", "other") {
		t.Fatal("lockout must be per username")
	}
	g.success("127.0.0.1", "admin")
	if g.blocked("127.0.0.1", "admin") {
		t.Fatal("success should clear the lock")
	}
}
