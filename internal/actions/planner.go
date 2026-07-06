package actions

import (
	"fmt"

	"github.com/hyturing/compose-pulse/internal/dag"
)

// Project contains Compose metadata needed to run project-scoped commands.
type Project struct {
	Name        string
	ConfigFiles []string
}

// Command is an executable program plus arguments.
type Command struct {
	Program string
	Args    []string
}

// Step is one command in an action plan.
type Step struct {
	Service string
	Label   string
	Command Command
}

// Plan describes a dependency-aware action and the commands needed to run it.
type Plan struct {
	Title           string
	Steps           []Step
	RequiresConfirm bool
	ConfirmText     string
}

// PlanRestartSelected builds a plan to restart one Compose service.
func PlanRestartSelected(project Project, service string) Plan {
	return Plan{
		Title: "Restart " + service,
		Steps: []Step{{
			Service: service,
			Label:   "restart " + service,
			Command: composeCommand(project, "restart", service),
		}},
	}
}

// PlanRestartDependents builds a plan to restart downstream dependents.
func PlanRestartDependents(project Project, graph *dag.Graph, service string) Plan {
	dependents := downstreamServices(graph, service)
	steps := make([]Step, 0, len(dependents))
	for _, dep := range dependents {
		steps = append(steps, Step{
			Service: dep,
			Label:   "restart " + dep,
			Command: composeCommand(project, "restart", dep),
		})
	}
	return Plan{
		Title:           "Restart dependents of " + service,
		Steps:           steps,
		RequiresConfirm: len(steps) > 0,
		ConfirmText:     fmt.Sprintf("Restart %d dependent service(s)?", len(steps)),
	}
}

// PlanRebuildSelected builds a plan to rebuild one service without dependencies.
func PlanRebuildSelected(project Project, service string) Plan {
	return Plan{
		Title: "Rebuild " + service,
		Steps: []Step{{
			Service: service,
			Label:   "rebuild " + service,
			Command: composeCommand(project, "up", "-d", "--no-deps", "--build", service),
		}},
	}
}

// PlanRebuildAndRestartDependents rebuilds a service then restarts dependents.
func PlanRebuildAndRestartDependents(project Project, graph *dag.Graph, service string) Plan {
	steps := []Step{{
		Service: service,
		Label:   "rebuild " + service,
		Command: composeCommand(project, "up", "-d", "--no-deps", "--build", service),
	}}
	for _, dep := range downstreamServices(graph, service) {
		steps = append(steps, Step{
			Service: dep,
			Label:   "restart " + dep,
			Command: composeCommand(project, "restart", dep),
		})
	}
	return Plan{
		Title:           "Rebuild " + service + " and restart dependents",
		Steps:           steps,
		RequiresConfirm: len(steps) > 1,
		ConfirmText:     fmt.Sprintf("Run %d rebuild/restart step(s)?", len(steps)),
	}
}

// PlanExecShell builds a plan to open a shell in a running container.
func PlanExecShell(containerID string) Plan {
	return Plan{
		Title: "Exec shell",
		Steps: []Step{{
			Service: containerID,
			Label:   "exec shell",
		}},
	}
}

// PlanExecCommand builds a one-shot in-TUI docker exec command.
func PlanExecCommand(containerID, command string) Plan {
	return Plan{
		Title: "Exec",
		Steps: []Step{{
			Service: containerID,
			Label:   "$ " + command,
			Command: Command{
				Program: "docker",
				Args:    []string{"exec", containerID, "sh", "-lc", command},
			},
		}},
	}
}

func composeCommand(project Project, args ...string) Command {
	out := []string{"compose"}
	if project.Name != "" {
		out = append(out, "-p", project.Name)
	}
	for _, path := range project.ConfigFiles {
		out = append(out, "-f", path)
	}
	out = append(out, args...)
	return Command{Program: "docker", Args: out}
}

func downstreamServices(graph *dag.Graph, service string) []string {
	if graph == nil {
		return nil
	}
	start, ok := graph.ByName[service]
	if !ok || start == nil {
		return nil
	}
	seen := map[string]bool{service: true}
	var visit func(*dag.Node)
	visit = func(node *dag.Node) {
		for _, child := range node.Children {
			if child == nil || seen[child.Name] {
				continue
			}
			seen[child.Name] = true
			visit(child)
		}
	}
	visit(start)

	var out []string
	for _, node := range graph.Ordered {
		if node != nil && node.Name != service && seen[node.Name] {
			out = append(out, node.Name)
		}
	}
	return out
}
