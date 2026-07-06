package dag

import (
	"fmt"
	"sort"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/docker"
)

// Graph holds the full dependency graph.
type Graph struct {
	Roots   []*Node          // nodes with no dependencies (in-degree 0)
	ByName  map[string]*Node // O(1) service lookup
	Ordered []*Node          // all nodes in topological order (Kahn's)
	Visual  []*Node          // nodes in depth-first render order (matches tree row positions)
}

// Build constructs the DAG from a parsed compose Config.
// Returns an error if a circular dependency is detected.
func Build(cfg *compose.Config) (*Graph, error) {
	byName := make(map[string]*Node, len(cfg.Services))
	serviceNames := sortedServiceNames(cfg.Services)
	for _, name := range serviceNames {
		byName[name] = &Node{Name: name, State: docker.StatePending}
	}

	inDegree := make(map[string]int, len(cfg.Services))
	for _, name := range serviceNames {
		svc := cfg.Services[name]
		deps := sortedMapKeys(svc.DependsOn)
		byName[name].Deps = deps
		if len(deps) > 0 {
			byName[name].DepConditions = make(map[string]string, len(deps))
			for _, dep := range deps {
				cond := svc.DependsOn[dep].Condition
				if cond == "" {
					cond = "service_started"
				}
				byName[name].DepConditions[dep] = cond
			}
		}
		inDegree[name] = len(deps)

		for _, dep := range deps {
			if _, ok := byName[dep]; !ok {
				return nil, fmt.Errorf("service %q depends on unknown service %q", name, dep)
			}
			byName[dep].Children = append(byName[dep].Children, byName[name])
		}

		if len(deps) > 0 {
			byName[deps[0]].TreeChildren = append(byName[deps[0]].TreeChildren, byName[name])
		}
	}

	for _, node := range byName {
		sort.Slice(node.Children, func(i, j int) bool {
			return node.Children[i].Name < node.Children[j].Name
		})
		sort.Slice(node.TreeChildren, func(i, j int) bool {
			return node.TreeChildren[i].Name < node.TreeChildren[j].Name
		})
	}

	queue := make([]*Node, 0)
	for _, name := range serviceNames {
		if inDegree[name] == 0 {
			queue = append(queue, byName[name])
		}
	}

	ordered := make([]*Node, 0, len(byName))
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		ordered = append(ordered, n)

		for _, child := range n.Children {
			inDegree[child.Name]--
			if child.Level < n.Level+1 {
				child.Level = n.Level + 1
			}
			if inDegree[child.Name] == 0 {
				queue = append(queue, child)
				sort.Slice(queue, func(i, j int) bool { return queue[i].Name < queue[j].Name })
			}
		}
	}

	if len(ordered) != len(byName) {
		return nil, fmt.Errorf("circular dependency detected in docker-compose.yml")
	}

	roots := make([]*Node, 0)
	for _, n := range ordered {
		if len(n.Deps) == 0 {
			roots = append(roots, n)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Name < roots[j].Name })

	// Build visual order: depth-first over TreeChildren from each root.
	// This matches the exact row order the renderer produces.
	visual := make([]*Node, 0, len(byName))
	var dfs func(*Node)
	dfs = func(n *Node) {
		visual = append(visual, n)
		for _, child := range n.TreeChildren {
			dfs(child)
		}
	}
	for _, root := range roots {
		dfs(root)
	}

	return &Graph{
		Roots:   roots,
		ByName:  byName,
		Ordered: ordered,
		Visual:  visual,
	}, nil
}

func sortedMapKeys(m compose.DependsOn) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedServiceNames(services map[string]compose.Service) []string {
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
