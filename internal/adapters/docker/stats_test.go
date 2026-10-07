package docker

import (
	"math"
	"testing"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
)

func oneShot(total, system uint64) dockertypes.StatsJSON {
	var s dockertypes.StatsJSON
	s.CPUStats = dockertypes.CPUStats{
		CPUUsage:    dockertypes.CPUUsage{TotalUsage: total},
		SystemUsage: system,
		OnlineCPUs:  2,
	}
	return s
}

func TestCPUTracker_UsesPreviousSampleForOneShotStats(t *testing.T) {
	tr := newCPUTracker()
	now := time.Unix(1_000, 0)

	// A container that has burned lots of CPU since boot but is idle now
	// must not report its lifetime average on the first sample.
	if got := tr.percent("web", oneShot(5_000, 10_000), now); got != 0 {
		t.Fatalf("first sample = %v, want 0", got)
	}

	// 250 of 1000 system ticks on 2 CPUs: 25% * 2 = 50%.
	got := tr.percent("web", oneShot(5_250, 11_000), now.Add(3*time.Second))
	if math.Abs(got-50) > 1e-9 {
		t.Fatalf("second sample = %v, want 50", got)
	}
}

func TestCPUTracker_PrefersDaemonPreCPU(t *testing.T) {
	tr := newCPUTracker()
	s := oneShot(1_100, 2_000)
	s.PreCPUStats = dockertypes.CPUStats{CPUUsage: dockertypes.CPUUsage{TotalUsage: 1_000}, SystemUsage: 1_000}

	// 100 of 1000 on 2 CPUs = 20%, even with no stored baseline.
	if got := tr.percent("db", s, time.Unix(0, 0)); math.Abs(got-20) > 1e-9 {
		t.Fatalf("percent = %v, want 20", got)
	}
}

func TestNetworkAndBlkioBytes(t *testing.T) {
	var s dockertypes.StatsJSON
	s.Networks = map[string]dockertypes.NetworkStats{
		"eth0": {RxBytes: 10, TxBytes: 20},
		"eth1": {RxBytes: 5, TxBytes: 7},
	}
	s.BlkioStats.IoServiceBytesRecursive = []dockertypes.BlkioStatEntry{
		{Op: "Read", Value: 3},
		{Op: "Write", Value: 8},
		{Op: "read", Value: 1},
	}
	rx, tx := networkBytes(s)
	if rx != 15 || tx != 27 {
		t.Fatalf("net %d/%d, want 15/27", rx, tx)
	}
	r, w := blkioBytes(s)
	if r != 4 || w != 8 {
		t.Fatalf("blkio %d/%d, want 4/8", r, w)
	}
}

func TestCPUTracker_StaleBaselineIsDropped(t *testing.T) {
	tr := newCPUTracker()
	start := time.Unix(1_000, 0)
	tr.percent("web", oneShot(1_000, 1_000), start)

	got := tr.percent("web", oneShot(9_000, 9_000), start.Add(cpuSampleTTL+time.Second))
	if got != 0 {
		t.Fatalf("sample after TTL = %v, want 0 (fresh baseline)", got)
	}
}

func TestMapDiskUsage(t *testing.T) {
	du := dockertypes.DiskUsage{
		LayersSize: 1_000,
		Images: []*image.Summary{
			{Size: 600, SharedSize: 100, Containers: 1},
			{Size: 300, SharedSize: 100, Containers: 0},
			nil,
		},
		Containers: []*dockertypes.Container{
			{SizeRw: 10, State: "running"},
			{SizeRw: 40, State: "exited"},
		},
		Volumes: []*volume.Volume{
			{UsageData: &volume.UsageData{Size: 70, RefCount: 1}},
			{UsageData: &volume.UsageData{Size: 30, RefCount: 0}},
			{UsageData: &volume.UsageData{Size: -1, RefCount: 0}},
		},
		BuildCache: []*dockertypes.BuildCache{
			{Size: 5, InUse: true},
			{Size: 7, Shared: true},
			{Size: 11},
		},
	}

	got := mapDiskUsage(du)

	if got.Images.Count != 2 || got.Images.Active != 1 || got.Images.SizeBytes != 1_000 || got.Images.ReclaimableBytes != 200 {
		t.Errorf("images = %+v", got.Images)
	}
	if got.Containers.Count != 2 || got.Containers.Active != 1 || got.Containers.SizeBytes != 50 || got.Containers.ReclaimableBytes != 40 {
		t.Errorf("containers = %+v", got.Containers)
	}
	if got.Volumes.Count != 3 || got.Volumes.Active != 1 || got.Volumes.SizeBytes != 100 || got.Volumes.ReclaimableBytes != 30 {
		t.Errorf("volumes = %+v", got.Volumes)
	}
	if got.BuildCache.Count != 3 || got.BuildCache.Active != 1 || got.BuildCache.SizeBytes != 23 || got.BuildCache.ReclaimableBytes != 11 {
		t.Errorf("build cache = %+v", got.BuildCache)
	}
}
