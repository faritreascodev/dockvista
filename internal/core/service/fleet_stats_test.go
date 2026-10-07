package service_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"dockvista/internal/adapters/store"
	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
)

// perIDStatsDocker answers stats per container and fails for one of them.
type perIDStatsDocker struct {
	*fakeDocker
	failID string
}

func (f *perIDStatsDocker) ContainerStats(ctx context.Context, id string) (domain.Stats, error) {
	f.statsCalls.Add(1)
	if id == f.failID {
		return domain.Stats{}, errors.New("container went away")
	}
	return domain.Stats{ContainerID: id, CPUPercent: 1}, nil
}

func TestFleetStats_SamplesOnlyRunningAndSkipsFailures(t *testing.T) {
	fake := &perIDStatsDocker{fakeDocker: newFakeDocker(), failID: "gone"}
	fake.containers = []domain.Container{
		{ID: "web", State: domain.StateRunning},
		{ID: "db", State: domain.StateRunning},
		{ID: "gone", State: domain.StateRunning},
		{ID: "old", State: domain.StateExited},
	}
	svc := service.New(fake, store.New(), nil)
	if err := svc.RefreshOnce(context.Background()); err != nil {
		t.Fatalf("RefreshOnce: %v", err)
	}

	got := svc.FleetStats(context.Background())
	ids := make([]string, 0, len(got))
	for _, s := range got {
		ids = append(ids, s.ContainerID)
	}
	sort.Strings(ids)
	if len(ids) != 2 || ids[0] != "db" || ids[1] != "web" {
		t.Fatalf("FleetStats ids = %v, want [db web]", ids)
	}
	if calls := fake.statsCalls.Load(); calls != 3 {
		t.Fatalf("daemon stats calls = %d, want 3 (exited container must not be sampled)", calls)
	}
}
