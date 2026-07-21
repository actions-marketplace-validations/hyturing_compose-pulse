package record_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/record"
	"github.com/hyturing/compose-pulse/internal/replay"
	"github.com/hyturing/compose-pulse/internal/store/sqlite"
)

type fakeConfig struct {
	err error
	raw []byte
}

func (f fakeConfig) ComposeConfig(string, []string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.raw, nil
}

func TestRecordPreContainerFailure(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "out.json")
	dbPath := filepath.Join(dir, "out.db")

	res, err := record.Run(context.Background(), record.Options{
		Command:      []string{"docker", "compose", "-f", "missing.yml", "up"},
		OutputJSON:   jsonPath,
		DBPath:       dbPath,
		ConfigRunner: fakeConfig{err: os.ErrNotExist},
		SkipDocker:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range res.Run.Events {
		if ev.Phase == model.PhaseFailed {
			found = true
		}
		if ev.Data != nil {
			if stage, _ := ev.Data["stage"].(string); stage == "compose_config" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected pre-container failure event, got %+v", res.Run.Events)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal(err)
	}
	if _, err := replay.LoadRunJSON(jsonPath); err != nil {
		t.Fatal(err)
	}
}

func TestPhaseTimelinePersistedAndReplayed(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "run.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	started := time.Date(2026, 7, 22, 6, 0, 0, 0, time.UTC)
	run := model.NewRun("phase-tl", started)
	run.Project = "demo"
	events := []model.Event{
		{Timestamp: started.Add(time.Second), Source: model.SourceCompose, Project: "demo", Service: "api", Phase: model.PhaseConfigured, Type: model.EventTypeLifecycle, Severity: model.SeverityInfo, Message: "configured"},
		{Timestamp: started.Add(2 * time.Second), Source: model.SourceDocker, Project: "demo", Service: "api", Phase: model.PhaseCreated, Type: model.EventTypeLifecycle, Severity: model.SeverityInfo, Message: "create"},
		{Timestamp: started.Add(3 * time.Second), Source: model.SourceDocker, Project: "demo", Service: "api", Phase: model.PhaseStarted, Type: model.EventTypeLifecycle, Severity: model.SeverityInfo, Message: "start"},
		{Timestamp: started.Add(4 * time.Second), Source: model.SourceDocker, Project: "demo", Service: "api", Phase: model.PhaseHealthy, Type: model.EventTypeLifecycle, Severity: model.SeverityInfo, Message: "health_status: healthy"},
	}
	if err := store.EnsureRun(run); err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if err := store.WriteEvent(run, ev); err != nil {
			t.Fatal(err)
		}
	}
	svc := run.Services["demo/api"]
	if svc == nil || len(svc.PhaseHistory) < 3 {
		t.Fatalf("phase history: %+v", svc)
	}
	replayed, err := store.LoadRunFromEvents("phase-tl")
	if err != nil {
		t.Fatal(err)
	}
	got := replayed.Services["demo/api"]
	if got == nil || got.Phase != model.PhaseHealthy {
		t.Fatalf("replayed service: %+v", got)
	}
	if len(got.PhaseHistory) < 3 {
		t.Fatalf("replayed history: %+v", got.PhaseHistory)
	}
}

func TestRecordCommandFailureWithoutComposeConfig(t *testing.T) {
	dir := t.TempDir()
	res, err := record.Run(context.Background(), record.Options{
		Command:    []string{"false"},
		OutputJSON: filepath.Join(dir, "out.json"),
		DBPath:     filepath.Join(dir, "out.db"),
		SkipDocker: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(res.JSONPath, "out.json") {
		t.Fatalf("json path: %s", res.JSONPath)
	}
}

func TestRecordDoesNotPersistRawConfigSecrets(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "out.json")
	dbPath := filepath.Join(dir, "out.db")
	const secret = "cpulse-test-secret-9f72c1"

	res, err := record.Run(context.Background(), record.Options{
		Command:    []string{"sh", "-c", "true # docker compose"},
		OutputJSON: jsonPath,
		DBPath:     dbPath,
		ConfigRunner: fakeConfig{raw: []byte(`
services:
  api:
    image: api:dev
    environment:
      API_TOKEN: ` + secret + `
`)},
		SkipDocker: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.EffectiveConfig == nil {
		t.Fatal("expected effective config")
	}
	if res.Run.EffectiveConfig.RawYAML != "" {
		t.Fatalf("raw YAML persisted: %q", res.Run.EffectiveConfig.RawYAML)
	}
	for _, path := range []string{jsonPath, dbPath} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), secret) {
			t.Fatalf("secret persisted in %s", path)
		}
	}
}

// TestRecordSurfacesDeadlineExceeded reproduces the `test-startup --timeout`
// exit-code contract: cmd/cpulse relies on `errors.Is(err, context.DeadlineExceeded)`
// to distinguish a timeout (exit 2) from a launch failure (exit 3).
func TestRecordSurfacesDeadlineExceeded(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	res, err := record.Run(ctx, record.Options{
		Command:    []string{"sleep", "5"},
		OutputJSON: filepath.Join(dir, "out.json"),
		DBPath:     filepath.Join(dir, "out.db"),
		SkipDocker: true,
	})
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("record.Run did not respect the context deadline: took %s", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if res == nil || res.ExitCode == 0 {
		t.Fatalf("expected a non-zero exit code result, got %+v", res)
	}
}

// TestRecordDoesNotTreatManualCancelAsDeadlineExceeded ensures Ctrl+C during
// `cpulse record`/`cpulse up` (plain cancellation, no deadline) is unaffected
// by the deadline-exceeded surfacing above.
func TestRecordDoesNotTreatManualCancelAsDeadlineExceeded(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	res, err := record.Run(ctx, record.Options{
		Command:    []string{"sleep", "5"},
		OutputJSON: filepath.Join(dir, "out.json"),
		DBPath:     filepath.Join(dir, "out.db"),
		SkipDocker: true,
	})
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("manual cancellation misreported as deadline exceeded: %v", err)
	}
	if res == nil {
		t.Fatal("expected a result even on manual cancellation")
	}
}
