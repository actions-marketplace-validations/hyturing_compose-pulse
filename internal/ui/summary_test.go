package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestCountStates(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db":      {},
			"init":    {},
			"crasher": {},
			"api": {
				DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}},
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
	graph.ByName["crasher"].ContainerID = "c1"
	graph.ByName["crasher"].State = docker.StateExited
	graph.ByName["crasher"].ExitCode = intPtr(1)

	snap := &discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
		Standalone: []discover.Standalone{
			{ID: "s1", Name: "stray", State: docker.StateHealthy},
		},
	}

	counts := countStates(snap)
	if counts.Total != 4 {
		t.Errorf("Total = %d, want 4", counts.Total)
	}
	if counts.Starting != 1 {
		t.Errorf("Starting = %d, want 1", counts.Starting)
	}
	if counts.Completed != 1 {
		t.Errorf("Completed = %d, want 1", counts.Completed)
	}
	if counts.Failed != 1 {
		t.Errorf("Failed = %d, want 1", counts.Failed)
	}
	if counts.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1", counts.Blocked)
	}
	if counts.Standalone != 1 {
		t.Errorf("Standalone = %d, want 1", counts.Standalone)
	}
}

func TestProjectLabel(t *testing.T) {
	if got := projectLabel(nil); got != "no projects" {
		t.Errorf("projectLabel(nil) = %q, want %q", got, "no projects")
	}
	one := &discover.Snapshot{Projects: []discover.Project{{Name: "dte"}}}
	if got := projectLabel(one); got != "dte" {
		t.Errorf("projectLabel(one) = %q, want dte", got)
	}
	many := &discover.Snapshot{Projects: []discover.Project{{Name: "a"}, {Name: "b"}}}
	if got := projectLabel(many); got != "2 projects" {
		t.Errorf("projectLabel(many) = %q, want '2 projects'", got)
	}
}

func TestRenderSummaryBar(t *testing.T) {
	counts := stateCounts{Total: 11, Healthy: 8, Failed: 1, Completed: 1, Blocked: 1}
	out := stripANSI(renderSummaryBar(counts, "dte", 500*time.Millisecond, 200))
	for _, want := range []string{"cpulse", "dte", "11 services", "8 healthy", "1 failed", "1 completed", "updated 0.5s ago"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderSummaryBar output missing %q: %q", want, out)
		}
	}
}

func TestRenderSummaryBar_ClampsLongAgo(t *testing.T) {
	counts := stateCounts{Total: 1, Healthy: 1}
	out := stripANSI(renderSummaryBar(counts, "dte", 30*time.Second, 100))
	if !strings.Contains(out, "updated 9.9s+ ago") {
		t.Errorf("expected clamped ago text, got %q", out)
	}
}
