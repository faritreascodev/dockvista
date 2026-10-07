package service

import (
	"errors"
	"strings"
	"testing"

	"dockvista/internal/core/domain"
)

func TestAuthService_SetupTokenDiesWithFirstAdmin(t *testing.T) {
	store := &fakeCredentialStore{}
	svc := NewAuthService(store, []byte("test-secret"), "test-setup-token")
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// Simulate credentials.json being deleted under a running process.
	store.users = nil
	if err := svc.Setup("intruder", "another-password", "test-setup-token"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("reused setup token: err = %v, want ErrUnauthorized", err)
	}
}

func TestAuthService_SetupEnforcesPasswordBounds(t *testing.T) {
	cases := map[string]string{
		"too short":       "short",
		"past bcrypt cap": strings.Repeat("a", domain.MaxPasswordBytes+1),
	}
	for name, password := range cases {
		t.Run(name, func(t *testing.T) {
			svc := newTestAuthService()
			if err := svc.Setup("admin", password, "test-setup-token"); !errors.Is(err, domain.ErrWeakPassword) {
				t.Fatalf("err = %v, want ErrWeakPassword", err)
			}
		})
	}
}

func TestAuthService_ChangePassword(t *testing.T) {
	svc := newTestAuthService()
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	old, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, err := svc.VerifySession(old); err != nil {
		t.Fatalf("VerifySession before change: %v", err)
	}

	if _, _, err := svc.ChangePassword("admin", "wrong-current", "new-password-123"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("wrong current password: err = %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := svc.ChangePassword("admin", "correct-horse-battery", "short"); !errors.Is(err, domain.ErrWeakPassword) {
		t.Fatalf("weak new password: err = %v, want ErrWeakPassword", err)
	}

	fresh, _, err := svc.ChangePassword("admin", "correct-horse-battery", "new-password-123")
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := svc.VerifySession(old); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("session from before the change: err = %v, want ErrUnauthorized", err)
	}
	if _, err := svc.VerifySession(fresh); err != nil {
		t.Fatalf("token returned by ChangePassword: %v", err)
	}
	if _, _, err := svc.Login("admin", "correct-horse-battery"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("old password still logs in: err = %v", err)
	}
	if _, _, err := svc.Login("admin", "new-password-123"); err != nil {
		t.Fatalf("new password: %v", err)
	}
}
