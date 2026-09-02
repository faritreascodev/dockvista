package docker

import dockertypes "github.com/docker/docker/api/types"

// cpuPercent implements the same delta-based formula the Docker CLI uses:
// (cpuDelta / systemDelta) * onlineCPUs * 100. Both deltas are computed
// between the current and previous samples the daemon returns in one
// stats payload, so a single one-shot read is enough (no need to keep a
// streaming connection open just to compute a percentage).
func cpuPercent(s dockertypes.StatsJSON) float64 {
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemUsage) - float64(s.PreCPUStats.SystemUsage)

	if systemDelta <= 0 || cpuDelta <= 0 {
		return 0
	}

	onlineCPUs := float64(s.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(s.CPUStats.CPUUsage.PercpuUsage))
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
