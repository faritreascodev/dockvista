package domain

// SystemInfo describes the host and daemon DockVista is attached to.
type SystemInfo struct {
	Name              string
	ServerVersion     string
	OperatingSystem   string
	OSType            string
	Architecture      string
	KernelVersion     string
	StorageDriver     string
	CPUs              int
	MemoryBytes       int64
	Containers        int
	ContainersRunning int
	ContainersPaused  int
	ContainersStopped int
	Images            int
}

// DiskUsageCategory is one row of `docker system df`. Reclaimable is what a
// prune of that category would free: space not held by anything in use.
type DiskUsageCategory struct {
	Count            int
	Active           int
	SizeBytes        int64
	ReclaimableBytes int64
}

// DiskUsage is the daemon's storage broken down the way `docker system df`
// reports it.
type DiskUsage struct {
	Images     DiskUsageCategory
	Containers DiskUsageCategory
	Volumes    DiskUsageCategory
	BuildCache DiskUsageCategory
}
