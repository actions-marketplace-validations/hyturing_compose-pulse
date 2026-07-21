package profiler_test

import (
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/profiler"
)

func TestCaptureTimings(t *testing.T) {
	start := time.Unix(0, 0).UTC()
	run := model.NewRun("r", start)
	run.Services["api"] = &model.Service{
		Name: "api",
		PhaseHistory: []model.PhaseTransition{
			{Phase: model.PhaseCreated, Timestamp: start},
			{Phase: model.PhaseStarted, Timestamp: start.Add(2 * time.Second)},
			{Phase: model.PhaseHealthy, Timestamp: start.Add(5 * time.Second)},
		},
		UpdatedAt: start.Add(5 * time.Second),
	}
	got := profiler.CaptureTimings(run)
	if len(got) != 1 || len(got[0].Phases) != 3 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Phases[0].Duration != 2*time.Second {
		t.Fatalf("created duration = %s", got[0].Phases[0].Duration)
	}
}
