package httpapi

import (
	"strings"
	"testing"
)

func TestIsValidSince(t *testing.T) {
	if !isValidSince("") || !isValidSince("5m") || !isValidSince("1h") || !isValidSince("1710000000") {
		t.Fatal("expected common since values to pass")
	}
	if isValidSince("yesterday") || isValidSince(";rm") {
		t.Fatal("expected garbage since to fail")
	}
}

func TestSplitLogTimestamp(t *testing.T) {
	ts, msg := splitLogTimestamp("2024-01-02T15:04:05.000000000Z hello world")
	if ts == "" || msg != "hello world" {
		t.Fatalf("ts=%q msg=%q", ts, msg)
	}
	ts, msg = splitLogTimestamp("no timestamp here")
	if ts != "" || msg != "no timestamp here" {
		t.Fatalf("plain line ts=%q msg=%q", ts, msg)
	}
}

func TestDownloadFilename(t *testing.T) {
	if got := downloadFilename(`/tmp/"evil".bin`); got == `/tmp/"evil".bin` || strings.Contains(got, `"`) {
		t.Fatalf("filename not sanitized: %q", got)
	}
	if got := downloadFilename("/"); got != "file" {
		t.Fatalf("root name = %q", got)
	}
}

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
