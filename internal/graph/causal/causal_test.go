package causal_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/graph/causal"
	"github.com/hyturing/compose-pulse/internal/model"
)

func TestAnalyze_LinearChain(t *testing.T) {
	run := model.NewRun("linear", time.Unix(0, 0).UTC())
	run.EffectiveConfig = &model.EffectiveConfig{
		Services: []model.EffectiveService{
			{Name: "postgres"},
			{Name: "api", DependsOn: map[string]string{"postgres": "service_healthy"}},
			{Name: "worker", DependsOn: map[string]string{"api": "service_healthy"}},
		},
	}
	run.ApplyEvents([]model.Event{
		{Timestamp: time.Unix(1, 0).UTC(), Service: "postgres", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "unhealthy"},
		{Timestamp: time.Unix(2, 0).UTC(), Service: "api", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "blocked"},
		{Timestamp: time.Unix(3, 0).UTC(), Service: "worker", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "blocked"},
	})

	got := causal.Analyze(run)
	if got == nil {
		t.Fatal("Analyze = nil")
	}
	if got.FirstFailure != "postgres" {
		t.Fatalf("FirstFailure = %q, want postgres", got.FirstFailure)
	}
	if !reflect.DeepEqual(got.BlockedServices, []string{"api", "worker"}) {
		t.Fatalf("BlockedServices = %v", got.BlockedServices)
	}
	if got.Priority["postgres"] != 0 || got.Priority["api"] != 1 {
		t.Fatalf("Priority = %#v", got.Priority)
	}
}

func TestAnalyze_DiamondOneCulprit(t *testing.T) {
	run := model.NewRun("diamond", time.Unix(0, 0).UTC())
	run.EffectiveConfig = &model.EffectiveConfig{
		Services: []model.EffectiveService{
			{Name: "postgres"},
			{Name: "redis"},
			{Name: "api", DependsOn: map[string]string{"postgres": "service_started", "redis": "service_started"}},
		},
	}
	run.ApplyEvents([]model.Event{
		{Timestamp: time.Unix(1, 0).UTC(), Service: "postgres", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "down"},
		{Timestamp: time.Unix(2, 0).UTC(), Service: "redis", Phase: model.PhaseHealthy, Type: model.EventTypeState, Severity: model.SeverityInfo, Message: "ok"},
		{Timestamp: time.Unix(3, 0).UTC(), Service: "api", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "blocked"},
	})

	got := causal.Analyze(run)
	if got == nil || got.FirstFailure != "postgres" {
		t.Fatalf("got %+v, want first=postgres", got)
	}
	if !reflect.DeepEqual(got.BlockedServices, []string{"api"}) {
		t.Fatalf("BlockedServices = %v", got.BlockedServices)
	}
}

func TestAnalyze_RestartCascade(t *testing.T) {
	run := model.NewRun("cascade", time.Unix(0, 0).UTC())
	run.EffectiveConfig = &model.EffectiveConfig{
		Services: []model.EffectiveService{
			{Name: "db"},
			{Name: "api", DependsOn: map[string]string{"db": "service_started"}},
		},
	}
	run.ApplyEvents([]model.Event{
		{Timestamp: time.Unix(1, 0).UTC(), Service: "db", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "exit 1"},
		{Timestamp: time.Unix(2, 0).UTC(), Service: "api", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "connection refused"},
		{Timestamp: time.Unix(3, 0).UTC(), Service: "db", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "exit 1 again"},
	})

	got := causal.Analyze(run)
	if got == nil || got.FirstFailure != "db" {
		t.Fatalf("got %+v", got)
	}
	if !reflect.DeepEqual(got.BlockedServices, []string{"api"}) {
		t.Fatalf("BlockedServices = %v", got.BlockedServices)
	}
}
