package ui

import (
	"strings"
)

const (
	scrollBarWidth      = 1
	scrollBarTrackGlyph = "│"
	scrollBarThumbGlyph = "┃"
)

func styledScrollBar(thumb bool) string {
	if thumb {
		return styleScrollThumb.Render(scrollBarThumbGlyph)
	}
	return styleScrollTrack.Render(scrollBarTrackGlyph)
}

type logDisplayRow struct {
	text       string
	sourceLine int
	lineStart  bool
}

func buildLogDisplayRows(sourceLines []string, wrapWidth int) []logDisplayRow {
	if wrapWidth < 1 {
		wrapWidth = 1
	}
	var rows []logDisplayRow
	for i, line := range sourceLines {
		line = normalizeLogLine(line)
		parts := wrapToWidth(line, wrapWidth)
		for j, part := range parts {
			rows = append(rows, logDisplayRow{
				text:       part,
				sourceLine: i,
				lineStart:  j == 0,
			})
		}
	}
	return rows
}

// scrollBarThumbRange returns the visible row range [start, end) for the thumb.
func scrollBarThumbRange(visible, scroll, total int) (start, end int) {
	if visible < 1 || total <= visible {
		return 0, visible
	}
	maxScroll := total - visible
	thumbSize := visible * visible / total
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > visible {
		thumbSize = visible
	}
	if maxScroll <= 0 {
		return 0, thumbSize
	}
	start = scroll * (visible - thumbSize) / maxScroll
	return start, start + thumbSize
}

func renderScrollBarCell(row, visible, scroll, total int) string {
	if total <= visible {
		return styledScrollBar(false)
	}
	start, end := scrollBarThumbRange(visible, scroll, total)
	if row >= start && row < end {
		return styledScrollBar(true)
	}
	return styledScrollBar(false)
}

func renderLogDisplayRow(row logDisplayRow, width int, marked bool) string {
	prefix := "  "
	if marked && row.lineStart {
		prefix = styleLogMarker.Render("▸") + " "
	}
	return padLine(prefix+row.text, width)
}

type logViewportConfig struct {
	sourceLines  []string
	displayRows  []logDisplayRow
	scroll       int
	cursor       int
	follow       bool
	waiting      bool
	width        int
	visibleLines int
}

func (c *logViewportConfig) ensureDisplayRows() {
	if c.displayRows != nil {
		return
	}
	textW := c.width - scrollBarWidth
	if textW < logLinePrefixW+1 {
		textW = logLinePrefixW + 1
	}
	c.displayRows = buildLogDisplayRows(c.sourceLines, textW-logLinePrefixW)
}

func (c logViewportConfig) markedRow(row logDisplayRow) bool {
	c.ensureDisplayRows()
	if c.waiting || len(c.displayRows) == 0 {
		return false
	}
	if c.follow {
		last := len(c.sourceLines) - 1
		return row.lineStart && row.sourceLine == last
	}
	if c.cursor < 0 || c.cursor >= len(c.displayRows) {
		return false
	}
	src := c.displayRows[c.cursor].sourceLine
	return row.lineStart && row.sourceLine == src
}

func renderLogViewport(c logViewportConfig) string {
	if c.visibleLines < 1 {
		c.visibleLines = 1
	}
	if c.width < scrollBarWidth+1 {
		c.width = scrollBarWidth + 1
	}
	cfg := c
	cfg.ensureDisplayRows()

	textWidth := cfg.width - scrollBarWidth
	displayRows := cfg.displayRows
	visible := cfg.visibleLines
	total := len(displayRows)

	maxScroll := total - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := cfg.scroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	out := make([]string, 0, visible)
	for i := 0; i < visible; i++ {
		bar := renderScrollBarCell(i, visible, scroll, total)
		if scroll+i >= total {
			out = append(out, padLine("", textWidth)+bar)
			continue
		}
		row := displayRows[scroll+i]
		out = append(out, renderLogDisplayRow(row, textWidth, cfg.markedRow(row))+bar)
	}
	return strings.Join(out, "\n")
}

func tailScroll(total, visible int) int {
	maxScroll := total - visible
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func padMetaLine(line string, width int) string {
	textW := width - scrollBarWidth
	if textW < 1 {
		textW = 1
	}
	return padLine(line, textW) + styledScrollBar(false)
}

func lineHasScrollBarSuffix(s string) bool {
	plain := stripANSI(s)
	return strings.HasSuffix(plain, scrollBarTrackGlyph) || strings.HasSuffix(plain, scrollBarThumbGlyph)
}
