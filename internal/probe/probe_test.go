package probe

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestRunWarnsWhenHealthcheckMissing(t *testing.T) {
	report := Run(context.Background(), "api", nil, nil, nil)

	if report.Service != "api" {
		t.Fatalf("service = %q, want api", report.Service)
	}
	if len(report.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(report.Steps))
	}
	if report.Steps[0].Status != StepWarn {
		t.Fatalf("status = %v, want warn", report.Steps[0].Status)
	}
	if len(report.Suggestions) == 0 {
		t.Fatal("expected a suggestion")
	}
}

func TestRunExecutesCMDHealthcheck(t *testing.T) {
	exec := scriptedExec(t, map[string]execResult{
		"sh -c command -v curl":       {output: "/usr/bin/curl\n", exitCode: 0},
		"curl -f http://localhost:80": {output: "ok\n", exitCode: 0},
	})
	hc := &docker.HealthcheckSpec{Test: []string{"CMD", "curl", "-f", "http://localhost:80"}}

	report := Run(context.Background(), "web", hc, nil, exec)

	assertStep(t, report, "healthcheck binary", StepPass)
	assertStep(t, report, "healthcheck command", StepPass)
}

func TestRunExecutesCMDSHELLHealthcheck(t *testing.T) {
	exec := scriptedExec(t, map[string]execResult{
		"sh -c command -v curl":                  {output: "/usr/bin/curl\n", exitCode: 0},
		"sh -c curl -f http://localhost:8080/hc": {output: "ok\n", exitCode: 0},
	})
	hc := &docker.HealthcheckSpec{Test: []string{"CMD-SHELL", "curl -f http://localhost:8080/hc"}}

	report := Run(context.Background(), "web", hc, nil, exec)

	assertStep(t, report, "healthcheck binary", StepPass)
	assertStep(t, report, "healthcheck command", StepPass)
}

func TestRunFailsWhenHealthcheckBinaryMissing(t *testing.T) {
	exec := scriptedExec(t, map[string]execResult{
		"sh -c command -v pg_isready": {exitCode: 1},
	})
	hc := &docker.HealthcheckSpec{Test: []string{"CMD", "pg_isready"}}

	report := Run(context.Background(), "db", hc, nil, exec)

	assertStep(t, report, "healthcheck binary", StepFail)
	if !hasSuggestion(report, "pg_isready") {
		t.Fatalf("suggestions = %v, want missing binary mention", report.Suggestions)
	}
}

func TestRunWarnsWhenShellUnavailableAndSkipsRest(t *testing.T) {
	exec := scriptedExec(t, map[string]execResult{
		"sh -c command -v curl": {exitCode: 127, err: errors.New("exec: sh: not found")},
	})
	hc := &docker.HealthcheckSpec{Test: []string{"CMD", "curl", "-f", "http://localhost:80"}}

	report := Run(context.Background(), "web", hc, []string{"8080:80/tcp"}, exec)

	assertStep(t, report, "container shell", StepWarn)
	if len(report.Steps) != 1 {
		t.Fatalf("steps = %v, want shell warning only", report.Steps)
	}
}

func TestRunChecksPrivateContainerPorts(t *testing.T) {
	exec := scriptedExec(t, map[string]execResult{
		"sh -c command -v curl":       {output: "/usr/bin/curl\n", exitCode: 0},
		"curl -f http://localhost:80": {output: "ok\n", exitCode: 0},
		"sh -c nc -z 127.0.0.1 80":    {exitCode: 0},
		"sh -c nc -z 127.0.0.1 5432":  {output: "refused\n", exitCode: 1},
	})
	hc := &docker.HealthcheckSpec{Test: []string{"CMD", "curl", "-f", "http://localhost:80"}}

	report := Run(context.Background(), "web", hc, []string{"8080:80/tcp", "5432/tcp"}, exec)

	assertStep(t, report, "port 80/tcp", StepPass)
	assertStep(t, report, "port 5432/tcp", StepFail)
}

func TestRunWarnsForNonLocalhostHealthcheckURL(t *testing.T) {
	exec := scriptedExec(t, map[string]execResult{
		"sh -c command -v curl":             {output: "/usr/bin/curl\n", exitCode: 0},
		"curl -f http://10.1.2.3:8080/live": {output: "ok\n", exitCode: 0},
	})
	hc := &docker.HealthcheckSpec{Test: []string{"CMD", "curl", "-f", "http://10.1.2.3:8080/live"}}

	report := Run(context.Background(), "web", hc, nil, exec)

	assertStep(t, report, "healthcheck URL host", StepWarn)
	if !hasSuggestion(report, "localhost") {
		t.Fatalf("suggestions = %v, want localhost suggestion", report.Suggestions)
	}
}

type execResult struct {
	output   string
	exitCode int
	err      error
}

func scriptedExec(t *testing.T, script map[string]execResult) Exec {
	t.Helper()
	return func(_ context.Context, cmd []string) (string, int, error) {
		key := strings.Join(cmd, " ")
		result, ok := script[key]
		if !ok {
			t.Fatalf("unexpected command %q", key)
		}
		return result.output, result.exitCode, result.err
	}
}

func assertStep(t *testing.T, report *Report, label string, status StepStatus) {
	t.Helper()
	for _, step := range report.Steps {
		if step.Label == label {
			if step.Status != status {
				t.Fatalf("%s status = %v, want %v", label, step.Status, status)
			}
			return
		}
	}
	t.Fatalf("missing step %q in %#v", label, report.Steps)
}

func hasSuggestion(report *Report, needle string) bool {
	for _, suggestion := range report.Suggestions {
		if strings.Contains(suggestion, needle) {
			return true
		}
	}
	return false
}
