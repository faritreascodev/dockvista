package domain

import (
	"net/url"
	"regexp"
	"strings"
	"time"
)

const LocalEnvironmentID = "local"

type EnvironmentKind string

const (
	EnvKindLocal EnvironmentKind = "local"
	EnvKindTCP   EnvironmentKind = "tcp"
)

type Environment struct {
	ID        string
	Name      string
	Kind      EnvironmentKind
	Host      string
	Reachable bool
	Version   string
	CreatedAt time.Time
}

type EnvironmentSpec struct {
	Name     string
	Kind     EnvironmentKind
	Host     string
	TLSCA    string
	TLSCert  string
	TLSKey   string
}

var envNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9 ._-]{0,39}$`)

func ValidateEnvironmentName(name string) error {
	if !envNamePattern.MatchString(strings.TrimSpace(name)) {
		return ErrInvalidInput
	}
	return nil
}

func ParseEnvironmentKind(s string) (EnvironmentKind, error) {
	switch EnvironmentKind(s) {
	case EnvKindTCP:
		return EnvKindTCP, nil
	case "ssh":
		return "", ErrSSHUnsupported
	default:
		return "", ErrInvalidInput
	}
}

func ValidateTCPHost(host string) error {
	u, err := url.Parse(strings.TrimSpace(host))
	if err != nil || u.Scheme != "tcp" || u.Host == "" {
		return ErrInvalidInput
	}
	if u.Port() == "" {
		return ErrInvalidInput
	}
	return nil
}

const MaxEnvironments = 20
