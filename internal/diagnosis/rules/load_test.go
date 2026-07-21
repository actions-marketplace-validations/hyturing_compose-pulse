package rules_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/rules"
	"github.com/hyturing/compose-pulse/internal/model"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/diagnosis/rules -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func loadPhase2Run(t *testing.T, name string) *engine.RunContext {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "runs", "phase2", name+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var stored model.Run
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	started := stored.StartedAt
	if started.IsZero() {
		started = time.Unix(0, 0).UTC()
	}
	run := model.NewRun(stored.ID, started)
	run.Project = stored.Project
	run.Invocation = stored.Invocation
	run.EffectiveConfig = stored.EffectiveConfig
	run.ApplyEvents(stored.Events)
	return engine.NewRunContext(run)
}

func findingByRule(findings []model.Finding, ruleID string) *model.Finding {
	for i := range findings {
		if findings[i].RuleID == ruleID {
			return &findings[i]
		}
	}
	return nil
}

func evaluateAll(ctx *engine.RunContext) []model.Finding {
	return engine.Evaluate(ctx, rules.DefaultRules())
}
