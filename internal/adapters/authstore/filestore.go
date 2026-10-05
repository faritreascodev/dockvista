// Package authstore persists the single admin account and the session
// signing secret to small JSON/binary files under a data directory, so
// login survives a process restart without pulling in a database.
package authstore

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

const (
	credentialsFile = "credentials.json"
	secretFile      = "session_secret"
	secretLen       = 32
)

// FileStore implements ports.CredentialStore against a JSON file.
type FileStore struct {
	mu   sync.RWMutex
	path string
}

// New ensures dataDir exists (0700) and returns a FileStore backed by
// <dataDir>/credentials.json.
func New(dataDir string) (*FileStore, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("authstore: create data dir: %w", err)
	}
	return &FileStore{path: filepath.Join(dataDir, credentialsFile)}, nil
}

func (s *FileStore) read() (domain.User, bool, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.User{}, false, nil
	}
	if err != nil {
		return domain.User{}, false, fmt.Errorf("authstore: read credentials: %w", err)
	}
	var u domain.User
	if err := json.Unmarshal(data, &u); err != nil {
		return domain.User{}, false, fmt.Errorf("authstore: decode credentials: %w", err)
	}
	return u, true, nil
}

func (s *FileStore) IsInitialized() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok, err := s.read()
	return ok, err
}

func (s *FileStore) CreateAdmin(user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok, err := s.read(); err != nil {
		return err
	} else if ok {
		return domain.ErrAlreadyInitialized
	}

	return s.write(user)
}

func (s *FileStore) write(user domain.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("authstore: encode credentials: %w", err)
	}

	// Write via a temp file + rename so a crash mid-write never leaves a
	// half-written credentials.json behind.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("authstore: write credentials: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("authstore: commit credentials: %w", err)
	}
	return nil
}

// BumpSessionGeneration increments the stored generation and persists it.
// Every session token signed with the previous value stops verifying.
func (s *FileStore) BumpSessionGeneration() (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok, err := s.read()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, domain.ErrUnauthorized
	}
	user.SessionGeneration++
	if err := s.write(user); err != nil {
		return 0, err
	}
	return user.SessionGeneration, nil
}

func (s *FileStore) GetUser(username string) (domain.User, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok, err := s.read()
	if err != nil || !ok || u.Username != username {
		return domain.User{}, false, err
	}
	return u, true, nil
}

var _ ports.CredentialStore = (*FileStore)(nil)

// LoadOrCreateSessionSecret returns the per-install HMAC signing key at
// <dataDir>/session_secret, generating and persisting a random one on first
// run. Existing sessions stay valid across restarts because the key doesn't
// change; a fresh install (or a wiped data dir) invalidates all of them.
func LoadOrCreateSessionSecret(dataDir string) ([]byte, error) {
	path := filepath.Join(dataDir, secretFile)

	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("authstore: read session secret: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("authstore: create data dir: %w", err)
	}

	secret := make([]byte, secretLen)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("authstore: generate session secret: %w", err)
	}
	if err := os.WriteFile(path, secret, 0o600); err != nil {
		return nil, fmt.Errorf("authstore: write session secret: %w", err)
	}
	return secret, nil
}
