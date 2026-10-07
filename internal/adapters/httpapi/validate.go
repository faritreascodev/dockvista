package httpapi

import (
	"regexp"
	"strconv"
)

// containerIDPattern matches valid Docker container IDs and names: the
// engine accepts full/short hex IDs as well as user-assigned names, both of
// which fit this charset (see moby/moby's own container name validator).
// Rejecting anything outside it here means we never hand attacker-controlled
// path segments straight to the Docker socket.
var containerIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func isValidContainerID(id string) bool {
	return containerIDPattern.MatchString(id)
}

// imageRefPattern matches an image ID (bare hex or "sha256:<hex>") or a
// reference (registry/namespace/repo:tag or repo@sha256:digest) — permissive
// enough for real-world references while still rejecting garbage input. No
// command injection risk either way: these values only ever reach the
// Docker SDK's HTTP client, never a shell.
var imageRefPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_./:@-]{0,255}$`)

func isValidImageRef(ref string) bool {
	return imageRefPattern.MatchString(ref)
}

// isValidResourceName validates a volume or network name/ID — same charset
// as a container ID (Docker applies the same naming rule to all three).
func isValidResourceName(name string) bool {
	return containerIDPattern.MatchString(name)
}

var portNumberPattern = regexp.MustCompile(`^[0-9]{1,5}$`)

func isValidPortNumber(port string) bool {
	if !portNumberPattern.MatchString(port) {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

func isValidProtocol(proto string) bool {
	return proto == "tcp" || proto == "udp"
}

var validSincePattern = regexp.MustCompile(`^([0-9]{1,12}|[0-9]{1,6}[smhd]|[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.+-]+)$`)

func isValidSince(since string) bool {
	return since == "" || validSincePattern.MatchString(since)
}

func isValidRestartPolicy(policy string) bool {
	switch policy {
	case "no", "always", "on-failure", "unless-stopped":
		return true
	default:
		return false
	}
}
