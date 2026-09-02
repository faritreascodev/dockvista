package service_test

import (
	"context"
	"testing"

	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
)

type fakeNetworkClient struct {
	networks       []domain.Network
	connectCalls   int
	disconnectArgs [2]string
	pruneN         int
}

func (f *fakeNetworkClient) ListNetworks(ctx context.Context) ([]domain.Network, error) {
	return f.networks, nil
}

func (f *fakeNetworkClient) CreateNetwork(ctx context.Context, name, driver string, labels map[string]string) (domain.Network, error) {
	return domain.Network{ID: "net1", Name: name, Driver: driver}, nil
}

func (f *fakeNetworkClient) RemoveNetwork(ctx context.Context, id string) error {
	return nil
}

func (f *fakeNetworkClient) ConnectNetwork(ctx context.Context, networkID, containerID string) error {
	f.connectCalls++
	return nil
}

func (f *fakeNetworkClient) DisconnectNetwork(ctx context.Context, networkID, containerID string, force bool) error {
	f.disconnectArgs = [2]string{networkID, containerID}
	return nil
}

func (f *fakeNetworkClient) PruneNetworks(ctx context.Context) (int, error) {
	return f.pruneN, nil
}

func TestNetworkService_Create(t *testing.T) {
	svc := service.NewNetworkService(&fakeNetworkClient{})

	n, err := svc.Create(context.Background(), "app-net", "bridge", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if n.Name != "app-net" || n.ID != "net1" {
		t.Fatalf("unexpected network: %+v", n)
	}
}

func TestNetworkService_ConnectAndDisconnect(t *testing.T) {
	fake := &fakeNetworkClient{}
	svc := service.NewNetworkService(fake)

	if err := svc.Connect(context.Background(), "net1", "c1"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if fake.connectCalls != 1 {
		t.Fatalf("expected 1 connect call, got %d", fake.connectCalls)
	}

	if err := svc.Disconnect(context.Background(), "net1", "c1", true); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if fake.disconnectArgs != [2]string{"net1", "c1"} {
		t.Fatalf("unexpected disconnect args: %v", fake.disconnectArgs)
	}
}

func TestNetworkService_Prune(t *testing.T) {
	svc := service.NewNetworkService(&fakeNetworkClient{pruneN: 4})

	deleted, err := svc.Prune(context.Background())
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 4 {
		t.Fatalf("expected 4, got %d", deleted)
	}
}
