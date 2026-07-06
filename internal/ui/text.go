package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func truncateToWidth(s string, width int) string {
	if width < 1 {
		return s
	}
	plain := stripANSI(s)
	if runewidth.StringWidth(plain) <= width {
		return s
	}
	return runewidth.Truncate(plain, width, "…")
}

func truncateToVisibleWidth(s string, width int) string {
	if width < 1 {
		return s
	}
	plain := stripANSI(s)
	if runewidth.StringWidth(plain) <= width {
		return s
	}
	truncated := runewidth.Truncate(plain, width, "…")
	return truncated
}

func padLine(s string, width int) string {
	pad := width - lipgloss.Width(s)
	if pad <= 0 {
		return truncateToVisibleWidth(s, width)
	}
	return s + strings.Repeat(" ", pad)
}

// ensurePanelLine fits a pre-rendered panel body line to innerW without clipping
// the scrollbar column on the right edge.
func ensurePanelLine(s string, width int) string {
	if width < 1 {
		return s
	}
	w := lipgloss.Width(s)
	if w <= width {
		if w < width {
			return s + strings.Repeat(" ", width-w)
		}
		return s
	}
	if width <= scrollBarWidth {
		return truncateToVisibleWidth(s, width)
	}
	contentW := width - scrollBarWidth
	plain := stripANSI(s)
	content := runewidth.Truncate(plain, contentW, "…")
	pad := contentW - lipgloss.Width(content)
	bar := styledScrollBar(false)
	if lineHasScrollBarSuffix(s) && strings.HasSuffix(plain, scrollBarThumbGlyph) {
		bar = styledScrollBar(true)
	}
	return content + strings.Repeat(" ", pad) + bar
}
