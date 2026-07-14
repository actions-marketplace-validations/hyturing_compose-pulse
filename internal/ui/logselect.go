package ui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
)

// logSelection is a source-line range within the current log buffer (after
// find query). Indices are into displayLogLines(), inclusive.
type logSelection struct {
	has      bool
	dragging bool
	start    int
	end      int
}

type logCopiedMsg struct {
	lines int
	err   error
}

func (s logSelection) normalized() (lo, hi int) {
	if s.start <= s.end {
		return s.start, s.end
	}
	return s.end, s.start
}

func (s logSelection) containsSource(line int) bool {
	if !s.has {
		return false
	}
	lo, hi := s.normalized()
	return line >= lo && line <= hi
}

func selectedLogText(lines []string, sel logSelection) string {
	if !sel.has || len(lines) == 0 {
		return ""
	}
	lo, hi := sel.normalized()
	if lo < 0 {
		lo = 0
	}
	if hi >= len(lines) {
		hi = len(lines) - 1
	}
	if lo > hi {
		return ""
	}
	return strings.Join(lines[lo:hi+1], "\n")
}

func (m *Model) clearLogSelection() bool {
	if !m.logSel.has && !m.logSel.dragging {
		return false
	}
	m.logSel = logSelection{}
	return true
}

func (m *Model) clampLogSelection() {
	if !m.logSel.has {
		return
	}
	n := len(m.displayLogLines())
	if n == 0 {
		m.logSel = logSelection{}
		return
	}
	if m.logSel.start < 0 {
		m.logSel.start = 0
	}
	if m.logSel.end < 0 {
		m.logSel.end = 0
	}
	if m.logSel.start >= n {
		m.logSel.start = n - 1
	}
	if m.logSel.end >= n {
		m.logSel.end = n - 1
	}
}

// logBarDrag tracks click-drag on the log scrollbar thumb/track.
type logBarDrag struct {
	active bool
	grab   int // pointer offset from thumb top while dragging
}

func (m Model) logsHaveViewport() bool {
	if m.actionMode != actionModeNone {
		return false
	}
	switch m.viewMode {
	case viewZoom:
		return true
	case viewDashboard:
		if m.selectionIsProject || m.mainTab != tabLogs {
			return false
		}
		_, _, _, compact := dashboardLayout(m.width, m.height)
		return !compact
	default:
		return false
	}
}

// logViewportRect is the screen rectangle covering log viewport rows
// (including prefix + scrollbar). Coordinates match tea.MouseMsg.
func (m Model) logViewportRect() (x, y, w, h int, ok bool) {
	if !m.logsHaveViewport() {
		return 0, 0, 0, 0, false
	}

	if m.viewMode == viewZoom {
		w = m.logViewportWidth()
		h = m.logVisibleLines()
		if w < 1 || h < 1 {
			return 0, 0, 0, 0, false
		}
		return 0, 1, w, h, true
	}

	leftW, mainW, _, compact := dashboardLayout(m.width, m.height)
	if compact || mainW < 3 {
		return 0, 0, 0, 0, false
	}

	visible := m.visibleRows()
	if m.cursor >= len(visible) || !isSelectable(visible[m.cursor]) {
		return 0, 0, 0, 0, false
	}
	row := visible[m.cursor]

	// summary (0) + top border (1) + title (2) → body starts at y=3
	y = summaryBarHeight + 1 + 1
	switch row.Kind {
	case RowComposeNode:
		y += 2 // project summary + tab strip
	case RowStandalone:
		// logs fill the body from the first content row
	default:
		return 0, 0, 0, 0, false
	}

	x = leftW + 1
	w = mainW - 2
	h = m.mainPanelVisibleLines()
	if w < 1 || h < 1 {
		return 0, 0, 0, 0, false
	}
	return x, y, w, h, true
}

// logScrollBarAt reports the relative bar row when (x,y) is on the scrollbar.
func (m Model) logScrollBarAt(x, y int) (relRow int, ok bool) {
	x0, y0, w, h, ok := m.logViewportRect()
	if !ok || w < 1 || h < 1 {
		return 0, false
	}
	if x != x0+w-1 || y < y0 || y >= y0+h {
		return 0, false
	}
	return y - y0, true
}

func (m Model) logSourceAt(x, y int) (sourceLine int, ok bool) {
	x0, y0, w, h, ok := m.logViewportRect()
	if !ok || x < x0 || x >= x0+w || y < y0 || y >= y0+h {
		return 0, false
	}
	displayRows := m.logDisplayRows()
	if len(displayRows) == 0 {
		return 0, false
	}
	scroll, _ := m.previewLogScroll(displayRows, h)
	idx := scroll + (y - y0)
	if idx < 0 || idx >= len(displayRows) {
		return 0, false
	}
	return displayRows[idx].sourceLine, true
}

func (m *Model) beginLogSelection(sourceLine int) {
	m.logFollow = false
	// Keep the find query (highlights stay), but leave the input so keys
	// like ^C / esc apply to the selection instead of the find box.
	m.logFindFocus = false
	m.logSel = logSelection{
		has:      true,
		dragging: true,
		start:    sourceLine,
		end:      sourceLine,
	}
}

func (m *Model) extendLogSelection(sourceLine int) {
	if !m.logSel.dragging {
		return
	}
	m.logSel.end = sourceLine
	m.clampLogSelection()
}

func (m Model) copyLogSelectionCmd() tea.Cmd {
	m.clampLogSelection()
	text := selectedLogText(m.displayLogLines(), m.logSel)
	if text == "" {
		return nil
	}
	lo, hi := m.logSel.normalized()
	lines := hi - lo + 1
	return func() tea.Msg {
		err := writeClipboard(text)
		return logCopiedMsg{lines: lines, err: err}
	}
}

// handleCtrlC copies the log selection when one exists; otherwise no-ops.
// Quit is only via q (use Control+C, not Cmd+C — terminals steal ⌘C).
func (m Model) handleCtrlC() (tea.Model, tea.Cmd) {
	if !m.logSel.has {
		return m, nil
	}
	return m, m.copyLogSelectionCmd()
}

func writeClipboard(text string) error {
	_, _ = fmt.Fprint(os.Stderr, osc52.New(text))
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	case "linux":
		if err := pipeToClipboard("wl-copy", text); err == nil {
			return nil
		}
		return pipeToClipboard("xclip", text, "-selection", "clipboard")
	default:
		return nil
	}
}

func pipeToClipboard(name, text string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (m *Model) setLogScroll(scroll int) {
	displayRows := m.logDisplayRows()
	visible := m.logPanelVisibleLines()
	maxScroll := m.logMaxScroll(visible)
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	m.logFollow = false
	m.logScroll = scroll
	if len(displayRows) == 0 {
		m.logCursor = 0
		return
	}
	if m.logCursor < m.logScroll {
		m.logCursor = m.logScroll
	}
	if m.logCursor >= m.logScroll+visible {
		m.logCursor = m.logScroll + visible - 1
	}
	if m.logCursor >= len(displayRows) {
		m.logCursor = len(displayRows) - 1
	}
	if m.logCursor < 0 {
		m.logCursor = 0
	}
}

func (m *Model) applyLogBarRow(relRow int) {
	displayRows := m.logDisplayRows()
	visible := m.logPanelVisibleLines()
	total := len(displayRows)
	thumbStart := relRow - m.logBarDrag.grab
	m.setLogScroll(scrollFromThumbStart(thumbStart, visible, total))
}

func (m *Model) beginLogBarDrag(relRow int) {
	displayRows := m.logDisplayRows()
	visible := m.logPanelVisibleLines()
	total := len(displayRows)
	scroll, _ := m.previewLogScroll(displayRows, visible)
	thumbStart, thumbEnd := scrollBarThumbRange(visible, scroll, total)
	grab := 0
	if relRow >= thumbStart && relRow < thumbEnd {
		grab = relRow - thumbStart
	} else {
		// Track click: jump so the thumb centers on the click when possible.
		thumbSize := scrollBarThumbSize(visible, total)
		grab = thumbSize / 2
	}
	m.logBarDrag = logBarDrag{active: true, grab: grab}
	m.clearLogSelection()
	m.applyLogBarRow(relRow)
}

func (m *Model) endLogBarDrag() {
	m.logBarDrag = logBarDrag{}
}

func (m *Model) handleLogMouse(msg tea.MouseMsg) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.endLogBarDrag()
		m.scrollLogLine(-1)
		return
	case tea.MouseButtonWheelDown:
		m.endLogBarDrag()
		m.scrollLogLine(1)
		return
	}

	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			return
		}
		if rel, ok := m.logScrollBarAt(msg.X, msg.Y); ok {
			m.beginLogBarDrag(rel)
			return
		}
		m.endLogBarDrag()
		if src, ok := m.logSourceAt(msg.X, msg.Y); ok {
			m.beginLogSelection(src)
			return
		}
		m.clearLogSelection()
		return

	case tea.MouseActionMotion:
		if m.logBarDrag.active {
			if rel, ok := m.logScrollBarAt(msg.X, msg.Y); ok {
				m.applyLogBarRow(rel)
				return
			}
			// Allow dragging slightly off the bar column while keeping Y tracking.
			if _, y0, _, h, ok := m.logViewportRect(); ok {
				rel := msg.Y - y0
				if rel < 0 {
					rel = 0
				}
				if rel >= h {
					rel = h - 1
				}
				m.applyLogBarRow(rel)
			}
			return
		}
		if !m.logSel.dragging {
			return
		}
		if src, ok := m.logSourceAt(msg.X, msg.Y); ok {
			m.extendLogSelection(src)
		}
		return

	case tea.MouseActionRelease:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonNone {
			return
		}
		if m.logBarDrag.active {
			if rel, ok := m.logScrollBarAt(msg.X, msg.Y); ok {
				m.applyLogBarRow(rel)
			}
			m.endLogBarDrag()
			return
		}
		if !m.logSel.dragging {
			return
		}
		if src, ok := m.logSourceAt(msg.X, msg.Y); ok {
			m.logSel.end = src
		}
		m.logSel.dragging = false
		m.clampLogSelection()
	}
}
