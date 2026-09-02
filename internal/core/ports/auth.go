package ports

import "dockvista/internal/core/domain"

// CredentialStore persists the single admin account. Concrete implementation
// lives in internal/adapters/authstore.
type CredentialStore interface {
	IsInitialized() (bool, error)
	CreateAdmin(user domain.User) error
	GetUser(username string) (domain.User, bool, error)
}
