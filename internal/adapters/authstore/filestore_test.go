package authstore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dockvista/internal/adapters/authstore"
	"dockvista/internal/core/domain"
)

func TestFileStore_CreateAdminThenGetUser(t *testing.T) {
	store, err := authstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if initialized, err := store.IsInitialized(); err != nil || initialized {
		t.Fatalf("expected not initialized, got initialized=%v err=%v", initialized, err)
	}

	admin := domain.User{Username: "admin", PasswordHash: "hashed"}
	if err := store.CreateAdmin(admin); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	if initialized, err := store.IsInitialized(); err != nil || !initialized {
		t.Fatalf("expected initialized, got initialized=%v err=%v", initialized, err)
	}

	got, ok, err := store.GetUser("admin")
	if err != nil || !ok {
		t.Fatalf("GetUser: ok=%v err=%v", ok, err)
	}
	if got.Username != "admin" || got.PasswordHash != "hashed" || got.Role != domain.RoleAdmin {
		t.Fatalf("got %+v, want admin with RoleAdmin", got)
	}
	if got.SessionGeneration != 1 {
		t.Fatalf("generation = %d, want 1", got.SessionGeneration)
	}

	if _, ok, err := store.GetUser("nobody"); err != nil || ok {
		t.Fatalf("expected unknown user to miss, got ok=%v err=%v", ok, err)
	}
}

func TestFileStore_CreateAdminTwiceFails(t *testing.T) {
	store, err := authstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := store.CreateAdmin(domain.User{Username: "admin", PasswordHash: "hashed"}); err != nil {
		t.Fatalf("first CreateAdmin: %v", err)
	}
	err = store.CreateAdmin(domain.User{Username: "someone-else", PasswordHash: "other"})
	if !errors.Is(err, domain.ErrAlreadyInitialized) {
		t.Fatalf("expected ErrAlreadyInitialized, got %v", err)
	}
}

func TestFileStore_PersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()

	first, err := authstore.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := first.CreateAdmin(domain.User{Username: "admin", PasswordHash: "hashed"}); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	second, err := authstore.New(dir)
	if err != nil {
		t.Fatalf("New (second instance): %v", err)
	}
	if initialized, err := second.IsInitialized(); err != nil || !initialized {
		t.Fatalf("expected credentials to survive across instances, initialized=%v err=%v", initialized, err)
	}
}

func TestLoadOrCreateSessionSecret_StableAcrossCalls(t *testing.T) {
	dir := t.TempDir()

	first, err := authstore.LoadOrCreateSessionSecret(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateSessionSecret: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected non-empty secret")
	}

	second, err := authstore.LoadOrCreateSessionSecret(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateSessionSecret (second call): %v", err)
	}

	if string(first) != string(second) {
		t.Fatal("expected the same secret to be reloaded, not regenerated")
	}
}

func TestFileStore_BumpSessionGenerationPersists(t *testing.T) {
	dir := t.TempDir()
	store, err := authstore.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := store.CreateAdmin(domain.User{Username: "admin", PasswordHash: "hashed", SessionGeneration: 1}); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	next, err := store.BumpSessionGeneration("admin")
	if err != nil || next != 2 {
		t.Fatalf("BumpSessionGeneration = %d, %v", next, err)
	}

	reloaded, err := authstore.New(dir)
	if err != nil {
		t.Fatalf("New (reloaded): %v", err)
	}
	user, ok, err := reloaded.GetUser("admin")
	if err != nil || !ok || user.SessionGeneration != 2 {
		t.Fatalf("persisted generation: ok=%v err=%v user=%+v", ok, err, user)
	}
}

func TestFileStore_MigratesV1Credentials(t *testing.T) {
	dir := t.TempDir()
	legacy := []byte(`{"Username":"farit","PasswordHash":"hashed","SessionGeneration":3}`)
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), legacy, 0o600); err != nil {
		t.Fatalf("write v1: %v", err)
	}

	store, err := authstore.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, ok, err := store.GetUser("farit")
	if err != nil || !ok {
		t.Fatalf("GetUser: ok=%v err=%v", ok, err)
	}
	if got.Role != domain.RoleAdmin || got.SessionGeneration != 3 {
		t.Fatalf("migrated user = %+v, want admin gen 3", got)
	}

	if _, err := store.BumpSessionGeneration("farit"); err != nil {
		t.Fatalf("BumpSessionGeneration: %v", err)
	}
	reloaded, err := authstore.New(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	again, ok, err := reloaded.GetUser("farit")
	if err != nil || !ok || again.Role != domain.RoleAdmin || again.SessionGeneration != 4 {
		t.Fatalf("persisted v2 wrap: ok=%v err=%v user=%+v", ok, err, again)
	}
}

func TestFileStore_AcceptInviteCreatesUser(t *testing.T) {
	store, err := authstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := store.CreateAdmin(domain.User{Username: "admin", PasswordHash: "h"}); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	inv := domain.Invite{
		ID:        "inv-1",
		TokenHash: "deadbeef",
		Role:      domain.RoleViewer,
		CreatedBy: "admin",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := store.SaveInvite(inv); err != nil {
		t.Fatalf("SaveInvite: %v", err)
	}
	if err := store.AcceptInvite("deadbeef", domain.User{Username: "look", PasswordHash: "vh"}); err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	got, ok, err := store.GetUser("look")
	if err != nil || !ok || got.Role != domain.RoleViewer {
		t.Fatalf("viewer: ok=%v err=%v user=%+v", ok, err, got)
	}
	if err := store.AcceptInvite("deadbeef", domain.User{Username: "other", PasswordHash: "x"}); !errors.Is(err, domain.ErrInviteInvalid) {
		t.Fatalf("reused invite: %v", err)
	}
}
