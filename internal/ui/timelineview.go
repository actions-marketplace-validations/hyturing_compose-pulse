package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/hyturing/compose-pulse/internal/dag"
	tlpkg "github.com/hyturing/compose-pulse/internal/timeline"
)

// tickIntervals is the candidate set of axis tick spacings, chosen so 5-8
// ticks fit the bar width.
var tickIntervals = []time.Duration{
	1 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second,
	30 * time.Second, 60 * time.Second, 120 * time.Second, 300 * time.Second,
}

func pickTickInterval(elapsed time.Duration) time.Duration {
	for _, iv := range tickIntervals {
		if elapsed <= 0 {
			return iv
		}
		if float64(elapsed)/float64(iv) <= 8 {
			return iv
		}
	}
	return tickIntervals[len(tickIntervals)-1]
}

func round(f float64) int {
	return int(math.Round(f))
}

func timelineSegmentStyle(state dag.DisplayState) lipgloss.Style {
	switch state {
	case dag.DisplayBlocked:
		return styleTLSegBlocked
	case dag.DisplayPending:
		return styleTLSegPending
	case dag.DisplayMissing:
		return styleTLSegFailed
	case dag.DisplayStarting:
		return styleTLSegStarting
	case dag.DisplayUnhealthy, dag.DisplayDegraded:
		return styleTLSegUnhealthy
	case dag.DisplayFailed:
		return styleTLSegFailed
	case dag.DisplayHealthy, dag.DisplayCompleted:
		return styleTLSegHealthy
	default:
		return styleTLSegPending
	}
}

func timelineLegendStyle(state dag.DisplayState) lipgloss.Style {
	switch state {
	case dag.DisplayBlocked:
		return styleTLLegendBlocked
	case dag.DisplayPending:
		return styleTLLegendPending
	case dag.DisplayMissing, dag.DisplayFailed:
		return styleTLLegendFailed
	case dag.DisplayStarting:
		return styleTLLegendStarting
	case dag.DisplayHealthy, dag.DisplayCompleted:
		return styleTLLegendHealthy
	default:
		return styleTLLegendPending
	}
}

// timelineBarGlyph is a mid-cell horizontal bar (same visual weight as a
// half-block) so ◆/▶/×/◀ sit on the same vertical center as the track.
const timelineBarGlyph = "▬"

func renderSegmentFill(width int, state dag.DisplayState) string {
	if width <= 0 {
		return ""
	}
	return timelineSegmentStyle(state).Render(strings.Repeat(timelineBarGlyph, width))
}

// timelineStatusMarker returns the compact status glyph + plain state word.
func timelineStatusMarker(state dag.DisplayState) (marker, word string) {
	word = string(state)
	switch state {
	case dag.DisplayHealthy:
		return "●", word
	case dag.DisplayStarting:
		return "◐", word
	case dag.DisplayBlocked:
		return "◌", word
	case dag.DisplayPending:
		return "○", word
	case dag.DisplayMissing:
		return "○", word
	case dag.DisplayFailed, dag.DisplayUnhealthy:
		return "×", word
	case dag.DisplayCompleted:
		return "◆", word
	case dag.DisplayDegraded:
		return "↻", word
	default:
		return "?", word
	}
}

func styleTimelineStatus(state dag.DisplayState, marker string) string {
	switch state {
	case dag.DisplayHealthy, dag.DisplayCompleted:
		return styleStateHealthy.Render(marker)
	case dag.DisplayStarting:
		return styleStateStarting.Render(marker)
	case dag.DisplayBlocked:
		return styleStateBlocked.Render(marker)
	case dag.DisplayFailed, dag.DisplayUnhealthy, dag.DisplayMissing:
		return styleStateFailed.Render(marker)
	case dag.DisplayPending, dag.DisplayDegraded:
		return styleStatePending.Render(marker)
	default:
		return styleDim.Render(marker)
	}
}

func timelineEndMarker(final dag.DisplayState) (string, lipgloss.Style) {
	switch final {
	case dag.DisplayCompleted, dag.DisplayHealthy:
		return "◆", styleStateHealthy
	case dag.DisplayFailed, dag.DisplayUnhealthy, dag.DisplayMissing:
		return "×", styleStateFailed
	case dag.DisplayBlocked, dag.DisplayPending:
		return "▶", styleDim
	case dag.DisplayStarting, dag.DisplayDegraded:
		return "▶", styleStateStarting
	default:
		return "▶", styleDim
	}
}

const timelineStatusColW = 12 // "● starting " etc.

// timelineTreeSpan is a tracker span ordered/indented like the compose pstree.
type timelineTreeSpan struct {
	tlpkg.Span
	linePrefix string
}

// timelineTreeSpans maps flat tracker spans onto the project dependency tree
// order (same walk as the Graph tab). Services not yet observed are skipped.
func timelineTreeSpans(project Row, spans []tlpkg.Span) []timelineTreeSpan {
	byName := make(map[string]tlpkg.Span, len(spans))
	for _, s := range spans {
		byName[s.Service] = s
	}
	var out []timelineTreeSpan
	for _, r := range graphTabRows(project) {
		if r.Node == nil {
			continue
		}
		span, ok := byName[r.Node.Name]
		if !ok {
			continue
		}
		out = append(out, timelineTreeSpan{Span: span, linePrefix: r.linePrefix})
	}
	return out
}

func renderTimelineLegend(leftW int) string {
	item := func(state dag.DisplayState, label string) string {
		// Width() keeps trailing spaces; background swatches share the text baseline.
		return timelineLegendStyle(state).Width(2).Render(" ") + " " + styleDim.Render(label)
	}
	parts := []string{
		item(dag.DisplayBlocked, "blocked"),
		item(dag.DisplayPending, "pending"),
		item(dag.DisplayMissing, "missing"),
		item(dag.DisplayStarting, "starting"),
		item(dag.DisplayHealthy, "healthy"),
		item(dag.DisplayFailed, "failed"),
	}
	return strings.Repeat(" ", leftW) + strings.Join(parts, "  ")
}

// renderTimelineBars is the pure, testable startup-trace waterfall renderer.
// selectedIdx marks which service row expands (-1 = none).
func renderTimelineBars(rows []timelineTreeSpan, elapsed time.Duration, nameColW, barW, selectedIdx int) []string {
	if barW < 1 {
		barW = 1
	}
	if nameColW < 1 {
		nameColW = 1
	}
	elapsedSecs := elapsed.Seconds()
	if elapsedSecs < 1 {
		elapsedSecs = 1
	}
	pxPerSec := float64(barW) / elapsedSecs

	leftW := 2 + nameColW + timelineStatusColW // cursor + name + status
	var out []string
	out = append(out, renderAxisLabels(elapsedSecs, leftW, barW, pxPerSec))
	out = append(out, renderAxisTicks(elapsedSecs, leftW, barW, pxPerSec))
	out = append(out, renderNowCursorLine(leftW, barW))

	for i, row := range rows {
		selected := i == selectedIdx
		out = append(out, renderTimelineRow(row, elapsed, nameColW, barW, pxPerSec, selected)...)
	}
	out = append(out, "", renderTimelineLegend(leftW))
	return out
}

func renderAxisLabels(elapsedSecs float64, leftW, barW int, pxPerSec float64) string {
	interval := pickTickInterval(time.Duration(elapsedSecs * float64(time.Second)))
	buf := []rune(strings.Repeat(" ", leftW+barW))
	for t := 0.0; t <= elapsedSecs+0.01; t += interval.Seconds() {
		col := leftW + round(t*pxPerSec)
		label := fmt.Sprintf("%ds", int(t))
		if t >= 60 {
			label = fmt.Sprintf("%dm%ds", int(t)/60, int(t)%60)
		}
		placeLabel(buf, col, label)
	}
	return styleAxis.Render(string(buf))
}

func renderAxisTicks(elapsedSecs float64, leftW, barW int, pxPerSec float64) string {
	interval := pickTickInterval(time.Duration(elapsedSecs * float64(time.Second)))
	// Continuous horizontal axis under the labels; ticks sit on the line.
	buf := []rune(strings.Repeat(" ", leftW) + strings.Repeat("─", barW))
	for t := 0.0; t <= elapsedSecs+0.01; t += interval.Seconds() {
		col := leftW + round(t*pxPerSec)
		if col >= leftW && col < leftW+barW {
			buf[col] = '┬'
		}
	}
	// Live edge marker at the right of the track.
	nowCol := leftW + barW - 1
	if nowCol >= 0 && nowCol < len(buf) {
		buf[nowCol] = '┤'
	}
	return styleAxis.Render(string(buf))
}

func renderNowCursorLine(leftW, barW int) string {
	pad := strings.Repeat(" ", leftW+barW-1)
	return pad + styleTLNow.Render("│") + " " + styleTLNow.Render("now")
}

func placeLabel(buf []rune, col int, label string) {
	for i, r := range label {
		idx := col + i
		if idx < 0 || idx >= len(buf) {
			continue
		}
		buf[idx] = r
	}
}

func renderTimelineRow(row timelineTreeSpan, elapsed time.Duration, nameColW, barW int, pxPerSec float64, selected bool) []string {
	cursor := "  "
	if selected {
		cursor = styleAccentCursor("▸ ")
	}

	prefix := row.linePrefix
	nameBudget := nameColW - runewidth.StringWidth(prefix)
	if nameBudget < 1 {
		nameBudget = 1
	}
	name := runewidth.Truncate(row.Service, nameBudget-1, "")
	name = prefix + name
	name += strings.Repeat(" ", nameColW-runewidth.StringWidth(name))
	if selected {
		name = styleName.Render(name)
	}

	marker, word := timelineStatusMarker(row.Final)
	status := styleTimelineStatus(row.Final, marker) + " " + styleDim.Render(word)
	statusPlainW := runewidth.StringWidth(stripANSI(status))
	if statusPlainW < timelineStatusColW {
		status += strings.Repeat(" ", timelineStatusColW-statusPlainW)
	}

	bar := renderTimelineBar(row.Span, elapsed, barW, pxPerSec)
	main := cursor + name + status + bar

	lines := []string{main}
	if !selected {
		return lines
	}

	// Segment duration labels.
	var labels []string
	for _, seg := range row.Segments {
		if seg.Dur <= 0 {
			continue
		}
		labels = append(labels, fmt.Sprintf("%s %s", string(seg.State), formatDuration(seg.Dur)))
	}
	indent := 2 + nameColW
	if len(labels) > 0 {
		lines = append(lines, strings.Repeat(" ", indent)+styleDim.Render(strings.Join(labels, "   ")))
	}
	if row.WaitsOn != "" {
		lines = append(lines, strings.Repeat(" ", indent)+styleDetailNeeds.Render("↳ waiting for "+row.WaitsOn))
	} else if row.AttachedLate {
		lines = append(lines, strings.Repeat(" ", indent)+styleDim.Render("◀ already ready when cpulse attached"))
	}
	return lines
}

func styleAccentCursor(s string) string {
	return lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render(s)
}

func renderTimelineBar(span tlpkg.Span, elapsed time.Duration, barW int, pxPerSec float64) string {
	if barW < 1 {
		return ""
	}
	// Reserve one cell for the end marker (◆/▶/×).
	trackW := barW - 1
	if trackW < 1 {
		trackW = 1
	}

	offsetSecs := elapsed.Seconds() - span.Duration.Seconds()
	if offsetSecs < 0 {
		offsetSecs = 0
	}
	leading := round(offsetSecs * pxPerSec)
	if leading > trackW {
		leading = trackW
	}

	var bar strings.Builder
	used := 0

	if span.AttachedLate {
		bar.WriteString(styleDim.Render("◀"))
		used = 1
		if used < trackW {
			fill := trackW - used
			state := span.Final
			if len(span.Segments) > 0 {
				state = span.Segments[len(span.Segments)-1].State
			}
			bar.WriteString(renderSegmentFill(fill, state))
			used = trackW
		}
	} else {
		bar.WriteString(strings.Repeat(" ", leading))
		used = leading
		for _, seg := range span.Segments {
			if used >= trackW {
				break
			}
			width := round(seg.Dur.Seconds() * pxPerSec)
			if seg.Dur > 0 && width < 1 {
				width = 1
			}
			if used+width > trackW {
				width = trackW - used
			}
			if width <= 0 {
				continue
			}
			bar.WriteString(renderSegmentFill(width, seg.State))
			used += width
		}
	}

	marker, style := timelineEndMarker(span.Final)
	bar.WriteString(style.Render(marker))
	used++
	for used < barW {
		bar.WriteString(" ")
		used++
	}
	return bar.String()
}

// renderTimelineTab is the project Timeline tab.
func renderTimelineTab(m Model, project Row, width int) string {
	if m.timeline == nil {
		return padMetaLine(styleDim.Render("no timeline data yet"), width)
	}
	spans := m.timeline.Spans(project.ProjectName, time.Now())
	rows := timelineTreeSpans(project, spans)
	if len(rows) == 0 {
		return padMetaLine(styleDim.Render("no timeline data yet"), width)
	}

	nameColW := 0
	for _, r := range rows {
		w := runewidth.StringWidth(r.linePrefix) + runewidth.StringWidth(r.Service)
		if w > nameColW {
			nameColW = w
		}
	}
	nameColW++
	if nameColW > width/3 {
		nameColW = width / 3
	}
	if nameColW < 4 {
		nameColW = 4
	}
	leftW := 2 + nameColW + timelineStatusColW
	barW := width - leftW - 4 - scrollBarWidth
	if barW < 8 {
		barW = 8
	}

	elapsed := time.Duration(0)
	for _, r := range rows {
		if r.Duration > elapsed {
			elapsed = r.Duration
		}
	}

	selected := m.timelineCursor
	if selected < 0 || selected >= len(rows) {
		selected = 0
	}

	lines := renderTimelineBars(rows, elapsed, nameColW, barW, selected)
	visible := lines
	if scroll := m.timelineScroll; scroll > 0 && scroll < len(lines) {
		visible = lines[scroll:]
	}

	var sb strings.Builder
	for i, l := range visible {
		sb.WriteString(padMetaLine(l, width))
		if i != len(visible)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
