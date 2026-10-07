package enginehub

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"dockvista/internal/adapters/broker"
	dockeradapter "dockvista/internal/adapters/docker"
	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
)

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

var containerListActions = map[string]bool{
	"create": true, "start": true, "stop": true, "die": true, "destroy": true,
	"pause": true, "unpause": true, "restart": true, "rename": true, "kill": true,
	"update": true,
}

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
