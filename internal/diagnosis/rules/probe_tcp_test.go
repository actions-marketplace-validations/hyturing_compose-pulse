package rules_test

import (
	"testing"

	"github.com/hyturing/compose-pulse/internal/model"
)

func TestProbeTCPRefusedRule_FiresHigh(t *testing.T) {
	ctx := loadPhase2Run(t, "probe.tcp_refused")
	findings := evaluateAll(ctx)
	f := findingByRule(findings, "probe.tcp_refused_not_listening")
	if f == nil {
		t.Fatalf("rule did not fire; got %#v", findings)
	}
	if f.Confidence != model.ConfidenceHigh {
		t.Fatalf("confidence = %s", f.Confidence)
	}
	if f.Service != "api" {
		t.Fatalf("service = %q", f.Service)
	}
}
