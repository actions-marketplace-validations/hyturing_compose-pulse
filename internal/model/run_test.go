package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
)

func TestEventRunJSONRoundTripSchemaVersion(t *testing.T) {
	started := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	run := model.NewRun("run-1", started)
	run.Project = "demo"
	run.ApplyEvent(model.Event{
		Timestamp:   started.Add(time.Second),
		Source:      model.SourceContainer,
		Project:     "demo",
		Service:     "api",
		ContainerID: "abc123",
		Phase:       model.PhaseHealthy,
		Type:        model.EventTypeState,
		Severity:    model.SeverityInfo,
		Message:     "api healthy",
		Data: map[string]any{
			"image":  "api:latest",
			"status": "Up 1 second (healthy)",
			"ports":  []string{"8080:80/tcp"},
		},
	})

	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded model.Run
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SchemaVersion != model.SchemaVersion {
		t.Fatalf("schema_version=%d, want %d", decoded.SchemaVersion, model.SchemaVersion)
	}
	if decoded.ID != "run-1" || decoded.Project != "demo" {
		t.Fatalf("identity mismatch: %+v", decoded)
	}
	if len(decoded.Events) != 1 {
		t.Fatalf("events=%d, want 1", len(decoded.Events))
	}
	ev := decoded.Events[0]
	if ev.Source != model.SourceContainer || ev.Phase != model.PhaseHealthy || ev.Type != model.EventTypeState {
		t.Fatalf("event enums mismatch: %+v", ev)
	}
	svc := decoded.Services["demo/api"]
	if svc == nil || svc.Phase != model.PhaseHealthy || svc.Image != "api:latest" {
		t.Fatalf("service mismatch: %+v", svc)
	}
}

func TestApplyEventMonotonicPhase(t *testing.T) {
	run := model.NewRun("r", time.Now().UTC())
	run.ApplyEvent(model.Event{Service: "db", Phase: model.PhaseStarted, Type: model.EventTypeState, Source: model.SourceContainer})
	run.ApplyEvent(model.Event{Service: "db", Phase: model.PhaseConfigured, Type: model.EventTypeState, Source: model.SourceContainer})
	if got := run.Services["db"].Phase; got != model.PhaseStarted {
		t.Fatalf("phase=%s, want started (monotonic)", got)
	}
	run.ApplyEvent(model.Event{Service: "db", Phase: model.PhaseFailed, Type: model.EventTypeState, Source: model.SourceContainer})
	if got := run.Services["db"].Phase; got != model.PhaseFailed {
		t.Fatalf("phase=%s, want failed", got)
	}
}
