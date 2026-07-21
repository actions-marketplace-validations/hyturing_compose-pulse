package testsupport_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/testsupport"
)

func TestScenariosExist(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "scenarios")
	want := []string{
		"crash-loop",
		"unhealthy",
		"missing-dependency",
		"invalid-healthcheck",
		"port-conflict",
		"bind-mount-failure",
		"missing-env-var",
	}
	for _, id := range want {
		compose := filepath.Join(root, id, "compose.yml")
		readme := filepath.Join(root, id, "README.md")
		if _, err := os.Stat(compose); err != nil {
			t.Errorf("missing %s: %v", compose, err)
		}
		if _, err := os.Stat(readme); err != nil {
			t.Errorf("missing %s: %v", readme, err)
		}
	}
}

func TestSyntheticRunNoDocker(t *testing.T) {
	run := testsupport.SyntheticRun("demo", map[string]model.ServicePhase{
		"api": model.PhaseFailed,
		"db":  model.PhaseHealthy,
	})
	if run.Services["demo/api"].Phase != model.PhaseFailed {
		t.Fatal("expected failed api")
	}
}
