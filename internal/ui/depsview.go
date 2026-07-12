package ui

import (
	"fmt"
	"strings"

	"github.com/hyturing/compose-pulse/internal/dag"
)

// depsJumpTargets returns the service names in the exact order the Deps tab
// renders them, so `enter` + depsCursor can jump the left-column selection
// to the right row (TUI-DESIGN.md §4.3).
func depsJumpTargets(row Row) []string {
	if row.Kind != RowComposeNode {
		return nil
	}
	insp := buildServiceInspector(row)
	var out []string
	for _, d := range insp.Dependencies {
		out = append(out, d.Name)
	}
	out = append(out, dag.DirectDependents(row.Graph, row.Node.Name)...)
	out = append(out, dag.TransitiveDependents(row.Graph, row.Node.Name)...)
	return out
}

// viaParent finds which direct dependent's subtree a transitive dependent
// belongs to, for the "via <x>" annotation.
func viaParent(g *dag.Graph, service, transitiveDep string) string {
	for _, direct := range dag.DirectDependents(g, service) {
		if direct == transitiveDep {
			return direct
		}
		for _, sub := range dag.AllDependents(g, direct) {
			if sub == transitiveDep {
				return direct
			}
		}
	}
	return ""
}

// restartOrder is the selected service followed by every dependent
// (direct + transitive) in topological order — the fix-up sequence after a
// restart (TUI-DESIGN.md §4.3).
func restartOrder(g *dag.Graph, service string) []string {
	return append([]string{service}, dag.AllDependents(g, service)...)
}

func depGlyphFor(satisfied bool, state dag.DisplayState) string {
	if satisfied {
		return glyphCompleted
	}
	return displayIndicator(state, 0)
}

// renderDepsTab is the service Deps tab (TUI-DESIGN.md §4.3): both
// directions plus the fix order, folding in the old "impact view".
func renderDepsTab(m Model, row Row, width int) string {
	insp := buildServiceInspector(row)
	direct := dag.DirectDependents(row.Graph, row.Node.Name)
	transitive := dag.TransitiveDependents(row.Graph, row.Node.Name)

	var lines []string
	lines = append(lines, styleDim.Render("waits on"))
	if len(insp.Dependencies) == 0 {
		lines = append(lines, "  none")
	}
	cursorIdx := 0
	for _, d := range insp.Dependencies {
		marker := "  "
		if cursorIdx == m.depsCursor {
			marker = styleLogMarker.Render("▸") + " "
		}
		unmet := "met"
		if !d.Satisfied {
			unmet = "unmet"
		}
		lines = append(lines, fmt.Sprintf("%s%s %-14s %-10s %s", marker, depGlyphFor(d.Satisfied, d.State), d.Name, d.Condition, unmet))
		cursorIdx++
	}

	lines = append(lines, "", styleDim.Render("blocks"))
	if len(direct)+len(transitive) == 0 {
		lines = append(lines, "  none")
	}
	for _, dep := range direct {
		marker := "  "
		if cursorIdx == m.depsCursor {
			marker = styleLogMarker.Render("▸") + " "
		}
		depState, _ := dag.Display(row.Graph.ByName[dep], row.Graph)
		lines = append(lines, fmt.Sprintf("%s%s %-14s direct", marker, displayIndicator(depState, 0), dep))
		cursorIdx++
	}
	for _, dep := range transitive {
		marker := "  "
		if cursorIdx == m.depsCursor {
			marker = styleLogMarker.Render("▸") + " "
		}
		depState, _ := dag.Display(row.Graph.ByName[dep], row.Graph)
		via := viaParent(row.Graph, row.Node.Name, dep)
		lines = append(lines, fmt.Sprintf("%s%s %-14s via %s", marker, displayIndicator(depState, 0), dep, via))
		cursorIdx++
	}

	if order := restartOrder(row.Graph, row.Node.Name); len(order) > 1 {
		lines = append(lines, "", styleDim.Render("restart order: "+strings.Join(order, " → ")))
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
