package domain

import "errors"

// Sentinel errors the service layer wraps around lower-level failures so
// transport adapters (HTTP) can map them to the right status code with
// errors.Is, instead of string-matching error messages.
var (
	ErrNotFound      = errors.New("resource not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrEngineTimeout = errors.New("docker engine timed out")
	ErrEngine        = errors.New("docker engine error")
	ErrNotRunning    = errors.New("container is not running")
	ErrNoShell       = errors.New("container has no shell")
	ErrTooLarge      = errors.New("file too large")
	ErrIsDirectory      = errors.New("path is a directory")
	ErrSSHUnsupported   = errors.New("SSH endpoints are not available yet")
	ErrLocalProtected   = errors.New("the local environment cannot be removed")
	ErrBuildUnsupported = errors.New("compose build is not supported; set image: on every service")
	ErrBindOutsideStack = errors.New("bind mounts must stay inside the stack workspace")
)
