package docker

import (
	"sync"
	"time"

	dockertypes "github.com/docker/docker/api/types"
)

// cpuSampleTTL is how long a previous CPU reading stays usable as the
// baseline for the next one. Past that, the delta would average over a
// window nobody asked about.
const cpuSampleTTL = time.Minute

type cpuSample struct {
	stats dockertypes.CPUStats
	at    time.Time
}

// cpuTracker remembers the last CPU counters seen per container. One-shot
// stats (the only kind that answers without a ~1s wait) leave precpu_stats
// empty, so without a baseline of our own the formula below would report
// the container's average since the host booted instead of its current
// load.
type cpuTracker struct {
	mu   sync.Mutex
	prev map[string]cpuSample
}

func newCPUTracker() *cpuTracker {
	return &cpuTracker{prev: make(map[string]cpuSample)}
}

// percent returns CPU% for this sample against the best baseline available:
// the daemon's own precpu_stats when present, otherwise the previous sample
// for the same container. The first sample with neither reports 0.
func (t *cpuTracker) percent(id string, s dockertypes.StatsJSON, now time.Time) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	for key, sample := range t.prev {
		if now.Sub(sample.at) > cpuSampleTTL {
			delete(t.prev, key)
		}
	}

	pre := s.PreCPUStats
	if pre.SystemUsage == 0 {
		last, ok := t.prev[id]
		if !ok {
			t.prev[id] = cpuSample{stats: s.CPUStats, at: now}
			return 0
		}
		pre = last.stats
	}
	t.prev[id] = cpuSample{stats: s.CPUStats, at: now}
	return cpuPercent(s.CPUStats, pre)
}

// cpuPercent implements the same delta-based formula the Docker CLI uses:
// (cpuDelta / systemDelta) * onlineCPUs * 100.
func cpuPercent(cur, pre dockertypes.CPUStats) float64 {
	cpuDelta := float64(cur.CPUUsage.TotalUsage) - float64(pre.CPUUsage.TotalUsage)
	systemDelta := float64(cur.SystemUsage) - float64(pre.SystemUsage)

	if systemDelta <= 0 || cpuDelta <= 0 {
		return 0
	}

	onlineCPUs := float64(cur.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(cur.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}

	return (cpuDelta / systemDelta) * onlineCPUs * 100.0
}

// memoryUsage returns usage with the page cache subtracted, matching what
// `docker stats` displays (raw cgroup "usage" otherwise double-counts
// reclaimable page cache as if it were resident container memory).
func memoryUsage(s dockertypes.StatsJSON) uint64 {
	usage := s.MemoryStats.Usage
	if cache, ok := s.MemoryStats.Stats["cache"]; ok && cache < usage {
		usage -= cache
	} else if inactiveFile, ok := s.MemoryStats.Stats["inactive_file"]; ok && inactiveFile < usage {
		usage -= inactiveFile
	}
	return usage
}

func memoryPercent(usage, limit uint64) float64 {
	if limit == 0 {
		return 0
	}
	return float64(usage) / float64(limit) * 100.0
}

func networkBytes(s dockertypes.StatsJSON) (rx, tx uint64) {
	for _, n := range s.Networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}
	return
}

func blkioBytes(s dockertypes.StatsJSON) (read, write uint64) {
	for _, e := range s.BlkioStats.IoServiceBytesRecursive {
		switch e.Op {
		case "Read", "read":
			read += e.Value
		case "Write", "write":
			write += e.Value
		}
	}
	return
}
