package discover

import (
	"testing"

	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestApplyStatesFrom_PropagatesExitCode(t *testing.T) {
	running := []docker.ContainerInfo{
		{
			ID: "1", Labels: map[string]string{
				labelProject: "app", labelService: "worker",
			}, State: docker.StateHealthy,
		},
	}
	snap, err := FromContainers(running)
	if err != nil {
		t.Fatal(err)
	}

	exitCode := 1
	exited := []docker.ContainerInfo{
		{
			ID: "1", Labels: map[string]string{
				labelProject: "app", labelService: "worker",
			}, State: docker.StateExited, ExitCode: &exitCode, Image: "worker:latest",
			Ports: []string{"8080/tcp"}, Created: 1000,
		},
	}
	next, err := FromContainers(exited)
	if err != nil {
		t.Fatal(err)
	}

	if !snap.SameStructure(next) {
		t.Fatal("expected identical topology between polls")
	}
	snap.ApplyStatesFrom(next)

	node := snap.Projects[0].Graph.ByName["worker"]
	if node.State != docker.StateExited {
		t.Errorf("State = %v, want Exited", node.State)
	}
	if node.ExitCode == nil || *node.ExitCode != 1 {
		t.Errorf("ExitCode = %v, want 1", node.ExitCode)
	}
	if node.Image != "worker:latest" {
		t.Errorf("Image = %q, want worker:latest", node.Image)
	}
	if len(node.Ports) != 1 || node.Ports[0] != "8080/tcp" {
		t.Errorf("Ports = %v, want [8080/tcp]", node.Ports)
	}
	if node.CreatedAt != 1000 {
		t.Errorf("CreatedAt = %d, want 1000", node.CreatedAt)
	}
}
