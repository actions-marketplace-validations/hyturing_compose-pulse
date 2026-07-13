package ui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func intPtr(i int) *int { return &i }

func TestFormatComposeLine_StatesRenderCleanly(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db":   {},
			"init": {},
			"api": {
				DependsOn: compose.DependsOn{
					"db": {Condition: "service_healthy"},
				},
			},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["db"].ContainerID = "d1"
	graph.ByName["db"].State = docker.StateStarting
	graph.ByName["init"].ContainerID = "i1"
	graph.ByName["init"].State = docker.StateExited
	graph.ByName["init"].ExitCode = intPtr(0)

	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	out := renderView(rows, firstSelectable(rows), 80)

	if !strings.Contains(out, "✓") {
		t.Error("expected completed glyph ✓ for exited-0 init job")
	}
	if !strings.Contains(out, "exit 0") {
		t.Error("expected exit 0 hint for completed init job")
	}
	if !strings.Contains(out, "blocked") {
		t.Error("expected blocked state label for api")
	}
	if !strings.Contains(out, "←1 deps") {
		t.Error("expected blocked hint '←1 deps' for api")
	}
	if strings.Contains(out, "also←") {
		t.Error("did not expect legacy also← noise in rendered tree")
	}
}

func TestFormatComposeLine_MissingDependency(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db-init": {},
			"django": {DependsOn: compose.DependsOn{
				"db-init": {Condition: "service_completed_successfully"},
			}},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["django"].ContainerID = "d1"
	graph.ByName["django"].State = docker.StateHealthy

	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "allocore", Graph: graph}},
	})
	out := renderView(rows, firstSelectable(rows), 100)

	if !strings.Contains(out, "missing") {
		t.Fatalf("expected missing state label for deleted db-init, got:\n%s", out)
	}
	if !strings.Contains(out, "no container") {
		t.Fatalf("expected 'no container' detail for missing dep, got:\n%s", out)
	}
	if !strings.Contains(out, "blocked") {
		t.Fatalf("expected django blocked on missing dep, got:\n%s", out)
	}
}

func TestNameColumn_AlignsAcrossDepths(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db":  {},
			"app": {DependsOn: compose.DependsOn{"db": {}}},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})

	col := nameColumn(rows, 40)
	if col < 1 {
		t.Fatalf("nameColumn = %d, want >= 1", col)
	}
	if col > 40 {
		t.Fatalf("nameColumn = %d, want <= maxWidth 40", col)
	}
}

func TestFormatComposeLine_ColumnAligned(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db":  {},
			"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["db"].ContainerID = "d1"
	graph.ByName["db"].State = docker.StateExited
	graph.ByName["db"].ExitCode = intPtr(0)
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	cols := computeGraphColumns(rows, 72)
	m := Model{}

	var stateStarts []int
	for _, r := range rows {
		if r.Kind != RowComposeNode {
			continue
		}
		line := formatComposeLine(m, r, cols)
		plain := stripANSI(line)
		label := displayStateLabel(displayAndWaiting(r))
		idx := visualIndex(plain, label)
		if idx < 0 {
			t.Fatalf("state %q not in %q", label, plain)
		}
		stateStarts = append(stateStarts, idx)
	}
	if len(stateStarts) < 2 {
		t.Fatal("expected at least 2 service rows")
	}
	for i := 1; i < len(stateStarts); i++ {
		if stateStarts[i] != stateStarts[0] {
			t.Fatalf("state column misaligned: starts at %v", stateStarts)
		}
	}
}

func TestFormatProjectHeader_ColumnAlignedWithServices(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db":  {},
			"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["db"].ContainerID = "d1"
	graph.ByName["db"].State = docker.StateExited
	graph.ByName["db"].ExitCode = intPtr(0)
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "demo", Graph: graph}},
	})
	cols := computeGraphColumns(rows, 72)
	m := Model{}

	var projLine, svcLine string
	var svcRow Row
	for _, r := range rows {
		switch r.Kind {
		case RowProjectHeader:
			projLine = stripANSI(formatProjectHeader(r, 0, cols))
		case RowComposeNode:
			if svcLine == "" {
				svcRow = r
				svcLine = stripANSI(formatComposeLine(m, r, cols))
			}
		}
	}
	if projLine == "" || svcLine == "" {
		t.Fatal("expected project and service lines")
	}
	if !strings.Contains(projLine, "demo") {
		t.Fatalf("expected project name in header: %q", projLine)
	}
	if strings.Contains(projLine, "fail") || strings.Contains(projLine, "wait") || strings.Contains(projLine, "svc") {
		t.Fatalf("project row should not carry summary counts: %q", projLine)
	}
	want := cols.nameW + graphColGap
	svcLabel := displayStateLabel(displayAndWaiting(svcRow))
	if got := visualIndex(svcLine, svcLabel); got != want {
		t.Fatalf("service STATUS col at %d, want %d\nsvc=%q", got, want, svcLine)
	}
}

func TestFormatProjectSummaryLine_ShowsCounts(t *testing.T) {
	cfg := &compose.Config{Services: map[string]compose.Service{"db": {}, "api": {}}}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["db"].ContainerID = "d1"
	graph.ByName["db"].State = docker.StateExited
	graph.ByName["db"].ExitCode = intPtr(1)
	line := stripANSI(formatProjectSummaryLine(graph, 0, 80))
	if !strings.Contains(line, "2 services") {
		t.Fatalf("expected service count, got %q", line)
	}
	if !strings.Contains(line, "fail") {
		t.Fatalf("expected fail bucket, got %q", line)
	}
}

// visualIndex returns the display-column index of substr in s, or -1.
func visualIndex(s, substr string) int {
	byteIdx := strings.Index(s, substr)
	if byteIdx < 0 {
		return -1
	}
	return runewidth.StringWidth(s[:byteIdx])
}
