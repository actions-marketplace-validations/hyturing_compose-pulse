package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/dag"
)

// renderDashboard renders the dashboard shell: one left dependency-graph
// panel (all compose projects, each with its pstree) plus one main panel on
// the right, wrapped in a summary bar and status bar.
func renderDashboard(m Model) string {
	width := m.width
	if width < 1 {
		width = 80
	}
	height := m.height
	if height < 1 {
		height = 24
	}

	leftW, mainW, panelH, compact := dashboardLayout(width, height)

	visible := m.visibleRows()
	innerH := panelInnerHeight(panelH)
	innerW := leftW - 2
	cols := computeGraphColumns(visible, innerW)
	var graphContent string
	if len(visible) == 0 && m.rowFilter != filterAll {
		graphContent = styleDim.Render(emptyFilterMessage(m.rowFilter))
	} else {
		graphContent = renderGraphContent(m, visible, m.cursor, m.graphScroll, innerW, innerH)
	}
	leftPanel := renderPanel(
		formatGraphColumnHeader(cols, m.rowFilter),
		graphContent,
		leftW, panelH,
		m.panelFocus == focusLeft,
		false,
	)

	var panels string
	if compact {
		panels = leftPanel
	} else {
		var mainContent string
		if m.actionMode != actionModeNone {
			mainContent = renderActionView(m, mainW-2)
		} else {
			mainContent = renderMainPanel(m, mainW-2)
		}
		mainPanel := renderPanel(mainPanelTitle(m), mainContent, mainW, panelH, m.panelFocus == focusMain, true)
		panels = joinPanelsHorizontal(leftPanel, mainPanel, panelH)
	}

	var sinceUpdate time.Duration
	if !m.lastPoll.IsZero() {
		sinceUpdate = time.Since(m.lastPoll)
	}
	summary := renderSummaryBar(countStates(m.snapshot), projectLabel(m.snapshot), sinceUpdate, width)

	status := renderStatusBar(width, formatStatusHints(width, statusHintContext(m)))
	return summary + "\n" + panels + "\n" + status
}

// renderGraphContent renders the left panel body: every compose project
// header followed by its pstree dependency graph. cursor/scroll index into rows.
func renderGraphContent(m Model, rows []Row, cursor, scroll, width, maxLines int) string {
	if len(rows) == 0 {
		return styleDim.Render("No containers found.")
	}

	cols := computeGraphColumns(rows, width)

	var lines []string
	for i, row := range rows {
		var line string
		switch row.Kind {
		case RowProjectHeader:
			line = formatProjectHeader(row, m.spinFrame, cols)
		case RowStandaloneHeader:
			line = formatStandaloneHeader(row.Label, cols)
		case RowComposeNode:
			line = formatComposeLine(m, row, cols)
		case RowStandalone:
			line = formatStandaloneLine(m, row, cols)
		case RowSpacer:
			line = ""
		}
		if i == cursor && isSelectable(row) {
			// Strip nested cell colors first — lipgloss Background does not
			// reliably paint across existing ANSI segments, which left a
			// one-cell cursor blob instead of a full-row highlight.
			line = styleSelected.Width(width).Render(padPlain(stripANSI(line), width))
		} else {
			line = padVisible(line, width)
		}
		lines = append(lines, line)
	}

	if scroll < 0 {
		scroll = 0
	}
	if scroll >= len(lines) {
		scroll = maxInt(0, len(lines)-1)
	}
	end := scroll + maxLines
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[scroll:end], "\n")
}

// clampGraphScroll keeps the left-panel scroll positioned so the cursor
// row stays on screen.
func (m *Model) clampGraphScroll() {
	visible := m.visibleRows()
	visibleCount := len(visible)
	if visibleCount == 0 {
		m.graphScroll = 0
		return
	}
	_, _, panelH, _ := dashboardLayout(m.width, m.height)
	maxLines := panelInnerHeight(panelH)
	if maxLines < 1 {
		maxLines = 1
	}

	if m.cursor < m.graphScroll {
		m.graphScroll = m.cursor
	}
	if m.cursor >= m.graphScroll+maxLines {
		m.graphScroll = m.cursor - maxLines + 1
	}
	if m.graphScroll < 0 {
		m.graphScroll = 0
	}
	maxScroll := visibleCount - maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.graphScroll > maxScroll {
		m.graphScroll = maxScroll
	}
}

// formatProjectHeader renders a compose project section row on the grid:
// name only — counts live in the right-panel summary under the title.
func formatProjectHeader(row Row, spinFrame int, cols graphColumns) string {
	_ = spinFrame
	nameCell := padVisible(
		styleSectionHeader.Render("▸ ")+styleProjectName.Render(row.ProjectName),
		cols.nameW,
	)
	stateCell := padVisible("", cols.stateW)
	detailCell := padVisible("", cols.detailW)
	if !cols.showStats {
		return joinGraphRow(nameCell, stateCell, detailCell, "", "")
	}
	return joinGraphRow(
		nameCell, stateCell, detailCell,
		padVisible("", graphCPUColWidth),
		padVisible("", graphMEMColWidth),
	)
}

// formatStandaloneHeader spans the full row width so the section title is not
// clipped to the SERVICE column.
func formatStandaloneHeader(label string, cols graphColumns) string {
	return padVisible(styleSectionHeader.Render(label), cols.totalWidth())
}

// totalWidth is the exact visible width of a rendered graph row for these columns.
func (c graphColumns) totalWidth() int {
	w := c.nameW + graphColGap + c.stateW + graphColGap + c.detailW
	if c.showStats {
		w += graphColGap + graphStatsWidth()
	}
	return w
}

// formatProjectSummaryLine is the right-panel summary under the title:
// "6 services  ✕1 fail  ┄1 wait  …"
func formatProjectSummaryLine(g *dag.Graph, spinFrame, width int) string {
	n := 0
	if g != nil {
		n = len(g.Ordered)
	}
	line := styleDim.Render(fmt.Sprintf("%d services", n))
	if parts := projectSummaryParts(g, spinFrame); len(parts) > 0 {
		line += "  " + strings.Join(parts, "  ")
	}
	return padMetaLine(line, width)
}

// projectSummaryParts renders nonzero state-count buckets for a project
// in a compact form (glyph + count + short word).
func projectSummaryParts(g *dag.Graph, spinFrame int) []string {
	if g == nil {
		return nil
	}
	var up, starting, waiting, failed, unhealthy, completed, pending int
	for _, n := range g.Ordered {
		state, _ := dag.Display(n, g)
		switch state {
		case dag.DisplayHealthy:
			up++
		case dag.DisplayStarting:
			starting++
		case dag.DisplayBlocked:
			waiting++
		case dag.DisplayFailed:
			failed++
		case dag.DisplayUnhealthy:
			unhealthy++
		case dag.DisplayCompleted:
			completed++
		case dag.DisplayPending:
			pending++
		}
	}

	var parts []string
	add := func(count int, glyph, word string) {
		if count > 0 {
			parts = append(parts, fmt.Sprintf("%s%d %s", glyph, count, word))
		}
	}
	add(failed, glyphFailed, "fail")
	add(unhealthy, glyphUnhealthy, "sick")
	add(waiting, glyphPending, "wait")
	add(starting, glyphStartingFrames[spinFrame%len(glyphStartingFrames)], "run")
	add(up, glyphHealthy, "up")
	add(completed, glyphCompleted, "done")
	add(pending, glyphPending, "pend")
	return parts
}

// renderView renders a flat graph, used by pure render tests.
func renderView(rows []Row, cursor, spinFrame, width int) string {
	m := Model{spinFrame: spinFrame}
	return renderGraphContent(m, rows, cursor, 0, width, len(rows)+1)
}
