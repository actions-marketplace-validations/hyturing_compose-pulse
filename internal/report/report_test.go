package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/report"
	"github.com/hyturing/compose-pulse/internal/report/html"
	jsonreport "github.com/hyturing/compose-pulse/internal/report/json"
	"github.com/hyturing/compose-pulse/internal/report/logs"
	"github.com/hyturing/compose-pulse/internal/report/markdown"
	"github.com/hyturing/compose-pulse/internal/report/sarif"
)

func sampleRun() *model.Run {
	start := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	run := model.NewRun("demo-failed", start)
	run.Project = "demo"
	run.Invocation = &model.Invocation{
		Command:       []string{"docker", "compose", "up"},
		EnvNames:      []string{"PATH", "DB_PASSWORD"},
		DockerVersion: "28.0.0",
	}
	run.EffectiveConfig = &model.EffectiveConfig{
		Services: []model.EffectiveService{
			{Name: "db"},
			{Name: "api", DependsOn: map[string]string{"db": "service_started"}},
		},
	}
	run.ApplyEvents([]model.Event{
		{Timestamp: start.Add(time.Second), Service: "db", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "Exited (1)", Source: model.SourceContainer},
		{Timestamp: start.Add(2 * time.Second), Service: "api", Phase: model.PhaseFailed, Type: model.EventTypeState, Severity: model.SeverityError, Message: "blocked", Source: model.SourceContainer},
		{Timestamp: start.Add(3 * time.Second), Service: "api", Phase: model.PhaseFailed, Type: model.EventTypeLog, Severity: model.SeverityError, Message: "password=supersecret connection refused", Source: model.SourceLog},
		{Timestamp: start.Add(4 * time.Second), Service: "api", Phase: model.PhaseFailed, Type: model.EventTypeLog, Severity: model.SeverityError, Message: "password=supersecret connection refused", Source: model.SourceLog},
		{Timestamp: start.Add(5 * time.Second), Service: "api", Phase: model.PhaseFailed, Type: model.EventTypeLog, Severity: model.SeverityError, Message: "password=supersecret connection refused", Source: model.SourceLog},
	})
	run.Findings = []model.Finding{{
		RuleID:          "process.exit_127",
		Service:         "db",
		RootCause:       "command not found (exit 127)",
		Confidence:      model.ConfidenceHigh,
		BlockedServices: []string{"api"},
		SuggestedFixes:  []string{"Fix the entrypoint"},
		Evidence:        []string{"exit_code=127"},
	}}
	return run
}

func TestBuildAndRenderFormats(t *testing.T) {
	run := sampleRun()
	rep := report.Build(run, run.Findings)
	rep.GeneratedAt = time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	if rep.RootCause == "" || rep.Reproduction == "" {
		t.Fatalf("report incomplete: %+v", rep)
	}
	if len(rep.LogWindows) == 0 {
		t.Fatal("expected log windows")
	}
	if len(rep.Redaction) == 0 {
		t.Fatal("expected redaction summary")
	}

	md := markdown.Render(rep)
	if !strings.Contains(md, "process.exit_127") {
		t.Fatalf("markdown missing rule id:\n%s", md)
	}
	htmlOut := html.Render(rep)
	js, err := jsonreport.MarshalReport(rep)
	if err != nil {
		t.Fatal(err)
	}
	sarifOut, err := sarif.Render(rep)
	if err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]string{
		"markdown": md,
		"html":     htmlOut,
		"json":     string(js),
		"sarif":    string(sarifOut),
	} {
		if strings.Contains(body, "supersecret") {
			t.Fatalf("%s leaked secret", name)
		}
		if logs.ContainsSecrets(body) && !strings.Contains(body, "[REDACTED]") {
			t.Fatalf("%s still looks secretful", name)
		}
	}

	var sarifDoc map[string]any
	if err := json.Unmarshal(sarifOut, &sarifDoc); err != nil {
		t.Fatal(err)
	}
	if sarifDoc["version"] != "2.1.0" {
		t.Fatalf("sarif version = %v", sarifDoc["version"])
	}

	dir := filepath.Join("..", "..", "testdata", "golden")
	_ = os.MkdirAll(dir, 0o755)
	writeGolden(t, filepath.Join(dir, "report-demo.md"), md)
	writeGolden(t, filepath.Join(dir, "report-demo.html"), htmlOut)
	writeGolden(t, filepath.Join(dir, "report-demo.json"), string(js)+"\n")
	writeGolden(t, filepath.Join(dir, "report-demo.sarif"), string(sarifOut)+"\n")
}

func writeGolden(t *testing.T, path, body string) {
	t.Helper()
	// Stabilize generated_at in JSON for golden
	if strings.HasSuffix(path, ".json") {
		var m map[string]any
		if err := json.Unmarshal([]byte(body), &m); err == nil {
			m["generated_at"] = "2026-07-22T00:00:00Z"
			if b, err := json.MarshalIndent(m, "", "  "); err == nil {
				body = string(b) + "\n"
			}
		}
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLogReductionCollapsesRepeats(t *testing.T) {
	run := sampleRun()
	windows, _ := logs.Reduce(run)
	if len(windows) == 0 || windows[0].Count < 3 {
		t.Fatalf("windows = %+v", windows)
	}
	if windows[0].Note == "" {
		t.Fatal("expected collapse note")
	}
}
