package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dockvista/internal/core/domain"
)

// readOnlyContainers answers the read routes the router test touches; any
// other call panics through the embedded nil interface, which would mean a
// blocked route reached its handler.
type readOnlyContainers struct {
	containerService
}

func (readOnlyContainers) ListContainers() []domain.Container { return nil }
func (readOnlyContainers) FleetStats(ctx context.Context) []domain.Stats {
	return []domain.Stats{{ContainerID: "web"}}
}

func newTestRouter(t *testing.T, opts Options) http.Handler {
	t.Helper()
	return NewRouter(Services{
		Containers: readOnlyContainers{},
		Auth:       &fakeAuth{token: "good", user: "admin"},
	}, nil, nil, opts)
}

func authed(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "good"})
	setSameOrigin(req)
	return req
}

func TestRouter_ReadOnlyBlocksWritesAndShell(t *testing.T) {
	router := newTestRouter(t, Options{ReadOnly: true})

	blocked := []struct{ method, path string }{
		{http.MethodPost, "/api/containers"},
		{http.MethodPost, "/api/containers/web/stop"},
		{http.MethodDelete, "/api/containers/web"},
		{http.MethodGet, "/api/containers/web/exec"},
		{http.MethodPost, "/api/images/pull"},
		{http.MethodPost, "/api/images/prune"},
		{http.MethodDelete, "/api/volumes?name=data"},
		{http.MethodPost, "/api/networks"},
		{http.MethodPost, "/api/storage/cleanup"},
		{http.MethodPost, "/api/stacks"},
		{http.MethodPost, "/api/stacks/demo/up"},
		{http.MethodPost, "/api/stacks/demo/down"},
		{http.MethodPost, "/api/environments"},
		{http.MethodPut, "/api/registries"},
	}
	for _, tc := range blocked {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, authed(tc.method, tc.path, "{}"))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
		})
	}

	for _, path := range []string{"/api/containers", "/api/stats"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authed(http.MethodGet, path, ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s in read-only mode: status = %d, want 200", path, rec.Code)
		}
	}
}

func TestRouter_MeReportsReadOnly(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		rec := httptest.NewRecorder()
		newTestRouter(t, Options{ReadOnly: readOnly}).ServeHTTP(rec, authed(http.MethodGet, "/api/auth/me", ""))
		want := `"readOnly":false`
		if readOnly {
			want = `"readOnly":true`
		}
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("readOnly=%v: body %s does not contain %s", readOnly, rec.Body.String(), want)
		}
	}
}

func TestRouter_SetsContentSecurityPolicy(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(t, Options{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"script-src 'self'", "frame-ancestors 'none'", "object-src 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Fatalf("CSP %q is missing %q", csp, directive)
		}
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("CSP allows inline scripts: %q", csp)
	}
}

func TestRouter_PasswordChangeNeedsSession(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(`{}`))
	setSameOrigin(req)
	newTestRouter(t, Options{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRouter_RejectsCrossOriginMutate(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/containers/web/stop", strings.NewReader("{}"))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "good"})
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	newTestRouter(t, Options{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "cross-origin") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestRouter_RejectsMutateWithoutOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/containers/web/stop", strings.NewReader("{}"))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "good"})
	rec := httptest.NewRecorder()
	newTestRouter(t, Options{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestRouter_ViewerCannotMutate(t *testing.T) {
	router := NewRouter(Services{
		Containers: readOnlyContainers{},
		Auth:       &fakeAuth{token: "good", user: "look", role: domain.RoleViewer},
	}, nil, nil, Options{})

	blocked := []struct{ method, path string }{
		{http.MethodPost, "/api/containers/web/stop"},
		{http.MethodGet, "/api/containers/web/exec"},
		{http.MethodPost, "/api/storage/cleanup"},
		{http.MethodPost, "/api/users/invite"},
	}
	for _, tc := range blocked {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, authed(tc.method, tc.path, `{"role":"viewer"}`))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
		})
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authed(http.MethodGet, "/api/containers", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("viewer GET containers: status = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authed(http.MethodGet, "/api/auth/me", ""))
	body := rec.Body.String()
	if !strings.Contains(body, `"role":"viewer"`) || !strings.Contains(body, `"readOnly":true`) {
		t.Fatalf("viewer /me body = %s", body)
	}
}

func TestRouter_OperatorCannotManageUsers(t *testing.T) {
	router := NewRouter(Services{
		Containers: readOnlyContainers{},
		Auth:       &fakeAuth{token: "good", user: "ops", role: domain.RoleOperator},
	}, nil, nil, Options{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authed(http.MethodPost, "/api/users/invite", `{"role":"viewer"}`))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("operator invite: status = %d, want 403", rec.Code)
	}
}

func TestRouter_ReadyzIsPublic(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(t, Options{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil system: status = %d, want 503", rec.Code)
	}

	rec = httptest.NewRecorder()
	NewRouter(Services{
		Containers: readOnlyContainers{},
		Auth:       &fakeAuth{token: "good", user: "admin"},
		System:     pingOK{},
	}, nil, nil, Options{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("ping ok: status = %d body %q", rec.Code, rec.Body.String())
	}
}

type pingOK struct{}

func (pingOK) Info(ctx context.Context) (domain.SystemInfo, error) {
	return domain.SystemInfo{}, nil
}
func (pingOK) DiskUsage(ctx context.Context) (domain.DiskUsage, error) {
	return domain.DiskUsage{}, nil
}
func (pingOK) InvalidateDiskUsage() {}
func (pingOK) Ping(ctx context.Context) error {
	return nil
}

func TestRouter_InvitePeekIsPublic(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/invite?token=nope", nil)
	newTestRouter(t, Options{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
