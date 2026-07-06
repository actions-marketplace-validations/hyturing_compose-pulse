package discover

import (
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/docker"
)

// Snapshot is the full discovery result shown in the TUI.
type Snapshot struct {
	Projects   []Project
	Standalone []Standalone
}

// Project is a single Docker Compose stack with a dependency graph.
type Project struct {
	Name        string
	Graph       *dag.Graph
	Containers  map[string]string // service name -> container ID
	ConfigFiles []string          // compose files used to create the project
}

// Standalone is a container not managed by Docker Compose.
type Standalone struct {
	ID    string
	Name  string
	Image string
	State docker.ContainerState
}
