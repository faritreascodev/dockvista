package regstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"

	"dockvista/internal/core/domain"
)

const (
	docFile    = "registries.json"
	docVersion = 1
)

type stored struct {
	ID       string `json:"id"`
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type doc struct {
	Version int      `json:"version"`
	Items   []stored `json:"items"`
}

type Store struct {
	mu   sync.Mutex
	path string
}

func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(dataDir, docFile)}, nil
}

func (s *Store) List() ([]domain.Registry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Registry, 0, len(d.Items))
	for _, it := range d.Items {
		out = append(out, domain.Registry{ID: it.ID, Host: it.Host, Username: it.Username})
	}
	return out, nil
}

func (s *Store) Upsert(host, username, password string) (domain.Registry, error) {
	host = domain.NormalizeRegistryHost(host)
	if host == "" || username == "" || password == "" {
		return domain.Registry{}, domain.ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return domain.Registry{}, err
	}
	for i, it := range d.Items {
		if it.Host == host {
			it.Username = username
			it.Password = password
			d.Items[i] = it
			if err := s.write(d); err != nil {
				return domain.Registry{}, err
			}
			return domain.Registry{ID: it.ID, Host: host, Username: username}, nil
		}
	}
	if len(d.Items) >= domain.MaxRegistries {
		return domain.Registry{}, domain.ErrInvalidInput
	}
	it := stored{ID: uuid.NewString(), Host: host, Username: username, Password: password}
	d.Items = append(d.Items, it)
	if err := s.write(d); err != nil {
		return domain.Registry{}, err
	}
	return domain.Registry{ID: it.ID, Host: host, Username: username}, nil
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
	return s.write(d)
}

func (s *Store) AuthForImage(ref string) (username, password string, ok bool) {
	host := domain.RegistryHostFromImage(ref)
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return "", "", false
	}
	for _, it := range d.Items {
		if it.Host == host {
			return it.Username, it.Password, true
		}
	}
	return "", "", false
}

func (s *Store) read() (doc, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return doc{Version: docVersion}, nil
	}
	if err != nil {
		return doc{}, fmt.Errorf("regstore: read: %w", err)
	}
	var d doc
	if err := json.Unmarshal(data, &d); err != nil {
		return doc{}, fmt.Errorf("regstore: decode: %w", err)
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
