package ui

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func isSelectable(row Row) bool {
	return row.Kind == RowComposeNode || row.Kind == RowStandalone
}

// formatComposeLine renders one compose-service row: glyph, column-aligned
// name, state label, and a short hint (exit code / blocker / dep count).
func formatComposeLine(row Row, spinFrame, nameCol int) string {
	n := row.Node
	display, waitingOn := dag.Display(n, row.Graph)
	indicator := displayIndicator(display, spinFrame)
	name := styleName.Render(n.Name)

	prefixW := runewidth.StringWidth(stripANSI(row.linePrefix))
	nameW := runewidth.StringWidth(n.Name)
	padW := nameCol - prefixW
	if padW < nameW+1 {
		padW = nameW + 1
	}
	paddedName := name + strings.Repeat(" ", padW-nameW)

	segments := []string{
		row.linePrefix + indicator + " " + paddedName,
		styleDim.Render(displayStateLabel(display)),
	}
	if hint := displayHint(display, waitingOn, n); hint != "" {
		segments = append(segments, styleDim.Render(hint))
	}
	return strings.Join(segments, "  ")
}

// nameColumn computes the shared name column width for a set of rows: the
// widest visible (prefix + name) combination, capped at half the panel width.
func nameColumn(rows []Row, panelWidth int) int {
	maxLen := 0
	for _, r := range rows {
		if r.Kind != RowComposeNode {
			continue
		}
		w := runewidth.StringWidth(stripANSI(r.linePrefix)) + runewidth.StringWidth(r.Node.Name)
		if w > maxLen {
			maxLen = w
		}
	}
	half := panelWidth / 2
	if maxLen > half {
		maxLen = half
	}
	if maxLen < 1 {
		maxLen = 1
	}
	return maxLen
}

func displayStateLabel(d dag.DisplayState) string {
	if d == dag.DisplayPending {
		return "not started"
	}
	return string(d)
}

func displayHint(d dag.DisplayState, waitingOn []string, n *dag.Node) string {
	switch d {
	case dag.DisplayFailed:
		if n.ExitCode != nil {
			return fmt.Sprintf("exit %d", *n.ExitCode)
		}
		return "exit ?"
	case dag.DisplayCompleted:
		return "exit 0"
	case dag.DisplayBlocked:
		if len(waitingOn) == 0 {
			return ""
		}
		first := waitingOn[0]
		label := first
		if n.DepConditions[first] == "service_healthy" {
			label = first + ":healthy"
		}
		if extra := len(waitingOn) - 1; extra > 0 {
			label += fmt.Sprintf(" +%d", extra)
		}
		return label
	case dag.DisplayHealthy:
		if len(n.Deps) > 0 {
			return fmt.Sprintf("+%d deps", len(n.Deps))
		}
		return ""
	default:
		return ""
	}
}

// displayIndicator renders the glyph for a derived DisplayState.
func displayIndicator(d dag.DisplayState, frame int) string {
	switch d {
	case dag.DisplayHealthy:
		return glyphHealthy
	case dag.DisplayStarting:
		return glyphStartingFrames[frame%len(glyphStartingFrames)]
	case dag.DisplayUnhealthy:
		return glyphUnhealthy
	case dag.DisplayCompleted:
		return glyphCompleted
	case dag.DisplayFailed:
		return glyphFailed
	case dag.DisplayDegraded:
		return glyphDegraded
	case dag.DisplayBlocked, dag.DisplayPending:
		return glyphPending
	default:
		return "?"
	}
}

// stateIndicator renders the glyph for a raw container state — used for
// standalone containers, which have no DAG and thus no DisplayState.
func stateIndicator(s docker.ContainerState, frame int) string {
	switch s {
	case docker.StateHealthy:
		return glyphHealthy
	case docker.StateStarting:
		return glyphStartingFrames[frame%len(glyphStartingFrames)]
	case docker.StateUnhealthy:
		return glyphUnhealthy
	case docker.StatePending:
		return glyphPending
	case docker.StateExited:
		return glyphFailed
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
