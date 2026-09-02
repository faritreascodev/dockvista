package domain

import "errors"

// User is the single administrator account DockVista supports. There is no
// multi-user/role model — anyone who authenticates has full Docker socket
// access, the same trust boundary Portainer and Docker Desktop themselves
// operate under.
type User struct {
	Username     string
	PasswordHash string
}

var (
	ErrAlreadyInitialized = errors.New("admin account already exists")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUnauthorized       = errors.New("unauthorized")
)
