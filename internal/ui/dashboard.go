package ui

import (
	"strings"
)

func renderDashboard(m Model) string {
	width := m.width
	if width < 1 {
		width = 80
	}
	height := m.height
	if height < 1 {
		height = 24
	}

	leftW, rightW, panelH, compact := dashboardLayout(width, height)
	innerH := panelInnerHeight(panelH)

	graphContent := renderGraphContent(m.rows, m.cursor, m.graphScroll, m.spinFrame, leftW-2, innerH-1)
	leftPanel := renderPanel("[1] Services", graphContent, leftW, panelH, m.panelFocus == focusGraph, false)

	var panels string
	if compact {
		panels = leftPanel
	} else {
		previewContent := renderPreview(m, rightW-2)
		rightPanel := renderPanel("[2] Details", previewContent, rightW, panelH, m.panelFocus == focusPreview, true)
		panelLines := panelRenderedHeight(leftPanel)
		if h := panelRenderedHeight(rightPanel); h > panelLines {
			panelLines = h
		}
		panels = joinPanelsHorizontal(leftPanel, rightPanel, panelLines)
	}

	statusText := " tab/←→: switch panel   ↑↓: navigate   a: actions   enter: fullscreen   q: quit"
	if m.panelFocus == focusPreview {
		statusText = " tab/←→: switch panel   ↑↓/wheel: scroll logs   g: follow   a: actions   enter: fullscreen   q: quit"
	}
	status := renderStatusBar(width, statusText)
	return panels + "\n" + status
}

func renderGraphContent(rows []Row, cursor, scroll, spinFrame, width, maxLines int) string {
	if len(rows) == 0 {
		return styleDim.Render("No containers found.")
	}

	var lines []string
	for i, row := range rows {
		var line string
		switch row.Kind {
		case RowProjectHeader, RowStandaloneHeader:
			line = styleSectionHeader.Render(row.Label)
		case RowComposeNode:
			line = formatComposeLine(row, spinFrame)
		case RowStandalone:
			line = formatStandaloneLine(row, stateIndicator(row.Standalone.State, spinFrame))
		}
		if i == cursor && isSelectable(row) {
			line = styleSelected.Render(padLine(truncateToVisibleWidth(line, width), width))
		} else {
			line = truncateToVisibleWidth(line, width)
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
	visible := lines[scroll:end]
	return strings.Join(visible, "\n")
}

func (m *Model) clampGraphScroll() {
	if len(m.rows) == 0 {
		m.graphScroll = 0
		return
	}
	leftW, _, panelH, _ := dashboardLayout(m.width, m.height)
	maxLines := panelInnerHeight(panelH) - 1
	if maxLines < 1 {
		maxLines = 1
	}
	_ = leftW

	if m.cursor < m.graphScroll {
		m.graphScroll = m.cursor
	}
	if m.cursor >= m.graphScroll+maxLines {
		m.graphScroll = m.cursor - maxLines + 1
	}
	if m.graphScroll < 0 {
		m.graphScroll = 0
	}
	maxScroll := len(m.rows) - maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.graphScroll > maxScroll {
		m.graphScroll = maxScroll
	}
}
