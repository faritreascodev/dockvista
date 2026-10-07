package domain

import (
	"errors"
	"regexp"
	"time"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

func ParseRole(s string) (Role, error) {
	switch Role(s) {
	case RoleAdmin, RoleOperator, RoleViewer:
		return Role(s), nil
	default:
		return "", ErrInvalidInput
	}
}

func (r Role) CanMutate() bool {
	return r == RoleAdmin || r == RoleOperator
}

func (r Role) CanManageUsers() bool {
	return r == RoleAdmin
}

// User is one account on this instance. The first user created by setup is
// always an admin. Everyone else arrives through a one-time invite.
type User struct {
	Username          string
	PasswordHash      string
	Role              Role
	SessionGeneration uint64
	CreatedAt         time.Time
}

type Principal struct {
	Username string
	Role     Role
}

type Invite struct {
	ID        string
	TokenHash string
	Role      Role
	CreatedBy string
	ExpiresAt time.Time
	UsedAt    time.Time
}

func (inv Invite) Used() bool {
	return !inv.UsedAt.IsZero()
}

var (
	ErrAlreadyInitialized = errors.New("admin account already exists")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrWeakPassword       = errors.New("password must be 8 to 72 bytes")
	ErrUserExists         = errors.New("username already exists")
	ErrLastAdmin          = errors.New("cannot remove or demote the last admin")
	ErrInviteInvalid      = errors.New("invite is invalid or expired")
	ErrLockedOut          = errors.New("too many failed logins")
)

const (
	MinPasswordBytes  = 8
	MaxPasswordBytes  = 72
	InviteTTL         = 72 * time.Hour
	MaxPendingInvites = 20
	MaxUsers          = 50
)

func ValidatePassword(password string) error {
	if n := len(password); n < MinPasswordBytes || n > MaxPasswordBytes {
		return ErrWeakPassword
	}
	return nil
}

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return ErrInvalidInput
	}
	return nil
}
