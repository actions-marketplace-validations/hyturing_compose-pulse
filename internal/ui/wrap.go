package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// normalizeLogLine expands tabs and strips carriage returns so lipgloss width
// matches terminal rendering (tabs otherwise reflow inside bordered panels).
func normalizeLogLine(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\r", "")
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' {
			b.WriteString("    ")
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// wrapToWidth breaks plain text into lines at most width cells wide, preferring
// word boundaries and line wrapping.
func wrapToWidth(s string, width int) []string {
	s = normalizeLogLine(s)
	if width < 1 {
		return []string{s}
	}
	plain := stripANSI(s)
	if runewidth.StringWidth(plain) <= width {
		return []string{plain}
	}

	var lines []string
	remaining := plain
	for runewidth.StringWidth(remaining) > width {
		chunk := runewidth.Truncate(remaining, width, "")
		if chunk == "" {
			r, size := utf8.DecodeRuneInString(remaining)
			lines = append(lines, string(r))
			remaining = remaining[size:]
			continue
		}
		cut := len(chunk)
		if idx := strings.LastIndex(chunk, " "); idx > 0 && idx >= width/4 {
			cut = idx
		}
		lines = append(lines, strings.TrimRight(remaining[:cut], " "))
		remaining = strings.TrimLeft(remaining[cut:], " ")
	}
	if remaining != "" {
		lines = append(lines, remaining)
	}
	return lines
}
