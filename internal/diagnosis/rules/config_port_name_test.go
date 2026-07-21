package rules_test

import (
	"testing"

	"github.com/hyturing/compose-pulse/internal/model"
)

func TestConfigPortNameRules_FireOnFixtures(t *testing.T) {
	cases := []struct {
		runID   string
		ruleID  string
		service string
	}{
		{"config.missing_env_var", "config.missing_env_var", "app"},
		{"config.invalid_compose", "config.invalid_compose", ""},
		{"port.host_occupied", "port.host_occupied", "b"},
		{"name.container_conflict", "name.container_conflict", "db"},
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
			if tc.service != "" && f.Service != tc.service {
				t.Fatalf("service = %q, want %q", f.Service, tc.service)
			}
			if len(f.Evidence) == 0 {
				t.Fatal("expected evidence")
			}
			if len(f.SuggestedFixes) == 0 {
				t.Fatal("expected suggested fix")
			}
		})
	}
}

func TestConfigPortNameRules_StaySilentOnUnrelated(t *testing.T) {
	// missing_env_var fixture must not trigger port/name rules
	ctx := loadPhase2Run(t, "config.missing_env_var")
	findings := evaluateAll(ctx)
	for _, id := range []string{"port.host_occupied", "name.container_conflict", "config.invalid_compose"} {
		if f := findingByRule(findings, id); f != nil {
			t.Fatalf("rule %q falsely fired on missing_env_var: %+v", id, f)
		}
	}
}
