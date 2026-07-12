package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/probe"
)

// probeCmd runs the health probe for one container in a tea.Cmd goroutine
// (Task 2.5, TUI-DESIGN.md §4.4) — read-only commands only, and only on the
// explicit `enter` keypress that triggers this.
func probeCmd(dc *docker.Client, containerID string, hc *docker.HealthcheckSpec, ports []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		exec := func(ctx context.Context, cmd []string) (string, int, error) {
			return dc.ExecCapture(ctx, containerID, cmd)
		}
		report := probe.Run(ctx, containerID, hc, ports, exec)
		return probeMsg{containerID: containerID, report: report}
	}
}

func stepGlyph(s probe.StepStatus) string {
	switch s {
	case probe.StepPass:
		return glyphCompleted
	case probe.StepFail:
		return glyphFailed
	case probe.StepWarn:
		return glyphDegraded
	default:
		return styleDim.Render("–")
	}
}

// renderHealthTab is the service Health tab (TUI-DESIGN.md §4.4).
func renderHealthTab(m Model, row Row, width int) string {
	var lines []string
	info := m.inspects[row.ContainerID]

	var hc *docker.HealthcheckSpec
	if info != nil {
		hc = info.Healthcheck
	}
	if hc == nil {
		lines = append(lines, styleDim.Render("no healthcheck configured"))
		lines = append(lines, "  → add a healthcheck so cpulse can explain startup readiness")
	} else {
		lines = append(lines, styleDim.Render("healthcheck"))
		lines = append(lines, fmt.Sprintf("  test      %s", strings.Join(hc.Test, " ")))
		lines = append(lines, fmt.Sprintf("  interval  %s   timeout  %s   retries  %d", hc.Interval, hc.Timeout, hc.Retries))
		if hc.StartPeriod > 0 {
			lines = append(lines, fmt.Sprintf("  start_period  %s", hc.StartPeriod))
		}
	}

	if info != nil && info.Health != nil {
		lines = append(lines, "", styleDim.Render("status"))
		lines = append(lines, fmt.Sprintf("  %s   failing streak %d", info.Health.Status, info.Health.FailingStreak))
		if n := len(info.Health.Log); n > 0 {
			last := info.Health.Log[n-1]
			lines = append(lines, "", styleDim.Render("last probe"))
			lines = append(lines, fmt.Sprintf("  exit %d: %s", last.ExitCode, truncateOneLine(last.Output, 200)))
		}
	}

	lines = append(lines, "")
	switch {
	case m.probeLoading:
		lines = append(lines, spinnerFrames[m.spinFrame%len(spinnerFrames)]+" running probe…")
	case m.probeReport != nil && m.probeFor == row.ContainerID:
		lines = append(lines, styleDim.Render("probe results"))
		for _, step := range m.probeReport.Steps {
			lines = append(lines, fmt.Sprintf("  %s %s: %s", stepGlyph(step.Status), step.Label, truncateOneLine(step.Output, 160)))
		}
		for _, s := range m.probeReport.Suggestions {
			lines = append(lines, "  → "+s)
		}
		lines = append(lines, "", styleDim.Render("enter — run probe again"))
	default:
		lines = append(lines, styleDim.Render("enter — run probe now"))
	}

	var sb strings.Builder
	for i, l := range lines {
		sb.WriteString(padMetaLine(l, width))
		if i != len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func truncateOneLine(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
