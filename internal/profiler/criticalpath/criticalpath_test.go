package criticalpath_test

import (
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/profiler/criticalpath"
)

func TestCompute_Linear(t *testing.T) {
	run := timedRun(t, map[string]time.Duration{
		"postgres": 10 * time.Second,
		"api":      5 * time.Second,
		"worker":   2 * time.Second,
	}, map[string][]string{
		"api":    {"postgres"},
		"worker": {"api"},
	})
	p := criticalpath.Compute(run)
	if p == nil || p.Total != 17*time.Second {
		t.Fatalf("path = %+v", p)
	}
	if len(p.Segments) != 3 || p.Segments[0].Service != "postgres" || p.Segments[2].Service != "worker" {
		t.Fatalf("segments = %+v", p.Segments)
	}
}

func TestCompute_ParallelPicksHeavierBranch(t *testing.T) {
	run := timedRun(t, map[string]time.Duration{
		"db":    3 * time.Second,
		"cache": 8 * time.Second,
		"api":   1 * time.Second,
	}, map[string][]string{
		"api": {"db", "cache"},
	})
	p := criticalpath.Compute(run)
	if p == nil {
		t.Fatal("nil path")
	}
	// heaviest chain is cache → api = 9s (db→api would be 4s)
	if p.Total != 9*time.Second {
		t.Fatalf("total = %s, want 9s; segs=%+v", p.Total, p.Segments)
	}
	if p.Segments[0].Service != "cache" {
		t.Fatalf("first = %s, want cache", p.Segments[0].Service)
	}
}

func timedRun(t *testing.T, costs map[string]time.Duration, deps map[string][]string) *model.Run {
	t.Helper()
	start := time.Unix(0, 0).UTC()
	run := model.NewRun("t", start)
	cfg := &model.EffectiveConfig{}
	for name, d := range costs {
		svcDeps := map[string]string{}
		for _, dep := range deps[name] {
			svcDeps[dep] = "service_started"
		}
		cfg.Services = append(cfg.Services, model.EffectiveService{Name: name, DependsOn: svcDeps})
		run.Services[name] = &model.Service{
			Name: name,
			PhaseHistory: []model.PhaseTransition{
				{Phase: model.PhaseStarted, Timestamp: start},
				{Phase: model.PhaseHealthy, Timestamp: start.Add(d)},
			},
			UpdatedAt: start.Add(d),
			Phase:     model.PhaseHealthy,
		}
	}
	ended := start.Add(30 * time.Second)
	run.EndedAt = &ended
	run.EffectiveConfig = cfg
	return run
}
