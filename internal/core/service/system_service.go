package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

// diskUsageCacheTTL is how long one /system/df result is reused. Every open
// overview tab asks for it, and the daemon sizes every layer to answer.
const diskUsageCacheTTL = 15 * time.Second

const diskUsageCallTimeout = 45 * time.Second

// SystemService reports daemon details and storage usage.
type SystemService struct {
	docker ports.SystemClient
	now    func() time.Time

	diskFlight flight
	diskMu     sync.Mutex
	disk       *cachedDiskUsage
}

type cachedDiskUsage struct {
	usage domain.DiskUsage
	until time.Time
}

func NewSystemService(docker ports.SystemClient) *SystemService {
	return &SystemService{docker: docker, now: time.Now}
}

func (s *SystemService) Info(ctx context.Context) (domain.SystemInfo, error) {
	info, err := s.docker.SystemInfo(ctx)
	if err != nil {
		return domain.SystemInfo{}, fmt.Errorf("service: system info: %w", err)
	}
	return info, nil
}

// Ping is for /readyz: the process is up (liveness is /healthz); this asks
// whether the daemon still answers.
func (s *SystemService) Ping(ctx context.Context) error {
	_, err := s.docker.Ping(ctx)
	if err != nil {
		return fmt.Errorf("service: ping: %w", err)
	}
	return nil
}

// DiskUsage serves a recent sample when one exists, and otherwise shares a
// single daemon call between every concurrent caller.
func (s *SystemService) DiskUsage(ctx context.Context) (domain.DiskUsage, error) {
	if usage, ok := s.cachedDisk(); ok {
		return usage, nil
	}
	v, err := s.diskFlight.Do("disk", func() (any, error) {
		if usage, ok := s.cachedDisk(); ok {
			return usage, nil
		}
		// Detached for the same reason as Stats: the first caller leaving
		// must not fail the sample the others are waiting on.
		cctx, cancel := context.WithTimeout(context.Background(), diskUsageCallTimeout)
		defer cancel()
		usage, err := s.docker.DiskUsage(cctx)
		if err != nil {
			return domain.DiskUsage{}, err
		}
		s.diskMu.Lock()
		s.disk = &cachedDiskUsage{usage: usage, until: s.now().Add(diskUsageCacheTTL)}
		s.diskMu.Unlock()
		return usage, nil
	})
	if err != nil {
		return domain.DiskUsage{}, fmt.Errorf("service: disk usage: %w", err)
	}
	usage, _ := v.(domain.DiskUsage)
	return usage, nil
}

// InvalidateDiskUsage drops the cached sample, so the next read after a
// prune reflects the space that was just freed.
func (s *SystemService) InvalidateDiskUsage() {
	s.diskMu.Lock()
	s.disk = nil
	s.diskMu.Unlock()
}

func (s *SystemService) cachedDisk() (domain.DiskUsage, bool) {
	s.diskMu.Lock()
	defer s.diskMu.Unlock()
	if s.disk == nil || s.now().After(s.disk.until) {
		return domain.DiskUsage{}, false
	}
	return s.disk.usage, true
}
