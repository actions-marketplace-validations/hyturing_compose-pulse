package ui

import (
	"strings"
	"time"
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

	visibleRows := m.visibleRows()
	var graphContent string
	if len(visibleRows) == 0 && m.rowFilter != filterAll {
		graphContent = styleDim.Render(emptyFilterMessage(m.rowFilter))
	} else {
		graphContent = renderGraphContent(visibleRows, m.cursor, m.graphScroll, m.spinFrame, leftW-2, innerH-1)
	}
	leftPanel := renderPanel(leftPanelTitle(m), graphContent, leftW, panelH, m.panelFocus == focusGraph, false)

	var panels string
	if compact {
		panels = leftPanel
	} else {
		previewContent := renderPreview(m, rightW-2)
		rightPanel := renderPanel(inspectorTitle(m), previewContent, rightW, panelH, m.panelFocus == focusPreview, true)
		panelLines := panelRenderedHeight(leftPanel)
		if h := panelRenderedHeight(rightPanel); h > panelLines {
			panelLines = h
		}
		panels = joinPanelsHorizontal(leftPanel, rightPanel, panelLines)
	}

	var sinceUpdate time.Duration
	if !m.lastPoll.IsZero() {
		sinceUpdate = time.Since(m.lastPoll)
	}
	summary := renderSummaryBar(countStates(m.snapshot), projectLabel(m.snapshot), sinceUpdate, width)

	statusText := " ↑↓ move   tab/←→ switch panel   enter logs   1-3 tabs   f failed   b blocked   a actions   ? help   q quit"
	if m.panelFocus == focusPreview {
		statusText = " ↑↓/wheel scroll   tab/←→ switch panel   g follow   a actions   ? help   q quit"
	}
	status := renderStatusBar(width, statusText)
	return summary + "\n" + panels + "\n" + status
}

func renderGraphContent(rows []Row, cursor, scroll, spinFrame, width, maxLines int) string {
	if len(rows) == 0 {
		return styleDim.Render("No containers found.")
	}

	nameCol := nameColumn(rows, width)

	var lines []string
	for i, row := range rows {
		var line string
		switch row.Kind {
		case RowProjectHeader, RowStandaloneHeader:
			line = styleSectionHeader.Render(row.Label)
		case RowComposeNode:
			line = formatComposeLine(row, spinFrame, nameCol)
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
	visibleCount := len(m.visibleRows())
	if visibleCount == 0 {
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
	maxScroll := visibleCount - maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.graphScroll > maxScroll {
		m.graphScroll = maxScroll
	}
}
