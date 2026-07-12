package ui

import (
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

// rowFilter narrows the visible row set to a state subset.
type rowFilter int

// The set of row filters selectable from the dashboard. `f` cycles
// all -> failed -> waiting -> all (TUI-DESIGN.md §3.2).
const (
	filterAll     rowFilter = iota
	filterFailed            // DisplayFailed + DisplayUnhealthy
	filterWaiting           // DisplayBlocked
)

// nextFilter advances the services filter cycle.
func nextFilter(cur rowFilter) rowFilter {
	switch cur {
	case filterAll:
		return filterFailed
	case filterFailed:
		return filterWaiting
	default:
		return filterAll
	}
}

// RowKind identifies a navigable or decorative row in the view.
type RowKind int

// Row kind constants for the unified view.
const (
	RowProjectHeader RowKind = iota
	RowComposeNode
	RowStandaloneHeader
	RowStandalone
	RowSpacer // blank line between project sections (not selectable)
)

// Row is one line in the unified navigation order.
type Row struct {
	Kind        RowKind
	Label       string
	ProjectName string
	ConfigFiles []string
	Graph       *dag.Graph
	Node        *dag.Node
	Standalone  *discover.Standalone
	ContainerID string
	linePrefix  string
}

// BuildRows flattens a snapshot into render/navigation order.
func BuildRows(snap *discover.Snapshot) []Row {
	if snap == nil {
		return nil
	}

	var rows []Row
	for i, project := range snap.Projects {
		if i > 0 {
			rows = append(rows, Row{Kind: RowSpacer})
		}
		rows = append(rows, Row{
			Kind:        RowProjectHeader,
			Label:       "COMPOSE · " + project.Name,
			ProjectName: project.Name,
			ConfigFiles: project.ConfigFiles,
			Graph:       project.Graph,
		})
		rows = appendComposeRows(rows, project.Name, project.ConfigFiles, project.Graph)
	}

	if len(snap.Standalone) > 0 {
		if len(rows) > 0 {
			rows = append(rows, Row{Kind: RowSpacer})
		}
		rows = append(rows, Row{
			Kind:  RowStandaloneHeader,
			Label: "OTHER CONTAINERS",
		})
		for i := range snap.Standalone {
			s := &snap.Standalone[i]
			rows = append(rows, Row{
				Kind:        RowStandalone,
				Label:       s.Image,
				Standalone:  s,
				ContainerID: s.ID,
			})
		}
	}
	return rows
}

func appendComposeRows(rows []Row, projectName string, configFiles []string, g *dag.Graph) []Row {
	var walk func(n *dag.Node, prefix string, isLast, isRoot bool)
	walk = func(n *dag.Node, prefix string, isLast, isRoot bool) {
		var linePrefix string
		var childPrefix string
		switch {
		case isRoot:
			linePrefix = "  "
			childPrefix = "  "
		case isLast:
			linePrefix = prefix + "└─ "
			childPrefix = prefix + "   "
		default:
			linePrefix = prefix + "├─ "
			childPrefix = prefix + "│  "
		}

		rows = append(rows, Row{
			Kind:        RowComposeNode,
			ProjectName: projectName,
			ConfigFiles: configFiles,
			Graph:       g,
			Node:        n,
			ContainerID: n.ContainerID,
			linePrefix:  linePrefix,
		})

		for i, child := range n.TreeChildren {
			walk(child, childPrefix, i == len(n.TreeChildren)-1, false)
		}
	}

	for i, root := range g.Roots {
		walk(root, "", i == len(g.Roots)-1, true)
	}
	return rows
}

// rowKey returns a stable identifier for cursor preservation across rebuilds.
func rowKey(r Row) string {
	switch r.Kind {
	case RowProjectHeader:
		return "project:" + r.ProjectName
	case RowComposeNode:
		return "compose:" + r.ProjectName + ":" + r.Node.Name
	case RowStandalone:
		return "standalone:" + r.Standalone.ID
	default:
		return ""
	}
}

func findRowByKey(rows []Row, key string) int {
	if key == "" {
		return -1
	}
	for i, r := range rows {
		if rowKey(r) == key {
			return i
		}
	}
	return -1
}

func clampCursor(cur int, rows []Row) int {
	if len(rows) == 0 {
		return 0
	}
	if cur >= len(rows) {
		cur = len(rows) - 1
	}
	if isSelectable(rows[cur]) {
		return cur
	}
	return firstSelectable(rows)
}

// filterRows returns rows matching f. The project row is always kept — it is
// the project's own selectable summary, not a service, so the services
// filter never hides it. The standalone header is kept only when at least
// one of its child rows matches. Tree line prefixes are flattened to two
// spaces in filtered views (the filtered list is not a tree).
func filterRows(rows []Row, f rowFilter) []Row {
	if f == filterAll {
		return rows
	}

	var out []Row
	var pendingHeader Row
	hasPendingHeader := false
	headerEmitted := false

	for _, r := range rows {
		switch r.Kind {
		case RowProjectHeader:
			out = append(out, r)
			hasPendingHeader = false
			headerEmitted = false
			continue
		case RowSpacer:
			// Keep project gaps in filtered views too.
			out = append(out, r)
			continue
		case RowStandaloneHeader:
			pendingHeader = r
			hasPendingHeader = true
			headerEmitted = false
			continue
		}
		if !rowMatchesFilter(r, f) {
			continue
		}
		if hasPendingHeader && !headerEmitted {
			out = append(out, pendingHeader)
			headerEmitted = true
		}
		flat := r
		flat.linePrefix = "  "
		out = append(out, flat)
	}
	return out
}

// emptyFilterMessage is shown in the left panel when a filter matches nothing.
func emptyFilterMessage(f rowFilter) string {
	switch f {
	case filterFailed:
		return "No failed services 🎉"
	case filterWaiting:
		return "No waiting services 🎉"
	default:
		return "No containers found."
	}
}

func rowMatchesFilter(r Row, f rowFilter) bool {
	switch r.Kind {
	case RowComposeNode:
		state, _ := dag.Display(r.Node, r.Graph)
		switch f {
		case filterFailed:
			return state == dag.DisplayFailed || state == dag.DisplayUnhealthy
		case filterWaiting:
			return state == dag.DisplayBlocked
		}
	case RowStandalone:
		if f == filterFailed {
			return r.Standalone.State == docker.StateExited
		}
	}
	return false
}

func rowLabel(r Row) string {
	switch r.Kind {
	case RowComposeNode:
		return r.Node.Name
	case RowStandalone:
		return r.Standalone.Name
	default:
		return r.Label
	}
}
