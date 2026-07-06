package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func isSelectable(row Row) bool {
	return row.Kind == RowComposeNode || row.Kind == RowStandalone
}

func formatComposeLine(row Row, spinFrame int) string {
	n := row.Node
	eff, waitingOn := dag.EffectiveState(n, row.Graph)
	indicator := stateIndicator(eff, spinFrame)
	name := styleName.Render(n.Name)
	stateTxt := formatEffectiveStateLabel(eff, waitingOn, n)

	var extraDeps string
	if len(n.Deps) > 1 {
		extraDeps = styleDim.Render(" also←" + strings.Join(n.Deps[1:], ","))
	}

	return fmt.Sprintf("%s%s %s %s%s", row.linePrefix, indicator, name, stateTxt, extraDeps)
}

func formatEffectiveStateLabel(eff docker.ContainerState, waitingOn []string, n *dag.Node) string {
	if eff == docker.StatePending {
		if len(waitingOn) > 0 {
			parts := make([]string, len(waitingOn))
			for i, w := range waitingOn {
				cond := n.DepConditions[w]
				if cond == "" {
					cond = "service_started"
				}
				if cond == "service_healthy" {
					parts[i] = w + ":healthy"
				} else {
					parts[i] = w
				}
			}
			return styleDim.Render("(pending ← " + strings.Join(parts, ", ") + ")")
		}
		return styleDim.Render("(not started)")
	}
	return styleDim.Render("(" + n.State.String() + ")")
}

func stateIndicator(s docker.ContainerState, frame int) string {
	switch s {
	case docker.StateHealthy:
		return lipgloss.NewStyle().Foreground(colorHealthy).Render("●")
	case docker.StateStarting:
		spinner := spinnerFrames[frame%len(spinnerFrames)]
		return lipgloss.NewStyle().Foreground(colorStarting).Render(spinner)
	case docker.StateUnhealthy:
		return lipgloss.NewStyle().Foreground(colorUnhealthy).Render("●")
	case docker.StatePending:
		return lipgloss.NewStyle().Foreground(colorPending).Render("○")
	case docker.StateExited:
		return lipgloss.NewStyle().Foreground(colorUnhealthy).Render("✕")
	default:
		return "?"
	}
}

func formatStandaloneLine(r Row, indicator string) string {
	name := styleName.Render(r.Standalone.Name)
	image := styleDim.Render(r.Label)
	stateTxt := styleDim.Render("(" + r.Standalone.State.String() + ")")
	return fmt.Sprintf("  %s %s %s %s", indicator, name, image, stateTxt)
}

// renderView renders a flat graph (used in tests).
func renderView(rows []Row, cursor, spinFrame, width int) string {
	return renderGraphContent(rows, cursor, 0, spinFrame, width, len(rows)+1)
}
