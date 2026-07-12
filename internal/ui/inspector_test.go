package ui

import (
	"strings"
	"testing"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func inspectorFixture(t *testing.T) *dag.Graph {
	t.Helper()
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"init": {},
			"postgres": {
				DependsOn: compose.DependsOn{"init": {Condition: "service_completed_successfully"}},
			},
			"api": {
				DependsOn: compose.DependsOn{"postgres": {Condition: "service_healthy"}},
			},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["init"].ContainerID = "i1"
	graph.ByName["init"].State = docker.StateExited
	graph.ByName["init"].ExitCode = intPtr(0)
	graph.ByName["postgres"].ContainerID = "p1"
	graph.ByName["postgres"].State = docker.StateUnhealthy
	return graph
}

func TestBuildServiceInspector_BlockedWaitingOn(t *testing.T) {
	graph := inspectorFixture(t)
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	idx := findRowByKey(rows, "compose:app:api")
	if idx < 0 {
		t.Fatal("api row not found")
	}

	insp := buildServiceInspector(rows[idx])
	if insp.DisplayState != dag.DisplayBlocked {
		t.Fatalf("DisplayState = %v, want blocked", insp.DisplayState)
	}
	if len(insp.WaitingOn) != 1 || insp.WaitingOn[0].Name != "postgres" {
		t.Fatalf("WaitingOn = %#v, want [postgres]", insp.WaitingOn)
	}
	if insp.WaitingOn[0].Condition != "service_healthy" {
		t.Errorf("WaitingOn condition = %q, want service_healthy", insp.WaitingOn[0].Condition)
	}
	if insp.WaitingOn[0].Satisfied {
		t.Error("expected postgres to be unsatisfied")
	}
}

func TestBuildServiceInspector_CompletedInitSatisfiesDependent(t *testing.T) {
	graph := inspectorFixture(t)
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	idx := findRowByKey(rows, "compose:app:postgres")
	if idx < 0 {
		t.Fatal("postgres row not found")
	}

	insp := buildServiceInspector(rows[idx])
	if len(insp.Dependencies) != 1 || insp.Dependencies[0].Name != "init" {
		t.Fatalf("Dependencies = %#v, want [init]", insp.Dependencies)
	}
	if !insp.Dependencies[0].Satisfied {
		t.Error("expected completed init dependency to be satisfied")
	}
	if len(insp.WaitingOn) != 0 {
		t.Errorf("WaitingOn = %#v, want none (init completed)", insp.WaitingOn)
	}
}

func TestBuildServiceInspector_Dependents(t *testing.T) {
	graph := inspectorFixture(t)
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	idx := findRowByKey(rows, "compose:app:postgres")
	if idx < 0 {
		t.Fatal("postgres row not found")
	}

	insp := buildServiceInspector(rows[idx])
	if len(insp.Dependents) != 1 || insp.Dependents[0].Name != "api" {
		t.Fatalf("Dependents = %#v, want [api]", insp.Dependents)
	}
}

func TestRenderInspectorLogs_WaitingContent(t *testing.T) {
	graph := inspectorFixture(t)
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	idx := findRowByKey(rows, "compose:app:api")
	m := Model{
		rows:           rows,
		cursor:         idx,
		selectedRowKey: rowKey(rows[idx]),
		logWaiting:     true,
		width:          100,
		height:         30,
	}

	out := stripANSI(renderInspectorLogs(m, 60))
	if !strings.Contains(out, "Blocked by") || !strings.Contains(out, "postgres") {
		t.Errorf("expected blocked-by content, got:\n%s", out)
	}
}

func TestRenderDepsTab_ShowsWaitsOnAndBlocks(t *testing.T) {
	graph := inspectorFixture(t)
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	idx := findRowByKey(rows, "compose:app:postgres")
	m := Model{rows: rows, cursor: idx}

	out := stripANSI(renderDepsTab(m, rows[idx], 60))
	if !strings.Contains(out, "waits on") {
		t.Errorf("expected 'waits on' section, got:\n%s", out)
	}
	if !strings.Contains(out, "init") {
		t.Errorf("expected init dependency listed, got:\n%s", out)
	}
	if !strings.Contains(out, "blocks") || !strings.Contains(out, "api") {
		t.Errorf("expected blocks section with api, got:\n%s", out)
	}
}

func TestLeftPanelTitle_ReflectsFilter(t *testing.T) {
	m := Model{}
	if got := stripANSI(leftPanelTitle(m)); !strings.Contains(got, "SERVICE") || !strings.Contains(got, "STATUS") || !strings.Contains(got, "DETAIL") {
		t.Errorf("leftPanelTitle(all) = %q, want column headers", got)
	}
	m.rowFilter = filterFailed
	if got := stripANSI(leftPanelTitle(m)); !strings.Contains(got, "failed") {
		t.Errorf("leftPanelTitle(failed) = %q, want failed in STATUS header", got)
	}
	m.rowFilter = filterWaiting
	if got := stripANSI(leftPanelTitle(m)); !strings.Contains(got, "waiting") {
		t.Errorf("leftPanelTitle(waiting) = %q, want waiting in STATUS header", got)
	}
}
