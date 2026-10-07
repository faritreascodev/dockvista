// Command dockvista runs the DockVista server: it connects to the local
// Docker daemon, polls container state in the background, exposes a REST
// API, and serves the embedded web UI — all as a single binary.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dockvista/internal/adapters/audit"
	"dockvista/internal/adapters/authstore"
	"dockvista/internal/adapters/enginehub"
	"dockvista/internal/adapters/httpapi"
	"dockvista/internal/config"
	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
	"dockvista/internal/platform/embedweb"
)

func main() {
	maybeReadyz()
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()

	// Cancelled on SIGINT/SIGTERM; drives both the background collector's
	// shutdown and the start of the HTTP server's graceful drain.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hub, err := enginehub.Start(ctx, cfg.DataDir, cfg.PollInterval, log)
	if err != nil {
		return err
	}
	defer hub.Close()

	local, err := hub.Resolve(ctx, domain.LocalEnvironmentID)
	if err != nil {
		return err
	}

	credStore, err := authstore.New(cfg.DataDir)
	if err != nil {
		return err
	}
	initialized, err := credStore.IsInitialized()
	if err != nil {
		return err
	}
	setupToken := cfg.SetupToken
	if !initialized && setupToken == "" {
		setupToken, err = newSetupToken()
		if err != nil {
			return err
		}
		log.Warn("admin account is not created yet; this setup token is required once", "token", setupToken)
	}
	sessionSecret, err := authstore.LoadOrCreateSessionSecret(cfg.DataDir)
	if err != nil {
		return err
	}
	authSvc := service.NewAuthService(credStore, sessionSecret, setupToken)
	authSvc.ConfigureSessions(cfg.IdleTimeout, 0)

	auditLog, err := audit.New(cfg.DataDir, log)
	if err != nil {
		return err
	}

	staticHandler, err := embedweb.Handler()
	if err != nil {
		return err
	}

	warnIfExposed(log, cfg)

	router := httpapi.NewRouter(httpapi.Services{
		Containers: local.Containers,
		Auth:       authSvc,
		Events:     local.Events,
		Images:     local.Images,
		Volumes:    local.Volumes,
		Networks:   local.Networks,
		System:     hub.LocalSystem(),
		Storage:    local.Storage,
		Engines:    hub,
		Registries: hub.Registries(),
	}, staticHandler, log, httpapi.Options{
		CookieSecure: cfg.CookieSecure,
		ReadOnly:     cfg.ReadOnly,
		Audit:        auditLog,
	})
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		// No overall ReadTimeout/WriteTimeout: the log-streaming endpoint is
		// a long-lived response by design, and both are bounded instead by
		// the request context (see httpapi.handleContainerLogs).
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("dockvista listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received, draining connections")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	log.Info("dockvista stopped cleanly")
	return nil
}

// warnIfExposed logs once at startup when the server is reachable from other
// machines over plain HTTP. The address is the operator's call; the warning
// is there so it is never an accident.
func warnIfExposed(log *slog.Logger, cfg config.Config) {
	if cfg.ReadOnly {
		log.Info("read-only mode: container changes, pulls, prunes, and the terminal are disabled")
	}
	if isLoopbackAddr(cfg.Addr) || cfg.CookieSecure {
		return
	}
	log.Warn("listening beyond loopback without DOCKVISTA_COOKIE_SECURE; the session cookie crosses the network in clear text. Put TLS in front, or set DOCKVISTA_ADDR=127.0.0.1:8080",
		"addr", cfg.Addr)
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func newSetupToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
