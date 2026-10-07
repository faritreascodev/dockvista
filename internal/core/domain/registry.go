package domain

import (
	"strings"
)

const MaxRegistries = 30

type Registry struct {
	ID       string
	Host     string
	Username string
}

func NormalizeRegistryHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	h = strings.TrimSuffix(h, "/")
	if h == "index.docker.io" || h == "registry-1.docker.io" || h == "docker.io" {
		return "docker.io"
	}
	return h
}

// RegistryHostFromImage returns the registry hostname encoded in a pull
// reference. Bare names like "nginx" are Docker Hub.
func RegistryHostFromImage(ref string) string {
	ref = strings.TrimSpace(ref)
	if i := strings.IndexByte(ref, '@'); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.LastIndexByte(ref, ':'); i >= 0 && !strings.Contains(ref[i:], "/") {
		ref = ref[:i]
	}
	first := ref
	if i := strings.IndexByte(ref, '/'); i >= 0 {
		first = ref[:i]
	}
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return NormalizeRegistryHost(first)
	}
	return "docker.io"
}
