package testsupport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/replay"
)

// ScenarioDir returns testdata/scenarios/<id>.
func ScenarioDir(id string) string {
	return filepath.Join("testdata", "scenarios", id)
}

// LoadRecordedRun loads a recorded run JSON from testdata/runs (no Docker).
func LoadRecordedRun(t *testing.T, name string) *model.Run {
	t.Helper()
	path := filepath.Join("testdata", "runs", name)
	if _, err := os.Stat(path); err != nil {
		// Allow callers in nested packages.
		path = filepath.Join("..", "..", "testdata", "runs", name)
	}
	run, err := replay.LoadRunJSON(path)
	if err != nil {
		t.Fatalf("load recorded run %s: %v", name, err)
	}
	return run
}

// WriteRunJSON writes a run to path (test helper).
func WriteRunJSON(t *testing.T, path string, run *model.Run) {
	t.Helper()
	raw, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// SyntheticRun builds a minimal run for unit tests without Docker.
func SyntheticRun(project string, services map[string]model.ServicePhase) *model.Run {
	run := model.NewRun("synthetic", time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC))
	run.Project = project
	i := 0
	for name, phase := range services {
		i++
		run.ApplyEvent(model.Event{
			Timestamp:   run.StartedAt.Add(time.Duration(i) * time.Second),
			Source:      model.SourceContainer,
			Project:     project,
			Service:     name,
			ContainerID: fmt.Sprintf("c-%s", name),
			Phase:       phase,
			Type:        model.EventTypeState,
			Severity:    model.SeverityInfo,
			Message:     phase.String(),
		})
	}
	return run
}

// RequireDocker skips the test when Docker is unavailable or -short is set.
func RequireDocker(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping docker integration test in -short mode")
	}
	if os.Getenv("CPULSE_INTEGRATION") != "1" {
		t.Skip("set CPULSE_INTEGRATION=1 to run docker-backed fixture harness tests")
	}
}
