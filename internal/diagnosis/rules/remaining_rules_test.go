package rules_test

import (
	"testing"

	"github.com/hyturing/compose-pulse/internal/diagnosis/rules"
	"github.com/hyturing/compose-pulse/internal/model"
)

func TestRemainingRules_FireOnFixtures(t *testing.T) {
	cases := []struct {
		runID   string
		ruleID  string
		service string
	}{
		{"process.exit_126", "process.exit_126", "app"},
		{"process.exit_127", "process.exit_127", "app"},
		{"process.oom_killed", "process.oom_killed", "api"},
		{"health.missing_executable", "health.missing_executable", "api"},
		{"net.localhost_in_container", "net.localhost_in_container", "api"},
		{"race.depends_on_started", "race.depends_on_started", "api"},
		{"resource.oom", "resource.oom", "worker"},
		{"restart.identical_exit_loop", "restart.identical_exit_loop", "app"},
	}
	for _, tc := range cases {
		t.Run(tc.ruleID, func(t *testing.T) {
			ctx := loadPhase2Run(t, tc.runID)
			findings := evaluateAll(ctx)
			f := findingByRule(findings, tc.ruleID)
			if f == nil {
				t.Fatalf("rule %q did not fire; got %#v", tc.ruleID, findings)
			}
			if f.Confidence != model.ConfidenceHigh {
				t.Fatalf("confidence = %s, want high", f.Confidence)
			}
			if f.Service != tc.service {
				t.Fatalf("service = %q, want %q", f.Service, tc.service)
			}
			if len(f.Evidence) == 0 || len(f.SuggestedFixes) == 0 {
				t.Fatalf("missing evidence/fixes: %+v", f)
			}
		})
	}
}

func TestDefaultRules_CountAtLeast15(t *testing.T) {
	if n := len(rules.DefaultRules()); n < 15 {
		t.Fatalf("DefaultRules() = %d, want >= 15", n)
	}
}
