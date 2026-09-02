package service_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"dockvista/internal/adapters/store"
	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
)

// fakeDocker implements ports.DockerClient in memory, so the service layer
// can be tested without a real Docker daemon.
type fakeDocker struct {
	containers  []domain.Container
	stats       domain.Stats
	statsErr    error
	actionCalls map[string]int
	actionErr   error
}

func newFakeDocker() *fakeDocker {
	return &fakeDocker{actionCalls: map[string]int{}}
}

func (f *fakeDocker) Ping(ctx context.Context) (domain.EngineInfo, error) {
	return domain.EngineInfo{Version: "test", Reachable: true, Containers: len(f.containers)}, nil
}

func (f *fakeDocker) ListContainers(ctx context.Context) ([]domain.Container, error) {
	return f.containers, nil
}

func (f *fakeDocker) ContainerStats(ctx context.Context, id string) (domain.Stats, error) {
	return f.stats, f.statsErr
}

func (f *fakeDocker) StartContainer(ctx context.Context, id string) error {
	f.actionCalls["start"]++
	return f.actionErr
}

func (f *fakeDocker) StopContainer(ctx context.Context, id string) error {
	f.actionCalls["stop"]++
	return f.actionErr
}

func (f *fakeDocker) PauseContainer(ctx context.Context, id string) error {
	f.actionCalls["pause"]++
	return f.actionErr
}

func (f *fakeDocker) UnpauseContainer(ctx context.Context, id string) error {
	f.actionCalls["unpause"]++
	return f.actionErr
}

func (f *fakeDocker) RestartContainer(ctx context.Context, id string) error {
	f.actionCalls["restart"]++
	return f.actionErr
}

func (f *fakeDocker) StreamLogs(ctx context.Context, id, tail string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("log line\n")), nil
}

func (f *fakeDocker) CreateExec(ctx context.Context, containerID string) (string, error) {
	return "exec1", nil
}

func (f *fakeDocker) AttachExec(ctx context.Context, execID string) (io.ReadWriteCloser, error) {
	return nil, errors.New("not implemented in fake")
}

func (f *fakeDocker) ResizeExec(ctx context.Context, execID string, rows, cols uint) error {
	return nil
}

func (f *fakeDocker) CreateContainer(ctx context.Context, spec domain.ContainerSpec) (string, error) {
	return "new-container-id", nil
}

func (f *fakeDocker) RemoveContainer(ctx context.Context, id string, force bool) error {
	return nil
}

func (f *fakeDocker) InspectContainer(ctx context.Context, id string) ([]byte, error) {
	return []byte(`{}`), nil
}

func (f *fakeDocker) Events(ctx context.Context) (<-chan domain.Event, <-chan error) {
	events := make(chan domain.Event)
	errs := make(chan error)
	go func() {
		<-ctx.Done()
		close(events)
		close(errs)
	}()
	return events, errs
}

func (f *fakeDocker) Close() error { return nil }

func TestListContainers_ServesFromCacheNotDocker(t *testing.T) {
	fake := newFakeDocker()
	fake.containers = []domain.Container{{ID: "abc123", Name: "web"}}

	svc := service.New(fake, store.New(), nil)

	// Before any refresh, the cache is empty even though the fake "daemon"
	// already has a container — proves ListContainers never calls Docker
	// directly.
	if got := svc.ListContainers(); len(got) != 0 {
		t.Fatalf("expected empty cache before refresh, got %d", len(got))
	}

	if err := svc.RefreshOnce(context.Background()); err != nil {
		t.Fatalf("RefreshOnce: %v", err)
	}

	got := svc.ListContainers()
	if len(got) != 1 || got[0].ID != "abc123" {
		t.Fatalf("expected cached container abc123, got %+v", got)
	}
}

func TestGetContainer_NotFound(t *testing.T) {
	svc := service.New(newFakeDocker(), store.New(), nil)

	_, err := svc.GetContainer("missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLifecycleActions_CallExpectedDockerMethod(t *testing.T) {
	fake := newFakeDocker()
	svc := service.New(fake, store.New(), nil)
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"start", func() error { return svc.Start(ctx, "abc") }},
		{"stop", func() error { return svc.Stop(ctx, "abc") }},
		{"pause", func() error { return svc.Pause(ctx, "abc") }},
		{"unpause", func() error { return svc.Unpause(ctx, "abc") }},
		{"restart", func() error { return svc.Restart(ctx, "abc") }},
	}

	for _, tc := range cases {
		if err := tc.call(); err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if fake.actionCalls[tc.name] != 1 {
			t.Errorf("%s: expected exactly 1 call, got %d", tc.name, fake.actionCalls[tc.name])
		}
	}
}

func TestRunCollector_StopsOnContextCancel(t *testing.T) {
	fake := newFakeDocker()
	svc := service.New(fake, store.New(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunCollector(ctx, time.Millisecond)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunCollector did not stop after context cancellation")
	}
}
