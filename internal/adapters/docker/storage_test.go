package docker

import (
	"testing"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"

	"dockvista/internal/core/domain"
)

func TestMapInventory_AttributesUsers(t *testing.T) {
	du := dockertypes.DiskUsage{
		Containers: []*dockertypes.Container{{
			ID: "c1", Names: []string{"/web"}, ImageID: "sha256:aaa", State: "exited", SizeRw: 42,
			Mounts: []dockertypes.MountPoint{
				{Type: mount.TypeVolume, Name: "data"},
				{Type: mount.TypeBind, Source: "/srv"},
			},
		}},
		Images: []*image.Summary{
			{ID: "sha256:aaa", RepoTags: []string{"web:1"}, Size: 100, SharedSize: 30, Containers: 1},
			{ID: "sha256:bbbbbbbbbbbbbbbb", RepoTags: []string{"<none>:<none>"}, Size: 50, SharedSize: -1},
		},
		Volumes: []*volume.Volume{
			{Name: "data", Driver: "local", UsageData: &volume.UsageData{Size: 7, RefCount: 1}},
			{Name: "unsized", UsageData: &volume.UsageData{Size: -1}},
		},
	}

	items := map[string]domain.StorageItem{}
	for _, it := range mapInventory(du).Items {
		items[it.ID] = it
	}

	if c := items["c1"]; c.Name != "web" || c.InUse || c.SizeBytes != 42 {
		t.Fatalf("container = %+v", c)
	}
	if img := items["sha256:aaa"]; !img.InUse || img.SizeBytes != 70 || len(img.UsedBy) != 1 {
		t.Fatalf("image = %+v; want in use by c1 with 70 unique bytes", img)
	}
	if d := items["sha256:bbbbbbbbbbbbbbbb"]; len(d.Tags) != 0 || d.Name != "bbbbbbbbbbbb" || d.InUse {
		t.Fatalf("dangling image = %+v", d)
	}
	if v := items["data"]; !v.InUse || len(v.UsedBy) != 1 || v.UsedBy[0] != "c1" {
		t.Fatalf("volume = %+v", v)
	}
	if v := items["unsized"]; v.SizeKnown || v.InUse {
		t.Fatalf("unsized volume = %+v", v)
	}
}
