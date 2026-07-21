package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpEntry struct {
	key  string
	desc string
}

type helpGroup struct {
	title   string
	entries []helpEntry
}

// helpGroups is the complete key reference from TUI-DESIGN.md §7 — the
// contract for both the help overlay (?) and the x menu's keybindings
// section. Adding a key here means removing one.
var helpGroups = []helpGroup{
	{
		title: "Navigation",
		entries: []helpEntry{
			{"↑↓ / j k", "move in focused panel / select in timeline"},
			{"tab / ← →", "switch panel: left graph ↔ main"},
			{"1-4 / [ ]", "service tabs: logs / stats / deps / health"},
			{"1-3 / [ ]", "project tabs: doctor / timeline / graph"},
			{"enter", "zoom · jump to service · run probe"},
			{"esc", "back / un-zoom / clear filter — never quits"},
			{"q", "quit (in zoom: un-zoom)"},
			{"ctrl+c", "copy log selection"},
		},
	},
	{
		title: "Menus",
		entries: []helpEntry{
			{"x", "menu: actions for the selection + full keymap"},
			{"?", "help"},
		},
	},
	{
		title: "Service list",
		entries: []helpEntry{
			{"f", "cycle service filter: all → failed → waiting"},
		},
	},
	{
		title: "Logs find",
		entries: []helpEntry{
			{"box / ^F", "click status-bar box or ctrl+f to type"},
			{"enter / S-enter", "next / previous match"},
			{"esc", "clear query (again: unfocus)"},
			{"g / l", "follow / load older"},
			{"drag / ^C", "select / copy lines"},
		},
	},
	{
		title: "Jumps",
		entries: []helpEntry{
			{"d", "jump to doctor tab"},
			{"t", "jump to timeline tab"},
		},
	},
}

// keymapReferenceLines renders helpGroups as plain "key   desc" lines,
// reused by both the help overlay and the x menu's "── keybindings ──"
// section (TUI-DESIGN.md §5).
func keymapReferenceLines() []string {
	var lines []string
	for _, g := range helpGroups {
		for _, e := range g.entries {
			lines = append(lines, fmt.Sprintf("%-10s %s", e.key, e.desc))
		}
	}
	return lines
}

func renderHelp(m Model) string {
	width := m.width
	if width < 1 {
		width = 80
	}
	height := m.height
	if height < 1 {
		height = 24
	}

	var body strings.Builder
	for gi, g := range helpGroups {
		if gi > 0 {
			body.WriteString("\n")
		}
		body.WriteString(styleSectionHeader.Render(g.title))
		body.WriteString("\n")
		for _, e := range g.entries {
			fmt.Fprintf(&body, "  %-12s %s\n", e.key, e.desc)
		}
	}
	body.WriteString("\n")
	body.WriteString(styleDim.Render("press ?, q, or esc to close"))

	panelW := width - 10
	if panelW > 60 {
		panelW = 60
	}
	if panelW < 20 {
		panelW = width
	}
	panelH := height - 4
	if panelH < 5 {
		panelH = height
	}

	panel := renderPanel("Help", body.String(), panelW, panelH, true, false)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}
