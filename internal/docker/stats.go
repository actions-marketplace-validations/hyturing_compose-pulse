package docker

import (
	"context"
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types/container"
)

// StatsInfo is the distilled one-shot container stats sample.
type StatsInfo struct {
	CPUPercent float64
	MemUsage   uint64
	MemLimit   uint64
	NetRx      uint64
	NetTx      uint64
	PIDs       uint64
}

// Stats reads a one-shot sample and computes CPU% via the docker-CLI delta formula.
func (c *Client) Stats(ctx context.Context, containerID string) (*StatsInfo, error) {
	reader, err := c.api.ContainerStatsOneShot(ctx, containerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Body.Close() }()

	var raw container.StatsResponse
	if err := json.NewDecoder(reader.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return distillStats(raw), nil
}

func distillStats(s container.StatsResponse) *StatsInfo {
	info := &StatsInfo{
		CPUPercent: cpuPercent(s),
		MemUsage:   memUsage(s),
		MemLimit:   s.MemoryStats.Limit,
		PIDs:       s.PidsStats.Current,
	}
	for _, n := range s.Networks {
		info.NetRx += n.RxBytes
		info.NetTx += n.TxBytes
	}
	return info
}

// cpuPercent matches the docker CLI: (cpuDelta/systemDelta)*onlineCPUs*100.
func cpuPercent(s container.StatsResponse) float64 {
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemUsage) - float64(s.PreCPUStats.SystemUsage)
	if cpuDelta <= 0 || systemDelta <= 0 {
		return 0
	}
	online := float64(s.CPUStats.OnlineCPUs)
	if online == 0 {
		online = float64(len(s.CPUStats.CPUUsage.PercpuUsage))
	}
	if online == 0 {
		online = 1
	}
	return (cpuDelta / systemDelta) * online * 100
}

func memUsage(s container.StatsResponse) uint64 {
	usage := s.MemoryStats.Usage
	if s.MemoryStats.Stats == nil {
		return usage
	}
	// Match docker CLI: subtract inactive_file (cgroup v2) or cache (v1).
	if v, ok := s.MemoryStats.Stats["inactive_file"]; ok && v < usage {
		return usage - v
	}
	if v, ok := s.MemoryStats.Stats["cache"]; ok && v < usage {
		return usage - v
	}
	return usage
}

// DrainStatsBody is a test helper closing unused readers.
func DrainStatsBody(r io.ReadCloser) {
	if r != nil {
		_ = r.Close()
	}
}
