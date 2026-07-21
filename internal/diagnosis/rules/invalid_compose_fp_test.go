package rules_test

import (
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/rules"
	"github.com/hyturing/compose-pulse/internal/model"
)

func TestInvalidCompose_IgnoresInternalMergedConfigParseError(t *testing.T) {
	start := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	run := model.NewRun("ok-stack", start)
	run.ApplyEvent(model.Event{
		Timestamp: start,
		Source:    model.SourceCompose,
		Phase:     model.PhaseFailed,
		Type:      model.EventTypeLifecycle,
		Severity:  model.SeverityError,
		Message:   "parse merged config: yaml: unmarshal errors:\n  line 19: cannot unmarshal !!map into string",
		Data:      map[string]any{"stage": "compose_config"},
	})
	run.ApplyEvent(model.Event{
		Timestamp: start.Add(time.Second),
		Source:    model.SourceContainer,
		Service:   "web",
		Phase:     model.PhaseStarted,
		Type:      model.EventTypeState,
		Severity:  model.SeverityInfo,
		Message:   "Up",
	})
	findings := engine.Diagnose(run, rules.DefaultRules())
	for _, f := range findings {
		if f.RuleID == "config.invalid_compose" {
			t.Fatalf("false positive invalid_compose: %+v", f)
		}
	}
}
