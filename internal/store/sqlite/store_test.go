package sqlite_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/store/sqlite"
)

func TestStoreRoundTripAndReplayDeterminism(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.db")
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = store.Close() }()

	started := time.Date(2026, 7, 22, 2, 0, 0, 0, time.UTC)
	run := model.NewRun("run-det", started)
	run.Project = "demo"
	if err := store.EnsureRun(run); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	events := []model.Event{
		{
			Timestamp:   started.Add(time.Second),
			Source:      model.SourceContainer,
			Project:     "demo",
			Service:     "api",
			ContainerID: "c1",
			Phase:       model.PhaseStarted,
			Type:        model.EventTypeState,
			Severity:    model.SeverityInfo,
			Message:     "starting",
			Data:        map[string]any{"image": "api:1", "status": "Up"},
		},
		{
			Timestamp:   started.Add(2 * time.Second),
			Source:      model.SourceContainer,
			Project:     "demo",
			Service:     "api",
			ContainerID: "c1",
			Phase:       model.PhaseHealthy,
			Type:        model.EventTypeState,
			Severity:    model.SeverityInfo,
			Message:     "healthy",
			Data:        map[string]any{"image": "api:1", "status": "Up (healthy)", "ports": []string{"80/tcp"}},
		},
	}
	for _, ev := range events {
		if err := store.WriteEvent(run, ev); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	loaded, err := store.LoadRun("run-det")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	replayed, err := store.LoadRunFromEvents("run-det")
	if err != nil {
		t.Fatalf("replay events: %v", err)
	}

	// Compare derived service state (determinism).
	want, err := json.Marshal(normalizeDerived(loaded))
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(normalizeDerived(replayed))
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(got) {
		t.Fatalf("derived mismatch\nwant: %s\ngot:  %s", want, got)
	}

	// JSON file replay path used by cpulse replay.
	raw, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	var fromJSON model.Run
	if err := json.Unmarshal(raw, &fromJSON); err != nil {
		t.Fatal(err)
	}
	rebuilt := model.NewRun(fromJSON.ID, fromJSON.StartedAt)
	rebuilt.Project = fromJSON.Project
	rebuilt.SchemaVersion = fromJSON.SchemaVersion
	rebuilt.ApplyEvents(fromJSON.Events)
	want2, _ := json.Marshal(normalizeDerived(loaded))
	got2, _ := json.Marshal(normalizeDerived(rebuilt))
	if string(want2) != string(got2) {
		t.Fatalf("json replay mismatch\nwant: %s\ngot:  %s", want2, got2)
	}
}

type derived struct {
	SchemaVersion int              `json:"schema_version"`
	ID            string           `json:"id"`
	Project       string           `json:"project"`
	Services      []*model.Service `json:"services"`
}

func normalizeDerived(r *model.Run) derived {
	return derived{
		SchemaVersion: r.SchemaVersion,
		ID:            r.ID,
		Project:       r.Project,
		Services:      r.ServiceList(),
	}
}
