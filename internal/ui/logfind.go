package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const (
	logFindPlaceholder = "type to find…"
	logFindInputWidth  = 16 // rune cells inside the box
)

func (m Model) logFindMatches() []int {
	if m.logWaiting || m.logFind == "" {
		return nil
	}
	return matchingLineIndexes(m.logs, m.logFind)
}

func (m Model) logFindCounter() string {
	matches := m.logFindMatches()
	if m.logFind == "" {
		return ""
	}
	if len(matches) == 0 {
		return "0/0"
	}
	idx := m.logFindIdx
	if idx < 0 || idx >= len(matches) {
		idx = 0
	}
	return fmt.Sprintf("%d/%d", idx+1, len(matches))
}

func (m Model) currentLogFindSource() (source int, ok bool) {
	matches := m.logFindMatches()
	if len(matches) == 0 {
		return 0, false
	}
	idx := m.logFindIdx
	if idx < 0 || idx >= len(matches) {
		idx = 0
	}
	return matches[idx], true
}

// setLogFind updates the query and jumps to the first match.
func (m *Model) setLogFind(query string) {
	m.logFind = query
	m.logFindIdx = 0
	m.logFollow = false
	m.jumpToLogFind()
}

func (m *Model) jumpToLogFind() {
	matches := m.logFindMatches()
	if len(matches) == 0 {
		m.logFindIdx = 0
		return
	}
	if m.logFindIdx < 0 {
		m.logFindIdx = 0
	}
	if m.logFindIdx >= len(matches) {
		m.logFindIdx = len(matches) - 1
	}
	target := matches[m.logFindIdx]
	displayRows := m.logDisplayRows()
	for i, row := range displayRows {
		if row.sourceLine == target && row.lineStart {
			m.logCursor = i
			break
		}
	}
	m.clampLogCursor()
}

func (m *Model) logFindStep(delta int) {
	matches := m.logFindMatches()
	if len(matches) == 0 {
		return
	}
	m.logFollow = false
	m.logFindIdx = (m.logFindIdx + delta) % len(matches)
	if m.logFindIdx < 0 {
		m.logFindIdx += len(matches)
	}
	m.jumpToLogFind()
}

func (m Model) showLogFindBar() bool {
	if m.logWaiting || m.actionMode != actionModeNone {
		return false
	}
	if m.viewMode == viewZoom {
		return true
	}
	if m.viewMode != viewDashboard {
		return false
	}
	if m.selectionIsProject {
		return false
	}
	visible := m.visibleRows()
	if m.cursor < len(visible) && visible[m.cursor].Kind == RowStandalone {
		return true
	}
	return m.mainTab == tabLogs
}

// syncLogFindFocus drops find focus when the find box is not on screen.
// Focusing the box is explicit: click it, or press ctrl+f.
func (m *Model) syncLogFindFocus() {
	if !m.showLogFindBar() {
		m.logFindFocus = false
	}
}

// formatLogFindBox is a dedicated always-visible input: [ query█     ] 4/36
func formatLogFindBox(m Model) string {
	var text string
	switch {
	case m.logFind == "" && !m.logFindFocus:
		text = logFindPlaceholder
	case m.logFindFocus:
		text = m.logFind + "█"
	default:
		text = m.logFind
	}
	plain := stripANSI(text)
	if runewidth.StringWidth(plain) > logFindInputWidth {
		if m.logFindFocus {
			text = runewidth.Truncate(m.logFind, logFindInputWidth-1, "…") + "█"
		} else {
			text = runewidth.Truncate(plain, logFindInputWidth, "…")
		}
		plain = stripANSI(text)
	}
	pad := logFindInputWidth - runewidth.StringWidth(plain)
	if pad < 0 {
		pad = 0
	}
	inner := text + strings.Repeat(" ", pad)
	var boxed string
	if m.logFindFocus {
		boxed = styleLogFindBoxFocus.Render("[" + inner + "]")
	} else {
		boxed = styleLogFindBox.Render("[" + inner + "]")
	}
	counter := m.logFindCounter()
	if counter == "" {
		return boxed
	}
	return boxed + " " + styleDim.Render(counter)
}

// logFindBoxWidth is the on-screen width of the find box (+ counter).
func logFindBoxWidth(m Model) int {
	return lipgloss.Width(formatLogFindBox(m))
}

// renderStatusWithFind puts hints on the left and the find box on the right.
func renderStatusWithFind(m Model, width int) string {
	if width < 1 {
		width = 80
	}
	left := formatStatusHints(width, statusHintContext(m))
	if !m.showLogFindBar() {
		return renderStatusBar(width, left)
	}
	box := formatLogFindBox(m)
	boxW := lipgloss.Width(box)
	leftBudget := width - boxW - 1
	if leftBudget < 10 {
		return styleStatusBar.Width(width).Render(truncateToVisibleWidth(stripANSI(box), width))
	}
	left = formatStatusHints(leftBudget, statusHintContext(m))
	left = truncateToWidth(left, leftBudget)
	pad := leftBudget - lipgloss.Width(left)
	if pad < 0 {
		pad = 0
	}
	line := left + strings.Repeat(" ", pad) + " " + box
	return styleStatusBar.Width(width).Render(line)
}

// renderZoomFooter is the zoom view bottom row: hints left, find box right.
func renderZoomFooter(m Model, width int) string {
	hint := " q back · ↑↓/wheel · drag · ^C · g · l"
	switch {
	case m.copyNotice != "":
		hint = " " + m.copyNotice
	case m.logSel.has:
		hint = " ^C copy · esc clear sel · q back"
	case m.logFindFocus:
		hint = " enter next · shift+enter prev · esc clears query"
	}
	if width < 1 {
		width = 80
	}
	box := formatLogFindBox(m)
	boxW := lipgloss.Width(box)
	leftBudget := width - boxW - 1
	if leftBudget < 8 {
		return styleLogFooter.Width(width).Render(box)
	}
	left := truncateToWidth(hint, leftBudget)
	pad := leftBudget - lipgloss.Width(left)
	if pad < 0 {
		pad = 0
	}
	return styleLogFooter.Width(width).Render(left + strings.Repeat(" ", pad) + " " + box)
}

// statusFindHit reports whether (x,y) is on the dashboard status-bar find box.
func (m Model) statusFindHit(x, y int) bool {
	if m.viewMode != viewDashboard || !m.showLogFindBar() {
		return false
	}
	if y != m.height-1 {
		return false
	}
	boxW := logFindBoxWidth(m)
	return x >= m.width-boxW
}

// zoomFindHit reports whether (x,y) is on the zoom footer find box.
func (m Model) zoomFindHit(x, y int) bool {
	if m.viewMode != viewZoom || !m.showLogFindBar() {
		return false
	}
	if y != m.height-1 {
		return false
	}
	boxW := logFindBoxWidth(m)
	return x >= m.width-boxW
}

func isShiftEnter(msg interface{ String() string }) bool {
	return msg.String() == "shift+enter"
}
