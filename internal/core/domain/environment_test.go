package domain

import "testing"

func TestValidateTCPHost(t *testing.T) {
	if err := ValidateTCPHost("tcp://192.0.2.10:2376"); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if err := ValidateTCPHost("unix:///var/run/docker.sock"); err == nil {
		t.Fatal("unix must be rejected")
	}
	if err := ValidateTCPHost("tcp://host"); err == nil {
		t.Fatal("missing port must be rejected")
	}
}

func TestRegistryHostFromImage(t *testing.T) {
	cases := map[string]string{
		"nginx":                          "docker.io",
		"library/nginx:latest":           "docker.io",
		"ghcr.io/acme/app:1":             "ghcr.io",
		"localhost:5000/foo":             "localhost:5000",
		"registry.example.com/a/b@sha256:dead": "registry.example.com",
	}
	for ref, want := range cases {
		if got := RegistryHostFromImage(ref); got != want {
			t.Errorf("%q: got %q want %q", ref, got, want)
		}
	}
}

func TestParseEnvironmentKindSSH(t *testing.T) {
	if _, err := ParseEnvironmentKind("ssh"); err != ErrSSHUnsupported {
		t.Fatalf("got %v", err)
	}
}
