package service_test

import (
	"context"
	"testing"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
)

type fakeSwarm struct {
	snap domain.SwarmSnapshot
}

func (f fakeSwarm) SwarmSnapshot(ctx context.Context) (domain.SwarmSnapshot, error) {
	return f.snap, nil
}

func TestSwarmService_InactiveHasEmptySlices(t *testing.T) {
	svc := service.NewSwarmService(fakeSwarm{snap: domain.SwarmSnapshot{State: "inactive"}})
	got, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "inactive" || got.Nodes == nil || got.Services == nil {
		t.Fatalf("%+v", got)
	}
}
