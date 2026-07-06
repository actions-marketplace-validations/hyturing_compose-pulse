package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const statusBarHeight = 1

func dashboardLayout(width, height int) (leftW, rightW, panelH int, compact bool) {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 24
	}
	panelH = height - statusBarHeight
	if width < 80 || height < 20 {
		return width, 0, panelH, true
	}
	leftW = width * 2 / 5
	rightW = width - leftW
	return leftW, rightW, panelH, false
}

func panelInnerHeight(panelH int) int {
	if panelH < 1 {
		return 1
	}
	return panelH
}

func renderPanel(title, content string, width, height int, focused, withScrollBar bool) string {
	if width < 2 {
		width = 2
	}
	if height < 2 {
		height = 2
	}
	style := stylePanel
	if focused {
		style = stylePanelFocus
	}
	innerW := width - 2
	innerH := height

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
