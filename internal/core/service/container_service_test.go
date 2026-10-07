package service_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
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
	statsCalls  atomic.Int32
	listCalls   atomic.Int32
	listGate    chan struct{}
	listEntered chan struct{}
	listOnce    sync.Once
	actionCalls map[string]int
	actionErr   error
	createID    string
	createErr   error
	listDir     func(path string) (domain.DirListing, error)
	statDir     bool
	statSize    int64
}

func newFakeDocker() *fakeDocker {
	return &fakeDocker{actionCalls: map[string]int{}}
}

func (f *fakeDocker) Ping(ctx context.Context) (domain.EngineInfo, error) {
	return domain.EngineInfo{Version: "test", Reachable: true, Containers: len(f.containers)}, nil
}

func (f *fakeDocker) ListContainers(ctx context.Context) ([]domain.Container, error) {
	f.listCalls.Add(1)
	if f.listEntered != nil {
		f.listOnce.Do(func() { close(f.listEntered) })
	}
	if f.listGate != nil {
		<-f.listGate
	}
	return f.containers, nil
}

func (f *fakeDocker) ContainerStats(ctx context.Context, id string) (domain.Stats, error) {
	f.statsCalls.Add(1)
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

func (f *fakeDocker) StreamLogs(ctx context.Context, id string, opts domain.LogStreamOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("log line\n")), nil
}

func (f *fakeDocker) InspectFilesystem(ctx context.Context, id string) (domain.ContainerFS, error) {
	return domain.ContainerFS{Running: true}, nil
}

func (f *fakeDocker) ListContainerDir(ctx context.Context, id, path string) (domain.DirListing, error) {
	if f.listDir != nil {
		return f.listDir(path)
	}
	return domain.DirListing{Path: path, Running: true, Dir: true}, nil
}

func (f *fakeDocker) StatContainerPath(ctx context.Context, id, path string) (domain.FSEntry, error) {
	size := f.statSize
	if size == 0 && !f.statDir {
		size = 4
	}
	return domain.FSEntry{Name: "f", Path: path, SizeBytes: size, Dir: f.statDir}, nil
}

func (f *fakeDocker) CopyContainerFile(ctx context.Context, id, path string) (io.ReadCloser, domain.FSEntry, error) {
	return io.NopCloser(strings.NewReader("data")), domain.FSEntry{Name: "f", Path: path, SizeBytes: 4}, nil
}

func (f *fakeDocker) ContainerChanges(ctx context.Context, id string) (domain.FSChangeList, error) {
	return domain.FSChangeList{}, nil
}

func (f *fakeDocker) LogsMultiplexed(ctx context.Context, id string) (bool, error) {
	return true, nil
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

func (f *fakeDocker) CreateContainer(ctx context.Context, spec domain.ContainerSpec) (string, bool, error) {
	if f.createErr != nil {
		id := f.createID
		if id == "" {
			id = "new-container-id"
		}
		return id, false, f.createErr
	}
	return "new-container-id", true, nil
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

func TestRefreshOnce_CollapsesConcurrentLists(t *testing.T) {
	fake := newFakeDocker()
	fake.containers = []domain.Container{{ID: "abc123abc123", Name: "web"}}
	fake.listEntered = make(chan struct{})
	fake.listGate = make(chan struct{})

	svc := service.New(fake, store.New(), nil)
	ctx := context.Background()

	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			errCh <- svc.RefreshOnce(ctx)
		}()
	}

	select {
	case <-fake.listEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("list never started")
	}
	// Let the other callers join the in-flight list before releasing it.
	time.Sleep(100 * time.Millisecond)
	close(fake.listGate)

	for i := 0; i < 8; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("RefreshOnce: %v", err)
		}
	}
	if got := fake.listCalls.Load(); got != 1 {
		t.Fatalf("expected 1 daemon list for 8 callers, got %d", got)
	}
	if got := svc.ListContainers(); len(got) != 1 || got[0].Name != "web" {
		t.Fatalf("cache = %+v", got)
	}
}

func TestCreate_StartFailureKeepsIDAndRefreshes(t *testing.T) {
	fake := newFakeDocker()
	fake.createID = "created-but-stopped"
	fake.createErr = errors.New("start failed")
	fake.containers = []domain.Container{{ID: "created-but-stopped", Name: "orphan"}}

	svc := service.New(fake, store.New(), nil)
	id, started, err := svc.Create(context.Background(), domain.ContainerSpec{Image: "nginx"})
	if err == nil || started || id != "created-but-stopped" {
		t.Fatalf("id=%q started=%v err=%v", id, started, err)
	}
	if got := svc.ListContainers(); len(got) != 1 {
		t.Fatalf("expected the created container in cache, got %d", len(got))
	}
}

func TestStats_ReusesSampleInsideTTL(t *testing.T) {
	fake := newFakeDocker()
	fake.stats = domain.Stats{ContainerID: "abc", CPUPercent: 1}
	svc := service.New(fake, store.New(), nil)
	ctx := context.Background()

	if _, err := svc.Stats(ctx, "abc"); err != nil {
		t.Fatalf("first Stats: %v", err)
	}
	if _, err := svc.Stats(ctx, "abc"); err != nil {
		t.Fatalf("second Stats: %v", err)
	}
	if got := fake.statsCalls.Load(); got != 1 {
		t.Fatalf("expected 1 daemon stats call, got %d", got)
	}
}

// ctxAwareStatsDocker fails the stats call when the context it receives is
// already done, which is what a real daemon client does.
type ctxAwareStatsDocker struct {
	*fakeDocker
}

func (f ctxAwareStatsDocker) ContainerStats(ctx context.Context, id string) (domain.Stats, error) {
	if err := ctx.Err(); err != nil {
		return domain.Stats{}, err
	}
	return f.fakeDocker.ContainerStats(ctx, id)
}

func TestStats_CancelledCallerDoesNotFailTheSharedSample(t *testing.T) {
	fake := newFakeDocker()
	fake.stats = domain.Stats{ContainerID: "abc", CPUPercent: 2}
	svc := service.New(ctxAwareStatsDocker{fake}, store.New(), nil)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Stats(cancelled, "abc"); err != nil {
		t.Fatalf("Stats with cancelled caller ctx: %v", err)
	}
}

func TestListFiles_RejectsRelativePath(t *testing.T) {
	svc := service.New(newFakeDocker(), store.New(), nil)
	if _, err := svc.ListFiles(context.Background(), "web", "etc/passwd"); err == nil {
		t.Fatal("expected invalid path")
	}
}

func TestFileContent_RejectsDirectory(t *testing.T) {
	fake := newFakeDocker()
	fake.statDir = true
	svc := service.New(fake, store.New(), nil)
	_, _, err := svc.FileContent(context.Background(), "web", "/app")
	if !errors.Is(err, domain.ErrIsDirectory) {
		t.Fatalf("err = %v, want ErrIsDirectory", err)
	}
}

func TestFileContent_RejectsOversize(t *testing.T) {
	fake := newFakeDocker()
	fake.statSize = domain.MaxFileDownloadBytes + 1
	svc := service.New(fake, store.New(), nil)
	_, _, err := svc.FileContent(context.Background(), "web", "/app/big.bin")
	if !errors.Is(err, domain.ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}
