package service

import (
	"context"
	"fmt"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/ports"
)

type SwarmService struct {
	docker ports.SwarmClient
}

func NewSwarmService(docker ports.SwarmClient) *SwarmService {
	return &SwarmService{docker: docker}
}

func (s *SwarmService) Snapshot(ctx context.Context) (domain.SwarmSnapshot, error) {
	snap, err := s.docker.SwarmSnapshot(ctx)
	if err != nil {
		return domain.SwarmSnapshot{}, fmt.Errorf("service: swarm snapshot: %w", err)
	}
	if snap.Nodes == nil {
		snap.Nodes = []domain.SwarmNode{}
	}
	if snap.Services == nil {
		snap.Services = []domain.SwarmService{}
	}
	return snap, nil
}
