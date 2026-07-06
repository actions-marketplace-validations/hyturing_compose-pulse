package actions

import (
	"reflect"
	"testing"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
)

func TestPlanRestartSelected(t *testing.T) {
	plan := PlanRestartSelected(Project{
		Name:        "app",
		ConfigFiles: []string{"/tmp/compose.yml"},
	}, "api")

	if plan.RequiresConfirm {
		t.Fatal("single-service restart should not require confirmation")
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	want := []string{"compose", "-p", "app", "-f", "/tmp/compose.yml", "restart", "api"}
	if !reflect.DeepEqual(plan.Steps[0].Command.Args, want) {
		t.Fatalf("args = %#v, want %#v", plan.Steps[0].Command.Args, want)
	}
}

func TestPlanRestartDependentsUsesTopologicalOrder(t *testing.T) {
	graph := buildActionTestGraph(t)
	plan := PlanRestartDependents(Project{Name: "app"}, graph, "db")

	if !plan.RequiresConfirm {
		t.Fatal("multi-service restart should require confirmation")
	}
	got := stepServices(plan)
	want := []string{"api", "worker", "gateway"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("restart order = %#v, want %#v", got, want)
	}
}

func TestPlanRebuildSelectedUsesNoDepsBuild(t *testing.T) {
	plan := PlanRebuildSelected(Project{Name: "app"}, "api")

	want := []string{"compose", "-p", "app", "up", "-d", "--no-deps", "--build", "api"}
	if !reflect.DeepEqual(plan.Steps[0].Command.Args, want) {
		t.Fatalf("args = %#v, want %#v", plan.Steps[0].Command.Args, want)
	}
}

func TestPlanRebuildAndRestartDependents(t *testing.T) {
	graph := buildActionTestGraph(t)
	plan := PlanRebuildAndRestartDependents(Project{Name: "app"}, graph, "api")

	if !plan.RequiresConfirm {
		t.Fatal("rebuild with dependents should require confirmation")
	}
	got := stepServices(plan)
	want := []string{"api", "worker", "gateway"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("step services = %#v, want %#v", got, want)
	}
	if plan.Steps[0].Command.Args[3] != "up" {
		t.Fatalf("first step should rebuild selected service, got %#v", plan.Steps[0].Command.Args)
	}
}

func TestPlanExecShellStoresContainerForInTUIExec(t *testing.T) {
	plan := PlanExecShell("abc123")

	if len(plan.Steps) != 1 || plan.Steps[0].Service != "abc123" {
		t.Fatalf("expected exec shell to store container id, got %#v", plan.Steps)
	}
}

func TestPlanExecCommandRunsShellCommandWithoutTTY(t *testing.T) {
	plan := PlanExecCommand("abc123", "pwd")

	want := []string{"exec", "abc123", "sh", "-lc", "pwd"}
	if !reflect.DeepEqual(plan.Steps[0].Command.Args, want) {
		t.Fatalf("args = %#v, want %#v", plan.Steps[0].Command.Args, want)
	}
}

func buildActionTestGraph(t *testing.T) *dag.Graph {
	t.Helper()
	graph, err := dag.Build(&compose.Config{
		Services: map[string]compose.Service{
			"db":      {},
			"api":     {DependsOn: compose.DependsOn{"db": {}}},
			"worker":  {DependsOn: compose.DependsOn{"api": {}}},
			"gateway": {DependsOn: compose.DependsOn{"api": {}, "worker": {}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func stepServices(plan Plan) []string {
	services := make([]string, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		services = append(services, step.Service)
	}
	return services
}
