package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/model"
)

func TestTUIRendersFromRunUpdate(t *testing.T) {
	composePath := filepath.Join(t.TempDir(), "compose.yml")
	if err := os.WriteFile(composePath, []byte("services:\n  db:\n    image: postgres\n  api:\n    image: api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db":  {},
			"api": {},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range graph.Ordered {
		switch n.Name {
		case "db":
			n.ContainerID = "cdb"
			n.State = docker.StateHealthy
		case "api":
			n.ContainerID = "capi"
			n.State = docker.StateExited
			code := 1
			n.ExitCode = &code
		}
	}
	snap := &discover.Snapshot{
		Projects: []discover.Project{{
			Name:        "demo",
			Graph:       graph,
			ConfigFiles: []string{composePath},
			Containers:  map[string]string{"db": "cdb", "api": "capi"},
		}},
	}

	run := model.NewRun("ui-test", time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC))
	run.Project = "demo"
	run.ApplyEvent(model.Event{
		Timestamp: run.StartedAt, Source: model.SourceContainer, Project: "demo",
		Service: "api", ContainerID: "capi", Phase: model.PhaseFailed,
		Type: model.EventTypeState, Severity: model.SeverityError, Message: "Exited (1)",
	})

	m := Model{
		snapshot: snap,
		run:      run,
		rows:     BuildRows(snap),
		width:    120,
		height:   40,
	}
	m.cursor = firstSelectable(m.rows)

	updated, _ := m.Update(RunUpdateMsg{Run: run, Snapshot: snap})
	m = updated.(Model)
	if m.Run() == nil || m.Run().Services["demo/api"] == nil {
		t.Fatal("expected run aggregate on model after RunUpdateMsg")
	}
	if m.Run().Services["demo/api"].Phase != model.PhaseFailed {
		t.Fatalf("phase=%s, want failed", m.Run().Services["demo/api"].Phase)
	}

	view := stripANSI(m.View())
	if !strings.Contains(view, "api") && !strings.Contains(view, "demo") {
		t.Fatalf("view missing service/project markers:\n%s", view)
	}
}
