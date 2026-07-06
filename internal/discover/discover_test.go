package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestFromContainers_TwoProjects(t *testing.T) {
	containers := []docker.ContainerInfo{
		{
			ID: "1", Labels: map[string]string{
				labelProject: "alpha", labelService: "web",
			}, State: docker.StateHealthy,
		},
		{
			ID: "2", Labels: map[string]string{
				labelProject: "beta", labelService: "api",
			}, State: docker.StateHealthy,
		},
	}

	snap, err := FromContainers(containers)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(snap.Projects))
	}
	if snap.Projects[0].Name != "alpha" || snap.Projects[1].Name != "beta" {
		t.Errorf("unexpected project order: %q, %q", snap.Projects[0].Name, snap.Projects[1].Name)
	}
}

func TestFromContainers_Standalone(t *testing.T) {
	containers := []docker.ContainerInfo{
		{
			ID: "1", Names: []string{"/stray"}, Image: "nginx:alpine",
			Labels: map[string]string{}, State: docker.StateHealthy,
		},
		{
			ID: "2", Labels: map[string]string{
				labelProject: "app", labelService: "api",
			}, State: docker.StateHealthy,
		},
	}

	snap, err := FromContainers(containers)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Standalone) != 1 || snap.Standalone[0].Name != "stray" {
		t.Fatalf("expected standalone stray, got %+v", snap.Standalone)
	}
	if len(snap.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(snap.Projects))
	}
}

func TestFromContainers_DependsOnLabel(t *testing.T) {
	containers := []docker.ContainerInfo{
		{
			ID: "db", Labels: map[string]string{
				labelProject: "app", labelService: "postgres",
			}, State: docker.StateHealthy,
		},
		{
			ID: "api", Labels: map[string]string{
				labelProject:   "app",
				labelService:   "api",
				labelDependsOn: `{"postgres":{"condition":"service_started","required":true}}`,
			}, State: docker.StateHealthy,
		},
	}

	snap, err := FromContainers(containers)
	if err != nil {
		t.Fatal(err)
	}
	g := snap.Projects[0].Graph
	if len(g.Roots) != 1 || g.Roots[0].Name != "postgres" {
		t.Errorf("expected postgres as sole root, got %v", g.Roots)
	}
	if g.ByName["api"].ContainerID != "api" {
		t.Errorf("expected container ID on api node, got %q", g.ByName["api"].ContainerID)
	}
}

func TestFromContainers_ComposeFileFallback(t *testing.T) {
	composePath, err := filepath.Abs("../../testdata/docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}

	containers := []docker.ContainerInfo{
		{
			ID: "pg", Labels: map[string]string{
				labelProject:     "app",
				labelService:     "postgres",
				labelConfigFiles: composePath,
			}, State: docker.StateHealthy,
		},
	}

	snap, err := FromContainers(containers)
	if err != nil {
		t.Fatal(err)
	}
	g := snap.Projects[0].Graph
	if len(g.ByName) != 5 {
		t.Errorf("expected 5 services from compose file fallback, got %d", len(g.ByName))
	}
	if _, ok := g.ByName["frontend"]; !ok {
		t.Error("expected pending frontend from compose file fallback")
	}
	if g.ByName["postgres"].ContainerID != "pg" {
		t.Errorf("expected container ID on postgres, got %q", g.ByName["postgres"].ContainerID)
	}
	if got := snap.Projects[0].ConfigFiles; len(got) != 1 || got[0] != composePath {
		t.Fatalf("expected config files [%q], got %#v", composePath, got)
	}
}

func TestFromContainers_ConfigFilesAreSortedAndDeduped(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.yml")
	second := filepath.Join(dir, "b.yml")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("services:\n  api:\n    image: nginx\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	containers := []docker.ContainerInfo{
		{
			ID: "api",
			Labels: map[string]string{
				labelProject:     "app",
				labelService:     "api",
				labelConfigFiles: second + "," + first + "," + second,
			},
			State: docker.StateHealthy,
		},
	}

	snap, err := FromContainers(containers)
	if err != nil {
		t.Fatal(err)
	}
	got := snap.Projects[0].ConfigFiles
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("config files = %#v, want sorted unique [%q %q]", got, first, second)
	}
}

func TestFromContainers_ComposeFileDoesNotDropDiscovered(t *testing.T) {
	composePath, err := filepath.Abs("../../testdata/docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}

	containers := []docker.ContainerInfo{
		{
			ID: "pg",
			Labels: map[string]string{
				labelProject:     "app",
				labelService:     "postgres",
				labelConfigFiles: composePath,
			},
			State: docker.StateHealthy,
		},
		{
			ID: "extra",
			Labels: map[string]string{
				labelProject: "app", labelService: "worker",
			},
			State: docker.StateHealthy,
		},
	}

	snap, err := FromContainers(containers)
	if err != nil {
		t.Fatal(err)
	}
	g := snap.Projects[0].Graph
	if _, ok := g.ByName["worker"]; !ok {
		t.Error("expected discovered worker service to remain after compose file merge")
	}
	if _, ok := g.ByName["frontend"]; !ok {
		t.Error("expected pending frontend from compose file merge")
	}
}

func TestParseDependsOnLabel_ColonFormat(t *testing.T) {
	deps := parseDependsOnLabel("postgres:service_healthy:false,redis:service_healthy:false")
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	if deps["postgres"].Condition != "service_healthy" {
		t.Errorf("postgres condition = %q, want service_healthy", deps["postgres"].Condition)
	}
	if deps["redis"].Condition != "service_healthy" {
		t.Errorf("redis condition = %q, want service_healthy", deps["redis"].Condition)
	}
}

func TestParseDependsOnLabel_JSONPreservesCondition(t *testing.T) {
	raw := `{"postgres":{"condition":"service_healthy","required":true}}`
	deps := parseDependsOnLabel(raw)
	if deps["postgres"].Condition != "service_healthy" {
		t.Errorf("condition = %q, want service_healthy", deps["postgres"].Condition)
	}
}

func TestFromContainers_AssignsState(t *testing.T) {
	containers := []docker.ContainerInfo{
		{
			ID: "db", Labels: map[string]string{
				labelProject: "app", labelService: "postgres",
			}, State: docker.StateHealthy,
		},
	}
	snap, err := FromContainers(containers)
	if err != nil {
		t.Fatal(err)
	}
	node := snap.Projects[0].Graph.ByName["postgres"]
	if node.State != docker.StateHealthy {
		t.Errorf("expected healthy state on node, got %v", node.State)
	}
}
