package ui

import (
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/hyturing/compose-pulse/internal/dag"
)

// graphTabRows builds the full-width project dependency graph: the exact
// Phase-1 tree walk (appendComposeRows), reused so ordering/indentation match
// the services panel (TUI-DESIGN.md §4.7 — pstree, never a canvas).
func graphTabRows(project Row) []Row {
	if project.Graph == nil {
		return nil
	}
	return appendComposeRows(nil, project.ProjectName, project.ConfigFiles, project.Graph)
}

func conditionShort(condition string) string {
	switch condition {
	case "service_healthy":
		return "healthy"
	case "service_completed_successfully":
		return "completed"
	default:
		return "started"
	}
}

// graphRowLine renders one graph-tab row: glyph, name, state, and — for
// dependent rows — each edge's condition plus whether it is met.
func graphRowLine(m Model, r Row, nameCol int) string {
	n := r.Node
	display, waitingOn := dag.Display(n, r.Graph)
	waiting := make(map[string]bool, len(waitingOn))
	for _, w := range waitingOn {
		waiting[w] = true
	}

	glyph := rowIndicator(m, r)
	head := r.linePrefix + glyph + " " + n.Name
	headW := runewidth.StringWidth(stripANSI(head))
	pad := nameCol - headW
	if pad < 1 {
		pad = 1
	}
	line := head + strings.Repeat(" ", pad) + styleDim.Render(displayStateLabel(display))

	if len(n.Deps) > 0 && r.linePrefix != "" {
		var conds []string
		for _, dep := range n.Deps {
			met := !waiting[dep]
			glyphStr := glyphCompleted
			if !met {
				glyphStr = glyphFailed
			}
			conds = append(conds, dep+":"+conditionShort(n.DepConditions[dep])+" "+glyphStr)
		}
		line += "   " + styleDim.Render("needs "+strings.Join(conds, " · "))
		if display == dag.DisplayBlocked {
			tail := "blocked"
			if hint := waitingSinceHint(m, r); hint != "" {
				if idx := strings.Index(hint, "· "); idx >= 0 {
					tail = "blocked " + hint[idx+2:]
				}
			}
			line += "  " + styleDim.Render("→  "+tail)
		}
	}
	return line
}

// renderGraphTab is the project Graph tab (TUI-DESIGN.md §4.7).
func renderGraphTab(m Model, project Row, width int) string {
	rows := graphTabRows(project)
	if len(rows) == 0 {
		return padMetaLine(styleDim.Render("no services"), width)
	}
	nameCol := 0
	for _, r := range rows {
		w := runewidth.StringWidth(stripANSI(r.linePrefix)) + 2 + runewidth.StringWidth(r.Node.Name)
		if w > nameCol {
			nameCol = w
		}
	}
	nameCol += 2

	var lines []string
	for i, r := range rows {
		line := graphRowLine(m, r, nameCol)
		if i == m.graphCursor {
			w := width - scrollBarWidth
			line = styleSelected.Width(w).Render(padPlain(stripANSI(line), w))
		}
		lines = append(lines, line)
	}

	scroll := m.graphScroll
	if scroll < 0 || scroll >= len(lines) {
		scroll = 0
	}
	if scroll < len(lines) {
		lines = lines[scroll:]
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
