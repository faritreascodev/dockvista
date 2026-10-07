package envstore

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
	docFile    = "environments.json"
	tlsDir     = "envs"
	docVersion = 1
)

type storedEnv struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Host      string    `json:"host"`
	CreatedAt time.Time `json:"createdAt"`
}

type doc struct {
	Version int         `json:"version"`
	Items   []storedEnv `json:"items"`
}

type TLSFiles struct {
	CA   string
	Cert string
	Key  string
}

type Store struct {
	mu      sync.Mutex
	dataDir string
	path    string
}

func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, tlsDir), 0o700); err != nil {
		return nil, fmt.Errorf("envstore: mkdir: %w", err)
	}
	return &Store{dataDir: dataDir, path: filepath.Join(dataDir, docFile)}, nil
}

func (s *Store) List() ([]domain.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Environment, 0, len(d.Items))
	for _, it := range d.Items {
		out = append(out, toDomain(it))
	}
	return out, nil
}

func (s *Store) Get(id string) (domain.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return domain.Environment{}, err
	}
	for _, it := range d.Items {
		if it.ID == id {
			return toDomain(it), nil
		}
	}
	return domain.Environment{}, domain.ErrNotFound
}

func (s *Store) TLSPaths(id string) (ca, cert, key string) {
	dir := filepath.Join(s.dataDir, tlsDir, id)
	pick := func(name string) string {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		return ""
	}
	return pick("ca.pem"), pick("cert.pem"), pick("key.pem")
}

func (s *Store) TLS(id string) (TLSFiles, error) {
	dir := filepath.Join(s.dataDir, tlsDir, id)
	read := func(name string) (string, error) {
		p := filepath.Join(dir, name)
		b, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	ca, err := read("ca.pem")
	if err != nil {
		return TLSFiles{}, err
	}
	cert, err := read("cert.pem")
	if err != nil {
		return TLSFiles{}, err
	}
	key, err := read("key.pem")
	if err != nil {
		return TLSFiles{}, err
	}
	return TLSFiles{CA: ca, Cert: cert, Key: key}, nil
}

func (s *Store) Create(spec domain.EnvironmentSpec) (domain.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return domain.Environment{}, err
	}
	if len(d.Items) >= domain.MaxEnvironments {
		return domain.Environment{}, domain.ErrInvalidInput
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	item := storedEnv{ID: id, Name: spec.Name, Kind: string(spec.Kind), Host: spec.Host, CreatedAt: now}
	if err := s.writeTLS(id, spec); err != nil {
		return domain.Environment{}, err
	}
	d.Items = append(d.Items, item)
	if err := s.write(d); err != nil {
		return domain.Environment{}, err
	}
	return toDomain(item), nil
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
	_ = os.RemoveAll(filepath.Join(s.dataDir, tlsDir, id))
	return nil
}

func (s *Store) writeTLS(id string, spec domain.EnvironmentSpec) error {
	dir := filepath.Join(s.dataDir, tlsDir, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("envstore: tls dir: %w", err)
	}
	write := func(name, body string) error {
		if body == "" {
			return nil
		}
		return os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600)
	}
	if err := write("ca.pem", spec.TLSCA); err != nil {
		return err
	}
	if err := write("cert.pem", spec.TLSCert); err != nil {
		return err
	}
	return write("key.pem", spec.TLSKey)
}

func (s *Store) read() (doc, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return doc{Version: docVersion}, nil
	}
	if err != nil {
		return doc{}, fmt.Errorf("envstore: read: %w", err)
	}
	var d doc
	if err := json.Unmarshal(data, &d); err != nil {
		return doc{}, fmt.Errorf("envstore: decode: %w", err)
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
		return fmt.Errorf("envstore: write: %w", err)
	}
	return os.Rename(tmp, s.path)
}

func toDomain(it storedEnv) domain.Environment {
	return domain.Environment{
		ID:        it.ID,
		Name:      it.Name,
		Kind:      domain.EnvironmentKind(it.Kind),
		Host:      it.Host,
		CreatedAt: it.CreatedAt,
	}
}
