package service

import (
	"errors"
	"testing"
	"time"

	"dockvista/internal/core/domain"
)

// fakeCredentialStore implements ports.CredentialStore in memory, mirroring
// the fakeDocker pattern used for the container service tests.
type fakeCredentialStore struct {
	user *domain.User
}

func (f *fakeCredentialStore) IsInitialized() (bool, error) {
	return f.user != nil, nil
}

func (f *fakeCredentialStore) CreateAdmin(user domain.User) error {
	if f.user != nil {
		return domain.ErrAlreadyInitialized
	}
	f.user = &user
	return nil
}

func (f *fakeCredentialStore) GetUser(username string) (domain.User, bool, error) {
	if f.user == nil || f.user.Username != username {
		return domain.User{}, false, nil
	}
	return *f.user, true, nil
}

func (f *fakeCredentialStore) BumpSessionGeneration() (uint64, error) {
	if f.user == nil {
		return 0, domain.ErrUnauthorized
	}
	f.user.SessionGeneration++
	return f.user.SessionGeneration, nil
}

func newTestAuthService() *AuthService {
	return NewAuthService(&fakeCredentialStore{}, []byte("test-secret"), "test-setup-token")
}

func TestAuthService_SetupThenLogin(t *testing.T) {
	svc := newTestAuthService()

	if initialized, _ := svc.IsInitialized(); initialized {
		t.Fatal("expected not initialized before setup")
	}

	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	if initialized, _ := svc.IsInitialized(); !initialized {
		t.Fatal("expected initialized after setup")
	}

	token, expiresAt, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty session token")
	}
	if !expiresAt.After(time.Now()) {
		t.Fatal("expected expiry in the future")
	}

	username, err := svc.VerifySession(token)
	if err != nil {
		t.Fatalf("VerifySession: %v", err)
	}
	if username != "admin" {
		t.Fatalf("expected username 'admin', got %q", username)
	}
}

func TestAuthService_SetupTwiceFails(t *testing.T) {
	svc := newTestAuthService()

	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("first Setup: %v", err)
	}
	if err := svc.Setup("someone-else", "another-password", "test-setup-token"); !errors.Is(err, domain.ErrAlreadyInitialized) {
		t.Fatalf("expected ErrAlreadyInitialized, got %v", err)
	}
}

func TestAuthService_LoginWrongPassword(t *testing.T) {
	svc := newTestAuthService()
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	if _, _, err := svc.Login("admin", "wrong-password"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_VerifySession_RejectsTamperedToken(t *testing.T) {
	svc := newTestAuthService()
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	token, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	tampered := token + "x"
	if _, err := svc.VerifySession(tampered); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for tampered token, got %v", err)
	}
}

func TestAuthService_VerifySession_RejectsExpiredToken(t *testing.T) {
	svc := newTestAuthService()

	// White-box: sign a token that already expired a minute ago, bypassing
	// the fixed 24h SessionDuration so the expiry branch is actually
	// exercised instead of just re-testing signature tampering.
	expired, err := svc.sign("admin", time.Now().Add(-time.Minute), 1)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := svc.VerifySession(expired); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for expired token, got %v", err)
	}
}

func TestAuthService_SetupRejectsWrongToken(t *testing.T) {
	svc := newTestAuthService()
	if err := svc.Setup("admin", "correct-horse-battery", "nope"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_RevokeSessionsInvalidatesToken(t *testing.T) {
	svc := newTestAuthService()
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	token, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := svc.RevokeSessions(); err != nil {
		t.Fatalf("RevokeSessions: %v", err)
	}
	if _, err := svc.VerifySession(token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected revoked token to fail, got %v", err)
	}
}

func TestAuthService_VerifySession_RejectsMalformedToken(t *testing.T) {
	svc := newTestAuthService()

	for _, tok := range []string{"", "no-dot-separator", "onlyone.part.extra"} {
		if _, err := svc.VerifySession(tok); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("token %q: expected ErrUnauthorized, got %v", tok, err)
		}
	}
}
