package dag

import (
	"testing"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func intPtr(i int) *int { return &i }

func TestDisplay_Matrix(t *testing.T) {
	tests := []struct {
		name      string
		state     docker.ContainerState
		exitCode  *int
		hasCtr    bool
		wantState DisplayState
	}{
		{"exited zero", docker.StateExited, intPtr(0), true, DisplayCompleted},
		{"exited nonzero", docker.StateExited, intPtr(1), true, DisplayFailed},
		{"exited nil exit code", docker.StateExited, nil, true, DisplayFailed},
		{"healthy", docker.StateHealthy, nil, true, DisplayHealthy},
		{"unhealthy", docker.StateUnhealthy, nil, true, DisplayUnhealthy},
		{"starting", docker.StateStarting, nil, true, DisplayStarting},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &compose.Config{Services: map[string]compose.Service{"solo": {}}}
			g, err := Build(cfg)
			if err != nil {
				t.Fatal(err)
			}
			node := g.ByName["solo"]
			if tt.hasCtr {
				node.ContainerID = "c1"
			}
			node.State = tt.state
			node.ExitCode = tt.exitCode

			got, waiting := Display(node, g)
			if got != tt.wantState {
				t.Errorf("Display() = %v, want %v", got, tt.wantState)
			}
			if len(waiting) != 0 {
				t.Errorf("expected no waiting deps, got %v", waiting)
			}
		})
	}
}

func TestDisplay_PendingNoContainer(t *testing.T) {
	cfg := &compose.Config{Services: map[string]compose.Service{"solo": {}}}
	g, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, waiting := Display(g.ByName["solo"], g)
	if got != DisplayPending {
		t.Errorf("Display() = %v, want pending", got)
	}
	if len(waiting) != 0 {
		t.Errorf("expected no waiting deps, got %v", waiting)
	}
}

func TestDisplay_Blocked(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db": {},
			"api": {
				DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}},
			},
		},
	}
	g, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	g.ByName["db"].ContainerID = "d1"
	g.ByName["db"].State = docker.StateStarting

	got, waiting := Display(g.ByName["api"], g)
	if got != DisplayBlocked {
		t.Errorf("Display() = %v, want blocked", got)
	}
	if len(waiting) != 1 || waiting[0] != "db" {
		t.Errorf("waiting = %v, want [db]", waiting)
	}
}

func TestDisplay_ServiceCompletedSuccessfully_FailedDepBlocksDependent(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"migrate": {},
			"api": {
				DependsOn: compose.DependsOn{"migrate": {Condition: "service_completed_successfully"}},
			},
		},
	}
	g, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	g.ByName["migrate"].ContainerID = "m1"
	g.ByName["migrate"].State = docker.StateExited
	g.ByName["migrate"].ExitCode = intPtr(1)

	got, waiting := Display(g.ByName["api"], g)
	if got != DisplayBlocked {
		t.Errorf("Display() = %v, want blocked", got)
	}
	if len(waiting) != 1 || waiting[0] != "migrate" {
		t.Errorf("waiting = %v, want [migrate]", waiting)
	}
}
