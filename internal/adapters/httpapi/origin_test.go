package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginMatchesHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/containers/web/stop", nil)
	if originMatchesHost(req) {
		t.Fatal("empty origin must fail")
	}
	req.Header.Set("Origin", "http://evil.example")
	if originMatchesHost(req) {
		t.Fatal("foreign origin must fail")
	}
	setSameOrigin(req)
	if !originMatchesHost(req) {
		t.Fatalf("same origin %q vs host %q", req.Header.Get("Origin"), req.Host)
	}
}

func TestOriginFallsBackToReferer(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Header.Set("Referer", "http://"+req.Host+"/login")
	if !originMatchesHost(req) {
		t.Fatal("referer of this host should pass")
	}
}

func TestNeedsOrigin(t *testing.T) {
	get := httptest.NewRequest(http.MethodGet, "/api/containers", nil)
	if needsOrigin(get) {
		t.Fatal("GET must not require Origin")
	}
	ws := httptest.NewRequest(http.MethodGet, "/api/containers/web/exec", nil)
	ws.Header.Set("Upgrade", "websocket")
	if !needsOrigin(ws) {
		t.Fatal("websocket upgrade must require Origin")
	}
	post := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	if !needsOrigin(post) {
		t.Fatal("POST must require Origin")
	}
}
