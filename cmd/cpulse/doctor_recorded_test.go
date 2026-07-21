package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/rules"
	"github.com/hyturing/compose-pulse/internal/report"
	jsonreport "github.com/hyturing/compose-pulse/internal/report/json"
	"github.com/hyturing/compose-pulse/internal/report/sarif"
)

func phase2Fixture(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "runs", "phase2", name+".json")
}

func TestDoctorRecorded_ExitFailureOnHighFinding(t *testing.T) {
	err := cmdDoctorRecorded(doctorRecordedOpts{
		file:    phase2Fixture(t, "config.missing_env_var"),
		jsonOut: true,
		failOn:  "high",
	})
	var e exitCodeError
	if !errors.As(err, &e) || e.code != exitFailure {
		t.Fatalf("err = %v, want exitFailure", err)
	}
}

func TestDoctorRecorded_JSONAndSARIFStableShape(t *testing.T) {
	run, err := loadRecordedRun("", "", "", phase2Fixture(t, "config.missing_env_var"), false)
	if err != nil {
		t.Fatal(err)
	}
	findings := engine.Diagnose(run, rules.DefaultRules())
	rep := report.Build(run, findings)

	js, err := jsonreport.MarshalReport(rep)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(js, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["schema_version"]; !ok {
		t.Fatalf("json missing schema_version: %s", js)
	}
	if !strings.Contains(string(js), "config.missing_env_var") && !strings.Contains(string(js), "required environment") {
		t.Fatalf("json missing diagnosis: %s", js)
	}

	sarifBody, err := sarif.Render(rep)
	if err != nil {
		t.Fatal(err)
	}
	var sdoc map[string]any
	if err := json.Unmarshal(sarifBody, &sdoc); err != nil {
		t.Fatal(err)
	}
	if sdoc["version"] != "2.1.0" {
		t.Fatalf("sarif version = %v", sdoc["version"])
	}
}

func TestWriteGitHubAnnotations(t *testing.T) {
	run, err := loadRecordedRun("", "", "", phase2Fixture(t, "config.missing_env_var"), false)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	writeGitHubAnnotations(&b, engine.Diagnose(run, rules.DefaultRules()))
	out := b.String()
	if !strings.Contains(out, "::error") {
		t.Fatalf("annotations = %q", out)
	}
}
