package rules_test

import (
	"testing"
)

func TestAdversarialFixtures_DoNotFalsePositive(t *testing.T) {
	cases := []struct {
		runID string
		rules []string
	}{
		{
			runID: "adversarial.eventually_healthy",
			rules: []string{"race.depends_on_started", "net.localhost_in_container", "restart.identical_exit_loop"},
		},
		{
			runID: "adversarial.exit_137_sigkill",
			rules: []string{"process.oom_killed", "resource.oom"},
		},
		{
			runID: "adversarial.stale_port_log",
			rules: []string{"port.host_occupied"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.runID, func(t *testing.T) {
			ctx := loadPhase2Run(t, tc.runID)
			findings := evaluateAll(ctx)
			for _, id := range tc.rules {
				if f := findingByRule(findings, id); f != nil {
					t.Fatalf("false positive %q on %s: %+v", id, tc.runID, f)
				}
			}
		})
	}
}
