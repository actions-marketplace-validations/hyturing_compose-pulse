package ui

import "github.com/mattn/go-runewidth"

// statusHintCore is always shown when the status bar has room; context
// suffixes are dropped first on narrow terminals.
const statusHintCore = "↑↓ move · tab/←→ · x · ? · esc back · q"

// statusHintContext returns a mode-specific suffix for the dashboard status bar.
func statusHintContext(m Model) string {
	if m.actionMode == actionModeMenu {
		return "↑↓ select · enter run"
	}
	if m.actionMode != actionModeNone {
		return ""
	}
	switch m.viewMode {
	case viewZoom:
		return "g follow · / n/N · l older"
	case viewHelp:
		return ""
	}
	if m.panelFocus == focusMain {
		return mainPanelStatusContext(m)
	}
	if m.selectionIsProject {
		return "1-3 doctor/timeline/graph · d/t · enter zoom"
	}
	return "1-4 logs/stats/deps/health · f filter · / · enter zoom"
}

func mainPanelStatusContext(m Model) string {
	if m.selectionIsProject {
		switch m.mainTab {
		case tabDoctor:
			return "↑↓ findings · enter jump"
		case tabTimeline:
			return "↑↓ select · enter jump"
		default:
			return "↑↓ scroll"
		}
	}
	switch m.mainTab {
	case tabLogs:
		return "g follow · / n/N · l · enter zoom"
	case tabHealth:
		return "enter run probe"
	default:
		return "↑↓ scroll · enter zoom"
	}
}

// formatStatusHints joins the core hints and context when both fit in width;
// otherwise returns core only (leading space matches the previous status-bar style).
func formatStatusHints(width int, context string) string {
	text := " " + statusHintCore
	if context == "" {
		return text
	}
	full := text + " · " + context
	if width < 1 || runewidth.StringWidth(full) <= width {
		return full
	}
	return text
}
