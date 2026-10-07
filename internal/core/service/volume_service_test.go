package service_test

import (
	"context"
	"io"
	"testing"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
)

type fakeVolumeClient struct {
	volumes   []domain.Volume
	created   domain.Volume
	removedID string
	pruneN    int
	pruneSize uint64
}

func (f *fakeVolumeClient) ListVolumes(ctx context.Context) ([]domain.Volume, error) {
	return f.volumes, nil
}

func (f *fakeVolumeClient) CreateVolume(ctx context.Context, name, driver string, labels map[string]string) (domain.Volume, error) {
	f.created = domain.Volume{Name: name, Driver: driver, Labels: labels}
	return f.created, nil
}

func (f *fakeVolumeClient) RemoveVolume(ctx context.Context, name string, force bool) error {
	f.removedID = name
	return nil
}

func (f *fakeVolumeClient) PruneVolumes(ctx context.Context) (int, uint64, error) {
	return f.pruneN, f.pruneSize, nil
}

func (f *fakeVolumeClient) ListVolumeDir(ctx context.Context, volumeName, relPath string) (domain.DirListing, error) {
	return domain.DirListing{Path: relPath, Reason: domain.ListReasonNotRunning}, nil
}

func (f *fakeVolumeClient) StatVolumePath(ctx context.Context, volumeName, relPath string) (domain.FSEntry, error) {
	return domain.FSEntry{}, domain.ErrNotFound
}

func (f *fakeVolumeClient) CopyVolumeFile(ctx context.Context, volumeName, relPath string) (io.ReadCloser, domain.FSEntry, error) {
	return nil, domain.FSEntry{}, domain.ErrNotFound
}

func TestVolumeService_CreateThenList(t *testing.T) {
	fake := &fakeVolumeClient{volumes: []domain.Volume{{Name: "data"}}}
	svc := service.NewVolumeService(fake)

	created, err := svc.Create(context.Background(), "data", "local", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "data" {
		t.Fatalf("expected volume named data, got %+v", created)
	}

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "data" {
		t.Fatalf("expected [data], got %+v", list)
	}
}

func TestVolumeService_Remove(t *testing.T) {
	fake := &fakeVolumeClient{}
	svc := service.NewVolumeService(fake)

	if err := svc.Remove(context.Background(), "data", true); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if fake.removedID != "data" {
		t.Fatalf("expected removedID 'data', got %q", fake.removedID)
	}
}

func TestVolumeService_Prune(t *testing.T) {
	fake := &fakeVolumeClient{pruneN: 2, pruneSize: 512}
	svc := service.NewVolumeService(fake)

	deleted, reclaimed, err := svc.Prune(context.Background())
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 2 || reclaimed != 512 {
		t.Fatalf("expected (2, 512), got (%d, %d)", deleted, reclaimed)
	}
}
