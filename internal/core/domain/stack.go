package domain

import (
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

const (
	MaxStackYAML      = 256 << 10
	MaxStacks         = 50
	ComposeProjectKey = "com.docker.compose.project"
	ComposeServiceKey = "com.docker.compose.service"
	StackLabelKey     = "dockvista.stack"
	DefaultGitRef     = "main"
)

type Stack struct {
	ID          string
	Name        string
	YAML        string
	GitURL      string
	GitRef      string
	ComposeFile string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

var stackNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

func ValidateStackName(name string) error {
	if !stackNamePattern.MatchString(strings.TrimSpace(name)) {
		return ErrInvalidInput
	}
	return nil
}

func ProjectNameFor(stackName string) string {
	return "dv-" + strings.TrimSpace(stackName)
}

var gitRefPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,200}$`)

// ValidateGitRemote allows only https remotes with no userinfo. file://, ssh,
// and git:// would either read the host or need keys we do not store.
func ValidateGitRemote(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Path == "" || u.Path == "/" {
		return ErrGitRemote
	}
	if u.Scheme != "https" {
		return ErrGitRemote
	}
	if u.User != nil {
		return ErrGitRemote
	}
	if strings.Contains(u.Host, "..") || strings.Contains(u.Path, "..") {
		return ErrGitRemote
	}
	return nil
}

func NormalizeGitRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return DefaultGitRef, nil
	}
	if strings.Contains(ref, "..") || !gitRefPattern.MatchString(ref) {
		return "", ErrInvalidInput
	}
	return ref, nil
}

func ValidateComposeRel(rel string) error {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" {
		return nil
	}
	if path.IsAbs(rel) || strings.HasPrefix(rel, "/") || strings.Contains(rel, "..") {
		return ErrInvalidInput
	}
	base := path.Base(rel)
	if !strings.HasSuffix(base, ".yml") && !strings.HasSuffix(base, ".yaml") {
		return ErrInvalidInput
	}
	return nil
}
