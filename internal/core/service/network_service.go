package service

import (
	"context"
	"fmt"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

// NetworkService is the use-case layer for network inspection and
// lifecycle control. Like ImageService/VolumeService it has no cache.
type NetworkService struct {
	docker ports.NetworkClient
}

func NewNetworkService(docker ports.NetworkClient) *NetworkService {
	return &NetworkService{docker: docker}
}

func (s *NetworkService) List(ctx context.Context) ([]domain.Network, error) {
	networks, err := s.docker.ListNetworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list networks: %w", err)
	}
	return networks, nil
}

func (s *NetworkService) Create(ctx context.Context, name, driver string, labels map[string]string) (domain.Network, error) {
	n, err := s.docker.CreateNetwork(ctx, name, driver, labels)
	if err != nil {
		return domain.Network{}, fmt.Errorf("service: create network: %w", err)
	}
	return n, nil
}

func (s *NetworkService) Remove(ctx context.Context, id string) error {
	if err := s.docker.RemoveNetwork(ctx, id); err != nil {
		return fmt.Errorf("service: remove network: %w", err)
	}
	return nil
}

func (s *NetworkService) Connect(ctx context.Context, networkID, containerID string) error {
	if err := s.docker.ConnectNetwork(ctx, networkID, containerID); err != nil {
		return fmt.Errorf("service: connect network: %w", err)
	}
	return nil
}

func (s *NetworkService) Disconnect(ctx context.Context, networkID, containerID string, force bool) error {
	if err := s.docker.DisconnectNetwork(ctx, networkID, containerID, force); err != nil {
		return fmt.Errorf("service: disconnect network: %w", err)
	}
	return nil
}

func (s *NetworkService) Prune(ctx context.Context) (deleted int, err error) {
	deleted, err = s.docker.PruneNetworks(ctx)
	if err != nil {
		return 0, fmt.Errorf("service: prune networks: %w", err)
	}
	return deleted, nil
}
