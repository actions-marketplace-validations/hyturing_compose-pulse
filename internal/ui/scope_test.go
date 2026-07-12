package ui

import (
	"testing"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestVisibleRows_ShowsAllProjectTrees(t *testing.T) {
	cfgA := &compose.Config{Services: map[string]compose.Service{
		"db":  {},
		"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
	}}
	cfgB := &compose.Config{Services: map[string]compose.Service{
		"redis": {},
		"web":   {DependsOn: compose.DependsOn{"redis": {}}},
	}}
	ga, err := dag.Build(cfgA)
	if err != nil {
		t.Fatal(err)
	}
	gb, err := dag.Build(cfgB)
	if err != nil {
		t.Fatal(err)
	}
	ga.ByName["db"].State = docker.StateHealthy
	ga.ByName["api"].State = docker.StateHealthy
	gb.ByName["redis"].State = docker.StateHealthy
	gb.ByName["web"].State = docker.StateHealthy

	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{
			{Name: "demo-broken", Graph: ga},
			{Name: "other", Graph: gb},
		},
	})
	m := Model{rows: rows}

	var names []string
	for _, r := range m.visibleRows() {
		switch r.Kind {
		case RowProjectHeader:
			names = append(names, "proj:"+r.ProjectName)
		case RowComposeNode:
			names = append(names, "svc:"+r.Node.Name)
		}
	}
	want := []string{
		"proj:demo-broken", "svc:db", "svc:api",
		"proj:other", "svc:redis", "svc:web",
	}
	if len(names) != len(want) {
		t.Fatalf("visible = %v, want %v", names, want)
	}
	for i, w := range want {
		if names[i] != w {
			t.Fatalf("visible[%d] = %q, want %q (full=%v)", i, names[i], w, names)
		}
	}
}

func TestDashboardLayout_LeftPanelRoomy(t *testing.T) {
	left, right, _, _ := dashboardLayout(140, 40)
	if left < 58 {
		t.Fatalf("leftW = %d, want at least 58 so CPU/MEM fit", left)
	}
	if left > 70 {
		t.Fatalf("leftW = %d, want at most 70", left)
	}
	if right < 42 {
		t.Fatalf("mainW = %d, want at least 42 for inspector tabs", right)
	}
}

func TestComputeGraphColumns_StatsFlushRight(t *testing.T) {
	cols := computeGraphColumns(nil, 62)
	if !cols.showStats {
		t.Fatalf("expected showStats at width 62, got cols=%+v", cols)
	}
	if cols.detailW != 12 {
		t.Fatalf("detailW = %d, want 12 (compact, not flexed)", cols.detailW)
	}
	// SERVICE absorbs leftover width so CPU/MEM sit flush on the right edge.
	contentW := cols.nameW + graphColGap + cols.stateW + graphColGap + cols.detailW + graphColGap + graphStatsWidth()
	if contentW != 62 {
		t.Fatalf("content width %d, want panel width 62 (stats flush right)", contentW)
	}
}

func TestComputeGraphColumns_ShortNamesStillFlushRight(t *testing.T) {
	cfg := &compose.Config{Services: map[string]compose.Service{
		"api": {DependsOn: compose.DependsOn{"db": {}}},
		"db":  {},
	}}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["db"].State = docker.StateStarting
	rows := filterRows(BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "demo", Graph: graph}},
	}), filterWaiting)

	const panelW = 70
	cols := computeGraphColumns(rows, panelW)
	got := cols.totalWidth()
	if got != panelW {
		t.Fatalf("totalWidth=%d, want %d so CPU/MEM flush to panel edge", got, panelW)
	}
}
