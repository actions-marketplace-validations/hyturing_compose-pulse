package dag

// DirectDependents returns the names of services that directly depend on service
// (i.e. node.Children), in Children order.
func DirectDependents(g *Graph, service string) []string {
	if g == nil {
		return nil
	}
	n, ok := g.ByName[service]
	if !ok || n == nil {
		return nil
	}
	out := make([]string, 0, len(n.Children))
	for _, c := range n.Children {
		if c != nil {
			out = append(out, c.Name)
		}
	}
	return out
}

// TransitiveDependents returns services that depend on service indirectly
// (excludes direct dependents and self), in graph topological order.
func TransitiveDependents(g *Graph, service string) []string {
	if g == nil {
		return nil
	}
	start, ok := g.ByName[service]
	if !ok || start == nil {
		return nil
	}
	direct := map[string]bool{}
	for _, c := range start.Children {
		if c != nil {
			direct[c.Name] = true
		}
	}
	seen := map[string]bool{service: true}
	var visit func(*Node)
	visit = func(node *Node) {
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
	for _, node := range g.Ordered {
		if node == nil || node.Name == service || direct[node.Name] || !seen[node.Name] {
			continue
		}
		out = append(out, node.Name)
	}
	return out
}

// AllDependents returns direct + transitive dependents in topo order (excludes self).
func AllDependents(g *Graph, service string) []string {
	if g == nil {
		return nil
	}
	start, ok := g.ByName[service]
	if !ok || start == nil {
		return nil
	}
	seen := map[string]bool{service: true}
	var visit func(*Node)
	visit = func(node *Node) {
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
	for _, node := range g.Ordered {
		if node != nil && node.Name != service && seen[node.Name] {
			out = append(out, node.Name)
		}
	}
	return out
}
