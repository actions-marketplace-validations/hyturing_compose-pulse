package engine_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/model"
)

type staticRule struct {
	id       string
	findings []model.Finding
}

func (r staticRule) ID() string          { return r.id }
func (r staticRule) Description() string { return r.id }
func (r staticRule) Evaluate(*engine.RunContext) []model.Finding {
	out := make([]model.Finding, len(r.findings))
	copy(out, r.findings)
	return out
}

func TestEvaluate_DedupesByRuleAndServiceKeepingHigherConfidence(t *testing.T) {
	ctx := engine.NewRunContext(model.NewRun("r1", time.Unix(0, 0).UTC()))
	rules := []engine.Rule{
		staticRule{id: "a", findings: []model.Finding{{
			RuleID: "a", Service: "api", Confidence: model.ConfidenceMedium, RootCause: "medium",
		}}},
		staticRule{id: "a", findings: []model.Finding{{
			RuleID: "a", Service: "api", Confidence: model.ConfidenceHigh, RootCause: "high",
		}}},
	}

	got := engine.Evaluate(ctx, rules)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Confidence != model.ConfidenceHigh || got[0].RootCause != "high" {
		t.Fatalf("got %+v, want high confidence finding", got[0])
	}
}

func TestEvaluate_SortsByCausalPriorityThenConfidenceThenRuleID(t *testing.T) {
	ctx := engine.NewRunContext(model.NewRun("r1", time.Unix(0, 0).UTC()))
	ctx.SetCausalPriority(map[string]int{
		"db":  0,
		"api": 1,
	})
	rules := []engine.Rule{
		staticRule{id: "z-rule", findings: []model.Finding{{
			RuleID: "z-rule", Service: "db", Confidence: model.ConfidenceMedium,
		}}},
		staticRule{id: "a-rule", findings: []model.Finding{{
			RuleID: "a-rule", Service: "api", Confidence: model.ConfidenceHigh,
		}}},
		staticRule{id: "b-rule", findings: []model.Finding{{
			RuleID: "b-rule", Service: "db", Confidence: model.ConfidenceHigh,
		}}},
	}

	got := engine.Evaluate(ctx, rules)
	var order []string
	for _, f := range got {
		order = append(order, f.RuleID+":"+f.Service)
	}
	want := []string{"b-rule:db", "z-rule:db", "a-rule:api"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestEvaluate_SkipsNilRulesAndEmptyFindings(t *testing.T) {
	ctx := engine.NewRunContext(model.NewRun("r1", time.Unix(0, 0).UTC()))
	rules := []engine.Rule{
		nil,
		staticRule{id: "ok", findings: []model.Finding{{RuleID: "ok", Service: "api", Confidence: model.ConfidenceHigh}}},
		staticRule{id: "empty", findings: nil},
	}
	got := engine.Evaluate(ctx, rules)
	if len(got) != 1 || got[0].RuleID != "ok" {
		t.Fatalf("got %+v", got)
	}
}

func TestRunContext_ServiceLookupAndEvents(t *testing.T) {
	run := model.NewRun("r1", time.Unix(0, 0).UTC())
	run.Project = "demo"
	run.ApplyEvent(model.Event{
		Timestamp: time.Unix(1, 0).UTC(),
		Source:    model.SourceContainer,
		Project:   "demo",
		Service:   "api",
		Phase:     model.PhaseFailed,
		Type:      model.EventTypeState,
		Severity:  model.SeverityError,
		Message:   "Exited (127)",
		Data:      map[string]any{"exit_code": 127},
	})
	ctx := engine.NewRunContext(run)

	svc := ctx.Service("api")
	if svc == nil || svc.ExitCode == nil || *svc.ExitCode != 127 {
		t.Fatalf("Service(api) = %+v", svc)
	}
	events := ctx.EventsForService("api")
	if len(events) != 1 || events[0].Message != "Exited (127)" {
		t.Fatalf("EventsForService = %+v", events)
	}
}
