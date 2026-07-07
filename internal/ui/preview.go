package ui

func (m Model) previewLogVisibleLines() int {
	_, _, panelH, compact := dashboardLayout(m.width, m.height)
	if compact {
		return 1
	}
	bodyLines := panelInnerHeight(panelH) - 1 // minus tab strip line
	if bodyLines < 1 {
		return 1
	}
	return bodyLines
}

// renderPreview renders the right panel body: the action view when an action
// is in progress, otherwise the selected service's Overview/Logs/Deps tab.
func renderPreview(m Model, width int) string {
	if m.actionMode != actionModeNone {
		return renderActionView(m, width)
	}
	visible := m.visibleRows()
	if m.cursor >= len(visible) || !isSelectable(visible[m.cursor]) {
		if m.rowFilter != filterAll {
			return padMetaLine(styleDim.Render(emptyFilterMessage(m.rowFilter)), width)
		}
		return padMetaLine(styleDim.Render("Select a service"), width)
	}

	row := visible[m.cursor]
	hasDeps := row.Kind == RowComposeNode
	tabStrip := renderInspectorTabStrip(m.inspectorTab, hasDeps)

	var body string
	switch {
	case row.Kind == RowStandalone:
		if m.inspectorTab == inspectorTabLogs {
			body = renderInspectorLogs(m, width)
		} else {
			body = renderStandaloneOverview(row, width)
		}
	case m.inspectorTab == inspectorTabLogs:
		body = renderInspectorLogs(m, width)
	case m.inspectorTab == inspectorTabDeps:
		body = renderInspectorDeps(buildServiceInspector(row), width)
	default:
		body = renderInspectorOverview(m, buildServiceInspector(row), width)
	}

	return padMetaLine(tabStrip, width) + "\n" + body
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
