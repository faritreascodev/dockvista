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
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dockvista/internal/adapters/authstore"
	"dockvista/internal/adapters/broker"
	dockeradapter "dockvista/internal/adapters/docker"
	"dockvista/internal/adapters/httpapi"
	"dockvista/internal/adapters/store"
	"dockvista/internal/config"
	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
	"dockvista/internal/platform/embedweb"
)

func main() {
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

	dockerClient, err := dockeradapter.New()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	svc := service.New(dockerClient, store.New(), log)
	go svc.RunCollector(ctx, cfg.PollInterval)

	eventBroker := broker.New[domain.Event]()
	go bridgeDockerEvents(ctx, dockerClient, svc, eventBroker, log)

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
	imageSvc := service.NewImageService(dockerClient)
	volumeSvc := service.NewVolumeService(dockerClient)
	networkSvc := service.NewNetworkService(dockerClient)

	staticHandler, err := embedweb.Handler()
	if err != nil {
		return err
	}

	router := httpapi.NewRouter(svc, authSvc, eventBroker, imageSvc, volumeSvc, networkSvc, staticHandler, log, cfg.CookieSecure)
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

// bridgeDockerEvents forwards the daemon's own event stream onto the SSE
// broker and, for container events, triggers an immediate cache refresh so
// the UI reflects a change (e.g. `docker stop` run from another terminal)
// within about a second instead of waiting for RunCollector's next tick.
// Reconnects on a short fixed delay if the stream drops — a daemon restart
// shouldn't permanently degrade the app back to poll-only.
func bridgeDockerEvents(ctx context.Context, docker *dockeradapter.Client, svc *service.ContainerService, b *broker.Broker[domain.Event], log *slog.Logger) {
	signal := newRefreshSignal()
	go runCoalescedRefresh(ctx, svc, signal, log)

	for ctx.Err() == nil {
		runEventStream(ctx, docker, signal, b, log)
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// refreshSignal is a coalescing doorbell. Nudge never blocks the caller and
// never queues more than one pending refresh.
type refreshSignal struct {
	ch chan struct{}
}

func newRefreshSignal() *refreshSignal {
	return &refreshSignal{ch: make(chan struct{}, 1)}
}

func (s *refreshSignal) Nudge() {
	select {
	case s.ch <- struct{}{}:
	default:
	}
}

// runCoalescedRefresh turns a burst of daemon events into one container
// list. The event reader only publishes and nudges; it does not call the
// daemon itself.
func runCoalescedRefresh(ctx context.Context, svc *service.ContainerService, signal *refreshSignal, log *slog.Logger) {
	const debounce = 200 * time.Millisecond
	for {
		select {
		case <-ctx.Done():
			return
		case <-signal.ch:
			timer := time.NewTimer(debounce)
		drain:
			for {
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-signal.ch:
				case <-timer.C:
					break drain
				}
			}
			if err := svc.RefreshOnce(ctx); err != nil && ctx.Err() == nil {
				log.Warn("event-triggered container refresh failed", "error", err)
			}
		}
	}
}

func newSetupToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// containerListActions are the container-event actions that actually change
// what ListContainers returns. Docker also reports exec_create/exec_start/
// exec_die for every health-check probe (frequently, once per container per
// probe interval) and attach/detach/resize/top for other tooling; treating
// those as "the list changed" would hammer the daemon and the frontend with
// a refresh several times a second on a fleet of any size.
var containerListActions = map[string]bool{
	"create": true, "start": true, "stop": true, "die": true, "destroy": true,
	"pause": true, "unpause": true, "restart": true, "rename": true, "kill": true,
	"update": true,
}

// isHealthStatusAction matches Docker's "health_status: healthy" /
// "health_status: unhealthy" / "health_status: running" action strings —
// infrequent (only on an actual state transition, not per-probe) and worth
// refreshing for, since it changes the human-readable status text.
func isHealthStatusAction(action string) bool {
	return strings.HasPrefix(action, "health_status")
}

func runEventStream(ctx context.Context, docker *dockeradapter.Client, signal *refreshSignal, b *broker.Broker[domain.Event], log *slog.Logger) {
	events, errs := docker.Events(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			b.Publish(evt)
			if evt.Type == "container" && (containerListActions[evt.Action] || isHealthStatusAction(evt.Action)) {
				signal.Nudge()
			}
		case err, ok := <-errs:
			if ok && err != nil {
				log.Warn("docker event stream disconnected, retrying", "error", err)
			}
			return
		}
	}
}
