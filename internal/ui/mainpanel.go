package ui

import (
	"strings"

	"github.com/hyturing/compose-pulse/internal/dag"
)

// Main-panel tabs for a selected service (TUI-DESIGN.md §4).
const (
	tabLogs = iota
	tabStats
	tabDeps
	tabHealth
)

// Main-panel tabs for the selected project row (TUI-DESIGN.md §4.5-4.7).
// These intentionally reuse 0/1/2 — the meaning of mainTab depends on
// Model.selectionIsProject.
const (
	tabDoctor = iota
	tabTimeline
	tabGraph
)

var serviceTabLabels = []string{"1 logs", "2 stats", "3 deps", "4 health"}
var projectTabLabels = []string{"1 doctor", "2 timeline", "3 graph"}

// mainTabCount returns how many tabs are available for the current
// selection kind, so `1-4`/`[`/`]` clamp correctly (TUI-DESIGN.md §4).
func mainTabCount(selectionIsProject bool) int {
	if selectionIsProject {
		return len(projectTabLabels)
	}
	return len(serviceTabLabels)
}

func clampMainTab(tab int, selectionIsProject bool) int {
	if n := mainTabCount(selectionIsProject); tab >= n {
		return n - 1
	}
	if tab < 0 {
		return 0
	}
	return tab
}

func renderMainTabStrip(tab int, selectionIsProject bool) string {
	labels := serviceTabLabels
	if selectionIsProject {
		labels = projectTabLabels
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		if i == tab {
			parts[i] = styleTabActive.Render("[" + l + "]")
		} else {
			parts[i] = styleDim.Render("[" + l + "]")
		}
	}
	return strings.Join(parts, " ")
}

// mainPanelTitle is the main panel's title: selected service/project name +
// state, or "Actions" while the x menu is open.
func mainPanelTitle(m Model) string {
	if m.actionMode != actionModeNone {
		return "Actions"
	}
	visible := m.visibleRows()
	if m.cursor >= len(visible) || !isSelectable(visible[m.cursor]) {
		return "Details"
	}
	row := visible[m.cursor]
	switch row.Kind {
	case RowProjectHeader:
		return row.ProjectName
	case RowStandalone:
		return row.Standalone.Name + " · " + row.Standalone.State.String()
	default:
		display, _ := dag.Display(row.Node, row.Graph)
		return row.Node.Name + " · " + displayStateLabel(display)
	}
}

func (m Model) mainPanelVisibleLines() int {
	_, _, panelH, compact := dashboardLayout(m.width, m.height)
	if compact {
		return 1
	}
	// title is outside the body; body = tab strip + optional project summary + content
	bodyLines := panelInnerHeight(panelH) - 1 // minus tab strip
	if m.selectionHasProjectSummary() {
		bodyLines-- // minus summary line under the title
	}
	if bodyLines < 1 {
		return 1
	}
	return bodyLines
}

func (m Model) selectionHasProjectSummary() bool {
	visible := m.visibleRows()
	if m.cursor >= len(visible) {
		return false
	}
	switch visible[m.cursor].Kind {
	case RowProjectHeader, RowComposeNode:
		return true
	default:
		return false
	}
}

// renderMainPanel renders the main panel body: the x menu when open,
// otherwise the tab that follows the current selection (service or
// project, TUI-DESIGN.md §4).
func renderMainPanel(m Model, width int) string {
	visible := m.visibleRows()
	if m.cursor >= len(visible) || !isSelectable(visible[m.cursor]) {
		if m.rowFilter != filterAll {
			return padMetaLine(styleDim.Render(emptyFilterMessage(m.rowFilter)), width)
		}
		return padMetaLine(styleDim.Render("Select a project or service"), width)
	}
	row := visible[m.cursor]

	if row.Kind == RowProjectHeader {
		summary := formatProjectSummaryLine(row.Graph, m.spinFrame, width)
		tabStrip := renderMainTabStrip(m.mainTab, true)
		body := renderProjectTabBody(m, row, width)
		return summary + "\n" + padMetaLine(tabStrip, width) + "\n" + body
	}

	if row.Kind == RowStandalone {
		// Standalone containers have no compose metadata: logs only.
		return renderInspectorLogs(m, width)
	}

	summary := formatProjectSummaryLine(row.Graph, m.spinFrame, width)
	tabStrip := renderMainTabStrip(m.mainTab, false)
	body := renderServiceTabBody(m, row, width)
	return summary + "\n" + padMetaLine(tabStrip, width) + "\n" + body
}

func renderServiceTabBody(m Model, row Row, width int) string {
	switch m.mainTab {
	case tabStats:
		return renderStatsTab(m, row, width)
	case tabDeps:
		return renderDepsTab(m, row, width)
	case tabHealth:
		return renderHealthTab(m, row, width)
	default:
		return renderInspectorLogs(m, width)
	}
}

func renderProjectTabBody(m Model, row Row, width int) string {
	switch m.mainTab {
	case tabTimeline:
		return renderTimelineTab(m, row, width)
	case tabGraph:
		return renderGraphTab(m, row, width)
	default:
		return renderDoctorTab(m, width)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
