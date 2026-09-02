package service

import (
	"context"
	"fmt"
	"io"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

// ImageService is the use-case layer for image inspection and lifecycle
// control. Unlike ContainerService it has no cache — images are listed
// directly from the daemon on every call, since they're not polled at a
// tight interval the way the container list is.
type ImageService struct {
	docker ports.ImageClient
}

func NewImageService(docker ports.ImageClient) *ImageService {
	return &ImageService{docker: docker}
}

func (s *ImageService) List(ctx context.Context) ([]domain.Image, error) {
	images, err := s.docker.ListImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list images: %w", err)
	}
	return images, nil
}

func (s *ImageService) Remove(ctx context.Context, id string, force bool) error {
	if err := s.docker.RemoveImage(ctx, id, force); err != nil {
		return fmt.Errorf("service: remove image: %w", err)
	}
	return nil
}

func (s *ImageService) Pull(ctx context.Context, ref string) (io.ReadCloser, error) {
	r, err := s.docker.PullImage(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("service: pull image: %w", err)
	}
	return r, nil
}

func (s *ImageService) Prune(ctx context.Context) (deleted int, spaceReclaimed uint64, err error) {
	deleted, spaceReclaimed, err = s.docker.PruneImages(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("service: prune images: %w", err)
	}
	return deleted, spaceReclaimed, nil
}
