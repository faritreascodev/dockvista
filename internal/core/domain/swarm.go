package domain

// SwarmSnapshot is the cluster view of the active engine. When the daemon
// is not in swarm mode, State is "inactive" and the slices are empty.
type SwarmSnapshot struct {
	State             string
	ControlAvailable  bool
	NodeID            string
	NodeAddr          string
	ClusterID         string
	Error             string
	Nodes             []SwarmNode
	Services          []SwarmService
}

type SwarmNode struct {
	ID            string
	Hostname      string
	Role          string
	Availability  string
	Status        string
	Addr          string
	EngineVersion string
	Manager       bool
	Leader        bool
}

type SwarmService struct {
	ID       string
	Name     string
	Image    string
	Mode     string
	Replicas int
	Running  int
	Ports    []string
}
