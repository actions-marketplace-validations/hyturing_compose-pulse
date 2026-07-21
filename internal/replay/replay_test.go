package replay_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/replay"
)

func TestLoadRunJSONDeterministic(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "runs", "demo-failed.json")
	run, err := replay.LoadRunJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if run.SchemaVersion != model.SchemaVersion {
		t.Fatalf("schema=%d", run.SchemaVersion)
	}
	api := run.Services["demo/api"]
	if api == nil || api.Phase != model.PhaseFailed {
		t.Fatalf("api service: %+v", api)
	}
	db := run.Services["demo/db"]
	if db == nil || db.Phase != model.PhaseHealthy {
		t.Fatalf("db service: %+v", db)
	}
}

func TestLoadRunJSON_PreservesEffectiveConfigAndInvocation(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "runs", "phase2", "race.depends_on_started.json")
	run, err := replay.LoadRunJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if run.EffectiveConfig == nil || len(run.EffectiveConfig.Services) == 0 {
		t.Fatalf("EffectiveConfig dropped: %+v", run.EffectiveConfig)
	}
	var api *model.EffectiveService
	for i := range run.EffectiveConfig.Services {
		if run.EffectiveConfig.Services[i].Name == "api" {
			api = &run.EffectiveConfig.Services[i]
			break
		}
	}
	if api == nil || api.DependsOn["postgres"] != "service_started" {
		t.Fatalf("api depends_on lost: %+v", api)
	}
}
