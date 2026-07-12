package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestCPUPercent(t *testing.T) {
	tests := []struct {
		name string
		s    container.StatsResponse
		want float64
	}{
		{
			name: "divide by zero",
			s:    container.StatsResponse{},
			want: 0,
		},
		{
			name: "basic delta",
			s: container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 200},
					SystemUsage: 1000,
					OnlineCPUs:  2,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage:    container.CPUUsage{TotalUsage: 100},
					SystemUsage: 500,
				},
			},
			// (100/500)*2*100 = 40
			want: 40,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cpuPercent(tt.s)
			if got != tt.want {
				t.Fatalf("cpuPercent = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemUsageSubtractsCache(t *testing.T) {
	s := container.StatsResponse{
		MemoryStats: container.MemoryStats{
			Usage: 1000,
			Stats: map[string]uint64{"inactive_file": 200},
		},
	}
	if got := memUsage(s); got != 800 {
		t.Fatalf("memUsage = %d, want 800", got)
	}
}
