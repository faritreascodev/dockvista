// Package store provides an in-memory, concurrency-safe cache of the
// container list, refreshed periodically by the background collector.
package store

import (
	"strings"
	"sync"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

// minIDPrefix is the shortest unambiguous Docker short-id we will resolve.
// Shorter prefixes collide too easily to guess.
const minIDPrefix = 12

// MemStore is a mutex-guarded snapshot of the last known container list. It
// is safe for concurrent use by the collector (writer) and HTTP handlers
// (readers).
type MemStore struct {
	mu         sync.RWMutex
	containers []domain.Container
	byID       map[string]domain.Container
	byName     map[string]domain.Container
}

// New returns an empty, ready-to-use MemStore.
func New() *MemStore {
	return &MemStore{
		containers: []domain.Container{},
		byID:       map[string]domain.Container{},
		byName:     map[string]domain.Container{},
	}
}

// Replace atomically swaps the cached container list.
func (s *MemStore) Replace(containers []domain.Container) {
	byID := make(map[string]domain.Container, len(containers))
	byName := make(map[string]domain.Container, len(containers))
	for _, c := range containers {
		byID[c.ID] = c
		if c.Name != "" {
			byName[c.Name] = c
		}
	}

	s.mu.Lock()
	s.containers = containers
	s.byID = byID
	s.byName = byName
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

// Get returns a cached container by full ID, name, or a unique ID prefix
// of at least 12 characters.
func (s *MemStore) Get(id string) (domain.Container, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if c, ok := s.byID[id]; ok {
		return c, true
	}
	if c, ok := s.byName[id]; ok {
		return c, true
	}
	if len(id) < minIDPrefix {
		return domain.Container{}, false
	}

	var found domain.Container
	matches := 0
	for full, c := range s.byID {
		if strings.HasPrefix(full, id) {
			found = c
			matches++
			if matches > 1 {
				return domain.Container{}, false
			}
		}
	}
	if matches == 1 {
		return found, true
	}
	return domain.Container{}, false
}

var _ ports.ContainerStore = (*MemStore)(nil)
