// Package httpapi is the HTTP transport adapter: it translates REST/SSE
// requests into calls against containerService and marshals the results
// back to JSON. Nothing outside this package (and the tests that import
// containerService as a fake) knows about net/http.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"dockvista/internal/adapters/regstore"
)

// Services groups the use cases the router dispatches to.
type Services struct {
	Containers containerService
	Auth       authService
	Events     eventSubscriber
	Images     imageService
	Volumes    volumeService
	Networks   networkService
	System     systemService
	Storage    storageService
	Engines    EngineResolver
	Registries *regstore.Store
}

// Options are the deployment switches that change how routes behave.
type Options struct {
	// CookieSecure marks the session cookie Secure behind a TLS proxy.
	CookieSecure bool
	// ReadOnly answers 403 on every route that changes daemon state or
	// opens a shell. Reads, login, logout, and password change still work.
	ReadOnly bool
	// Audit records logins and mutating API calls. Nil is a no-op.
	Audit Auditor
}

// NewRouter builds the full HTTP handler tree. staticHandler serves the
// built frontend (SPA) for every route not under /api; it is injected so
// this package never depends on the embed asset location.
//
// Every /api/* route requires a valid session except the auth bootstrap
// routes (status/setup/login). /healthz and /readyz are public probes —
// enforced by mounting all protected routes as a subtree behind requireAuth,
// so a new resource route added later is authenticated by default instead of
// by remembering to wrap it.
func NewRouter(s Services, staticHandler http.Handler, log *slog.Logger, opts Options) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	svc := s.Containers
	h := &handlers{svc: svc, events: s.Events, log: log, streams: newStreamGate(32)}
	ah := &authHandlers{
		svc:          s.Auth,
		cookieSecure: opts.CookieSecure,
		readOnly:     opts.ReadOnly,
		guard:        newLoginGuard(),
		audit:        opts.Audit,
	}
	ih := &imageHandlers{svc: s.Images, log: log}
	vh := &volumeHandlers{svc: s.Volumes}
	nh := &networkHandlers{svc: s.Networks}
	sh := &systemHandlers{svc: s.System}
	sth := &storageHandlers{svc: s.Storage}
	eh := &envHandlers{res: s.Engines, cookieSecure: opts.CookieSecure}
	sthStacks := &stackHandlers{}
	rh := &registryHandlers{store: s.Registries}

	// Shared across every mutating route: generous enough for normal UI use,
	// tight enough to blunt scripted abuse against a single local instance.
	rateLimited := rateLimitMiddleware(newRateLimiter(30, time.Minute))
	// Login gets its own, much stricter limiter — this is the one endpoint a
	// brute-force script would actually hammer. Password change checks the
	// current password, so it is the same kind of oracle.
	loginLimited := rateLimitMiddleware(newRateLimiter(5, time.Minute))
	setupLimited := rateLimitMiddleware(newRateLimiter(5, time.Minute))
	passwordLimited := rateLimitMiddleware(newRateLimiter(5, time.Minute))

	readOnly := denyWhenReadOnly(opts.ReadOnly)
	viewer := denyWhenViewer()
	adminOnly := denyUnlessAdmin()
	audited := auditMutating(opts.Audit)
	mutating := func(next http.Handler) http.Handler {
		return readOnly(viewer(rateLimited(audited(next))))
	}
	// Writes that change what is on disk also drop the cached `system df`.
	freesDisk := func(next http.Handler) http.Handler {
		return mutating(invalidatesDiskUsage(s.System)(next))
	}

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/auth/me", ah.handleMe)
	protected.HandleFunc("POST /api/auth/logout", ah.handleLogout)
	protected.Handle("POST /api/auth/password", passwordLimited(http.HandlerFunc(ah.handleChangePassword)))

	protected.HandleFunc("GET /api/engine", h.handleEngine)
	protected.HandleFunc("GET /api/system/info", sh.handleInfo)
	protected.HandleFunc("GET /api/system/df", sh.handleDiskUsage)

	protected.HandleFunc("GET /api/storage", sth.handleInventory)
	protected.HandleFunc("POST /api/storage/cleanup/preview", sth.handlePreview)
	protected.Handle("POST /api/storage/cleanup", freesDisk(http.HandlerFunc(sth.handleCleanup)))

	protected.HandleFunc("GET /api/stats", h.handleFleetStats)
	protected.HandleFunc("GET /api/containers", h.handleListContainers)
	protected.HandleFunc("GET /api/containers/{id}", h.handleGetContainer)
	protected.HandleFunc("GET /api/containers/{id}/stats", h.handleContainerStats)
	protected.HandleFunc("GET /api/containers/{id}/logs", h.handleContainerLogs)
	protected.HandleFunc("GET /api/containers/{id}/inspect", h.handleInspectContainer)
	protected.HandleFunc("GET /api/containers/{id}/filesystem", h.handleContainerFilesystem)
	protected.HandleFunc("GET /api/containers/{id}/files", h.handleContainerFiles)
	protected.HandleFunc("GET /api/containers/{id}/files/stat", h.handleContainerFileStat)
	protected.HandleFunc("GET /api/containers/{id}/files/content", h.handleContainerFileContent)
	protected.HandleFunc("GET /api/containers/{id}/changes", h.handleContainerChanges)
	protected.HandleFunc("GET /api/events", h.handleEvents)
	protected.Handle("GET /api/users", adminOnly(http.HandlerFunc(ah.handleListUsers)))
	protected.Handle("GET /api/users/invites", adminOnly(http.HandlerFunc(ah.handleListInvites)))
	protected.Handle("POST /api/users/invite", adminOnly(rateLimited(audited(http.HandlerFunc(ah.handleCreateInvite)))))
	protected.Handle("DELETE /api/users/{username}", adminOnly(rateLimited(audited(http.HandlerFunc(ah.handleDeleteUser)))))
	protected.Handle("PATCH /api/users/{username}", adminOnly(rateLimited(audited(http.HandlerFunc(ah.handleSetRole)))))

	protected.Handle("GET /api/containers/{id}/exec", readOnly(viewer(http.HandlerFunc(h.handleExec))))
	protected.Handle("POST /api/containers", mutating(http.HandlerFunc(h.handleCreateContainer)))
	protected.Handle("DELETE /api/containers/{id}", freesDisk(http.HandlerFunc(h.handleRemoveContainer)))
	protected.Handle("POST /api/containers/{id}/start", mutating(h.containerAction(func(s containerService, ctx context.Context, id string) error { return s.Start(ctx, id) })))
	protected.Handle("POST /api/containers/{id}/stop", mutating(h.containerAction(func(s containerService, ctx context.Context, id string) error { return s.Stop(ctx, id) })))
	protected.Handle("POST /api/containers/{id}/pause", mutating(h.containerAction(func(s containerService, ctx context.Context, id string) error { return s.Pause(ctx, id) })))
	protected.Handle("POST /api/containers/{id}/unpause", mutating(h.containerAction(func(s containerService, ctx context.Context, id string) error { return s.Unpause(ctx, id) })))
	protected.Handle("POST /api/containers/{id}/restart", mutating(h.containerAction(func(s containerService, ctx context.Context, id string) error { return s.Restart(ctx, id) })))

	protected.HandleFunc("GET /api/images", ih.handleList)
	protected.HandleFunc("GET /api/images/history", ih.handleHistory)
	protected.Handle("DELETE /api/images", freesDisk(http.HandlerFunc(ih.handleRemove)))
	protected.Handle("POST /api/images/pull", freesDisk(http.HandlerFunc(ih.handlePull)))
	protected.Handle("POST /api/images/prune", freesDisk(http.HandlerFunc(ih.handlePrune)))

	protected.HandleFunc("GET /api/volumes", vh.handleList)
	protected.HandleFunc("GET /api/volumes/{name}/files", vh.handleFiles)
	protected.HandleFunc("GET /api/volumes/{name}/files/content", vh.handleFileContent)
	protected.Handle("POST /api/volumes", mutating(http.HandlerFunc(vh.handleCreate)))
	protected.Handle("DELETE /api/volumes", freesDisk(http.HandlerFunc(vh.handleRemove)))
	protected.Handle("POST /api/volumes/prune", freesDisk(http.HandlerFunc(vh.handlePrune)))

	protected.HandleFunc("GET /api/networks", nh.handleList)
	protected.Handle("POST /api/networks", mutating(http.HandlerFunc(nh.handleCreate)))
	protected.Handle("DELETE /api/networks", mutating(http.HandlerFunc(nh.handleRemove)))
	protected.Handle("POST /api/networks/connect", mutating(http.HandlerFunc(nh.handleConnect)))
	protected.Handle("POST /api/networks/disconnect", mutating(http.HandlerFunc(nh.handleDisconnect)))
	protected.Handle("POST /api/networks/prune", mutating(http.HandlerFunc(nh.handlePrune)))

	protected.HandleFunc("GET /api/environments", eh.handleList)
	protected.Handle("POST /api/environments", adminOnly(mutating(http.HandlerFunc(eh.handleCreate))))
	protected.Handle("DELETE /api/environments/{id}", adminOnly(mutating(http.HandlerFunc(eh.handleDelete))))
	protected.Handle("POST /api/environments/{id}/select", rateLimited(http.HandlerFunc(eh.handleSelect)))

	protected.HandleFunc("GET /api/stacks", sthStacks.handleList)
	protected.HandleFunc("GET /api/stacks/{id}", sthStacks.handleGet)
	protected.Handle("POST /api/stacks", mutating(http.HandlerFunc(sthStacks.handleCreate)))
	protected.Handle("PUT /api/stacks/{id}", mutating(http.HandlerFunc(sthStacks.handleUpdate)))
	protected.Handle("DELETE /api/stacks/{id}", mutating(http.HandlerFunc(sthStacks.handleDelete)))
	protected.Handle("POST /api/stacks/{id}/up", mutating(http.HandlerFunc(sthStacks.handleUp)))
	protected.Handle("POST /api/stacks/{id}/down", mutating(http.HandlerFunc(sthStacks.handleDown)))
	protected.Handle("POST /api/stacks/{id}/sync", mutating(http.HandlerFunc(sthStacks.handleSync)))

	protected.Handle("GET /api/registries", adminOnly(http.HandlerFunc(rh.handleList)))
	protected.Handle("PUT /api/registries", adminOnly(mutating(http.HandlerFunc(rh.handleUpsert))))
	protected.Handle("DELETE /api/registries/{id}", adminOnly(mutating(http.HandlerFunc(rh.handleDelete))))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/auth/status", ah.handleStatus)
	mux.Handle("POST /api/auth/setup", setupLimited(http.HandlerFunc(ah.handleSetup)))
	mux.Handle("POST /api/auth/login", loginLimited(http.HandlerFunc(ah.handleLogin)))
	mux.Handle("GET /api/auth/invite", setupLimited(http.HandlerFunc(ah.handlePeekInvite)))
	mux.Handle("POST /api/auth/accept", setupLimited(http.HandlerFunc(ah.handleAcceptInvite)))
	mux.Handle("/api/", requireAuth(s.Auth)(bindEngine(s.Engines)(protected)))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", sh.handleReady)

	if staticHandler != nil {
		mux.Handle("/", staticHandler)
	}

	return chain(mux, securityHeaders, recoverMiddleware(log), loggingMiddleware(log), requireSameOrigin)
}
