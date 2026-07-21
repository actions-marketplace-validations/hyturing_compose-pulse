package engine_test

import (
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/model"
)

func TestDiagnose_AnnotatesBlockedServicesOnFirstFailure(t *testing.T) {
	run := model.NewRun("d1", time.Unix(0, 0).UTC())
	run.EffectiveConfig = &model.EffectiveConfig{
		Services: []model.EffectiveService{
			{Name: "db"},
			{Name: "api", DependsOn: map[string]string{"db": "service_started"}},
		},
	}
	run.ApplyEvents([]model.Event{
		{Timestamp: time.Unix(1, 0).UTC(), Service: "db", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "down", Data: map[string]any{"exit_code": 127}},
		{Timestamp: time.Unix(2, 0).UTC(), Service: "api", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "blocked"},
	})

	rules := []engine.Rule{
		staticRule{id: "process.exit_127", findings: []model.Finding{{
			RuleID: "process.exit_127", Service: "db", Confidence: model.ConfidenceHigh, RootCause: "command not found",
		}}},
		staticRule{id: "other", findings: []model.Finding{{
			RuleID: "other", Service: "api", Confidence: model.ConfidenceHigh, RootCause: "blocked",
		}}},
	}

	got := engine.Diagnose(run, rules)
	if len(got) < 1 {
		t.Fatal("expected findings")
	}
	var db *model.Finding
	for i := range got {
		if got[i].Service == "db" {
			db = &got[i]
			break
		}
	}
	if db == nil {
		t.Fatalf("missing db finding: %#v", got)
	}
	if len(db.BlockedServices) != 1 || db.BlockedServices[0] != "api" {
		t.Fatalf("BlockedServices = %v, want [api]", db.BlockedServices)
	}
	if got[0].Service != "db" {
		t.Fatalf("first finding service = %q, want db (causal order)", got[0].Service)
	}
}
