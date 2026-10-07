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
	users   []domain.User
	invites []domain.Invite
}

func (f *fakeCredentialStore) find(username string) int {
	for i, u := range f.users {
		if u.Username == username {
			return i
		}
	}
	return -1
}

func (f *fakeCredentialStore) IsInitialized() (bool, error) {
	return len(f.users) > 0, nil
}

func (f *fakeCredentialStore) CreateAdmin(user domain.User) error {
	if len(f.users) > 0 {
		return domain.ErrAlreadyInitialized
	}
	if user.Role == "" {
		user.Role = domain.RoleAdmin
	}
	f.users = []domain.User{user}
	return nil
}

func (f *fakeCredentialStore) CreateUser(user domain.User) error {
	if f.find(user.Username) >= 0 {
		return domain.ErrUserExists
	}
	f.users = append(f.users, user)
	return nil
}

func (f *fakeCredentialStore) GetUser(username string) (domain.User, bool, error) {
	i := f.find(username)
	if i < 0 {
		return domain.User{}, false, nil
	}
	return f.users[i], true, nil
}

func (f *fakeCredentialStore) ListUsers() ([]domain.User, error) {
	out := make([]domain.User, len(f.users))
	copy(out, f.users)
	return out, nil
}

func (f *fakeCredentialStore) DeleteUser(username string) error {
	i := f.find(username)
	if i < 0 {
		return domain.ErrNotFound
	}
	f.users = append(f.users[:i], f.users[i+1:]...)
	return nil
}

func (f *fakeCredentialStore) BumpSessionGeneration(username string) (uint64, error) {
	i := f.find(username)
	if i < 0 {
		return 0, domain.ErrUnauthorized
	}
	f.users[i].SessionGeneration++
	return f.users[i].SessionGeneration, nil
}

func (f *fakeCredentialStore) UpdatePassword(username, hash string) (uint64, error) {
	i := f.find(username)
	if i < 0 {
		return 0, domain.ErrUnauthorized
	}
	f.users[i].PasswordHash = hash
	f.users[i].SessionGeneration++
	return f.users[i].SessionGeneration, nil
}

func (f *fakeCredentialStore) UpdateRole(username string, role domain.Role) (uint64, error) {
	i := f.find(username)
	if i < 0 {
		return 0, domain.ErrUnauthorized
	}
	f.users[i].Role = role
	f.users[i].SessionGeneration++
	return f.users[i].SessionGeneration, nil
}

func (f *fakeCredentialStore) SaveInvite(invite domain.Invite) error {
	f.invites = append(f.invites, invite)
	return nil
}

func (f *fakeCredentialStore) ListInvites() ([]domain.Invite, error) {
	out := make([]domain.Invite, len(f.invites))
	copy(out, f.invites)
	return out, nil
}

func (f *fakeCredentialStore) AcceptInvite(tokenHash string, user domain.User) error {
	now := time.Now().UTC()
	for i := range f.invites {
		inv := &f.invites[i]
		if inv.TokenHash != tokenHash {
			continue
		}
		if inv.Used() || now.After(inv.ExpiresAt) {
			return domain.ErrInviteInvalid
		}
		if f.find(user.Username) >= 0 {
			return domain.ErrUserExists
		}
		if user.Role == "" {
			user.Role = inv.Role
		}
		inv.UsedAt = now
		f.users = append(f.users, user)
		return nil
	}
	return domain.ErrInviteInvalid
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

	p, err := svc.VerifySession(token)
	if err != nil {
		t.Fatalf("VerifySession: %v", err)
	}
	if p.Username != "admin" || p.Role != domain.RoleAdmin {
		t.Fatalf("principal = %+v, want admin/admin", p)
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

	expired, _, err := svc.sign("admin", time.Now().Add(-time.Minute), 1)
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
	if err := svc.RevokeSessions("admin"); err != nil {
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

type countingCredentialStore struct {
	fakeCredentialStore
	getUserCalls int
}

func (c *countingCredentialStore) GetUser(username string) (domain.User, bool, error) {
	c.getUserCalls++
	return c.fakeCredentialStore.GetUser(username)
}

func TestAuthService_VerifySession_ReadsStoreOnlyOnce(t *testing.T) {
	store := &countingCredentialStore{}
	svc := NewAuthService(store, []byte("test-secret"), "test-setup-token")
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	token, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	store.getUserCalls = 0
	for i := 0; i < 50; i++ {
		if _, err := svc.VerifySession(token); err != nil {
			t.Fatalf("VerifySession #%d: %v", i, err)
		}
	}
	if store.getUserCalls > 1 {
		t.Fatalf("50 verifications read the store %d times, want at most 1", store.getUserCalls)
	}
}

func TestAuthService_RevokeInvalidatesCachedSession(t *testing.T) {
	svc := newTestAuthService()
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	token, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, err := svc.VerifySession(token); err != nil {
		t.Fatalf("VerifySession before revoke: %v", err)
	}
	if err := svc.RevokeSessions("admin"); err != nil {
		t.Fatalf("RevokeSessions: %v", err)
	}
	if _, err := svc.VerifySession(token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("VerifySession after revoke = %v, want ErrUnauthorized", err)
	}
	fresh, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("Login after revoke: %v", err)
	}
	if _, err := svc.VerifySession(fresh); err != nil {
		t.Fatalf("new session after revoke: %v", err)
	}
}

func TestAuthService_InviteViewerCannotMutateOrInvite(t *testing.T) {
	svc := newTestAuthService()
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	admin := domain.Principal{Username: "admin", Role: domain.RoleAdmin}

	_, token, err := svc.CreateInvite(admin, domain.RoleViewer)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	peek, err := svc.PeekInvite(token)
	if err != nil {
		t.Fatalf("PeekInvite: %v", err)
	}
	if peek.Role != domain.RoleViewer || peek.TokenHash != "" {
		t.Fatalf("peek = %+v, want viewer with hash stripped", peek)
	}

	if err := svc.AcceptInvite(token, "look", "viewer-password"); err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	if err := svc.AcceptInvite(token, "look2", "viewer-password"); !errors.Is(err, domain.ErrInviteInvalid) {
		t.Fatalf("reused invite: %v, want ErrInviteInvalid", err)
	}

	lookTok, _, err := svc.Login("look", "viewer-password")
	if err != nil {
		t.Fatalf("viewer login: %v", err)
	}
	p, err := svc.VerifySession(lookTok)
	if err != nil {
		t.Fatalf("VerifySession: %v", err)
	}
	if p.Role.CanMutate() || p.Role.CanManageUsers() {
		t.Fatalf("viewer principal = %+v", p)
	}

	if _, _, err := svc.CreateInvite(p, domain.RoleOperator); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("viewer invite: %v, want ErrForbidden", err)
	}
}

func TestAuthService_OperatorCannotManageUsers(t *testing.T) {
	svc := newTestAuthService()
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	admin := domain.Principal{Username: "admin", Role: domain.RoleAdmin}
	_, token, err := svc.CreateInvite(admin, domain.RoleOperator)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if err := svc.AcceptInvite(token, "ops", "operator-password"); err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}

	ops := domain.Principal{Username: "ops", Role: domain.RoleOperator}
	if !ops.Role.CanMutate() {
		t.Fatal("operator should mutate docker")
	}
	if _, _, err := svc.CreateInvite(ops, domain.RoleViewer); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("operator invite: %v, want ErrForbidden", err)
	}
	if err := svc.DeleteUser(ops, "admin"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("operator delete: %v, want ErrForbidden", err)
	}
}

func TestAuthService_CannotRemoveLastAdmin(t *testing.T) {
	svc := newTestAuthService()
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	admin := domain.Principal{Username: "admin", Role: domain.RoleAdmin}
	if err := svc.DeleteUser(admin, "admin"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("self-delete: %v, want ErrForbidden", err)
	}
	if err := svc.SetRole(admin, "admin", domain.RoleViewer); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("demote last admin: %v, want ErrLastAdmin", err)
	}
}

func TestAuthService_LogoutDoesNotKickOtherUsers(t *testing.T) {
	svc := newTestAuthService()
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	admin := domain.Principal{Username: "admin", Role: domain.RoleAdmin}
	_, token, err := svc.CreateInvite(admin, domain.RoleOperator)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if err := svc.AcceptInvite(token, "ops", "operator-password"); err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}

	adminTok, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("admin login: %v", err)
	}
	opsTok, _, err := svc.Login("ops", "operator-password")
	if err != nil {
		t.Fatalf("ops login: %v", err)
	}
	if err := svc.RevokeSessions("ops"); err != nil {
		t.Fatalf("RevokeSessions: %v", err)
	}
	if _, err := svc.VerifySession(opsTok); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("ops token after logout: %v", err)
	}
	if _, err := svc.VerifySession(adminTok); err != nil {
		t.Fatalf("admin token after ops logout: %v", err)
	}
}

func TestAuthService_IdleTimeoutExpiresSession(t *testing.T) {
	svc := newTestAuthService()
	svc.ConfigureSessions(50*time.Millisecond, 5)
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	token, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, err := svc.VerifySession(token); err != nil {
		t.Fatalf("fresh session: %v", err)
	}
	time.Sleep(120 * time.Millisecond)
	if _, err := svc.VerifySession(token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("idle session: %v, want ErrUnauthorized", err)
	}
}

func TestAuthService_SessionCapDropsOldest(t *testing.T) {
	svc := newTestAuthService()
	svc.ConfigureSessions(time.Hour, 2)
	if err := svc.Setup("admin", "correct-horse-battery", "test-setup-token"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	first, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("login 1: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	second, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("login 2: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	third, _, err := svc.Login("admin", "correct-horse-battery")
	if err != nil {
		t.Fatalf("login 3: %v", err)
	}
	if _, err := svc.VerifySession(first); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("oldest session: %v, want ErrUnauthorized", err)
	}
	if _, err := svc.VerifySession(second); err != nil {
		t.Fatalf("second session: %v", err)
	}
	if _, err := svc.VerifySession(third); err != nil {
		t.Fatalf("third session: %v", err)
	}
}
