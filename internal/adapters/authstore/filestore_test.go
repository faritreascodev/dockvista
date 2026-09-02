package authstore_test

import (
	"errors"
	"testing"

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
	if got != admin {
		t.Fatalf("expected %+v, got %+v", admin, got)
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
