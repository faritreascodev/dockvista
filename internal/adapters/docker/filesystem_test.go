package docker

import (
	"testing"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"

	"dockvista/internal/core/domain"
)

func TestParseLS(t *testing.T) {
	entries, truncated := parseLS("/app", "bin/\nREADME\n..hidden\n\n")
	if truncated {
		t.Fatal("truncated")
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(entries), entries)
	}
	if !entries[0].Dir || entries[0].Path != "/app/bin" {
		t.Fatalf("first = %+v", entries[0])
	}
	if entries[1].Dir || entries[1].Path != "/app/README" {
		t.Fatalf("second = %+v", entries[1])
	}
}

func TestParseLS_RootAndCap(t *testing.T) {
	var raw string
	for i := 0; i < domain.MaxDirEntries+10; i++ {
		raw += "f\n"
	}
	entries, truncated := parseLS("/", raw)
	if !truncated {
		t.Fatal("want truncated")
	}
	if len(entries) != domain.MaxDirEntries {
		t.Fatalf("len = %d", len(entries))
	}
	if entries[0].Path != "/f" {
		t.Fatalf("root child path = %q", entries[0].Path)
	}
}

func TestMapMountsAndChanges(t *testing.T) {
	mounts := mapMounts([]dockertypes.MountPoint{
		{Type: mount.TypeBind, Source: "/run/desktop/mnt/host/c/data", Destination: "/data", RW: true},
		{Type: mount.TypeVolume, Name: "db", Source: "/var/lib/docker/volumes/db/_data", Destination: "/var/lib/mysql", Driver: "local", RW: false},
	})
	if len(mounts) != 2 || mounts[0].Type != "bind" || mounts[1].Name != "db" || mounts[1].RW {
		t.Fatalf("mounts = %+v", mounts)
	}

	list := mapChanges([]container.FilesystemChange{
		{Path: "/tmp/a", Kind: container.ChangeAdd},
		{Path: "/tmp/b", Kind: container.ChangeModify},
		{Path: "/tmp/c", Kind: container.ChangeDelete},
	})
	if list.Added != 1 || list.Modified != 1 || list.Deleted != 1 || list.Truncated {
		t.Fatalf("counts = %+v", list)
	}
	if list.Changes[0].Kind != "A" || list.Changes[1].Kind != "C" || list.Changes[2].Kind != "D" {
		t.Fatalf("kinds = %+v", list.Changes)
	}
}

func TestMapFilesystemFlags(t *testing.T) {
	rw := int64(12)
	root := int64(400)
	info := dockertypes.ContainerJSON{
		ContainerJSONBase: &dockertypes.ContainerJSONBase{
			State: &dockertypes.ContainerState{Running: true},
			HostConfig: &container.HostConfig{
				Privileged:     true,
				ReadonlyRootfs: true,
			},
			SizeRw:     &rw,
			SizeRootFs: &root,
		},
	}
	fs := mapFilesystem(info, true)
	if !fs.Running || !fs.Privileged || !fs.ReadonlyRootfs || fs.SizeRw != 12 || !fs.SizeKnown {
		t.Fatalf("fs = %+v", fs)
	}
}
