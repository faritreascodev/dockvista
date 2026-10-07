package service_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
)

type fakeImageClient struct {
	images    []domain.Image
	removeErr error
	removedID string
	pruneN    int
	pruneSize uint64
}

func (f *fakeImageClient) ListImages(ctx context.Context) ([]domain.Image, error) {
	return f.images, nil
}

func (f *fakeImageClient) RemoveImage(ctx context.Context, id string, force bool) error {
	f.removedID = id
	return f.removeErr
}

func (f *fakeImageClient) PullImage(ctx context.Context, ref, registryAuth string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(`{"status":"pulling"}` + "\n")), nil
}

func (f *fakeImageClient) ImageHistory(ctx context.Context, id string) ([]domain.ImageLayer, error) {
	return []domain.ImageLayer{{ID: id, CreatedBy: "FROM scratch"}}, nil
}

func (f *fakeImageClient) PruneImages(ctx context.Context) (int, uint64, error) {
	return f.pruneN, f.pruneSize, nil
}

func TestImageService_List(t *testing.T) {
	fake := &fakeImageClient{images: []domain.Image{{ID: "sha256:abc", RepoTags: []string{"nginx:latest"}}}}
	svc := service.NewImageService(fake)

	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != "sha256:abc" {
		t.Fatalf("expected 1 image sha256:abc, got %+v", got)
	}
}

func TestImageService_Remove(t *testing.T) {
	fake := &fakeImageClient{}
	svc := service.NewImageService(fake)

	if err := svc.Remove(context.Background(), "sha256:abc", true); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if fake.removedID != "sha256:abc" {
		t.Fatalf("expected removedID sha256:abc, got %q", fake.removedID)
	}
}

func TestImageService_RemoveWrapsError(t *testing.T) {
	sentinel := errors.New("boom")
	fake := &fakeImageClient{removeErr: sentinel}
	svc := service.NewImageService(fake)

	err := svc.Remove(context.Background(), "sha256:abc", false)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel error, got %v", err)
	}
}

func TestImageService_Pull(t *testing.T) {
	svc := service.NewImageService(&fakeImageClient{})

	r, err := svc.Pull(context.Background(), "nginx:latest")
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	defer r.Close()

	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !strings.Contains(string(body), "pulling") {
		t.Fatalf("expected pull progress stream, got %q", body)
	}
}

func TestImageService_Prune(t *testing.T) {
	fake := &fakeImageClient{pruneN: 3, pruneSize: 1024}
	svc := service.NewImageService(fake)

	deleted, reclaimed, err := svc.Prune(context.Background())
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 3 || reclaimed != 1024 {
		t.Fatalf("expected (3, 1024), got (%d, %d)", deleted, reclaimed)
	}
}
