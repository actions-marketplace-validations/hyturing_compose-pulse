package ui

import (
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
)

// RowKind identifies a navigable or decorative row in the view.
type RowKind int

// Row kind constants for the unified view.
const (
	RowProjectHeader RowKind = iota
	RowComposeNode
	RowStandaloneHeader
	RowStandalone
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
	for _, project := range snap.Projects {
		rows = append(rows, Row{
			Kind:  RowProjectHeader,
			Label: "COMPOSE · " + project.Name,
		})
		rows = appendComposeRows(rows, project.Name, project.ConfigFiles, project.Graph)
	}

	if len(snap.Standalone) > 0 {
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
