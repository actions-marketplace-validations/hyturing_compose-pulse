package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hyturing/compose-pulse/internal/docker"
)

// ringBufferSize is ~2 minutes of samples at the 2s stats tick (TUI-DESIGN.md §6).
const ringBufferSize = 60

// statsTickInterval is the separate, slower stats ticker. The 500ms state
// poll (RunUpdateMsg) is an intentional constraint and is never touched.
const statsTickInterval = 2 * time.Second

// ringBuffer holds the last ringBufferSize stats samples for one container.
type ringBuffer struct {
	samples []*docker.StatsInfo // fixed capacity, oldest overwritten
	next    int
	count   int
}

func newRingBuffer() *ringBuffer {
	return &ringBuffer{samples: make([]*docker.StatsInfo, ringBufferSize)}
}

func (r *ringBuffer) push(s *docker.StatsInfo) {
	r.samples[r.next] = s
	r.next = (r.next + 1) % len(r.samples)
	if r.count < len(r.samples) {
		r.count++
	}
}

// latest returns the most recently pushed sample, or nil when empty.
func (r *ringBuffer) latest() *docker.StatsInfo {
	if r == nil || r.count == 0 {
		return nil
	}
	idx := (r.next - 1 + len(r.samples)) % len(r.samples)
	return r.samples[idx]
}

// ordered returns samples oldest-to-newest.
func (r *ringBuffer) ordered() []*docker.StatsInfo {
	if r == nil || r.count == 0 {
		return nil
	}
	out := make([]*docker.StatsInfo, 0, r.count)
	start := (r.next - r.count + len(r.samples)) % len(r.samples)
	for i := 0; i < r.count; i++ {
		out = append(out, r.samples[(start+i)%len(r.samples)])
	}
	return out
}

func statsTickCmd() tea.Cmd {
	return tea.Tick(statsTickInterval, func(time.Time) tea.Msg {
		return statsTickMsg{}
	})
}

// statsSweepCmd samples every running container's stats in one batch. Errors
// are swallowed per-container: stats are decoration and must never crash or
// block the UI (TUI-DESIGN.md §6).
func statsSweepCmd(dc *docker.Client, containerIDs []string) tea.Cmd {
	return func() tea.Msg {
		samples := make(map[string]*docker.StatsInfo, len(containerIDs))
		for _, id := range containerIDs {
			if id == "" {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			info, err := dc.Stats(ctx, id)
			cancel()
			if err == nil && info != nil {
				samples[id] = info
			}
		}
		return statsMsg{samples: samples}
	}
}

// runningContainerIDs returns the container IDs of every running compose
// service in the snapshot — the stats sweep never touches waiting/exited
// containers.
func runningContainerIDs(rows []Row) []string {
	var ids []string
	for _, r := range rows {
		switch r.Kind {
		case RowComposeNode:
			if r.ContainerID == "" {
				continue
			}
			if isRunningDisplay(displayAndWaiting(r)) {
				ids = append(ids, r.ContainerID)
			}
		case RowStandalone:
			if r.ContainerID != "" && r.Standalone.State != docker.StateExited {
				ids = append(ids, r.ContainerID)
			}
		}
	}
	return ids
}

// sparkline renders samples as a bar chart of the ▁▂▃▄▅▆▇█ glyphs, scaled to
// max (CPU: window max; MEM: the container's memory limit).
func sparkline(values []float64, max float64) string {
	if len(values) == 0 {
		return ""
	}
	if max <= 0 {
		for _, v := range values {
			if v > max {
				max = v
			}
		}
	}
	if max <= 0 {
		max = 1
	}
	var b strings.Builder
	for _, v := range values {
		frac := v / max
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		idx := int(frac * float64(len(sparklineTicks)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparklineTicks) {
			idx = len(sparklineTicks) - 1
		}
		style := sparklineStyleFor(frac)
		b.WriteString(style.Render(string(sparklineTicks[idx])))
	}
	return b.String()
}

func sparklineStyleFor(frac float64) lipgloss.Style {
	switch {
	case frac >= 0.9:
		return styleSparklineRed
	case frac >= 0.7:
		return styleSparklineAmber
	default:
		return styleSparklineGreen
	}
}

// formatBytes renders byte counts the way the docker CLI does (MiB/GiB-ish,
// but decimal-suffixed to keep the stats tab readable at a glance).
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := "KMGTPE"
	return fmt.Sprintf("%.1f%ciB", float64(b)/float64(div), units[exp])
}

// renderStatsTab is the service Stats tab (TUI-DESIGN.md §4.2).
func renderStatsTab(m Model, row Row, width int) string {
	if !isRunningRow(row) {
		return padMetaLine(styleDim.Render("container not running — no stats"), width)
	}
	ring := m.stats[row.ContainerID]
	latest := ring.latest()
	if latest == nil {
		return padMetaLine(styleDim.Render("gathering stats…"), width)
	}

	samples := ring.ordered()
	cpuVals := make([]float64, len(samples))
	memVals := make([]float64, len(samples))
	memMax := 0.0
	for i, s := range samples {
		if s == nil {
			continue
		}
		cpuVals[i] = s.CPUPercent
		memVals[i] = float64(s.MemUsage)
		if s.MemLimit > 0 {
			memMax = float64(s.MemLimit)
		}
	}

	cpuLine := fmt.Sprintf("CPU   %5.1f%%          %s  (2 min)", latest.CPUPercent, sparkline(cpuVals, 100))

	var memLine string
	if latest.MemLimit > 0 {
		pct := float64(latest.MemUsage) / float64(latest.MemLimit) * 100
		memLine = fmt.Sprintf("MEM   %s / %s (%.0f%%)  %s", formatBytes(latest.MemUsage), formatBytes(latest.MemLimit), pct, sparkline(memVals, memMax))
	} else {
		memLine = fmt.Sprintf("MEM   %s  %s", formatBytes(latest.MemUsage), sparkline(memVals, memMax))
	}

	restarts := 0
	if info := m.inspects[row.ContainerID]; info != nil {
		restarts = info.RestartCount
	}
	netLine := fmt.Sprintf("net   ↓ %-8s ↑ %-8s   pids  %-6d restarts  %d",
		formatBytes(latest.NetRx), formatBytes(latest.NetTx), latest.PIDs, restarts)

	lines := []string{cpuLine, "", memLine, "", netLine}
	var sb strings.Builder
	for i, l := range lines {
		sb.WriteString(padMetaLine(l, width))
		if i != len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// listStatsColumns renders the dim, right-aligned CPU/MEM columns for the
// services panel (TUI-DESIGN.md §3.2). Returns "--"/"--" when not running or
// no sample has landed yet (ASCII so column width stays stable across terminals).
func listStatsColumns(m Model, row Row) (cpu, mem string) {
	if !isRunningRow(row) {
		return "--", "--"
	}
	latest := m.stats[row.ContainerID].latest()
	if latest == nil {
		return "--", "--"
	}
	return fmt.Sprintf("%.1f%%", latest.CPUPercent), formatBytes(latest.MemUsage)
}
