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
	"time"

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

// SessionDuration is how long an issued session cookie stays valid.
const SessionDuration = 8 * time.Hour

// AuthService owns account setup, login, and session token verification.
// The token is HMAC-signed and carries a generation counter stored with the
// account, so logout can invalidate every copy without a session table.
type AuthService struct {
	store      ports.CredentialStore
	secret     []byte
	setupToken string
}

func NewAuthService(store ports.CredentialStore, sessionSecret []byte, setupToken string) *AuthService {
	return &AuthService{store: store, secret: sessionSecret, setupToken: setupToken}
}

func (s *AuthService) IsInitialized() (bool, error) {
	return s.store.IsInitialized()
}

// Setup creates the single admin account. It fails if one already exists —
// there is no invite flow or multi-user support. token must match the
// one-time setup token issued at process start.
func (s *AuthService) Setup(username, password, token string) error {
	if !tokensEqual(token, s.setupToken) {
		return domain.ErrUnauthorized
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("service: hash password: %w", err)
	}
	if err := s.store.CreateAdmin(domain.User{
		Username:          username,
		PasswordHash:      string(hash),
		SessionGeneration: 1,
	}); err != nil {
		return fmt.Errorf("service: create admin: %w", err)
	}
	return nil
}

func tokensEqual(got, want string) bool {
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
		// Spend the same bcrypt time as a real compare so a missing account
		// is not obviously faster than a wrong password.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
		return "", time.Time{}, domain.ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", time.Time{}, domain.ErrInvalidCredentials
	}

	expiresAt = time.Now().Add(SessionDuration)
	token, err = s.sign(username, expiresAt, user.SessionGeneration)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("service: sign session: %w", err)
	}
	return token, expiresAt, nil
}

// VerifySession checks a session token's signature and expiry and returns
// the username it was issued for.
func (s *AuthService) VerifySession(token string) (string, error) {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return "", domain.ErrUnauthorized
	}

	wantSig := s.sigFor(payload)
	if subtle.ConstantTimeCompare([]byte(sig), []byte(wantSig)) != 1 {
		return "", domain.ErrUnauthorized
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", domain.ErrUnauthorized
	}
	parts := strings.SplitN(string(raw), "|", 4)
	if len(parts) != 4 {
		return "", domain.ErrUnauthorized
	}
	username, expUnix, genStr := parts[0], parts[1], parts[2]

	exp, err := strconv.ParseInt(expUnix, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", domain.ErrUnauthorized
	}
	gen, err := strconv.ParseUint(genStr, 10, 64)
	if err != nil {
		return "", domain.ErrUnauthorized
	}

	user, ok, err := s.store.GetUser(username)
	if err != nil || !ok || user.SessionGeneration != gen {
		return "", domain.ErrUnauthorized
	}
	return username, nil
}

// RevokeSessions bumps the account generation so every token already issued
// fails verification. Single-user logout is global on purpose: there is one
// account, and a stolen cookie must die with it.
func (s *AuthService) RevokeSessions() error {
	if _, err := s.store.BumpSessionGeneration(); err != nil {
		return fmt.Errorf("service: revoke sessions: %w", err)
	}
	return nil
}

func (s *AuthService) sign(username string, expiresAt time.Time, generation uint64) (string, error) {
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(
		[]byte(fmt.Sprintf("%s|%d|%d|%s", username, expiresAt.Unix(), generation, hex.EncodeToString(nonce))),
	)
	return payload + "." + s.sigFor(payload), nil
}

func (s *AuthService) sigFor(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
