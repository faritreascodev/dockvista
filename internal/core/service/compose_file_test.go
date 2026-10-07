package service

import (
	"strings"
	"testing"

	"dockvista/internal/core/domain"
)

func TestParseComposeYAML_RejectsBuildWithoutImage(t *testing.T) {
	_, err := parseComposeYAML("services:\n  web:\n    build: .\n", t.TempDir())
	if err != domain.ErrBuildUnsupported {
		t.Fatalf("got %v", err)
	}
}

func TestParseComposeYAML_Basic(t *testing.T) {
	dir := t.TempDir()
	parsed, err := parseComposeYAML(`
services:
  db:
    image: postgres:16
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
    depends_on: [db]
    environment:
      FOO: bar
volumes:
  data:
`, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Services) != 2 {
		t.Fatalf("services: %d", len(parsed.Services))
	}
	if parsed.Services[0].Name != "db" {
		t.Fatalf("topo: first %s", parsed.Services[0].Name)
	}
}

func TestParseComposeYAML_RejectsAbsBind(t *testing.T) {
	_, err := parseComposeYAML("services:\n  web:\n    image: nginx\n    volumes:\n      - /etc/passwd:/p\n", t.TempDir())
	if err != domain.ErrBindOutsideStack {
		t.Fatalf("got %v", err)
	}
}

func TestParseComposeYAML_RelativeBindStaysInside(t *testing.T) {
	dir := t.TempDir()
	parsed, err := parseComposeYAML("services:\n  web:\n    image: nginx\n    volumes:\n      - ./cfg:/cfg:ro\n", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Services[0].Binds) != 1 || !strings.Contains(parsed.Services[0].Binds[0], dir) {
		t.Fatalf("binds: %v", parsed.Services[0].Binds)
	}
}
