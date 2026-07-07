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

// helpGroups is the full key reference shown in the help overlay. Later
// phases append their own rows here (Health tab, doctor/timeline/impact/probe,
// report copy, watch cockpit) rather than inventing a new overlay.
var helpGroups = []helpGroup{
	{
		title: "Navigation",
		entries: []helpEntry{
			{"↑↓ / j k", "move selection"},
			{"tab / ← →", "switch panel"},
			{"enter", "fullscreen logs"},
			{"g / End", "follow logs"},
			{"q / ctrl+c", "quit"},
		},
	},
	{
		title: "Views",
		entries: []helpEntry{
			{"1", "overview tab"},
			{"2", "logs tab"},
			{"3", "deps tab"},
		},
	},
	{
		title: "Filters",
		entries: []helpEntry{
			{"f", "toggle failed filter"},
			{"b", "toggle blocked filter"},
			{"esc", "clear filter"},
		},
	},
	{
		title: "Actions",
		entries: []helpEntry{
			{"a", "open actions menu"},
			{"?", "toggle this help"},
		},
	},
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
	body.WriteString("\n" + styleDim.Render("press ?, q, or esc to close"))

	panelW := width - 10
	if panelW > 50 {
		panelW = 50
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
