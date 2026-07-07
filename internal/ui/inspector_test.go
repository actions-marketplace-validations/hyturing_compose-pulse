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

func TestRenderPreview_OverviewTab(t *testing.T) {
	graph := inspectorFixture(t)
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	idx := findRowByKey(rows, "compose:app:api")
	m := Model{rows: rows, cursor: idx, inspectorTab: inspectorTabOverview}

	out := stripANSI(renderPreview(m, 60))
	if !strings.Contains(out, "Status") || !strings.Contains(out, "blocked") {
		t.Errorf("expected overview status line, got:\n%s", out)
	}
	if !strings.Contains(out, "Blocked by") || !strings.Contains(out, "postgres:healthy") {
		t.Errorf("expected blocked-by line, got:\n%s", out)
	}
}

func TestRenderPreview_DepsTab(t *testing.T) {
	graph := inspectorFixture(t)
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	idx := findRowByKey(rows, "compose:app:postgres")
	m := Model{rows: rows, cursor: idx, inspectorTab: inspectorTabDeps}

	out := stripANSI(renderPreview(m, 60))
	if !strings.Contains(out, "Dependencies") {
		t.Errorf("expected Dependencies section, got:\n%s", out)
	}
	if !strings.Contains(out, "init") || !strings.Contains(out, "completed") {
		t.Errorf("expected completed init dependency, got:\n%s", out)
	}
	if !strings.Contains(out, "Direct dependents") || !strings.Contains(out, "api") {
		t.Errorf("expected direct dependents section with api, got:\n%s", out)
	}
}

func TestUpdateDashboard_TabKeysSwitchInspectorTab(t *testing.T) {
	m := actionTestModel(t)

	updated, _ := m.Update(keyMsg("2"))
	m = updated.(Model)
	if m.inspectorTab != inspectorTabLogs {
		t.Fatalf("inspectorTab = %d, want logs", m.inspectorTab)
	}

	updated, _ = m.Update(keyMsg("3"))
	m = updated.(Model)
	if m.inspectorTab != inspectorTabDeps {
		t.Fatalf("inspectorTab = %d, want deps", m.inspectorTab)
	}

	updated, _ = m.Update(keyMsg("1"))
	m = updated.(Model)
	if m.inspectorTab != inspectorTabOverview {
		t.Fatalf("inspectorTab = %d, want overview", m.inspectorTab)
	}
}

func TestUpdateDashboard_Tab3NoOpForStandalone(t *testing.T) {
	snap := &discover.Snapshot{
		Standalone: []discover.Standalone{
			{ID: "s1", Name: "stray", Image: "nginx:alpine", State: docker.StateHealthy},
		},
	}
	rows := BuildRows(snap)
	idx := findRowByKey(rows, "standalone:s1")
	m := Model{rows: rows, cursor: idx}

	updated, _ := m.Update(keyMsg("3"))
	m = updated.(Model)
	if m.inspectorTab == inspectorTabDeps {
		t.Fatal("expected 3 to be a no-op for standalone containers")
	}
}

func TestRenderPreview_StandaloneHasNoDepsTab(t *testing.T) {
	snap := &discover.Snapshot{
		Standalone: []discover.Standalone{
			{ID: "s1", Name: "stray", Image: "nginx:alpine", State: docker.StateHealthy},
		},
	}
	rows := BuildRows(snap)
	idx := findRowByKey(rows, "standalone:s1")
	m := Model{rows: rows, cursor: idx}

	out := stripANSI(renderPreview(m, 60))
	if strings.Contains(out, "Deps") {
		t.Errorf("expected no Deps tab for standalone container, got:\n%s", out)
	}
	if !strings.Contains(out, "stray") && !strings.Contains(out, "nginx:alpine") {
		t.Errorf("expected standalone overview content, got:\n%s", out)
	}
}
