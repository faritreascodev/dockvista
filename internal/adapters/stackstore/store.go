package stackstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"dockvista/internal/core/domain"
)

const (
	indexFile  = "stacks.json"
	stacksDir  = "stacks"
	composeRel = "compose.yml"
	docVersion = 1
)

type stored struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type doc struct {
	Version int      `json:"version"`
	Items   []stored `json:"items"`
}

type Store struct {
	mu      sync.Mutex
	dataDir string
	path    string
}

func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, stacksDir), 0o700); err != nil {
		return nil, fmt.Errorf("stackstore: mkdir: %w", err)
	}
	return &Store{dataDir: dataDir, path: filepath.Join(dataDir, indexFile)}, nil
}

func (s *Store) Dir(id string) string {
	return filepath.Join(s.dataDir, stacksDir, id)
}

func (s *Store) List() ([]domain.Stack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Stack, 0, len(d.Items))
	for _, it := range d.Items {
		st, err := s.loadLocked(it)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

func (s *Store) Get(id string) (domain.Stack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return domain.Stack{}, err
	}
	for _, it := range d.Items {
		if it.ID == id {
			return s.loadLocked(it)
		}
	}
	return domain.Stack{}, domain.ErrNotFound
}

func (s *Store) Create(name, yamlBody string) (domain.Stack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return domain.Stack{}, err
	}
	if len(d.Items) >= domain.MaxStacks {
		return domain.Stack{}, domain.ErrInvalidInput
	}
	for _, it := range d.Items {
		if it.Name == name {
			return domain.Stack{}, domain.ErrInvalidInput
		}
	}
	now := time.Now().UTC()
	it := stored{ID: uuid.NewString(), Name: name, CreatedAt: now, UpdatedAt: now}
	if err := s.writeYAML(it.ID, yamlBody); err != nil {
		return domain.Stack{}, err
	}
	d.Items = append(d.Items, it)
	if err := s.write(d); err != nil {
		return domain.Stack{}, err
	}
	return domain.Stack{ID: it.ID, Name: name, YAML: yamlBody, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) UpdateYAML(id, yamlBody string) (domain.Stack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return domain.Stack{}, err
	}
	for i, it := range d.Items {
		if it.ID != id {
			continue
		}
		it.UpdatedAt = time.Now().UTC()
		d.Items[i] = it
		if err := s.writeYAML(id, yamlBody); err != nil {
			return domain.Stack{}, err
		}
		if err := s.write(d); err != nil {
			return domain.Stack{}, err
		}
		return domain.Stack{ID: it.ID, Name: it.Name, YAML: yamlBody, CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt}, nil
	}
	return domain.Stack{}, domain.ErrNotFound
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return err
	}
	kept := d.Items[:0]
	found := false
	for _, it := range d.Items {
		if it.ID == id {
			found = true
			continue
		}
		kept = append(kept, it)
	}
	if !found {
		return domain.ErrNotFound
	}
	d.Items = kept
	if err := s.write(d); err != nil {
		return err
	}
	_ = os.RemoveAll(s.Dir(id))
	return nil
}

func (s *Store) loadLocked(it stored) (domain.Stack, error) {
	b, err := os.ReadFile(filepath.Join(s.Dir(it.ID), composeRel))
	if err != nil {
		return domain.Stack{}, fmt.Errorf("stackstore: read yaml: %w", err)
	}
	return domain.Stack{ID: it.ID, Name: it.Name, YAML: string(b), CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt}, nil
}

func (s *Store) writeYAML(id, body string) error {
	dir := s.Dir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, composeRel), []byte(body), 0o600)
}

func (s *Store) read() (doc, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return doc{Version: docVersion}, nil
	}
	if err != nil {
		return doc{}, fmt.Errorf("stackstore: read: %w", err)
	}
	var d doc
	if err := json.Unmarshal(data, &d); err != nil {
		return doc{}, fmt.Errorf("stackstore: decode: %w", err)
	}
	return d, nil
}

func (s *Store) write(d doc) error {
	d.Version = docVersion
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
