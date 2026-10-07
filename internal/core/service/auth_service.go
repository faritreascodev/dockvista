package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

// dummyPasswordHash keeps a missing-account login on the same bcrypt cost as
// a real one. Generated once so the pad itself is not a per-request cost.
var dummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("dockvista-timing-pad"), bcrypt.DefaultCost)
	if err != nil {
		panic("auth: dummy bcrypt hash: " + err.Error())
	}
	return hash
}()

const (
	// SessionDuration is the absolute lifetime of a cookie.
	SessionDuration = 8 * time.Hour
	// DefaultIdleTimeout signs a session out after this much quiet. A tab
	// that keeps polling counts as activity.
	DefaultIdleTimeout = 30 * time.Minute
	// DefaultMaxSessions is how many live cookies one account may hold.
	// The least-recently used one is dropped when a new login arrives.
	DefaultMaxSessions = 5
)

// AuthService owns account setup, login, invites, and session tokens.
// The token is HMAC-signed and carries a generation counter stored with the
// account, so logout can invalidate every copy of that user's cookies
// without a session table.
type AuthService struct {
	store  ports.CredentialStore
	secret []byte

	// setupToken is cleared once the first admin exists. Otherwise wiping
	// credentials.json on a running process would let the same token create
	// a second admin without a restart.
	setupMu    sync.Mutex
	setupToken string

	// accounts caches username, role, and generation so VerifySession does
	// not read the credential file on every request (SSE, polling, stats).
	// This service is the only writer; the cache is updated in place on
	// revoke, role change, and password change.
	cacheMu  sync.Mutex
	accounts map[string]cachedAccount

	sessMu      sync.Mutex
	live        map[string]liveSession // nonce → last seen
	dead        map[string]time.Time   // evicted nonce → forget after
	idleTimeout time.Duration
	maxSessions int
}

type liveSession struct {
	username string
	lastSeen time.Time
}

type cachedAccount struct {
	username   string
	role       domain.Role
	generation uint64
}

func NewAuthService(store ports.CredentialStore, sessionSecret []byte, setupToken string) *AuthService {
	return &AuthService{
		store:       store,
		secret:      sessionSecret,
		setupToken:  setupToken,
		accounts:    make(map[string]cachedAccount),
		live:        make(map[string]liveSession),
		dead:        make(map[string]time.Time),
		idleTimeout: DefaultIdleTimeout,
		maxSessions: DefaultMaxSessions,
	}
}

// ConfigureSessions overrides idle timeout and the per-user cookie cap.
// An idle duration of zero disables idle expiry (absolute SessionDuration
// still applies). maxSessions <= 0 leaves the default cap in place.
func (s *AuthService) ConfigureSessions(idle time.Duration, maxSessions int) {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	s.idleTimeout = idle
	if maxSessions > 0 {
		s.maxSessions = maxSessions
	}
}

func (s *AuthService) IsInitialized() (bool, error) {
	return s.store.IsInitialized()
}

// Setup creates the first admin. It fails if any account already exists.
// token must match the one-time setup token issued at process start.
func (s *AuthService) Setup(username, password, token string) error {
	s.setupMu.Lock()
	defer s.setupMu.Unlock()

	initialized, err := s.store.IsInitialized()
	if err != nil {
		return fmt.Errorf("service: read auth state: %w", err)
	}
	if initialized {
		return domain.ErrAlreadyInitialized
	}
	if !tokensMatch(token, s.setupToken) {
		return domain.ErrUnauthorized
	}
	if err := domain.ValidateUsername(username); err != nil {
		return err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("service: hash password: %w", err)
	}
	if err := s.store.CreateAdmin(domain.User{
		Username:          username,
		PasswordHash:      string(hash),
		Role:              domain.RoleAdmin,
		SessionGeneration: 1,
		CreatedAt:         time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("service: create admin: %w", err)
	}
	s.setupToken = ""
	return nil
}

// ChangePassword replaces this user's password after checking the current
// one. Every existing session for that user is revoked by the same write,
// and a fresh token is returned so the caller stays signed in.
func (s *AuthService) ChangePassword(username, current, next string) (token string, expiresAt time.Time, err error) {
	user, ok, err := s.store.GetUser(username)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("service: lookup user: %w", err)
	}
	if !ok || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		return "", time.Time{}, domain.ErrInvalidCredentials
	}
	if err := domain.ValidatePassword(next); err != nil {
		return "", time.Time{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("service: hash password: %w", err)
	}

	s.cacheMu.Lock()
	gen, err := s.store.UpdatePassword(username, string(hash))
	if err == nil {
		s.rememberLocked(cachedAccount{username: username, role: user.Role, generation: gen})
	}
	s.cacheMu.Unlock()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("service: update password: %w", err)
	}

	expiresAt = time.Now().Add(SessionDuration)
	token, nonce, err := s.sign(username, expiresAt, gen)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("service: sign session: %w", err)
	}
	s.dropLive(username)
	s.registerSession(username, nonce)
	return token, expiresAt, nil
}

func tokensMatch(got, want string) bool {
	if want == "" || got == "" {
		return false
	}
	sumGot := sha256.Sum256([]byte(got))
	sumWant := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(sumGot[:], sumWant[:]) == 1
}

// Login verifies credentials and returns a signed session token plus its
// expiry, suitable for a cookie value.
func (s *AuthService) Login(username, password string) (token string, expiresAt time.Time, err error) {
	user, ok, err := s.store.GetUser(username)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("service: lookup user: %w", err)
	}
	if !ok {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
		return "", time.Time{}, domain.ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", time.Time{}, domain.ErrInvalidCredentials
	}

	expiresAt = time.Now().Add(SessionDuration)
	token, nonce, err := s.sign(username, expiresAt, user.SessionGeneration)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("service: sign session: %w", err)
	}
	s.remember(cachedAccount{username: user.Username, role: user.Role, generation: user.SessionGeneration})
	s.registerSession(username, nonce)
	return token, expiresAt, nil
}

// VerifySession checks a session token's signature and expiry and returns
// the principal it was issued for.
func (s *AuthService) VerifySession(token string) (domain.Principal, error) {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return domain.Principal{}, domain.ErrUnauthorized
	}

	wantSig := s.sigFor(payload)
	if subtle.ConstantTimeCompare([]byte(sig), []byte(wantSig)) != 1 {
		return domain.Principal{}, domain.ErrUnauthorized
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	parts := strings.SplitN(string(raw), "|", 4)
	if len(parts) != 4 {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	username, expUnix, genStr, nonce := parts[0], parts[1], parts[2], parts[3]

	exp, err := strconv.ParseInt(expUnix, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	gen, err := strconv.ParseUint(genStr, 10, 64)
	if err != nil {
		return domain.Principal{}, domain.ErrUnauthorized
	}

	current, ok, err := s.cachedAccount(username)
	if err != nil || !ok || current.generation != gen {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	if err := s.touchSession(username, nonce); err != nil {
		return domain.Principal{}, err
	}
	role := current.role
	if role == "" {
		role = domain.RoleAdmin
	}
	return domain.Principal{Username: username, Role: role}, nil
}

func (s *AuthService) cachedAccount(username string) (cachedAccount, bool, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	if cached, ok := s.accounts[username]; ok {
		return cached, true, nil
	}
	user, ok, err := s.store.GetUser(username)
	if err != nil || !ok {
		return cachedAccount{}, false, err
	}
	role := user.Role
	if role == "" {
		role = domain.RoleAdmin
	}
	cached := cachedAccount{username: user.Username, role: role, generation: user.SessionGeneration}
	s.rememberLocked(cached)
	return cached, true, nil
}

func (s *AuthService) remember(acc cachedAccount) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.rememberLocked(acc)
}

func (s *AuthService) rememberLocked(acc cachedAccount) {
	s.accounts[acc.username] = acc
}

func (s *AuthService) forgetLocked(username string) {
	delete(s.accounts, username)
}

// RevokeSessions bumps this user's generation so every token already issued
// for them fails verification. Other users stay signed in.
func (s *AuthService) RevokeSessions(username string) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	gen, err := s.store.BumpSessionGeneration(username)
	if err != nil {
		return fmt.Errorf("service: revoke sessions: %w", err)
	}
	if cached, ok := s.accounts[username]; ok {
		cached.generation = gen
		s.rememberLocked(cached)
	}
	s.dropLive(username)
	return nil
}

func (s *AuthService) ListUsers() ([]domain.User, error) {
	users, err := s.store.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("service: list users: %w", err)
	}
	return users, nil
}

func (s *AuthService) ListInvites() ([]domain.Invite, error) {
	invites, err := s.store.ListInvites()
	if err != nil {
		return nil, fmt.Errorf("service: list invites: %w", err)
	}
	return invites, nil
}

// CreateInvite issues a one-time link. The plaintext token is returned once
// and only the SHA-256 hash is stored.
func (s *AuthService) CreateInvite(actor domain.Principal, role domain.Role) (invite domain.Invite, token string, err error) {
	if !actor.Role.CanManageUsers() {
		return domain.Invite{}, "", domain.ErrForbidden
	}
	if _, err := domain.ParseRole(string(role)); err != nil {
		return domain.Invite{}, "", domain.ErrInvalidInput
	}

	pending, err := s.pendingInviteCount()
	if err != nil {
		return domain.Invite{}, "", err
	}
	if pending >= domain.MaxPendingInvites {
		return domain.Invite{}, "", domain.ErrForbidden
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return domain.Invite{}, "", fmt.Errorf("service: generate invite: %w", err)
	}
	token = hex.EncodeToString(raw)
	invite = domain.Invite{
		ID:        uuid.NewString(),
		TokenHash: hashToken(token),
		Role:      role,
		CreatedBy: actor.Username,
		ExpiresAt: time.Now().UTC().Add(domain.InviteTTL),
	}
	if err := s.store.SaveInvite(invite); err != nil {
		return domain.Invite{}, "", fmt.Errorf("service: save invite: %w", err)
	}
	return invite, token, nil
}

func (s *AuthService) pendingInviteCount() (int, error) {
	invites, err := s.store.ListInvites()
	if err != nil {
		return 0, fmt.Errorf("service: list invites: %w", err)
	}
	now := time.Now().UTC()
	n := 0
	for _, inv := range invites {
		if !inv.Used() && now.Before(inv.ExpiresAt) {
			n++
		}
	}
	return n, nil
}

// PeekInvite is the public preview for the accept screen. It never returns
// the hash.
func (s *AuthService) PeekInvite(token string) (domain.Invite, error) {
	if token == "" {
		return domain.Invite{}, domain.ErrInviteInvalid
	}
	invites, err := s.store.ListInvites()
	if err != nil {
		return domain.Invite{}, fmt.Errorf("service: list invites: %w", err)
	}
	want := hashToken(token)
	now := time.Now().UTC()
	for _, inv := range invites {
		if inv.TokenHash != want {
			continue
		}
		if inv.Used() || now.After(inv.ExpiresAt) {
			return domain.Invite{}, domain.ErrInviteInvalid
		}
		inv.TokenHash = ""
		return inv, nil
	}
	return domain.Invite{}, domain.ErrInviteInvalid
}

func (s *AuthService) AcceptInvite(token, username, password string) error {
	if err := domain.ValidateUsername(username); err != nil {
		return err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return err
	}
	if token == "" {
		return domain.ErrInviteInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("service: hash password: %w", err)
	}
	if err := s.store.AcceptInvite(hashToken(token), domain.User{
		Username:          username,
		PasswordHash:      string(hash),
		SessionGeneration: 1,
		CreatedAt:         time.Now().UTC(),
	}); err != nil {
		return err
	}
	return nil
}

func (s *AuthService) DeleteUser(actor domain.Principal, username string) error {
	if !actor.Role.CanManageUsers() {
		return domain.ErrForbidden
	}
	if username == actor.Username {
		return domain.ErrForbidden
	}
	users, err := s.store.ListUsers()
	if err != nil {
		return fmt.Errorf("service: list users: %w", err)
	}
	target, ok := findUser(users, username)
	if !ok {
		return domain.ErrNotFound
	}
	if target.Role == domain.RoleAdmin && countAdmins(users) <= 1 {
		return domain.ErrLastAdmin
	}
	if err := s.store.DeleteUser(username); err != nil {
		return fmt.Errorf("service: delete user: %w", err)
	}
	s.cacheMu.Lock()
	s.forgetLocked(username)
	s.cacheMu.Unlock()
	s.dropLive(username)
	return nil
}

func (s *AuthService) SetRole(actor domain.Principal, username string, role domain.Role) error {
	if !actor.Role.CanManageUsers() {
		return domain.ErrForbidden
	}
	if _, err := domain.ParseRole(string(role)); err != nil {
		return domain.ErrInvalidInput
	}
	users, err := s.store.ListUsers()
	if err != nil {
		return fmt.Errorf("service: list users: %w", err)
	}
	target, ok := findUser(users, username)
	if !ok {
		return domain.ErrNotFound
	}
	if target.Role == domain.RoleAdmin && role != domain.RoleAdmin && countAdmins(users) <= 1 {
		return domain.ErrLastAdmin
	}
	s.cacheMu.Lock()
	gen, err := s.store.UpdateRole(username, role)
	if err == nil {
		s.rememberLocked(cachedAccount{username: username, role: role, generation: gen})
	}
	s.cacheMu.Unlock()
	if err != nil {
		return fmt.Errorf("service: update role: %w", err)
	}
	s.dropLive(username)
	return nil
}

func findUser(users []domain.User, username string) (domain.User, bool) {
	for _, u := range users {
		if u.Username == username {
			return u, true
		}
	}
	return domain.User{}, false
}

func countAdmins(users []domain.User) int {
	n := 0
	for _, u := range users {
		if u.Role == domain.RoleAdmin || u.Role == "" {
			n++
		}
	}
	return n
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *AuthService) sign(username string, expiresAt time.Time, generation uint64) (token string, nonce string, err error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	nonce = hex.EncodeToString(raw)
	payload := base64.RawURLEncoding.EncodeToString(
		[]byte(fmt.Sprintf("%s|%d|%d|%s", username, expiresAt.Unix(), generation, nonce)),
	)
	return payload + "." + s.sigFor(payload), nonce, nil
}

func (s *AuthService) registerSession(username, nonce string) {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	delete(s.dead, nonce)
	s.live[nonce] = liveSession{username: username, lastSeen: time.Now()}
	s.evictLocked(username)
}

func (s *AuthService) dropLive(username string) {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	until := time.Now().Add(SessionDuration)
	for nonce, ls := range s.live {
		if ls.username == username {
			delete(s.live, nonce)
			s.dead[nonce] = until
		}
	}
}

func (s *AuthService) touchSession(username, nonce string) error {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	now := time.Now()
	s.pruneDeadLocked(now)
	if until, killed := s.dead[nonce]; killed && now.Before(until) {
		return domain.ErrUnauthorized
	}
	ls, ok := s.live[nonce]
	if !ok {
		s.live[nonce] = liveSession{username: username, lastSeen: now}
		s.evictLocked(username)
		return nil
	}
	if s.idleTimeout > 0 && now.Sub(ls.lastSeen) > s.idleTimeout {
		delete(s.live, nonce)
		s.dead[nonce] = now.Add(SessionDuration)
		return domain.ErrUnauthorized
	}
	ls.lastSeen = now
	s.live[nonce] = ls
	return nil
}

func (s *AuthService) pruneDeadLocked(now time.Time) {
	for nonce, until := range s.dead {
		if !now.Before(until) {
			delete(s.dead, nonce)
		}
	}
}

func (s *AuthService) evictLocked(username string) {
	if s.maxSessions <= 0 {
		return
	}
	type item struct {
		nonce string
		seen  time.Time
	}
	owned := make([]item, 0, s.maxSessions+1)
	for nonce, ls := range s.live {
		if ls.username == username {
			owned = append(owned, item{nonce, ls.lastSeen})
		}
	}
	if len(owned) <= s.maxSessions {
		return
	}
	for i := 1; i < len(owned); i++ {
		j := i
		for j > 0 && owned[j].seen.Before(owned[j-1].seen) {
			owned[j], owned[j-1] = owned[j-1], owned[j]
			j--
		}
	}
	until := time.Now().Add(SessionDuration)
	for _, extra := range owned[:len(owned)-s.maxSessions] {
		delete(s.live, extra.nonce)
		s.dead[extra.nonce] = until
	}
}

func (s *AuthService) sigFor(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
