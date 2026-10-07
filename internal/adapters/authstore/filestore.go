// Package authstore persists accounts, invites, and the session signing
// secret as small JSON/binary files under a data directory.
package authstore

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

const (
	credentialsFile = "credentials.json"
	secretFile      = "session_secret"
	secretLen       = 32
	docVersion      = 2
)

type accountDoc struct {
	Version int             `json:"version"`
	Users   []domain.User   `json:"users"`
	Invites []domain.Invite `json:"invites,omitempty"`
}

// FileStore implements ports.CredentialStore against a JSON file.
type FileStore struct {
	mu   sync.RWMutex
	path string
}

func New(dataDir string) (*FileStore, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("authstore: create data dir: %w", err)
	}
	return &FileStore{path: filepath.Join(dataDir, credentialsFile)}, nil
}

func (s *FileStore) read() (accountDoc, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return accountDoc{Version: docVersion}, nil
	}
	if err != nil {
		return accountDoc{}, fmt.Errorf("authstore: read credentials: %w", err)
	}
	return parseDoc(data)
}

func parseDoc(data []byte) (accountDoc, error) {
	var doc accountDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return accountDoc{}, fmt.Errorf("authstore: decode credentials: %w", err)
	}
	if doc.Version >= 2 && len(doc.Users) > 0 {
		return doc, nil
	}
	var legacy domain.User
	if err := json.Unmarshal(data, &legacy); err == nil && legacy.Username != "" && len(doc.Users) == 0 {
		if legacy.Role == "" {
			legacy.Role = domain.RoleAdmin
		}
		return accountDoc{Version: docVersion, Users: []domain.User{legacy}}, nil
	}
	if doc.Version == 0 {
		doc.Version = docVersion
	}
	return doc, nil
}

func (s *FileStore) write(doc accountDoc) error {
	doc.Version = docVersion
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("authstore: encode credentials: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("authstore: write credentials: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("authstore: commit credentials: %w", err)
	}
	return nil
}

func (s *FileStore) IsInitialized() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, err := s.read()
	if err != nil {
		return false, err
	}
	return len(doc.Users) > 0, nil
}

func (s *FileStore) CreateAdmin(user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.read()
	if err != nil {
		return err
	}
	if len(doc.Users) > 0 {
		return domain.ErrAlreadyInitialized
	}
	if user.Role == "" {
		user.Role = domain.RoleAdmin
	}
	if user.SessionGeneration == 0 {
		user.SessionGeneration = 1
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	doc.Users = []domain.User{user}
	return s.write(doc)
}

func (s *FileStore) CreateUser(user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.read()
	if err != nil {
		return err
	}
	if findUser(doc.Users, user.Username) >= 0 {
		return domain.ErrUserExists
	}
	if len(doc.Users) >= domain.MaxUsers {
		return domain.ErrForbidden
	}
	if user.SessionGeneration == 0 {
		user.SessionGeneration = 1
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	doc.Users = append(doc.Users, user)
	return s.write(doc)
}

func (s *FileStore) GetUser(username string) (domain.User, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, err := s.read()
	if err != nil {
		return domain.User{}, false, err
	}
	i := findUser(doc.Users, username)
	if i < 0 {
		return domain.User{}, false, nil
	}
	return doc.Users[i], true, nil
}

func (s *FileStore) ListUsers() ([]domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, err := s.read()
	if err != nil {
		return nil, err
	}
	out := make([]domain.User, len(doc.Users))
	copy(out, doc.Users)
	return out, nil
}

func (s *FileStore) DeleteUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.read()
	if err != nil {
		return err
	}
	i := findUser(doc.Users, username)
	if i < 0 {
		return domain.ErrNotFound
	}
	doc.Users = append(doc.Users[:i], doc.Users[i+1:]...)
	return s.write(doc)
}

func (s *FileStore) BumpSessionGeneration(username string) (uint64, error) {
	return s.updateUser(username, func(u *domain.User) {
		u.SessionGeneration++
	})
}

func (s *FileStore) UpdatePassword(username, passwordHash string) (uint64, error) {
	return s.updateUser(username, func(u *domain.User) {
		u.PasswordHash = passwordHash
		u.SessionGeneration++
	})
}

func (s *FileStore) UpdateRole(username string, role domain.Role) (uint64, error) {
	return s.updateUser(username, func(u *domain.User) {
		u.Role = role
		u.SessionGeneration++
	})
}

func (s *FileStore) updateUser(username string, fn func(*domain.User)) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.read()
	if err != nil {
		return 0, err
	}
	i := findUser(doc.Users, username)
	if i < 0 {
		return 0, domain.ErrUnauthorized
	}
	fn(&doc.Users[i])
	if err := s.write(doc); err != nil {
		return 0, err
	}
	return doc.Users[i].SessionGeneration, nil
}

func (s *FileStore) SaveInvite(invite domain.Invite) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.read()
	if err != nil {
		return err
	}
	doc.Invites = append(doc.Invites, invite)
	return s.write(doc)
}

func (s *FileStore) ListInvites() ([]domain.Invite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, err := s.read()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Invite, len(doc.Invites))
	copy(out, doc.Invites)
	return out, nil
}

func (s *FileStore) AcceptInvite(tokenHash string, user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.read()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	idx := -1
	for i := range doc.Invites {
		inv := &doc.Invites[i]
		if inv.TokenHash != tokenHash {
			continue
		}
		if inv.Used() || now.After(inv.ExpiresAt) {
			return domain.ErrInviteInvalid
		}
		idx = i
		break
	}
	if idx < 0 {
		return domain.ErrInviteInvalid
	}
	if findUser(doc.Users, user.Username) >= 0 {
		return domain.ErrUserExists
	}
	if len(doc.Users) >= domain.MaxUsers {
		return domain.ErrForbidden
	}
	if user.Role == "" {
		user.Role = doc.Invites[idx].Role
	}
	if user.SessionGeneration == 0 {
		user.SessionGeneration = 1
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	doc.Invites[idx].UsedAt = now
	doc.Users = append(doc.Users, user)
	return s.write(doc)
}

func findUser(users []domain.User, username string) int {
	for i, u := range users {
		if u.Username == username {
			return i
		}
	}
	return -1
}

var _ ports.CredentialStore = (*FileStore)(nil)

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
