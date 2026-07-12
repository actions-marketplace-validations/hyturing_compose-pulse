package doctor

import (
	"reflect"
	"testing"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestFindRootCause_LinearBlockedChain(t *testing.T) {
	ctx := testContext(t, map[string]compose.Service{
		"postgres": {},
		"api": {
			DependsOn: compose.DependsOn{"postgres": {Condition: "service_healthy"}},
		},
		"worker": {
			DependsOn: compose.DependsOn{"api": {Condition: "service_healthy"}},
		},
	})
	setNode(ctx, "postgres", "pg1", docker.StateUnhealthy, nil)
	setNode(ctx, "api", "api1", docker.StateHealthy, nil)
	setNode(ctx, "worker", "worker1", docker.StateStarting, nil)
	ctx.Logs = func(containerID string, tail int) ([]string, error) {
		return []string{"booting", "fatal: auth failed"}, nil
	}

	got := FindRootCause(ctx)

	if got == nil {
		t.Fatal("FindRootCause() = nil, want root cause")
	}
	if want := []string{"postgres"}; !reflect.DeepEqual(got.Culprits, want) {
		t.Fatalf("Culprits = %v, want %v", got.Culprits, want)
	}
	if want := []string{"postgres", "api", "worker"}; !reflect.DeepEqual(got.CriticalPath, want) {
		t.Fatalf("CriticalPath = %v, want %v", got.CriticalPath, want)
	}
	if want := []string{"worker", "api", "postgres"}; !reflect.DeepEqual(got.Chains["worker"], want) {
		t.Fatalf("worker chain = %v, want %v", got.Chains["worker"], want)
	}
	if got.FirstLog["postgres"] != "fatal: auth failed" {
		t.Fatalf("FirstLog[postgres] = %q", got.FirstLog["postgres"])
	}
}

func TestFindRootCause_DiamondWithOneCulprit(t *testing.T) {
	ctx := testContext(t, map[string]compose.Service{
		"postgres": {},
		"redis":    {},
		"api": {
			DependsOn: compose.DependsOn{
				"postgres": {Condition: "service_healthy"},
				"redis":    {Condition: "service_healthy"},
			},
		},
		"worker": {
			DependsOn: compose.DependsOn{
				"postgres": {Condition: "service_healthy"},
				"redis":    {Condition: "service_healthy"},
			},
		},
		"frontend": {
			DependsOn: compose.DependsOn{"api": {Condition: "service_healthy"}},
		},
	})
	setNode(ctx, "postgres", "pg1", docker.StateUnhealthy, nil)
	setNode(ctx, "redis", "redis1", docker.StateHealthy, nil)
	setNode(ctx, "api", "api1", docker.StateHealthy, nil)
	setNode(ctx, "worker", "worker1", docker.StateHealthy, nil)
	setNode(ctx, "frontend", "front1", docker.StateHealthy, nil)

	got := FindRootCause(ctx)

	if got == nil {
		t.Fatal("FindRootCause() = nil, want root cause")
	}
	if want := []string{"postgres"}; !reflect.DeepEqual(got.Culprits, want) {
		t.Fatalf("Culprits = %v, want %v", got.Culprits, want)
	}
	for _, blocked := range []string{"api", "worker", "frontend"} {
		if chain := got.Chains[blocked]; len(chain) == 0 || chain[len(chain)-1] != "postgres" {
			t.Fatalf("%s chain = %v, want endpoint postgres", blocked, chain)
		}
	}
	if got.Chains["frontend"][0] != "frontend" {
		t.Fatalf("frontend chain = %v, want blocked-first chain", got.Chains["frontend"])
	}
	if want := []string{"postgres", "api", "frontend"}; !reflect.DeepEqual(got.CriticalPath, want) {
		t.Fatalf("CriticalPath = %v, want %v", got.CriticalPath, want)
	}
}

func TestFindRootCause_RestartLoopIsCulprit(t *testing.T) {
	ctx := testContext(t, map[string]compose.Service{
		"db":  {},
		"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
	})
	setNode(ctx, "db", "db1", docker.StateStarting, nil)
	ctx.Inspect = inspectMap(map[string]*docker.InspectInfo{"db1": {RestartCount: 3}})

	got := FindRootCause(ctx)

	if got == nil {
		t.Fatal("FindRootCause() = nil, want root cause")
	}
	if want := []string{"db"}; !reflect.DeepEqual(got.Culprits, want) {
		t.Fatalf("Culprits = %v, want %v", got.Culprits, want)
	}
}

func TestFindRootCause_ReturnsNilWhenNothingBrokenOrBlocked(t *testing.T) {
	ctx := testContext(t, map[string]compose.Service{
		"db":  {},
		"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
	})

	if got := FindRootCause(ctx); got != nil {
		t.Fatalf("FindRootCause() = %#v, want nil", got)
	}
}
