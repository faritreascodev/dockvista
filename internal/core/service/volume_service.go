package service

import (
	"context"
	"fmt"
	"io"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

// VolumeService is the use-case layer for volume inspection and lifecycle
// control. Like ImageService it has no cache — volumes are listed directly
// from the daemon on every call.
type VolumeService struct {
	docker ports.VolumeClient
}

func NewVolumeService(docker ports.VolumeClient) *VolumeService {
	return &VolumeService{docker: docker}
}

func (s *VolumeService) List(ctx context.Context) ([]domain.Volume, error) {
	volumes, err := s.docker.ListVolumes(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list volumes: %w", err)
	}
	return volumes, nil
}

func (s *VolumeService) Create(ctx context.Context, name, driver string, labels map[string]string) (domain.Volume, error) {
	v, err := s.docker.CreateVolume(ctx, name, driver, labels)
	if err != nil {
		return domain.Volume{}, fmt.Errorf("service: create volume: %w", err)
	}
	return v, nil
}

func (s *VolumeService) Remove(ctx context.Context, name string, force bool) error {
	if err := s.docker.RemoveVolume(ctx, name, force); err != nil {
		return fmt.Errorf("service: remove volume: %w", err)
	}
	return nil
}

func (s *VolumeService) Prune(ctx context.Context) (deleted int, spaceReclaimed uint64, err error) {
	deleted, spaceReclaimed, err = s.docker.PruneVolumes(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("service: prune volumes: %w", err)
	}
	return deleted, spaceReclaimed, nil
}

func (s *VolumeService) ListFiles(ctx context.Context, name, relPath string) (domain.DirListing, error) {
	listing, err := s.docker.ListVolumeDir(ctx, name, relPath)
	if err != nil {
		return domain.DirListing{}, fmt.Errorf("service: volume files: %w", err)
	}
	return listing, nil
}

func (s *VolumeService) FileStat(ctx context.Context, name, relPath string) (domain.FSEntry, error) {
	entry, err := s.docker.StatVolumePath(ctx, name, relPath)
	if err != nil {
		return domain.FSEntry{}, fmt.Errorf("service: volume stat: %w", err)
	}
	return entry, nil
}

func (s *VolumeService) FileContent(ctx context.Context, name, relPath string) (io.ReadCloser, domain.FSEntry, error) {
	rc, entry, err := s.docker.CopyVolumeFile(ctx, name, relPath)
	if err != nil {
		return nil, domain.FSEntry{}, fmt.Errorf("service: volume content: %w", err)
	}
	return rc, entry, nil
}
