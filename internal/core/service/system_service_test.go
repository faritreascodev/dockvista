package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dockvista/internal/core/domain"
)

type fakeSystemClient struct {
	diskCalls atomic.Int32
	gate      chan struct{}
}

func (f *fakeSystemClient) SystemInfo(ctx context.Context) (domain.SystemInfo, error) {
	return domain.SystemInfo{Name: "host", CPUs: 4}, nil
}

func (f *fakeSystemClient) Ping(ctx context.Context) (domain.EngineInfo, error) {
	return domain.EngineInfo{Reachable: true, Version: "test"}, nil
}

func (f *fakeSystemClient) DiskUsage(ctx context.Context) (domain.DiskUsage, error) {
	f.diskCalls.Add(1)
	if f.gate != nil {
		<-f.gate
	}
	return domain.DiskUsage{Images: domain.DiskUsageCategory{Count: 3, SizeBytes: 100}}, nil
}

func TestSystemService_Ping(t *testing.T) {
	svc := NewSystemService(&fakeSystemClient{})
	if err := svc.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestSystemService_DiskUsageIsCachedAndShared(t *testing.T) {
	fake := &fakeSystemClient{gate: make(chan struct{})}
	svc := NewSystemService(fake)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.DiskUsage(context.Background()); err != nil {
				t.Errorf("DiskUsage: %v", err)
			}
		}()
	}
	// Let every caller pile onto the in-flight call, then release it.
	time.Sleep(50 * time.Millisecond)
	close(fake.gate)
	wg.Wait()

	if _, err := svc.DiskUsage(context.Background()); err != nil {
		t.Fatalf("DiskUsage: %v", err)
	}
	if got := fake.diskCalls.Load(); got != 1 {
		t.Fatalf("11 reads hit the daemon %d times, want 1", got)
	}
}

func TestSystemService_DiskUsageExpiresAndInvalidates(t *testing.T) {
	fake := &fakeSystemClient{}
	svc := NewSystemService(fake)
	now := time.Unix(1_000, 0)
	svc.now = func() time.Time { return now }

	ctx := context.Background()
	mustDisk := func() {
		t.Helper()
		if _, err := svc.DiskUsage(ctx); err != nil {
			t.Fatalf("DiskUsage: %v", err)
		}
	}

	mustDisk()
	mustDisk()
	if got := fake.diskCalls.Load(); got != 1 {
		t.Fatalf("within TTL: %d daemon calls, want 1", got)
	}

	svc.InvalidateDiskUsage()
	mustDisk()
	if got := fake.diskCalls.Load(); got != 2 {
		t.Fatalf("after invalidate: %d daemon calls, want 2", got)
	}

	now = now.Add(diskUsageCacheTTL + time.Second)
	mustDisk()
	if got := fake.diskCalls.Load(); got != 3 {
		t.Fatalf("after TTL: %d daemon calls, want 3", got)
	}
}
