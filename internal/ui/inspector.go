package ui

import (
	"fmt"
	"time"

	"github.com/hyturing/compose-pulse/internal/dag"
)

// ServiceInspector is a normalized, render-ready view of one service,
// shared by the Deps tab and the doctor/graph tabs' edge-condition lookups.
type ServiceInspector struct {
	Name         string
	Project      string
	DisplayState dag.DisplayState
	StateLabel   string
	ExitCode     *int
	Image        string
	Ports        []string
	CreatedAgo   string
	ContainerID  string
	WaitingOn    []DependencyStatus
	Dependencies []DependencyStatus
	Dependents   []DependencyStatus
}

// DependencyStatus is one dependency/dependent entry rendered in the Deps tab.
type DependencyStatus struct {
	Name      string
	Condition string
	State     dag.DisplayState
	Satisfied bool
}

// buildServiceInspector normalizes a compose-node Row into a render-ready view.
func buildServiceInspector(row Row) ServiceInspector {
	n := row.Node
	display, waitingOn := dag.Display(n, row.Graph)
	waiting := make(map[string]struct{}, len(waitingOn))
	for _, w := range waitingOn {
		waiting[w] = struct{}{}
	}

	insp := ServiceInspector{
		Name:         n.Name,
		Project:      row.ProjectName,
		DisplayState: display,
		StateLabel:   displayStateLabel(display),
		ExitCode:     n.ExitCode,
		Image:        n.Image,
		Ports:        n.Ports,
		ContainerID:  shortContainerID(n.ContainerID),
	}
	if n.CreatedAt > 0 {
		insp.CreatedAgo = formatAgo(time.Since(time.Unix(n.CreatedAt, 0)))
	}

	for _, depName := range n.Deps {
		condition := n.DepConditions[depName]
		if condition == "" {
			condition = "service_started"
		}
		var depState dag.DisplayState
		if dep := row.Graph.ByName[depName]; dep != nil {
			depState, _ = dag.Display(dep, row.Graph)
		}
		_, unsatisfied := waiting[depName]
		status := DependencyStatus{
			Name:      depName,
			Condition: condition,
			State:     depState,
			Satisfied: !unsatisfied,
		}
		insp.Dependencies = append(insp.Dependencies, status)
		if unsatisfied {
			insp.WaitingOn = append(insp.WaitingOn, status)
		}
	}

	for _, child := range n.Children {
		childState, _ := dag.Display(child, row.Graph)
		insp.Dependents = append(insp.Dependents, DependencyStatus{
			Name:  child.Name,
			State: childState,
		})
	}

	return insp
}

func shortContainerID(id string) string {
	const shortLen = 12
	if len(id) > shortLen {
		return id[:shortLen]
	}
	return id
}

func formatAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// leftPanelTitle kept for tests that assert filter labeling via the
// SERVICE column header.
func leftPanelTitle(m Model) string {
	cols := computeGraphColumns(m.visibleRows(), 80)
	return formatGraphColumnHeader(cols, m.rowFilter)
}

// renderInspectorLogs is the Logs tab: the log viewport alone, no service metadata.
func renderInspectorLogs(m Model, width int) string {
	visible := m.logPanelVisibleLines()
	sourceLines := m.displayLogLines()
	if m.logWaiting {
		// displayLogLines already expands the waiting placeholder
	} else if len(m.logs) == 0 {
		return padMetaLine(styleDim.Render("Waiting for logs…"), width)
	}

	displayRows := buildLogDisplayRows(sourceLines, width-scrollBarWidth-logLinePrefixW)
	scroll, follow := m.previewLogScroll(displayRows, visible)
	cfg := logViewportConfig{
		sourceLines:  sourceLines,
		displayRows:  displayRows,
		scroll:       scroll,
		cursor:       m.logCursor,
		follow:       follow,
		waiting:      m.logWaiting,
		width:        width,
		visibleLines: visible,
		findPattern:  m.logFind,
	}
	if m.logSel.has {
		lo, hi := m.logSel.normalized()
		cfg.hasSel = true
		cfg.selStart = lo
		cfg.selEnd = hi
	}
	return renderLogViewport(cfg)
}
