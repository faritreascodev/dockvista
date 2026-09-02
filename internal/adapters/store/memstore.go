// Package store provides an in-memory, concurrency-safe cache of the
// container list, refreshed periodically by the background collector.
package store

import (
	"sync"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

// MemStore is a mutex-guarded snapshot of the last known container list. It
// is safe for concurrent use by the collector (writer) and HTTP handlers
// (readers).
type MemStore struct {
	mu         sync.RWMutex
	containers []domain.Container
	byID       map[string]domain.Container
}

// New returns an empty, ready-to-use MemStore.
func New() *MemStore {
	return &MemStore{
		containers: []domain.Container{},
		byID:       map[string]domain.Container{},
	}
}

// Replace atomically swaps the cached container list.
func (s *MemStore) Replace(containers []domain.Container) {
	byID := make(map[string]domain.Container, len(containers))
	for _, c := range containers {
		byID[c.ID] = c
	}

	s.mu.Lock()
	s.containers = containers
	s.byID = byID
	s.mu.Unlock()
}

// List returns a snapshot of all cached containers.
func (s *MemStore) List() []domain.Container {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.Container, len(s.containers))
	copy(out, s.containers)
	return out
}

// Get returns a single cached container by ID.
func (s *MemStore) Get(id string) (domain.Container, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.byID[id]
	return c, ok
}

var _ ports.ContainerStore = (*MemStore)(nil)
