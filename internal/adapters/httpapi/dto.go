package httpapi

import "dockvista/internal/core/domain"

// containerDTO is the wire representation of domain.Container. Keeping a
// dedicated DTO (rather than json-tagging the domain type directly) means
// the API's public contract can evolve independently of internal modeling.
type containerDTO struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Created int64             `json:"createdAt"` // unix seconds
	Ports   []portDTO         `json:"ports"`
	Labels  map[string]string `json:"labels,omitempty"`
}

type portDTO struct {
	PrivatePort uint16 `json:"privatePort"`
	PublicPort  uint16 `json:"publicPort,omitempty"`
	Type        string `json:"type"`
}

func newContainerDTO(c domain.Container) containerDTO {
	ports := make([]portDTO, 0, len(c.Ports))
	for _, p := range c.Ports {
		ports = append(ports, portDTO{
			PrivatePort: p.PrivatePort,
			PublicPort:  p.PublicPort,
			Type:        p.Type,
		})
	}
	return containerDTO{
		ID:      c.ID,
		Name:    c.Name,
		Image:   c.Image,
		State:   string(c.State),
		Status:  c.Status,
		Created: c.Created.Unix(),
		Ports:   ports,
		Labels:  c.Labels,
	}
}

func newContainerListDTO(containers []domain.Container) []containerDTO {
	out := make([]containerDTO, 0, len(containers))
	for _, c := range containers {
		out = append(out, newContainerDTO(c))
	}
	return out
}

type statsDTO struct {
	ContainerID   string  `json:"containerId"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryUsage   uint64  `json:"memoryUsageBytes"`
	MemoryLimit   uint64  `json:"memoryLimitBytes"`
	MemoryPercent float64 `json:"memoryPercent"`
}

func newStatsDTO(s domain.Stats) statsDTO {
	return statsDTO{
		ContainerID:   s.ContainerID,
		CPUPercent:    s.CPUPercent,
		MemoryUsage:   s.MemoryUsage,
		MemoryLimit:   s.MemoryLimit,
		MemoryPercent: s.MemoryPercent,
	}
}

type engineDTO struct {
	Version    string `json:"version"`
	Reachable  bool   `json:"reachable"`
	Containers int    `json:"containers"`
}

func newEngineDTO(e domain.EngineInfo) engineDTO {
	return engineDTO{Version: e.Version, Reachable: e.Reachable, Containers: e.Containers}
}

type actionResultDTO struct {
	Status string `json:"status"`
	ID     string `json:"id"`
}

type createContainerResult struct {
	Status  string `json:"status"`
	ID      string `json:"id"`
	Started bool   `json:"started"`
	Error   string `json:"error,omitempty"`
}

type imageDTO struct {
	ID          string            `json:"id"`
	RepoTags    []string          `json:"repoTags"`
	RepoDigests []string          `json:"repoDigests"`
	Size        int64             `json:"sizeBytes"`
	Containers  int64             `json:"containers"`
	Created     int64             `json:"createdAt"`
	Labels      map[string]string `json:"labels,omitempty"`
}

func newImageDTO(img domain.Image) imageDTO {
	return imageDTO{
		ID:          img.ID,
		RepoTags:    img.RepoTags,
		RepoDigests: img.RepoDigests,
		Size:        img.Size,
		Containers:  img.Containers,
		Created:     img.Created.Unix(),
		Labels:      img.Labels,
	}
}

func newImageListDTO(images []domain.Image) []imageDTO {
	out := make([]imageDTO, 0, len(images))
	for _, img := range images {
		out = append(out, newImageDTO(img))
	}
	return out
}

type pruneResultDTO struct {
	Deleted        int    `json:"deleted"`
	SpaceReclaimed uint64 `json:"spaceReclaimedBytes"`
}

type volumeDTO struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	CreatedAt  int64             `json:"createdAt"`
	Labels     map[string]string `json:"labels,omitempty"`
}

func newVolumeDTO(v domain.Volume) volumeDTO {
	return volumeDTO{
		Name:       v.Name,
		Driver:     v.Driver,
		Mountpoint: v.Mountpoint,
		CreatedAt:  v.CreatedAt.Unix(),
		Labels:     v.Labels,
	}
}

func newVolumeListDTO(volumes []domain.Volume) []volumeDTO {
	out := make([]volumeDTO, 0, len(volumes))
	for _, v := range volumes {
		out = append(out, newVolumeDTO(v))
	}
	return out
}

type networkDTO struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Scope      string            `json:"scope"`
	Internal   bool              `json:"internal"`
	Attachable bool              `json:"attachable"`
	CreatedAt  int64             `json:"createdAt"`
	Labels     map[string]string `json:"labels,omitempty"`
	Containers map[string]string `json:"containers,omitempty"`
}

func newNetworkDTO(n domain.Network) networkDTO {
	return networkDTO{
		ID:         n.ID,
		Name:       n.Name,
		Driver:     n.Driver,
		Scope:      n.Scope,
		Internal:   n.Internal,
		Attachable: n.Attachable,
		CreatedAt:  n.Created.Unix(),
		Labels:     n.Labels,
		Containers: n.Containers,
	}
}

func newNetworkListDTO(networks []domain.Network) []networkDTO {
	out := make([]networkDTO, 0, len(networks))
	for _, n := range networks {
		out = append(out, newNetworkDTO(n))
	}
	return out
}
