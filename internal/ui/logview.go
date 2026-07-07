package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/docker"
)

const (
	logTailLines   = 200
	logMaxLines    = 2000
	logLoadChunk   = 500
	logLinePrefixW = 2 // "▸ " or "  "
)

func waitForLogLine(ch <-chan docker.LogLineMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return logStreamDoneMsg{}
		}
		return logLineMsg{line: msg}
	}
}

func fetchMoreLogsCmd(dc *docker.Client, containerID string, currentLen int) tea.Cmd {
	return func() tea.Msg {
		lines, err := dc.FetchLogLines(context.Background(), containerID, currentLen+logLoadChunk)
		return logMoreMsg{lines: lines, prevLen: currentLen, err: err}
	}
}

func prependOlderLogs(current, fetched []string) (merged []string, added int, noMore bool) {
	if len(fetched) <= len(current) {
		return current, 0, true
	}
	older := fetched[:len(fetched)-len(current)]
	merged = append(append([]string{}, older...), current...)
	return merged, len(older), false
}

func filterLines(lines []string, pattern string) []string {
	if pattern == "" {
		return lines
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		var out []string
		for _, l := range lines {
			if strings.Contains(l, pattern) {
				out = append(out, l)
			}
		}
		return out
	}
	var out []string
	for _, l := range lines {
		if re.MatchString(l) {
			out = append(out, l)
		}
	}
	return out
}

func buildWaitingContent(m Model) string {
	idx := findRowByKey(m.rows, m.selectedRowKey)
	if idx < 0 || m.rows[idx].Kind != RowComposeNode {
		return "Waiting for container to start…"
	}
	row := m.rows[idx]
	_, waitingOn := dag.EffectiveState(row.Node, row.Graph)

	var sb strings.Builder
	sb.WriteString("Waiting for container to start…")
	if len(waitingOn) > 0 {
		sb.WriteString("\n\nBlocked by:")
		for _, w := range waitingOn {
			cond := row.Node.DepConditions[w]
			if cond == "" {
				cond = "service_started"
			}
			sb.WriteString("\n  • ")
			sb.WriteString(w)
			sb.WriteString(" (")
			sb.WriteString(cond)
			sb.WriteString(")")
		}
	} else {
		sb.WriteString("\n\nDependencies satisfied — waiting for Docker to create the container.")
	}
	return sb.String()
}

func (m Model) logContentHeight() int {
	h := m.height - 2
	if m.searching || (!m.logWaiting && m.logFilter != "") {
		h--
	}
	if h < 1 {
		return 1
	}
	return h
}

func (m Model) logVisibleLines() int {
	return m.logContentHeight()
}

func (m Model) logViewportWidth() int {
	if m.viewMode == viewLogFullscreen {
		w := m.width
		if w < 1 {
			w = 80
		}
		return w
	}
	_, rightW, _, compact := dashboardLayout(m.width, m.height)
	if compact {
		w := m.width
		if w < 1 {
			w = 80
		}
		return w
	}
	return rightW - 2
}

func (m Model) logDisplayRows() []logDisplayRow {
	wrapW := m.logViewportWidth() - scrollBarWidth - logLinePrefixW
	if wrapW < 1 {
		wrapW = 1
	}
	return buildLogDisplayRows(m.displayLogLines(), wrapW)
}

func (m Model) logMaxScroll(visibleLines int) int {
	rows := m.logDisplayRows()
	maxScroll := len(rows) - visibleLines
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (m Model) displayLogLines() []string {
	if m.logWaiting {
		return strings.Split(buildWaitingContent(m), "\n")
	}
	return filterLines(m.logs, m.logFilter)
}

func logTitleSuffix(m Model) string {
	if m.logWaiting {
		return ""
	}
	sourceLines := m.displayLogLines()
	total := len(sourceLines)
	if total == 0 {
		return ""
	}
	if m.logFollow {
		return fmt.Sprintf(" · %d lines · following", total)
	}
	displayRows := m.logDisplayRows()
	cur := 1
	if m.logCursor >= 0 && m.logCursor < len(displayRows) {
		cur = displayRows[m.logCursor].sourceLine + 1
	}
	suffix := fmt.Sprintf(" · line %d/%d · paused", cur, total)
	if m.logScroll == 0 && !m.logNoMoreHistory && !m.logLoading {
		suffix += " · l load more"
	}
	if m.logLoading {
		suffix += " · loading…"
	}
	return suffix
}

func (m *Model) clampLogCursor() {
	displayRows := m.logDisplayRows()
	if len(displayRows) == 0 {
		m.logCursor = 0
		m.logScroll = 0
		return
	}
	if m.logCursor >= len(displayRows) {
		m.logCursor = len(displayRows) - 1
	}
	if m.logCursor < 0 {
		m.logCursor = 0
	}
	visible := m.logVisibleLines()
	if m.viewMode == viewDashboard {
		visible = m.previewLogVisibleLines()
	}
	if m.logCursor < m.logScroll {
		m.logScroll = m.logCursor
	}
	if m.logCursor >= m.logScroll+visible {
		m.logScroll = m.logCursor - visible + 1
	}
	maxScroll := m.logMaxScroll(visible)
	if m.logScroll > maxScroll {
		m.logScroll = maxScroll
	}
}

func (m *Model) scrollToBottom() {
	displayRows := m.logDisplayRows()
	if len(displayRows) == 0 {
		m.logCursor = 0
		m.logScroll = 0
		return
	}
	visible := m.logVisibleLines()
	if m.viewMode == viewDashboard {
		visible = m.previewLogVisibleLines()
	}
	m.logCursor = len(displayRows) - 1
	m.logScroll = m.logMaxScroll(visible)
}

func (m *Model) scrollPage(delta int) {
	m.logFollow = false
	visible := m.logVisibleLines()
	if m.viewMode == viewDashboard {
		visible = m.previewLogVisibleLines()
	}
	m.logCursor += delta * visible
	m.clampLogCursor()
}

func (m *Model) scrollLogLine(delta int) {
	displayRows := m.logDisplayRows()
	if len(displayRows) == 0 {
		return
	}
	if delta > 0 {
		if m.logFollow || m.logCursor >= len(displayRows)-1 {
			m.logFollow = true
			m.scrollToBottom()
			return
		}
		m.logFollow = false
		m.logCursor++
	} else {
		m.logFollow = false
		if m.logCursor > 0 {
			m.logCursor--
		}
	}
	m.clampLogCursor()
}

func (m *Model) scrollHome() tea.Cmd {
	m.logFollow = false
	atTop := m.logScroll == 0 && m.logCursor == 0
	m.logCursor = 0
	m.logScroll = 0
	if atTop && !m.logNoMoreHistory && !m.logLoading && m.logContainerID != "" {
		m.logLoading = true
		return fetchMoreLogsCmd(m.docker, m.logContainerID, len(m.logs))
	}
	return nil
}

func (m *Model) applyLogMore(msg logMoreMsg) {
	m.logLoading = false
	if msg.err != nil {
		m.appendLogLine(fmt.Sprintf("load more error: %v", msg.err))
		return
	}
	merged, added, noMore := prependOlderLogs(m.logs, msg.lines)
	if noMore {
		m.logNoMoreHistory = true
		return
	}
	wrapW := m.logViewportWidth() - scrollBarWidth - logLinePrefixW
	before := buildLogDisplayRows(filterLines(m.logs, m.logFilter), wrapW)
	m.logs = merged
	if len(m.logs) > logMaxLines {
		m.logs = m.logs[len(m.logs)-logMaxLines:]
	}
	after := buildLogDisplayRows(filterLines(m.logs, m.logFilter), wrapW)
	addedDisplay := len(after) - len(before)
	_ = added
	m.logCursor += addedDisplay
	m.logScroll += addedDisplay
	m.clampLogCursor()
}

func (m *Model) appendLogLine(line string) {
	m.logs = append(m.logs, normalizeLogLine(line))
	if len(m.logs) > logMaxLines {
		m.logs = m.logs[len(m.logs)-logMaxLines:]
		displayRows := m.logDisplayRows()
		if !m.logFollow && m.logCursor >= len(displayRows) {
			m.logCursor = len(displayRows) - 1
		}
	}
}

func (m Model) previewLogScroll(displayRows []logDisplayRow, visible int) (scroll int, follow bool) {
	follow = m.logFollow
	if m.viewMode == viewDashboard && m.panelFocus == focusGraph {
		follow = true
	}
	if follow {
		return tailScroll(len(displayRows), visible), true
	}
	return m.logScroll, false
}

func renderLogFullscreen(m Model) string {
	width := m.logViewportWidth()
	if width < 1 {
		width = 80
	}

	sourceLines := m.displayLogLines()
	visibleCount := m.logVisibleLines()
	displayRows := m.logDisplayRows()

	scroll, follow := m.previewLogScroll(displayRows, visibleCount)
	if !follow && scroll > m.logMaxScroll(visibleCount) {
		scroll = m.logMaxScroll(visibleCount)
	}

	content := renderLogViewport(logViewportConfig{
		sourceLines:  sourceLines,
		displayRows:  displayRows,
		scroll:       scroll,
		cursor:       m.logCursor,
		follow:       follow,
		waiting:      m.logWaiting,
		width:        width,
		visibleLines: visibleCount,
	})
	if m.logWaiting {
		content = lipgloss.NewStyle().Italic(true).Render(content)
	}

	header := styleLogHeader.Width(width).Render(
		" cpulse · " + m.selectedSvc + logTitleSuffix(m),
	)

	var searchBar string
	if !m.logWaiting {
		if m.searching {
			searchBar = styleLogFooter.Width(width).Render(
				" " + styleSearchPrompt.Render("/") + styleSearchInput.Render(m.logFilter+"█"),
			)
		} else if m.logFilter != "" {
			searchBar = styleLogFooter.Width(width).Render(
				" " + styleSearchPrompt.Render("filter: ") + styleSearchInput.Render(m.logFilter),
			)
		}
	}

	footer := styleLogFooter.Width(width).Render(
		" q back · ↑↓/wheel scroll · g follow · / filter · l load more",
	)

	if searchBar != "" {
		return header + "\n" + content + "\n" + searchBar + "\n" + footer
	}
	return header + "\n" + content + "\n" + footer
}
