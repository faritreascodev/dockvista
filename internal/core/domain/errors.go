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
)
