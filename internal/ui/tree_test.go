package ui

import (
	"strings"
	"testing"

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
	out := renderView(rows, firstSelectable(rows), 0, 80)

	if !strings.Contains(out, "✓") {
		t.Error("expected completed glyph ✓ for exited-0 init job")
	}
	if !strings.Contains(out, "exit 0") {
		t.Error("expected exit 0 hint for completed init job")
	}
	if !strings.Contains(out, "blocked") {
		t.Error("expected blocked state label for api")
	}
	if !strings.Contains(out, "db:healthy") {
		t.Error("expected blocker hint db:healthy for api")
	}
	if strings.Contains(out, "also←") {
		t.Error("did not expect legacy also← noise in rendered tree")
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

	col := nameColumn(rows, 80)
	if col < 1 {
		t.Fatalf("nameColumn = %d, want >= 1", col)
	}
	if col > 40 {
		t.Fatalf("nameColumn = %d, want <= panelWidth/2 (40)", col)
	}
}
