package domain

import (
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
)

type Stack struct {
	ID        string
	Name      string
	YAML      string
	CreatedAt time.Time
	UpdatedAt time.Time
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
