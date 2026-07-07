package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/hyturing/compose-pulse/internal/dag"
)

// Inspector tab indices, matching the "1 Overview / 2 Logs / 3 Deps" keys.
const (
	inspectorTabOverview = iota
	inspectorTabLogs
	inspectorTabDeps
)

// ServiceInspector is a normalized, render-ready view of one service.
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

func lastLogLines(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}

// leftPanelTitle is the Services panel title, with a suffix when filtered.
func leftPanelTitle(m Model) string {
	switch m.rowFilter {
	case filterFailed:
		return "Services · failed"
	case filterBlocked:
		return "Services · blocked"
	default:
		return "Services"
	}
}

// inspectorTitle is the right panel title: selected service name + state,
// or a generic label when nothing is selected / an action menu is open.
func inspectorTitle(m Model) string {
	if m.actionMode != actionModeNone {
		return "Actions"
	}
	visible := m.visibleRows()
	if m.cursor >= len(visible) || !isSelectable(visible[m.cursor]) {
		return "Details"
	}
	row := visible[m.cursor]
	if row.Kind == RowStandalone {
		return row.Standalone.Name + " · " + row.Standalone.State.String()
	}
	display, _ := dag.Display(row.Node, row.Graph)
	return row.Node.Name + " · " + displayStateLabel(display)
}

func renderInspectorTabStrip(tab int, hasDeps bool) string {
	labels := []string{"1 Overview", "2 Logs"}
	if hasDeps {
		labels = append(labels, "3 Deps")
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		if i == tab {
			parts[i] = styleTabActive.Render("[" + l + "]")
		} else {
			parts[i] = styleDim.Render("[" + l + "]")
		}
	}
	return strings.Join(parts, " ")
}

func renderInspectorOverview(m Model, insp ServiceInspector, width int) string {
	type kv struct{ key, val string }
	var rows []kv
	rows = append(rows, kv{"Status", displayIndicator(insp.DisplayState, m.spinFrame) + " " + insp.StateLabel})
	if insp.ExitCode != nil {
		rows = append(rows, kv{"Exit code", fmt.Sprintf("%d", *insp.ExitCode)})
	}
	rows = append(rows, kv{"Project", insp.Project})
	if insp.Image != "" {
		rows = append(rows, kv{"Image", insp.Image})
	}
	if len(insp.Ports) > 0 {
		rows = append(rows, kv{"Ports", strings.Join(insp.Ports, ", ")})
	}
	if insp.CreatedAgo != "" {
		rows = append(rows, kv{"Created", insp.CreatedAgo})
	}
	if len(insp.WaitingOn) > 0 {
		rows = append(rows, kv{"Blocked by", blockedByLabel(insp.WaitingOn)})
	}

	labelW := 0
	for _, r := range rows {
		if w := runewidth.StringWidth(r.key); w > labelW {
			labelW = w
		}
	}

	var lines []string
	for _, r := range rows {
		pad := strings.Repeat(" ", labelW-runewidth.StringWidth(r.key)+2)
		lines = append(lines, r.key+pad+r.val)
	}
	lines = append(lines, strings.Repeat("─", maxInt(8, width-scrollBarWidth-4)))
	lines = append(lines, styleDim.Render("Last logs"))

	tail := lastLogLines(m.logs, 5)
	if len(tail) == 0 {
		lines = append(lines, styleDim.Render("No logs yet"))
	} else {
		for _, l := range tail {
			lines = append(lines, "› "+normalizeLogLine(l))
		}
	}

	var sb strings.Builder
	for i, l := range lines {
		sb.WriteString(padMetaLine(l, width))
		if i != len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func blockedByLabel(waiting []DependencyStatus) string {
	parts := make([]string, len(waiting))
	for i, w := range waiting {
		parts[i] = w.Name
		if w.Condition == "service_healthy" {
			parts[i] += ":healthy"
		}
	}
	return strings.Join(parts, ", ")
}

func renderInspectorDeps(insp ServiceInspector, width int) string {
	var lines []string
	if len(insp.WaitingOn) > 0 {
		lines = append(lines, styleDim.Render("Waiting on"))
		for _, d := range insp.WaitingOn {
			lines = append(lines, depLineWaiting(d))
		}
		lines = append(lines, "")
	}

	lines = append(lines, styleDim.Render("Dependencies"))
	if len(insp.Dependencies) == 0 {
		lines = append(lines, styleDim.Render("  none"))
	}
	for _, d := range insp.Dependencies {
		lines = append(lines, depLineDefault(d))
	}

	lines = append(lines, "", styleDim.Render("Direct dependents"))
	if len(insp.Dependents) == 0 {
		lines = append(lines, styleDim.Render("  none"))
	}
	for _, d := range insp.Dependents {
		lines = append(lines, depLineDefault(d))
	}

	var sb strings.Builder
	for i, l := range lines {
		sb.WriteString(padMetaLine(l, width))
		if i != len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// depGlyph renders ✓ for a satisfied dependency, otherwise its own state glyph.
func depGlyph(d DependencyStatus) string {
	if d.Satisfied {
		return glyphCompleted
	}
	return displayIndicator(d.State, 0)
}

func depLineDefault(d DependencyStatus) string {
	return fmt.Sprintf("  %s %s  %s", depGlyph(d), d.Name, displayStateLabel(d.State))
}

func depLineWaiting(d DependencyStatus) string {
	label := d.Name
	if needs := conditionNeeds(d.Condition); needs != "" {
		label += "  " + needs
	}
	return fmt.Sprintf("  %s %s  (%s)", depGlyph(d), label, displayStateLabel(d.State))
}

func conditionNeeds(condition string) string {
	switch condition {
	case "service_healthy":
		return "needs healthy"
	case "service_completed_successfully":
		return "needs completion"
	default:
		return ""
	}
}

// renderInspectorLogs is the Logs tab: the log viewport alone, no service metadata.
func renderInspectorLogs(m Model, width int) string {
	visible := m.previewLogVisibleLines()
	sourceLines := m.logs
	if m.logWaiting {
		sourceLines = strings.Split(buildWaitingContent(m), "\n")
	} else if len(sourceLines) == 0 {
		return padMetaLine(styleDim.Render("Waiting for logs…"), width)
	}

	displayRows := buildLogDisplayRows(sourceLines, width-scrollBarWidth-logLinePrefixW)
	scroll, follow := m.previewLogScroll(displayRows, visible)
	return renderLogViewport(logViewportConfig{
		sourceLines:  sourceLines,
		displayRows:  displayRows,
		scroll:       scroll,
		cursor:       m.logCursor,
		follow:       follow,
		waiting:      m.logWaiting,
		width:        width,
		visibleLines: visible,
	})
}

func renderStandaloneOverview(row Row, width int) string {
	lines := []string{
		"Name     " + row.Standalone.Name,
		"Status   " + row.Standalone.State.String(),
		"Image    " + row.Standalone.Image,
	}
	var sb strings.Builder
	for i, l := range lines {
		sb.WriteString(padMetaLine(l, width))
		if i != len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
