package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const statusBarHeight = 1
const summaryBarHeight = 1

// panelBorderHeight is the number of rows lipgloss's NormalBorder adds on
// top of a panel's content (one row above, one below) when renderPanel
// wraps it. Every place that reasons about how many rows a rendered panel
// occupies, or how many content rows fit inside it, must account for this.
const panelBorderHeight = 2

// dashboardLayout splits the terminal into the left dependency-graph panel
// and the main inspector panel (TUI-DESIGN.md §2). Below 80 cols or 20 rows
// the main panel is dropped and the left panel takes the full width.
//
// The left panel is intentionally roomy enough for glyph + name + state +
// a short hint (and CPU/MEM) without chopping words mid-label.
func dashboardLayout(width, height int) (leftW, mainW, panelH int, compact bool) {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 24
	}
	panelH = height - statusBarHeight - summaryBarHeight
	if width < 80 || height < 20 {
		return width, 0, panelH, true
	}
	// Left panel: wide enough for columns, but not so wide DETAIL looks empty.
	leftW = (width * 9) / 20 // 45%
	const minLeft = 58
	const maxLeft = 70
	const minMain = 42
	if leftW < minLeft {
		leftW = minLeft
	}
	if leftW > maxLeft {
		leftW = maxLeft
	}
	if width-leftW < minMain {
		leftW = width - minMain
	}
	if leftW < 40 {
		leftW = width / 2
	}
	mainW = width - leftW
	return leftW, mainW, panelH, false
}

// panelInnerHeight returns how many content rows (title + body) fit inside
// a panel whose rendered block, including its border, is panelH rows tall.
func panelInnerHeight(panelH int) int {
	innerH := panelH - panelBorderHeight
	if innerH < 1 {
		return 1
	}
	return innerH
}

// renderPanel renders a bordered panel whose total rendered height
// (border included) is exactly height rows.
func renderPanel(title, content string, width, height int, focused, withScrollBar bool) string {
	if width < 2 {
		width = 2
	}
	style := stylePanel
	if focused {
		style = stylePanelFocus
	}
	innerW := width - 2
	innerH := panelInnerHeight(height)

	lines := make([]string, 0, innerH)
	bodyLines := strings.Split(content, "\n")
	maxBody := innerH - 1

	if withScrollBar {
		lines = append(lines, padMetaLine(stylePanelTitle.Render(truncateToWidth(title, innerW-scrollBarWidth)), innerW))
	} else {
		lines = append(lines, stylePanelTitle.Render(truncateToWidth(title, innerW)))
	}

	for i := 0; i < maxBody; i++ {
		switch {
		case i < len(bodyLines):
			lines = append(lines, ensurePanelLine(bodyLines[i], innerW))
		case withScrollBar:
			lines = append(lines, padMetaLine("", innerW))
		default:
			lines = append(lines, strings.Repeat(" ", innerW))
		}
	}
	for len(lines) < innerH {
		if withScrollBar {
			lines = append(lines, padMetaLine("", innerW))
		} else {
			lines = append(lines, strings.Repeat(" ", innerW))
		}
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}

	return style.Width(innerW).Render(strings.Join(lines, "\n"))
}

func renderStatusBar(width int, text string) string {
	if width < 1 {
		width = 80
	}
	return styleStatusBar.Width(width).Render(truncateToWidth(text, width))
}

func joinPanelsHorizontal(left, right string, totalHeight int) string {
	_ = totalHeight // kept for call-site clarity; JoinHorizontal aligns heights.
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func panelRenderedHeight(rendered string) int {
	if rendered == "" {
		return 0
	}
	return strings.Count(rendered, "\n") + 1
}
