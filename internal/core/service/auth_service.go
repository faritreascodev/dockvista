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

// SessionDuration is how long an issued session cookie stays valid.
const SessionDuration = 24 * time.Hour

// AuthService owns account setup, login, and session token verification.
// Sessions are stateless (HMAC-signed, not stored server-side), so
// verification never touches the credential store.
type AuthService struct {
	store  ports.CredentialStore
	secret []byte
}

func NewAuthService(store ports.CredentialStore, sessionSecret []byte) *AuthService {
	return &AuthService{store: store, secret: sessionSecret}
}

func (s *AuthService) IsInitialized() (bool, error) {
	return s.store.IsInitialized()
}

// Setup creates the single admin account. It fails if one already exists —
// there is no invite flow or multi-user support.
func (s *AuthService) Setup(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("service: hash password: %w", err)
	}
	if err := s.store.CreateAdmin(domain.User{Username: username, PasswordHash: string(hash)}); err != nil {
		return fmt.Errorf("service: create admin: %w", err)
	}
	return nil
}

// Login verifies credentials and returns a signed session token plus its
// expiry, suitable for a cookie value.
func (s *AuthService) Login(username, password string) (token string, expiresAt time.Time, err error) {
	user, ok, err := s.store.GetUser(username)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("service: lookup user: %w", err)
	}
	if !ok {
		return "", time.Time{}, domain.ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", time.Time{}, domain.ErrInvalidCredentials
	}

	expiresAt = time.Now().Add(SessionDuration)
	token, err = s.sign(username, expiresAt)
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
	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 {
		return "", domain.ErrUnauthorized
	}
	username, expUnix := parts[0], parts[1]

	exp, err := strconv.ParseInt(expUnix, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", domain.ErrUnauthorized
	}
	return username, nil
}

func (s *AuthService) sign(username string, expiresAt time.Time) (string, error) {
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(
		[]byte(fmt.Sprintf("%s|%d|%s", username, expiresAt.Unix(), hex.EncodeToString(nonce))),
	)
	return payload + "." + s.sigFor(payload), nil
}

func (s *AuthService) sigFor(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
