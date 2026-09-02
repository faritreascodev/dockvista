package docker

import (
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"

	"dockvista/internal/core/domain"
)

// toDomainContainer maps the Docker SDK's summary type into our own,
// framework-free domain.Container so the rest of the app never has to
// import the SDK.
func toDomainContainer(c dockertypes.Container) domain.Container {
	name := "unnamed"
	if len(c.Names) > 0 {
		name = strings.TrimPrefix(c.Names[0], "/")
	}

	ports := make([]domain.Port, 0, len(c.Ports))
	for _, p := range c.Ports {
		ports = append(ports, domain.Port{
			PrivatePort: p.PrivatePort,
			PublicPort:  p.PublicPort,
			Type:        p.Type,
		})
	}

	return domain.Container{
		ID:      c.ID,
		Name:    name,
		Image:   c.Image,
		State:   domain.ContainerState(c.State),
		Status:  c.Status,
		Created: time.Unix(c.Created, 0).UTC(),
		Ports:   ports,
		Labels:  c.Labels,
	}
}
