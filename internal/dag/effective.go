package dag

import "github.com/hyturing/compose-pulse/internal/docker"

// EffectiveState returns the display state for a node, accounting for depends_on
// conditions. waitingOn lists unsatisfied dependency names when blocked.
func EffectiveState(node *Node, g *Graph) (docker.ContainerState, []string) {
	var waitingOn []string
	for _, depName := range node.Deps {
		condition := node.DepConditions[depName]
		if condition == "" {
			condition = "service_started"
		}
		dep := g.ByName[depName]
		if dep == nil || !dependencySatisfied(dep, condition) {
			waitingOn = append(waitingOn, depName)
		}
	}
	if len(waitingOn) > 0 {
		return docker.StatePending, waitingOn
	}
	if node.ContainerID == "" {
		return docker.StatePending, nil
	}
	return node.State, nil
}

func dependencySatisfied(dep *Node, condition string) bool {
	switch condition {
	case "service_healthy":
		return dep.State == docker.StateHealthy
	case "service_completed_successfully":
		return dep.State == docker.StateExited && dep.ExitCode != nil && *dep.ExitCode == 0
	default:
		if dep.ContainerID == "" {
			return false
		}
		switch dep.State {
		case docker.StateHealthy, docker.StateStarting, docker.StateUnhealthy:
			return true
		default:
			return false
		}
	}
}
