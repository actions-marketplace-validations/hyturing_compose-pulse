package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestRenderDashboard_TabSeparatedLogLines(t *testing.T) {
	logs := []string{
		`2026-07-06T09:36:10.583Z error 	worker-1 	db:error Invalid query failed. Code: 42P01. Message: relation "items" does not exist`,
	}
	for i := 0; i < 50; i++ {
		logs = append(logs, fmt.Sprintf(
			`2026-07-06T09:36:%02d.000Z info 	worker-1 	Starting queue executor %d with concurrency 1`,
			i%60, i,
		))
	}

	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"worker": {
				DependsOn: compose.DependsOn{"postgres": {Condition: "service_healthy"}},
			},
			"api": {
				DependsOn: compose.DependsOn{"postgres": {Condition: "service_healthy"}},
			},
			"postgres": {},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["worker"].ContainerID = "ww1"
	graph.ByName["worker"].State = docker.StateHealthy
	graph.ByName["postgres"].ContainerID = "pg1"
	graph.ByName["postgres"].State = docker.StateHealthy

	snap := &discover.Snapshot{
		Projects: []discover.Project{{Name: "myapp", Graph: graph}},
	}
	rows := BuildRows(snap)
	idx := findRowByKey(rows, "compose:myapp:worker")
	if idx < 0 {
		t.Fatal("worker row not found")
	}

	m := Model{
		snapshot:       snap,
		rows:           rows,
		cursor:         idx,
		selectedRowKey: rowKey(rows[idx]),
		selectedSvc:    "worker",
		logs:           logs,
		logFollow:      true,
		panelFocus:     focusMain,
		width:          120,
		height:         35,
	}
	out := renderDashboard(m)
	lines := strings.Split(out, "\n")

	for i, line := range lines {
		plain := stripANSI(line)
		w := runewidth.StringWidth(plain)
		if w > m.width {
			t.Fatalf("line %d width = %d, want <= %d: %q", i, w, m.width, plain)
		}
		if i > 0 && i < len(lines)-1 && w < m.width {
			t.Fatalf("line %d width = %d, want %d (panel join misalignment): %q", i, w, m.width, plain)
		}
		if i > 0 && i < len(lines)-1 && !hasRightPanelEdge(plain) {
			t.Fatalf("panel line %d missing right border at width %d: %q", i, w, plain)
		}
	}
}

func TestNormalizeLogLine_Tabs(t *testing.T) {
	in := "a\tb"
	out := normalizeLogLine(in)
	if strings.Contains(out, "\t") {
		t.Fatalf("expected tabs expanded, got %q", out)
	}
	if lipgloss.Width(out) <= lipgloss.Width(in) {
		t.Fatalf("expected expanded line to be wider: in=%d out=%d", lipgloss.Width(in), lipgloss.Width(out))
	}
	innerW := 70
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	lines := []string{strings.Repeat(" ", innerW)}
	for i := 0; i < 33; i++ {
		lines = append(lines, padLine(normalizeLogLine(
			fmt.Sprintf(`2026-07-06T09:36:%02d.000Z info 	worker-1 	queue %d`, i%60, i),
		), innerW))
	}
	rendered := style.Width(innerW).Render(strings.Join(lines, "\n"))
	if got := strings.Count(rendered, "\n") + 1; got != innerHWithBorder(len(lines)) {
		t.Fatalf("boxed lines = %d, want %d", got, innerHWithBorder(len(lines)))
	}
}

func innerHWithBorder(contentLines int) int {
	return contentLines + 2 // top + bottom border
}

func TestWrapToWidth_TabsDoNotBreakPanelWidth(t *testing.T) {
	line := "2026-07-06T09:36:10.583Z error \tworker-1 \tdb:error Invalid query"
	for _, wrapW := range []int{20, 40, 58} {
		for i, l := range wrapToWidth(line, wrapW) {
			if w := runewidth.StringWidth(l); w > wrapW {
				t.Fatalf("wrapW=%d line %d width=%d: %q", wrapW, i, w, l)
			}
		}
	}
	rows := buildLogDisplayRows([]string{line}, 50)
	out := renderLogViewport(logViewportConfig{
		sourceLines:  []string{line},
		displayRows:  rows,
		scroll:       0,
		width:        72,
		visibleLines: 5,
	})
	for i, l := range strings.Split(out, "\n") {
		if w := runewidth.StringWidth(stripANSI(l)); w != 72 {
			t.Fatalf("viewport line %d width=%d want=72: %q", i, w, stripANSI(l))
		}
	}
}

func TestPanelRenderedHeight_MatchesSplit(t *testing.T) {
	leftW, rightW, panelH, _ := dashboardLayout(120, 35)
	left := renderPanel("Services", strings.Repeat("x\n", panelH), leftW, panelH, true, false)
	right := renderPanel("Details", strings.Repeat("y\n", panelH), rightW, panelH, true, true)
	lh := panelRenderedHeight(left)
	rh := panelRenderedHeight(right)
	t.Logf("left lines=%d right lines=%d panelH=%d", lh, rh, panelH)
	if lh != rh {
		t.Fatalf("panel heights differ: left=%d right=%d", lh, rh)
	}
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	for i := range leftLines {
		lw := runewidth.StringWidth(stripANSI(leftLines[i]))
		if lw != leftW {
			t.Errorf("left line %d width=%d want=%d", i, lw, leftW)
		}
	}
	for i := range rightLines {
		rw := runewidth.StringWidth(stripANSI(rightLines[i]))
		if rw != rightW {
			t.Errorf("right line %d width=%d want=%d", i, rw, rightW)
		}
	}
}
