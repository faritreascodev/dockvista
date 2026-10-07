package ports

import "dockvista/internal/core/domain"

// CredentialStore persists accounts and invites as JSON under the data dir.
type CredentialStore interface {
	IsInitialized() (bool, error)
	CreateAdmin(user domain.User) error
	CreateUser(user domain.User) error
	GetUser(username string) (domain.User, bool, error)
	ListUsers() ([]domain.User, error)
	DeleteUser(username string) error
	BumpSessionGeneration(username string) (uint64, error)
	UpdatePassword(username, passwordHash string) (generation uint64, err error)
	UpdateRole(username string, role domain.Role) (generation uint64, err error)
	SaveInvite(invite domain.Invite) error
	ListInvites() ([]domain.Invite, error)
	AcceptInvite(tokenHash string, user domain.User) error
}
