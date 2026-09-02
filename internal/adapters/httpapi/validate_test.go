package httpapi

import "testing"

func TestIsValidContainerID(t *testing.T) {
	valid := []string{
		"c7d3e4f5a6b8",
		"api-server",
		"api_server.1",
		"A",
		"9abc",
	}
	for _, id := range valid {
		if !isValidContainerID(id) {
			t.Errorf("expected %q to be valid", id)
		}
	}

	invalid := []string{
		"",
		"../etc/passwd",
		"foo/bar",
		"foo bar",
		"foo;rm -rf /",
		"-leadingdash",
		string(make([]byte, 200)),
	}
	for _, id := range invalid {
		if isValidContainerID(id) {
			t.Errorf("expected %q to be invalid", id)
		}
	}
}
