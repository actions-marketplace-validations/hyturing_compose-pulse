package ui

import (
	"testing"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func filterFixtureSnapshot(t *testing.T) *discover.Snapshot {
	t.Helper()
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db": {},
			"api": {
				DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}},
			},
			"crasher": {},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["db"].ContainerID = "d1"
	graph.ByName["db"].State = docker.StateStarting
	graph.ByName["crasher"].ContainerID = "c1"
	graph.ByName["crasher"].State = docker.StateExited
	graph.ByName["crasher"].ExitCode = intPtr(1)

	return &discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
		Standalone: []discover.Standalone{
			{ID: "s1", Name: "healthy-stray", State: docker.StateHealthy},
			{ID: "s2", Name: "exited-stray", State: docker.StateExited},
		},
	}
}

func TestFilterRows_FailedKeepsHeaderAndMatches(t *testing.T) {
	rows := BuildRows(filterFixtureSnapshot(t))
	out := filterRows(rows, filterFailed)

	var gotHeader, gotProjectHeader, gotStandaloneHeader bool
	names := map[string]bool{}
	for _, r := range out {
		switch r.Kind {
		case RowProjectHeader:
			gotProjectHeader = true
			gotHeader = true
		case RowStandaloneHeader:
			gotStandaloneHeader = true
			gotHeader = true
		case RowComposeNode:
			names[r.Node.Name] = true
		case RowStandalone:
			names[r.Standalone.Name] = true
		}
	}
	if !gotHeader || !gotProjectHeader || !gotStandaloneHeader {
		t.Fatalf("expected both headers kept when they have matches, got project=%v standalone=%v", gotProjectHeader, gotStandaloneHeader)
	}
	if !names["crasher"] {
		t.Error("expected crasher (exit 1) in failed filter")
	}
	if !names["exited-stray"] {
		t.Error("expected exited-stray standalone in failed filter")
	}
	if names["db"] || names["api"] || names["healthy-stray"] {
		t.Errorf("did not expect non-failed rows in failed filter, got %v", names)
	}
}

func TestFilterRows_DropsHeaderWithNoMatches(t *testing.T) {
	rows := BuildRows(filterFixtureSnapshot(t))
	out := filterRows(rows, filterBlocked)

	for _, r := range out {
		if r.Kind == RowStandaloneHeader {
			t.Fatal("expected standalone header dropped: no standalone container is ever blocked")
		}
	}
	found := false
	for _, r := range out {
		if r.Kind == RowComposeNode && r.Node.Name == "api" {
			found = true
		}
	}
	if !found {
		t.Error("expected api (blocked on db:healthy) in blocked filter")
	}
}

func TestFilterRows_EmptyResult(t *testing.T) {
	cfg := &compose.Config{Services: map[string]compose.Service{"web": {}}}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["web"].ContainerID = "w1"
	graph.ByName["web"].State = docker.StateHealthy
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})

	out := filterRows(rows, filterFailed)
	if len(out) != 0 {
		t.Fatalf("expected empty result, got %d rows", len(out))
	}
}

func TestFilterRows_FlattensPrefix(t *testing.T) {
	rows := BuildRows(filterFixtureSnapshot(t))
	out := filterRows(rows, filterFailed)
	for _, r := range out {
		if r.Kind == RowComposeNode && r.linePrefix != "  " {
			t.Errorf("expected flattened prefix for %q, got %q", r.Node.Name, r.linePrefix)
		}
	}
}

func TestSetRowFilter_PreservesCursorByKey(t *testing.T) {
	snap := filterFixtureSnapshot(t)
	rows := BuildRows(snap)
	crasherIdx := findRowByKey(rows, "compose:app:crasher")
	if crasherIdx < 0 {
		t.Fatal("crasher row not found")
	}

	m := Model{rows: rows, cursor: crasherIdx}
	m.setRowFilter(filterFailed)

	visible := m.visibleRows()
	if m.cursor >= len(visible) {
		t.Fatalf("cursor %d out of range of %d visible rows", m.cursor, len(visible))
	}
	if visible[m.cursor].Kind != RowComposeNode || visible[m.cursor].Node.Name != "crasher" {
		t.Fatalf("expected cursor to stay on crasher after filtering, got %+v", visible[m.cursor])
	}
}

func TestSetRowFilter_ClearRestoresFullList(t *testing.T) {
	snap := filterFixtureSnapshot(t)
	rows := BuildRows(snap)
	m := Model{rows: rows, cursor: firstSelectable(rows)}

	m.setRowFilter(filterFailed)
	m.setRowFilter(filterAll)

	if len(m.visibleRows()) != len(rows) {
		t.Fatalf("expected full row list restored, got %d want %d", len(m.visibleRows()), len(rows))
	}
}

func TestUpdateDashboard_FilterKeysToggleAndClear(t *testing.T) {
	m := actionTestModel(t)

	updated, _ := m.Update(keyMsg("f"))
	m = updated.(Model)
	if m.rowFilter != filterFailed {
		t.Fatalf("rowFilter = %v, want filterFailed", m.rowFilter)
	}

	// Pressing f again toggles back to all.
	updated, _ = m.Update(keyMsg("f"))
	m = updated.(Model)
	if m.rowFilter != filterAll {
		t.Fatalf("rowFilter = %v, want filterAll after second f", m.rowFilter)
	}

	updated, _ = m.Update(keyMsg("b"))
	m = updated.(Model)
	if m.rowFilter != filterBlocked {
		t.Fatalf("rowFilter = %v, want filterBlocked", m.rowFilter)
	}

	updated, _ = m.Update(keyMsg("esc"))
	m = updated.(Model)
	if m.rowFilter != filterAll {
		t.Fatalf("rowFilter = %v, want filterAll after esc", m.rowFilter)
	}
}
