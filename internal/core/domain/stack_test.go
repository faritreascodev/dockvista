package domain

import "testing"

func TestValidateGitRemote(t *testing.T) {
	if err := ValidateGitRemote("https://github.com/acme/app.git"); err != nil {
		t.Fatalf("https: %v", err)
	}
	reject := []string{
		"",
		"http://github.com/acme/app.git",
		"file:///tmp/repo",
		"ssh://git@github.com/acme/app.git",
		"git@github.com:acme/app.git",
		"https://user:token@github.com/acme/app.git",
		"https://github.com/../etc",
		"https://github.com",
	}
	for _, raw := range reject {
		if err := ValidateGitRemote(raw); err != ErrGitRemote {
			t.Errorf("%q: got %v, want ErrGitRemote", raw, err)
		}
	}
}

func TestNormalizeGitRef(t *testing.T) {
	got, err := NormalizeGitRef("")
	if err != nil || got != DefaultGitRef {
		t.Fatalf("empty: %q %v", got, err)
	}
	if _, err := NormalizeGitRef("main"); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeGitRef("feature/x"); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeGitRef(".."); err == nil {
		t.Fatal(".. must fail")
	}
}

func TestValidateComposeRel(t *testing.T) {
	if err := ValidateComposeRel(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateComposeRel("deploy/compose.yml"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateComposeRel("../compose.yml"); err == nil {
		t.Fatal(".. must fail")
	}
	if err := ValidateComposeRel("/etc/compose.yml"); err == nil {
		t.Fatal("abs must fail")
	}
}
