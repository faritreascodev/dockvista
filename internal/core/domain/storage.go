package domain

import "time"

// StorageKind is the class of daemon object a StorageItem describes.
type StorageKind string

const (
	StorageContainer  StorageKind = "container"
	StorageImage      StorageKind = "image"
	StorageVolume     StorageKind = "volume"
	StorageBuildCache StorageKind = "buildcache"
)

// ProtectLabel opts a container, image, or volume out of every cleanup,
// whatever its state.
const ProtectLabel = "dockvista.protect"

// StorageItem is one object that occupies disk. The adapter fills the facts
// (size, users, state); the storage service decides Eligible and Reason.
type StorageItem struct {
	Kind StorageKind
	// ID is the container ID, image ID, volume name, or cache record ID.
	ID   string
	Name string
	// SizeBytes is what removing this item alone would free: the writable
	// layer for a container, the unshared layers for an image.
	SizeBytes  int64
	SizeKnown  bool
	CreatedAt  time.Time
	LastUsedAt time.Time

	// State is the container state; empty for other kinds.
	State   string
	Labels  map[string]string
	Project string
	// Tags are an image's repo:tag references; empty means dangling.
	Tags []string
	// Detail is a short kind-specific description (image of a container,
	// volume driver, cache record type).
	Detail string
	// UsedBy lists the container IDs that reference an image or mount a
	// volume, running or not.
	UsedBy []string
	// Shared marks a build cache record that other records depend on.
	Shared bool
	InUse  bool

	Eligible bool
	Reason   string
}

// StorageInventory is an itemized `docker system df`.
type StorageInventory struct {
	GeneratedAt time.Time
	Usage       DiskUsage
	Items       []StorageItem
}

// CleanupPlan names exactly what the caller wants removed. Nothing outside
// the plan is touched; build cache is all-or-nothing because the daemon
// prunes it as a unit.
type CleanupPlan struct {
	Containers []string
	Images     []string
	Volumes    []string
	BuildCache bool
}

// Size is the number of objects the plan names.
func (p CleanupPlan) Size() int {
	n := len(p.Containers) + len(p.Images) + len(p.Volumes)
	if p.BuildCache {
		n++
	}
	return n
}

type CleanupStatus string

const (
	CleanupRemoved CleanupStatus = "removed"
	CleanupSkipped CleanupStatus = "skipped"
	CleanupFailed  CleanupStatus = "failed"
	// CleanupPlanned is the status every accepted item gets in a preview.
	CleanupPlanned CleanupStatus = "planned"
)

type CleanupResult struct {
	Kind       StorageKind
	ID         string
	Name       string
	Status     CleanupStatus
	Message    string
	FreedBytes int64
}

type CleanupReport struct {
	DryRun     bool
	Results    []CleanupResult
	FreedBytes int64
}
