package rules_test

import (
	"testing"

	"github.com/hyturing/compose-pulse/internal/model"
)

func TestMountImageRules_FireOnFixtures(t *testing.T) {
	cases := []struct {
		runID   string
		ruleID  string
		service string
	}{
		{"mount.bind_source_missing", "mount.bind_source_missing", "app"},
		{"image.pull_denied", "image.pull_denied", "api"},
		{"image.manifest_missing", "image.manifest_missing", "api"},
		{"image.platform_mismatch", "image.platform_mismatch", "api"},
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

func TestImageRules_DoNotCrossFire(t *testing.T) {
	ctx := loadPhase2Run(t, "image.pull_denied")
	findings := evaluateAll(ctx)
	if f := findingByRule(findings, "image.manifest_missing"); f != nil {
		t.Fatalf("manifest_missing fired on pull_denied: %+v", f)
	}
	if f := findingByRule(findings, "image.platform_mismatch"); f != nil {
		t.Fatalf("platform_mismatch fired on pull_denied: %+v", f)
	}
}
