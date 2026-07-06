package ui

import (
	"strings"

	"github.com/hyturing/compose-pulse/internal/dag"
)

func renderPreviewMeta(m Model) string {
	if m.cursor >= len(m.rows) || !isSelectable(m.rows[m.cursor]) {
		return styleDim.Render("Select a service")
	}
	row := m.rows[m.cursor]
	var sb strings.Builder

	name := rowLabel(row)
	if row.Kind == RowComposeNode {
		eff, waitingOn := dag.EffectiveState(row.Node, row.Graph)
		sb.WriteString(styleName.Render(name))
		sb.WriteString(" · ")
		sb.WriteString(formatEffectiveStateLabel(eff, waitingOn, row.Node))
		sb.WriteString("\n")
		sb.WriteString(styleDim.Render("Project: " + row.ProjectName))
		if len(waitingOn) > 0 {
			sb.WriteString("\n")
			sb.WriteString(styleDim.Render("Blocked by: " + strings.Join(waitingOn, ", ")))
		}
	} else {
		sb.WriteString(styleName.Render(name))
		sb.WriteString("\n")
		sb.WriteString(styleDim.Render("Image: " + row.Label))
		sb.WriteString("\n")
		sb.WriteString(styleDim.Render("(" + row.Standalone.State.String() + ")"))
	}
	return sb.String()
}

func (m Model) previewLogVisibleLines() int {
	_, _, panelH, compact := dashboardLayout(m.width, m.height)
	if compact {
		return 1
	}
	bodyLines := panelInnerHeight(panelH) - 1
	metaLines := strings.Count(renderPreviewMeta(m), "\n") + 1
	visible := bodyLines - metaLines - 1 // separator
	if visible < 1 {
		return 1
	}
	return visible
}

func renderPreview(m Model, width int) string {
	if m.actionMode != actionModeNone {
		return renderActionView(m, width)
	}

	meta := renderPreviewMeta(m)
	if meta == styleDim.Render("Select a service") {
		return padMetaLine(meta, width)
	}

	metaLines := strings.Split(meta, "\n")
	var sb strings.Builder
	for _, line := range metaLines {
		sb.WriteString(padMetaLine(line, width))
		sb.WriteString("\n")
	}
	sb.WriteString(padMetaLine(strings.Repeat("─", maxInt(8, width-scrollBarWidth-4)), width))
	sb.WriteString("\n")

	visible := m.previewLogVisibleLines()
	sourceLines := m.logs
	if m.logWaiting {
		sourceLines = strings.Split(buildWaitingContent(m), "\n")
		displayRows := buildLogDisplayRows(sourceLines, width-scrollBarWidth-logLinePrefixW)
		scroll, follow := m.previewLogScroll(displayRows, visible)
		sb.WriteString(renderLogViewport(logViewportConfig{
			sourceLines:  sourceLines,
			displayRows:  displayRows,
			scroll:       scroll,
			cursor:       m.logCursor,
			follow:       follow,
			waiting:      true,
			width:        width,
			visibleLines: visible,
		}))
		return sb.String()
	}

	if len(sourceLines) == 0 {
		sb.WriteString(padMetaLine(styleDim.Render("Waiting for logs…"), width))
		return sb.String()
	}

	displayRows := buildLogDisplayRows(sourceLines, width-scrollBarWidth-logLinePrefixW)
	scroll, follow := m.previewLogScroll(displayRows, visible)
	sb.WriteString(renderLogViewport(logViewportConfig{
		sourceLines:  sourceLines,
		displayRows:  displayRows,
		scroll:       scroll,
		cursor:       m.logCursor,
		follow:       follow,
		width:        width,
		visibleLines: visible,
	}))
	return sb.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
