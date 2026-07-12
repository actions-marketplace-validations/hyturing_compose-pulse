package ui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
)

func TestWrapToWidth_WordBreak(t *testing.T) {
	long := `{"timestamp": "2026-03-13T10:00:00Z", "level": "INFO", "message": "hello world"}`
	lines := wrapToWidth(long, 40)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped lines, got %d", len(lines))
	}
	for _, line := range lines {
		if runewidth.StringWidth(line) > 40 {
			t.Errorf("line exceeds width 40: %q", line)
		}
	}
}

func TestRenderLogDisplayRow_MarkerOnly(t *testing.T) {
	const textW = 79
	row := logDisplayRow{text: "hello", sourceLine: 0, lineStart: true}
	line := renderLogDisplayRow(row, textW, true)
	plain := stripANSI(line)
	if !strings.Contains(plain, "▸") {
		t.Error("expected marker arrow")
	}
}

func TestRenderLogViewport_WrappedScrollBar(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = strings.Repeat("word ", 30)
	}
	displayRows := buildLogDisplayRows(lines, 40)
	out := renderLogViewport(logViewportConfig{
		sourceLines:  lines,
		displayRows:  displayRows,
		scroll:       10,
		cursor:       15,
		width:        80,
		visibleLines: 10,
	})
	for _, line := range strings.Split(out, "\n") {
		if w := runewidth.StringWidth(stripANSI(line)); w != 80 {
			t.Errorf("viewport line width = %d, want 80", w)
		}
	}
	start, end := scrollBarThumbRange(10, 10, len(displayRows))
	if end <= start {
		t.Errorf("expected visible thumb range, got [%d,%d)", start, end)
	}
}

func TestRenderLogFullscreen_FollowingShowsTailMarker(t *testing.T) {
	m := Model{
		selectedSvc: "api",
		logs:        []string{"line one", "line two"},
		logCursor:   1,
		logFollow:   true,
		logScroll:   0,
		width:       80,
		height:      24,
	}
	out := stripANSI(renderLogFullscreen(m))
	if !strings.Contains(out, "▸") {
		t.Error("expected tail marker while following")
	}
}

func TestRenderInspectorLogs_Viewport(t *testing.T) {
	logs := make([]string, 50)
	for i := range logs {
		logs[i] = "log line " + strings.Repeat("x", 20)
	}
	cfg := &compose.Config{
		Services: map[string]compose.Service{"api": {}},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["api"].ContainerID = "c1"
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	m := Model{
		rows:           rows,
		cursor:         firstSelectable(rows),
		logs:           logs,
		selectedRowKey: rowKey(rows[firstSelectable(rows)]),
		selectedSvc:    "api",
		logFollow:      true,
		width:          100,
		height:         30,
	}
	out := renderInspectorLogs(m, 58)
	if !strings.Contains(out, "▸") {
		t.Error("inspector logs should show tail marker on latest log line")
	}
	if !strings.Contains(stripANSI(out), "log line") {
		t.Error("inspector logs should include log content")
	}
}
