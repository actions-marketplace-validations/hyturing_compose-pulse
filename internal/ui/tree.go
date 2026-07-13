package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/docker"
)

const (
	graphStateColWidth = 9 // "completed" / "starting" / "pending"
	graphCPUColWidth   = 6 // "100.0%"
	graphMEMColWidth   = 9 // "1023.9MiB"
	graphColGap        = 2
)

func graphStatsWidth() int {
	return graphCPUColWidth + 1 + graphMEMColWidth
}

func isSelectable(row Row) bool {
	return row.Kind == RowProjectHeader || row.Kind == RowComposeNode || row.Kind == RowStandalone
}

// displayAndWaiting returns the derived DisplayState for a compose-node row;
// zero value for anything else.
func displayAndWaiting(row Row) dag.DisplayState {
	if row.Kind != RowComposeNode {
		return ""
	}
	display, _ := dag.Display(row.Node, row.Graph)
	return display
}

// isRunningDisplay reports whether d represents a container that is actually
// alive (running), regardless of health — the set the stats sweep samples.
func isRunningDisplay(d dag.DisplayState) bool {
	switch d {
	case dag.DisplayHealthy, dag.DisplayStarting, dag.DisplayUnhealthy, dag.DisplayDegraded:
		return true
	default:
		return false
	}
}

// isRunningRow reports whether row's container is currently running, for
// both compose services and standalone containers.
func isRunningRow(row Row) bool {
	switch row.Kind {
	case RowComposeNode:
		return isRunningDisplay(displayAndWaiting(row))
	case RowStandalone:
		return row.ContainerID != "" && row.Standalone.State != docker.StateExited
	default:
		return false
	}
}

// isDegraded reports the UI-side degraded override: a running container
// that has restarted repeatedly (TUI-DESIGN.md §8, doctor rule restart-loop).
func isDegraded(m Model, row Row) bool {
	if !isRunningRow(row) || row.ContainerID == "" {
		return false
	}
	info := m.inspects[row.ContainerID]
	return info != nil && info.RestartCount >= 3
}

// waitingSinceHint returns "on <dep> · <Ns>" once a blocked row has been
// waiting for at least 5s, sourced from the timeline tracker
// (TUI-DESIGN.md §3.2, §2.7.7).
func waitingSinceHint(m Model, row Row) string {
	if row.Kind != RowComposeNode || m.timeline == nil {
		return ""
	}
	for _, s := range m.timeline.Spans(row.ProjectName, time.Now()) {
		if s.Service != row.Node.Name || s.Final != dag.DisplayBlocked || len(s.Segments) == 0 {
			continue
		}
		dur := s.Segments[len(s.Segments)-1].Dur
		if dur < 5*time.Second {
			return ""
		}
		dep := s.WaitsOn
		if idx := strings.IndexByte(dep, ','); idx >= 0 {
			dep = dep[:idx]
		}
		dep = strings.SplitN(dep, ":", 2)[0]
		return fmt.Sprintf("on %s · %s", dep, formatDuration(dur))
	}
	return ""
}

// formatDuration renders a short duration like "40s" or "1m32s" (>=100s).
func formatDuration(d time.Duration) string {
	if d >= 100*time.Second {
		m := int(d.Minutes())
		s := int(d.Seconds()) - m*60
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

// graphColumns sizes the left-panel columns for a given inner width.
type graphColumns struct {
	nameW     int
	stateW    int
	detailW   int
	showStats bool
}

func computeGraphColumns(rows []Row, panelWidth int) graphColumns {
	_ = rows // width-driven layout; content no longer shrinks the SERVICE column
	cols := graphColumns{
		stateW:  graphStateColWidth,
		detailW: 12, // compact: "exit 137", "←99 deps" — do not flex
	}
	statsW := graphStatsWidth()
	const minName = 14

	minWithStats := minName + graphColGap + cols.stateW + graphColGap + cols.detailW + graphColGap + statsW
	cols.showStats = panelWidth >= minWithStats

	fixed := cols.stateW + graphColGap + cols.detailW + graphColGap
	if cols.showStats {
		fixed += graphColGap + statsW
	}
	nameBudget := panelWidth - fixed
	if nameBudget < minName {
		nameBudget = minName
	}

	// SERVICE takes all leftover width so CPU/MEM sit flush on the panel's
	// right edge (filtered views with short names used to leave a gap after MEM).
	cols.nameW = nameBudget
	return cols
}

// formatGraphColumnHeader is the left-panel title row: column labels aligned
// with formatComposeLine, plus an optional filter hint in the SERVICE cell.
func formatGraphColumnHeader(cols graphColumns, filter rowFilter) string {
	svc := "SERVICE"
	status := "STATUS"
	switch filter {
	case filterFailed:
		status = "failed"
	case filterWaiting:
		status = "waiting"
	}
	parts := []string{
		padVisible(styleDim.Render(svc), cols.nameW),
		padVisible(styleDim.Render(status), cols.stateW),
		padVisible(styleDim.Render("DETAIL"), cols.detailW),
	}
	if !cols.showStats {
		return joinGraphRow(parts[0], parts[1], parts[2], "", "")
	}
	return joinGraphRow(
		parts[0], parts[1], parts[2],
		padVisibleRight(styleDim.Render("CPU"), graphCPUColWidth),
		padVisibleRight(styleDim.Render("MEM"), graphMEMColWidth),
	)
}

// formatComposeLine renders one compose-service row as fixed columns:
// tree+name | STATUS | detail [| cpu mem].
func formatComposeLine(m Model, row Row, cols graphColumns) string {
	n := row.Node
	display, waitingOn := dag.Display(n, row.Graph)
	if isDegraded(m, row) {
		display = dag.DisplayDegraded
	}

	nameCell := padVisible(
		styleTreePrefix.Render(row.linePrefix)+
			rowIndicator(m, row)+" "+
			styleName.Render(n.Name),
		cols.nameW,
	)
	stateCell := padVisible(styleDisplayState(display).Render(displayStateLabel(display)), cols.stateW)

	detailPlain, detailStyle := displayDetail(display, waitingOn, n)
	if detailPlain == "" {
		if hint := waitingSinceHint(m, row); hint != "" {
			detailPlain, detailStyle = hint, styleDetailNeeds
		}
	}
	detailCell := padVisible(detailStyle.Render(detailPlain), cols.detailW)

	if !cols.showStats {
		return joinGraphRow(nameCell, stateCell, detailCell, "", "")
	}
	cpu, mem := listStatsColumns(m, row)
	return joinGraphRow(
		nameCell, stateCell, detailCell,
		padVisibleRight(styleDim.Render(cpu), graphCPUColWidth),
		padVisibleRight(styleDim.Render(mem), graphMEMColWidth),
	)
}

// padVisible pads or truncates s to exactly width visible columns (left-aligned).
func padVisible(s string, width int) string {
	if width < 1 {
		return ""
	}
	w := visibleWidth(s)
	if w == width {
		return s
	}
	if w > width {
		return truncateToVisibleWidth(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

// padVisibleRight pads or truncates s to exactly width visible columns (right-aligned),
// so CPU/MEM numbers line up under their headers.
func padVisibleRight(s string, width int) string {
	if width < 1 {
		return ""
	}
	w := visibleWidth(s)
	if w == width {
		return s
	}
	if w > width {
		return truncateToVisibleWidth(s, width)
	}
	return strings.Repeat(" ", width-w) + s
}

// padPlain pads or truncates plain (no-ANSI) text to exactly width columns.
func padPlain(s string, width int) string {
	if width < 1 {
		return ""
	}
	w := runewidth.StringWidth(s)
	if w == width {
		return s
	}
	if w > width {
		return runewidth.Truncate(s, width, "…")
	}
	return s + strings.Repeat(" ", width-w)
}

// visibleWidth is the terminal column count of s, ignoring ANSI.
func visibleWidth(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}

// rowIndicator renders the glyph for a row, applying the degraded override.
func rowIndicator(m Model, row Row) string {
	if isDegraded(m, row) {
		return glyphDegraded
	}
	if row.Kind == RowStandalone {
		return stateIndicator(row.Standalone.State, m.spinFrame)
	}
	display, _ := dag.Display(row.Node, row.Graph)
	return displayIndicator(display, m.spinFrame)
}

// nameColumn computes the shared name column width for compose rows: the
// widest visible (prefix + glyph + name), capped at maxWidth.
func nameColumn(rows []Row, maxWidth int) int {
	maxLen := 0
	for _, r := range rows {
		if r.Kind != RowComposeNode || r.Node == nil {
			continue
		}
		// prefix + glyph(1) + space + name
		w := runewidth.StringWidth(stripANSI(r.linePrefix)) + 1 + 1 + runewidth.StringWidth(r.Node.Name)
		if w > maxLen {
			maxLen = w
		}
	}
	if maxWidth > 0 && maxLen > maxWidth {
		maxLen = maxWidth
	}
	if maxLen < 1 {
		maxLen = 1
	}
	return maxLen
}

func displayStateLabel(d dag.DisplayState) string {
	if d == dag.DisplayPending {
		return "pending"
	}
	if d == dag.DisplayMissing {
		return "missing"
	}
	if d == dag.DisplayDegraded {
		return "degraded"
	}
	return string(d)
}

func styleDisplayState(d dag.DisplayState) lipgloss.Style {
	switch d {
	case dag.DisplayHealthy, dag.DisplayCompleted:
		return styleStateHealthy
	case dag.DisplayStarting, dag.DisplayDegraded:
		return styleStateStarting
	case dag.DisplayBlocked:
		return styleStateBlocked
	case dag.DisplayFailed, dag.DisplayUnhealthy, dag.DisplayMissing:
		return styleStateFailed
	case dag.DisplayPending:
		return styleStatePending
	default:
		return styleDim
	}
}

// displayDetail returns the plain detail text and its style for the DETAIL column.
func displayDetail(d dag.DisplayState, waitingOn []string, n *dag.Node) (string, lipgloss.Style) {
	switch d {
	case dag.DisplayFailed:
		if n != nil && n.ExitCode != nil {
			return fmt.Sprintf("exit %d", *n.ExitCode), styleDetailExitFail
		}
		return "exit ?", styleDetailExitFail
	case dag.DisplayCompleted:
		return "exit 0", styleDetailExitOK
	case dag.DisplayBlocked:
		n := len(waitingOn)
		if n == 0 {
			return "", styleDim
		}
		return fmt.Sprintf("←%d deps", n), styleDetailNeeds
	case dag.DisplayMissing:
		return "no container", styleDetailExitFail
	case dag.DisplayUnhealthy:
		return "health failing", styleDetailExitFail
	default:
		return "", styleDim
	}
}

// displayIndicator renders the glyph for a derived DisplayState.
func displayIndicator(d dag.DisplayState, frame int) string {
	switch d {
	case dag.DisplayHealthy:
		return glyphHealthy
	case dag.DisplayStarting:
		return glyphStartingFrames[frame%len(glyphStartingFrames)]
	case dag.DisplayUnhealthy:
		return glyphUnhealthy
	case dag.DisplayCompleted:
		return glyphCompleted
	case dag.DisplayFailed:
		return glyphFailed
	case dag.DisplayDegraded:
		return glyphDegraded
	case dag.DisplayMissing:
		return glyphMissing
	case dag.DisplayBlocked, dag.DisplayPending:
		return glyphPending
	default:
		return "?"
	}
}

// stateIndicator renders the glyph for a raw container state — used for
// standalone containers, which have no DAG and thus no DisplayState.
func stateIndicator(s docker.ContainerState, frame int) string {
	switch s {
	case docker.StateHealthy:
		return glyphHealthy
	case docker.StateStarting:
		return glyphStartingFrames[frame%len(glyphStartingFrames)]
	case docker.StateUnhealthy:
		return glyphUnhealthy
	case docker.StatePending:
		return glyphPending
	case docker.StateExited:
		return glyphFailed
	default:
		return "?"
	}
}

func formatStandaloneLine(m Model, r Row, cols graphColumns) string {
	indicator := rowIndicator(m, r)
	nameCell := padVisible(
		"  "+indicator+" "+styleName.Render(r.Standalone.Name),
		cols.nameW,
	)
	stateCell := padVisible(styleDim.Render(r.Standalone.State.String()), cols.stateW)
	detailCell := padVisible(styleDim.Render(r.Label), cols.detailW)
	if !cols.showStats {
		return joinGraphRow(nameCell, stateCell, detailCell, "", "")
	}
	cpu, mem := listStatsColumns(m, r)
	return joinGraphRow(
		nameCell, stateCell, detailCell,
		padVisibleRight(styleDim.Render(cpu), graphCPUColWidth),
		padVisibleRight(styleDim.Render(mem), graphMEMColWidth),
	)
}

// joinGraphRow joins the fixed graph columns with consistent gaps.
func joinGraphRow(name, state, detail, cpu, mem string) string {
	gap := strings.Repeat(" ", graphColGap)
	row := name + gap + state + gap + detail
	if cpu == "" && mem == "" {
		return row
	}
	return row + gap + cpu + " " + mem
}
