package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"dockvista/internal/adapters/stackstore"
	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
)

type fakeGit struct {
	writes string
}

func (f fakeGit) Clone(ctx context.Context, dir, remote, ref, username, token string) error {
	return os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(f.writes), 0o600)
}

func (f fakeGit) Update(ctx context.Context, dir, remote, ref, username, token string) error {
	return os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(f.writes), 0o600)
}

func TestStackService_CreateFromGit(t *testing.T) {
	store, err := stackstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewStackService(store, nil, fakeGit{writes: "services:\n  web:\n    image: nginx:alpine\n"})
	st, err := svc.CreateFromGit(context.Background(), "probe", "https://github.com/acme/app.git", "main", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if st.GitURL != "https://github.com/acme/app.git" || st.ComposeFile != "compose.yml" {
		t.Fatalf("%+v", st)
	}
	if _, err := svc.CreateFromGit(context.Background(), "bad", "file:///tmp/x", "main", "", "", ""); err != domain.ErrGitRemote {
		t.Fatalf("file remote: %v", err)
	}
}

func TestStackService_SyncRequiresGit(t *testing.T) {
	store, err := stackstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewStackService(store, nil, fakeGit{})
	st, err := svc.Create("paste", "services:\n  web:\n    image: nginx\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Sync(context.Background(), st.ID); err == nil {
		t.Fatal("paste stack must not sync")
	}
}
