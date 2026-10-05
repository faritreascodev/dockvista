// Package httpapi is the HTTP transport adapter: it translates REST/SSE
// requests into calls against containerService and marshals the results
// back to JSON. Nothing outside this package (and the tests that import
// containerService as a fake) knows about net/http.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// NewRouter builds the full HTTP handler tree. staticHandler serves the
// built frontend (SPA) for every route not under /api; it is injected so
// this package never depends on the embed asset location.
//
// Every /api/* route requires a valid session except the auth bootstrap
// routes (status/setup/login) and /healthz — enforced by mounting all
// protected routes as a subtree behind requireAuth, so a new resource route
// added later is authenticated by default instead of by remembering to wrap
// it.
func NewRouter(
	svc containerService,
	auth authService,
	events eventSubscriber,
	images imageService,
	volumes volumeService,
	networks networkService,
	staticHandler http.Handler,
	log *slog.Logger,
	cookieSecure bool,
) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	h := &handlers{svc: svc, events: events, log: log, streams: newStreamGate(32)}
	ah := &authHandlers{svc: auth, cookieSecure: cookieSecure}
	ih := &imageHandlers{svc: images, log: log}
	vh := &volumeHandlers{svc: volumes}
	nh := &networkHandlers{svc: networks}

	// Shared across every mutating route: generous enough for normal UI use,
	// tight enough to blunt scripted abuse against a single local instance.
	mutating := rateLimitMiddleware(newRateLimiter(30, time.Minute))
	// Login gets its own, much stricter limiter — this is the one endpoint a
	// brute-force script would actually hammer.
	loginLimited := rateLimitMiddleware(newRateLimiter(5, time.Minute))
	setupLimited := rateLimitMiddleware(newRateLimiter(5, time.Minute))

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/auth/me", ah.handleMe)
	protected.HandleFunc("POST /api/auth/logout", ah.handleLogout)
	protected.HandleFunc("GET /api/engine", h.handleEngine)
	protected.HandleFunc("GET /api/containers", h.handleListContainers)
	protected.HandleFunc("GET /api/containers/{id}", h.handleGetContainer)
	protected.HandleFunc("GET /api/containers/{id}/stats", h.handleContainerStats)
	protected.HandleFunc("GET /api/containers/{id}/logs", h.handleContainerLogs)
	protected.HandleFunc("GET /api/containers/{id}/inspect", h.handleInspectContainer)
	protected.HandleFunc("GET /api/events", h.handleEvents)
	protected.HandleFunc("GET /api/containers/{id}/exec", h.handleExec)
	protected.Handle("POST /api/containers", mutating(http.HandlerFunc(h.handleCreateContainer)))
	protected.Handle("DELETE /api/containers/{id}", mutating(http.HandlerFunc(h.handleRemoveContainer)))
	protected.Handle("POST /api/containers/{id}/start", mutating(h.containerAction(svc.Start)))
	protected.Handle("POST /api/containers/{id}/stop", mutating(h.containerAction(svc.Stop)))
	protected.Handle("POST /api/containers/{id}/pause", mutating(h.containerAction(svc.Pause)))
	protected.Handle("POST /api/containers/{id}/unpause", mutating(h.containerAction(svc.Unpause)))
	protected.Handle("POST /api/containers/{id}/restart", mutating(h.containerAction(svc.Restart)))

	protected.HandleFunc("GET /api/images", ih.handleList)
	protected.Handle("DELETE /api/images", mutating(http.HandlerFunc(ih.handleRemove)))
	protected.Handle("POST /api/images/pull", mutating(http.HandlerFunc(ih.handlePull)))
	protected.Handle("POST /api/images/prune", mutating(http.HandlerFunc(ih.handlePrune)))

	protected.HandleFunc("GET /api/volumes", vh.handleList)
	protected.Handle("POST /api/volumes", mutating(http.HandlerFunc(vh.handleCreate)))
	protected.Handle("DELETE /api/volumes", mutating(http.HandlerFunc(vh.handleRemove)))
	protected.Handle("POST /api/volumes/prune", mutating(http.HandlerFunc(vh.handlePrune)))

	protected.HandleFunc("GET /api/networks", nh.handleList)
	protected.Handle("POST /api/networks", mutating(http.HandlerFunc(nh.handleCreate)))
	protected.Handle("DELETE /api/networks", mutating(http.HandlerFunc(nh.handleRemove)))
	protected.Handle("POST /api/networks/connect", mutating(http.HandlerFunc(nh.handleConnect)))
	protected.Handle("POST /api/networks/disconnect", mutating(http.HandlerFunc(nh.handleDisconnect)))
	protected.Handle("POST /api/networks/prune", mutating(http.HandlerFunc(nh.handlePrune)))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/auth/status", ah.handleStatus)
	mux.Handle("POST /api/auth/setup", setupLimited(http.HandlerFunc(ah.handleSetup)))
	mux.Handle("POST /api/auth/login", loginLimited(http.HandlerFunc(ah.handleLogin)))
	mux.Handle("/api/", requireAuth(auth)(protected))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if staticHandler != nil {
		mux.Handle("/", staticHandler)
	}

	return chain(mux, securityHeaders, recoverMiddleware(log), loggingMiddleware(log))
}
