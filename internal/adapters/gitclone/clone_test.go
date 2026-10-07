package gitclone

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestCloneAndUpdateLocalRepo(t *testing.T) {
	src := t.TempDir()
	repo, err := git.PlainInit(src, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "compose.yml"), []byte("services:\n  web:\n    image: nginx\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("compose.yml"); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	c := New()
	if err := c.Clone(context.Background(), filepath.Join(dst, "clone"), src, "master", "", ""); err != nil {
		if err := c.Clone(context.Background(), filepath.Join(dst, "clone2"), src, "main", "", ""); err != nil {
			t.Fatalf("clone: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dst, "clone2", "compose.yml")); err != nil {
			t.Fatal(err)
		}
		return
	}
	if _, err := os.Stat(filepath.Join(dst, "clone", "compose.yml")); err != nil {
		t.Fatal(err)
	}
}
