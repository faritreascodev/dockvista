package domain

import (
	"path"
	"strings"
	"time"
)

const (
	// MaxFileDownloadBytes is the largest single file the archive API will
	// stream to a browser. Above this the UI should tell the user to exec
	// or copy from a shell instead of pulling it through DockVista.
	MaxFileDownloadBytes = 32 << 20

	// MaxDirEntries caps one directory listing so a container that dumped
	// tens of thousands of files in /tmp cannot blow up a response.
	MaxDirEntries = 500

	// MaxFSChanges caps docker diff. A busy writable layer still shows
	// counts for the remainder.
	MaxFSChanges = 2500

	// MaxContainerPath is a container-side path, not a host path.
	MaxContainerPath = 4096

	ListReasonNotRunning = "not_running"
	ListReasonNoShell    = "no_shell"
)

// ContainerMount is one mount point from inspect.
type ContainerMount struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Driver      string `json:"driver,omitempty"`
	Mode        string `json:"mode,omitempty"`
	RW          bool   `json:"rw"`
	Propagation string `json:"propagation,omitempty"`
}

// ContainerFS is the inspect-derived summary for the Files and Mounts tabs.
type ContainerFS struct {
	Running        bool             `json:"running"`
	Privileged     bool             `json:"privileged"`
	ReadonlyRootfs bool             `json:"readonlyRootfs"`
	SizeRw         int64            `json:"sizeRw"`
	SizeRootFs     int64            `json:"sizeRootFs"`
	SizeKnown      bool             `json:"sizeKnown"`
	Mounts         []ContainerMount `json:"mounts"`
}

// FSEntry is one name in a container path (file, directory, or mount).
type FSEntry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Dir        bool      `json:"dir"`
	SizeBytes  int64     `json:"sizeBytes"`
	Mode       string    `json:"mode,omitempty"`
	ModTime    time.Time `json:"modTime,omitempty"`
	LinkTarget string    `json:"linkTarget,omitempty"`
	Mount      bool      `json:"mount,omitempty"`
	MountType  string    `json:"mountType,omitempty"`
}

// DirListing is one directory (or a file, when Path is not a directory).
// Reason is set when we cannot list children but the request itself is fine
// (stopped container, distroless image with no ls).
type DirListing struct {
	Path      string    `json:"path"`
	Running   bool      `json:"running"`
	Dir       bool      `json:"dir"`
	Reason    string    `json:"reason,omitempty"`
	Truncated bool      `json:"truncated,omitempty"`
	Entry     FSEntry   `json:"entry"`
	Entries   []FSEntry `json:"entries"`
}

// FSChange is one docker-diff entry. Kind is A, C, or D.
type FSChange struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// FSChangeList is docker diff plus totals, even when the slice is capped.
type FSChangeList struct {
	Changes   []FSChange `json:"changes"`
	Truncated bool       `json:"truncated,omitempty"`
	Added     int        `json:"added"`
	Modified  int        `json:"modified"`
	Deleted   int        `json:"deleted"`
}

// CleanContainerPath accepts a Unix path inside a container. It uses path
// (not filepath) so a DockVista process on Windows still talks in container
// paths. ".." is resolved, not rejected, because /app/../etc is /etc inside
// the container and that is a legitimate browse.
func CleanContainerPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		p = "/"
	}
	if strings.ContainsRune(p, 0) {
		return "", ErrInvalidInput
	}
	if !strings.HasPrefix(p, "/") {
		return "", ErrInvalidInput
	}
	cleaned := path.Clean(p)
	if !strings.HasPrefix(cleaned, "/") {
		return "", ErrInvalidInput
	}
	if len(cleaned) > MaxContainerPath {
		return "", ErrInvalidInput
	}
	return cleaned, nil
}
