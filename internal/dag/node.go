package dag

import "github.com/hyturing/compose-pulse/internal/docker"

// Node represents a single service in the dependency graph.
type Node struct {
	Name          string
	Deps          []string          // all dependency names (sorted, for display)
	DepConditions map[string]string // dep name → condition (e.g. "service_healthy")
	Children      []*Node           // all services that depend on this node (full edge set, used by Kahn's)
	TreeChildren  []*Node           // dependants for which this node is the primary parent (used by renderer)
	State         docker.ContainerState
	Level         int    // render depth: root = 0
	ContainerID   string // Docker container ID (empty when pending / not yet running)

	// Runtime metadata copied from ContainerInfo each poll:
	ExitCode  *int
	Image     string
	Ports     []string
	CreatedAt int64 // unix seconds; 0 when no container exists yet
}
