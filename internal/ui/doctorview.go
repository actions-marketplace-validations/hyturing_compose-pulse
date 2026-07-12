package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/doctor"
)

// doctorCmd runs the doctor engine + root-cause analysis for one project in
// a tea.Cmd goroutine so the update loop never blocks on it (Task 2.3/2.4,
// TUI-DESIGN.md §4.5).
func doctorCmd(dc *docker.Client, project string, graph *dag.Graph, configFiles []string) tea.Cmd {
	return func() tea.Msg {
		ctx := buildDoctorContext(dc, project, graph, configFiles)
		findings := doctor.Run(ctx, doctor.DefaultRules())
		root := doctor.FindRootCause(ctx)
		return doctorMsg{project: project, findings: findings, root: root}
	}
}

func buildDoctorContext(dc *docker.Client, project string, graph *dag.Graph, configFiles []string) doctor.Context {
	var cfg *compose.Config
	if len(configFiles) > 0 {
		if parsed, err := compose.Parse(configFiles[0]); err == nil {
			cfg = parsed
		}
	}
	return doctor.Context{
		Project: &discover.Project{Name: project, Graph: graph, ConfigFiles: configFiles},
		Config:  cfg,
		Inspect: func(id string) (*docker.InspectInfo, error) {
			c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			return dc.Inspect(c, id)
		},
		Logs: func(id string, tail int) ([]string, error) {
			c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			return dc.FetchLogLines(c, id, tail)
		},
		Now: time.Now(),
	}
}

func severityGlyph(s doctor.Severity) string {
	switch s {
	case doctor.SeverityCritical:
		return styleCritical.Render("✕")
	case doctor.SeverityWarn:
		return styleWarn.Render("⚠")
	default:
		return styleInfo.Render("·")
	}
}

// renderDoctorTab is the project Doctor tab (TUI-DESIGN.md §4.5, the
// signature feature). Findings are addressable by index for `enter`.
func renderDoctorTab(m Model, width int) string {
	if m.doctorLoading {
		return padMetaLine(styleDim.Render(spinnerFrames[m.spinFrame%len(spinnerFrames)]+" Diagnosing…"), width)
	}
	if m.doctorFor == "" {
		return padMetaLine(styleDim.Render("Select the project row to diagnose it."), width)
	}

	var lines []string
	lines = append(lines, styleSectionHeader.Render("ROOT CAUSE"))
	if m.doctorRoot != nil && len(m.doctorRoot.Culprits) > 0 {
		root := m.doctorRoot
		for _, culprit := range root.Culprits {
			line := " " + styleCritical.Render("✕") + " " + culprit
			lines = append(lines, line)
			if msg := root.FirstLog[culprit]; msg != "" {
				lines = append(lines, "   "+styleDim.Render(fmt.Sprintf("%q", msg)))
			}
		}
		if len(root.CriticalPath) > 0 {
			lines = append(lines, " "+styleDim.Render("fix order: "+strings.Join(root.CriticalPath, " → ")))
		}
	} else {
		lines = append(lines, " "+styleDim.Render("nothing blocked or broken right now"))
	}

	lines = append(lines, "", styleSectionHeader.Render("FINDINGS"))
	if len(m.doctorFindings) == 0 {
		lines = append(lines, " "+styleDim.Render("no findings"))
	}
	for i, f := range m.doctorFindings {
		prefix := "  "
		if i == m.doctorCursor {
			prefix = styleLogMarker.Render("▸") + " "
		}
		line := fmt.Sprintf("%s%s %s: %s", prefix, severityGlyph(f.Severity), f.Service, f.Title)
		if len(f.Suggestion) > 0 {
			line += " " + styleDim.Render("→ "+f.Suggestion[0])
		}
		lines = append(lines, line)
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
