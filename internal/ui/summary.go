package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
)

// stateCounts tallies services across all projects by derived DisplayState.
type stateCounts struct {
	Total, Healthy, Starting, Blocked, Pending, Missing,
	Completed, Failed, Unhealthy, Standalone int
}

// countStates tallies every compose service in snap by its DisplayState.
func countStates(snap *discover.Snapshot) stateCounts {
	var c stateCounts
	if snap == nil {
		return c
	}
	for _, project := range snap.Projects {
		for _, node := range project.Graph.Ordered {
			c.Total++
			state, _ := dag.Display(node, project.Graph)
			switch state {
			case dag.DisplayHealthy:
				c.Healthy++
			case dag.DisplayStarting:
				c.Starting++
			case dag.DisplayBlocked:
				c.Blocked++
			case dag.DisplayUnhealthy:
				c.Unhealthy++
			case dag.DisplayFailed:
				c.Failed++
			case dag.DisplayCompleted:
				c.Completed++
			case dag.DisplayMissing:
				c.Missing++
			case dag.DisplayPending:
				c.Pending++
			}
		}
	}
	c.Standalone = len(snap.Standalone)
	return c
}

// projectLabel summarizes which project(s) a snapshot covers for the summary bar.
func projectLabel(snap *discover.Snapshot) string {
	if snap == nil || len(snap.Projects) == 0 {
		return "no projects"
	}
	if len(snap.Projects) == 1 {
		return snap.Projects[0].Name
	}
	return fmt.Sprintf("%d projects", len(snap.Projects))
}

// renderSummaryBar renders the top status line: project label, service
// counts by state, and a live "updated Ns ago" freshness indicator.
// Colored count tokens use foreground only — no per-token background, so
// the bar stays a single clean line on the terminal background.
func renderSummaryBar(counts stateCounts, label string, sinceUpdate time.Duration, width int) string {
	parts := []string{
		"cpulse",
		label,
		fmt.Sprintf("%d services", counts.Total),
	}

	buckets := []struct {
		n     int
		label string
		color lipgloss.Color
	}{
		{counts.Healthy, "healthy", colorHealthy},
		{counts.Starting, "starting", colorStarting},
		{counts.Blocked, "blocked", colorPending},
		{counts.Unhealthy, "unhealthy", colorUnhealthy},
		{counts.Failed, "failed", colorUnhealthy},
		{counts.Missing, "missing", colorUnhealthy},
		{counts.Completed, "completed", colorHealthy},
		{counts.Pending, "pending", colorPending},
	}
	for _, b := range buckets {
		if b.n > 0 {
			parts = append(parts, lipgloss.NewStyle().Foreground(b.color).Render(fmt.Sprintf("%d %s", b.n, b.label)))
		}
	}

	secs := sinceUpdate.Seconds()
	if secs > 9.9 {
		parts = append(parts, "updated 9.9s+ ago")
	} else {
		parts = append(parts, fmt.Sprintf("updated %.1fs ago", secs))
	}

	text := strings.Join(parts, "  ·  ")
	return styleSummaryBar.Width(width).Render(truncateToWidth(text, width))
}
