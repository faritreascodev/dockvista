package docker

import (
	"testing"

	"github.com/docker/docker/api/types/swarm"
)

func TestMapSwarmServices_ReplicatedImageAndPorts(t *testing.T) {
	replicas := uint64(3)
	in := []swarm.Service{{
		ID: "abc",
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{Name: "web"},
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: &swarm.ContainerSpec{Image: "nginx:alpine"},
			},
			Mode: swarm.ServiceMode{Replicated: &swarm.ReplicatedService{Replicas: &replicas}},
		},
		Endpoint: swarm.Endpoint{Ports: []swarm.PortConfig{{PublishedPort: 8080, TargetPort: 80, Protocol: swarm.PortConfigProtocolTCP}}},
		ServiceStatus: &swarm.ServiceStatus{RunningTasks: 2, DesiredTasks: 3},
	}}
	got := mapSwarmServices(in)
	if len(got) != 1 || got[0].Name != "web" || got[0].Image != "nginx:alpine" || got[0].Replicas != 3 || got[0].Running != 2 {
		t.Fatalf("%+v", got)
	}
	if len(got[0].Ports) != 1 || got[0].Ports[0] != "8080:80/tcp" {
		t.Fatalf("ports %+v", got[0].Ports)
	}
}
