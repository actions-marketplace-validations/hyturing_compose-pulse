package dag

import "github.com/hyturing/compose-pulse/internal/docker"

// DisplayState is the derived, user-facing state of a service. It folds
// together raw container state, exit codes, and dependency blocking.
type DisplayState string

// The set of derived display states, from fully healthy to terminal failure.
const (
	DisplayHealthy   DisplayState = "healthy"   // running, healthcheck passing or absent
	DisplayStarting  DisplayState = "starting"  // healthcheck in start period
	DisplayBlocked   DisplayState = "blocked"   // waiting on an unsatisfied depends_on
	DisplayPending   DisplayState = "pending"   // no container yet, no unsatisfied deps known
	DisplayCompleted DisplayState = "completed" // exited 0 (init/migration jobs)
	DisplayFailed    DisplayState = "failed"    // exited non-zero
	DisplayUnhealthy DisplayState = "unhealthy" // running but healthcheck failing
	DisplayDegraded  DisplayState = "degraded"  // reserved for Phase 2 (restart loops)
)

// Display derives the DisplayState for a node plus the list of unsatisfied
// dependency names when blocked.
func Display(n *Node, g *Graph) (DisplayState, []string) {
	eff, waitingOn := EffectiveState(n, g)
	if eff == docker.StatePending {
		if len(waitingOn) > 0 {
			return DisplayBlocked, waitingOn
		}
		return DisplayPending, nil
	}
	switch eff {
	case docker.StateStarting:
		return DisplayStarting, nil
	case docker.StateUnhealthy:
		return DisplayUnhealthy, nil
	case docker.StateExited:
		if n.ExitCode != nil && *n.ExitCode == 0 {
			return DisplayCompleted, nil
		}
		return DisplayFailed, nil
	default:
		return DisplayHealthy, nil
	}
}
