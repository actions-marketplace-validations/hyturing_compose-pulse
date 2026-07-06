package dag

import (
	"testing"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestEffectiveState_ServiceHealthyBlocked(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"postgres": {},
			"api": {
				DependsOn: compose.DependsOn{
					"postgres": {Condition: "service_healthy"},
				},
			},
		},
	}
	g, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	g.ByName["postgres"].ContainerID = "pg1"
	g.ByName["postgres"].State = docker.StateStarting
	g.ByName["api"].ContainerID = "api1"
	g.ByName["api"].State = docker.StateHealthy

	state, waiting := EffectiveState(g.ByName["api"], g)
	if state != docker.StatePending {
		t.Errorf("expected pending, got %v", state)
	}
	if len(waiting) != 1 || waiting[0] != "postgres" {
		t.Errorf("expected waiting on postgres, got %v", waiting)
	}
}

func TestEffectiveState_ServiceHealthySatisfied(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"postgres": {},
			"api": {
				DependsOn: compose.DependsOn{
					"postgres": {Condition: "service_healthy"},
				},
			},
		},
	}
	g, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	g.ByName["postgres"].ContainerID = "pg1"
	g.ByName["postgres"].State = docker.StateHealthy
	g.ByName["api"].ContainerID = "api1"
	g.ByName["api"].State = docker.StateStarting

	state, waiting := EffectiveState(g.ByName["api"], g)
	if state != docker.StateStarting {
		t.Errorf("expected starting, got %v", state)
	}
	if len(waiting) != 0 {
		t.Errorf("expected no waiting deps, got %v", waiting)
	}
}

func TestEffectiveState_NotStartedYet(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"postgres": {},
			"api": {
				DependsOn: compose.DependsOn{
					"postgres": {Condition: "service_started"},
				},
			},
		},
	}
	g, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	g.ByName["postgres"].ContainerID = "pg1"
	g.ByName["postgres"].State = docker.StateHealthy

	state, waiting := EffectiveState(g.ByName["api"], g)
	if state != docker.StatePending {
		t.Errorf("expected pending, got %v", state)
	}
	if len(waiting) != 0 {
		t.Errorf("expected not-started (no blockers), got waiting %v", waiting)
	}
}

func TestEffectiveState_DiamondDeps(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"postgres": {},
			"redis":    {},
			"api": {
				DependsOn: compose.DependsOn{
					"postgres": {Condition: "service_healthy"},
					"redis":    {Condition: "service_healthy"},
				},
			},
		},
	}
	g, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	g.ByName["postgres"].ContainerID = "pg"
	g.ByName["postgres"].State = docker.StateHealthy
	g.ByName["redis"].ContainerID = "rd"
	g.ByName["redis"].State = docker.StateStarting

	state, waiting := EffectiveState(g.ByName["api"], g)
	if state != docker.StatePending {
		t.Errorf("expected pending, got %v", state)
	}
	if len(waiting) != 1 || waiting[0] != "redis" {
		t.Errorf("expected waiting on redis, got %v", waiting)
	}
}
