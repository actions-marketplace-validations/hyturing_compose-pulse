package main

import (
	"strings"
	"testing"

	"github.com/hyturing/compose-pulse/internal/doctor"
)

func TestPrintDoctorReport(t *testing.T) {
	var b strings.Builder
	root := &doctor.RootCause{
		Culprits:     []string{"postgres"},
		CriticalPath: []string{"postgres", "api", "worker"},
		FirstLog:     map[string]string{"postgres": "FATAL: password authentication failed"},
	}
	findings := []doctor.Finding{{
		RuleID:     "unhealthy-service",
		Severity:   doctor.SeverityCritical,
		Service:    "postgres",
		Title:      "postgres is unhealthy",
		Evidence:   []string{"pg_isready → exit 1"},
		Suggestion: []string{"Run the health probe"},
	}}
	printDoctorReport(&b, "shop", root, findings)
	out := b.String()
	for _, want := range []string{
		"Project: shop",
		"Root cause:",
		"postgres",
		"Critical path: postgres → api → worker",
		"[critical] unhealthy-service · postgres",
		"→ Run the health probe",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
