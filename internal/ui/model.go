package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	actionspkg "github.com/hyturing/compose-pulse/internal/actions"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/doctor"
	"github.com/hyturing/compose-pulse/internal/probe"
	"github.com/hyturing/compose-pulse/internal/timeline"
)

// viewMode is the top-level view: the dashboard shell, its main panel
// zoomed fullscreen, or the help overlay. There is intentionally no
// per-feature fullscreen view — doctor/timeline/deps/health are all tabs of
// viewDashboard's main panel (TUI-DESIGN.md §2).
type viewMode int

const (
	viewDashboard viewMode = iota
	viewZoom
	viewHelp
)

// panelFocus is two-state: the left column (project + services panels) or
// the one main panel (TUI-DESIGN.md §2.7.1) — no three-zone cycle.
type panelFocus int

const (
	focusLeft panelFocus = iota
	focusMain
)

func firstSelectable(rows []Row) int {
	for i, r := range rows {
		if isSelectable(r) {
			return i
		}
	}
	return 0
}

// visibleRows returns the rows currently navigable/rendered: m.rows filtered
// by m.rowFilter. Every compose project keeps its full dependency pstree so
// the left panel reads as: project name → tree → next project → tree.
func (m Model) visibleRows() []Row {
	return filterRows(m.rows, m.rowFilter)
}

func rowProject(r Row) string {
	switch r.Kind {
	case RowProjectHeader, RowComposeNode:
		return r.ProjectName
	default:
		return ""
	}
}

// firstProjectName returns the first compose project in rows, if any.
func firstProjectName(rows []Row) string {
	for _, r := range rows {
		if r.Kind == RowProjectHeader && r.ProjectName != "" {
			return r.ProjectName
		}
	}
	return ""
}

func projectExists(rows []Row, name string) bool {
	if name == "" {
		return false
	}
	for _, r := range rows {
		if r.Kind == RowProjectHeader && r.ProjectName == name {
			return true
		}
	}
	return false
}

func nextSelectable(rows []Row, cur, delta int) int {
	if len(rows) == 0 {
		return 0
	}
	idx := cur
	for {
		idx += delta
		if idx < 0 || idx >= len(rows) {
			return cur
		}
		if isSelectable(rows[idx]) {
			return idx
		}
	}
}

// Model is the root Bubble Tea model.
type Model struct {
	snapshot *discover.Snapshot
	rows     []Row
	docker   *docker.Client
	pollCh   <-chan docker.PollMsg
	cancel   context.CancelFunc

	viewMode    viewMode
	panelFocus  panelFocus
	cursor      int
	graphScroll int // services-panel scroll offset

	selectedSvc        string
	selectedRowKey     string
	selectionIsProject bool
	focusedProject     string // last-selected compose project (for d/t project-tab jumps)
	mainTab            int

	logWaiting       bool
	logContainerID   string
	logs             []string
	logScroll        int
	logCursor        int
	logFollow        bool
	logNoMoreHistory bool
	logLoading       bool
	logCh            <-chan docker.LogLineMsg
	logCancel        context.CancelFunc
	searching        bool
	logFilter        string

	spinFrame     int
	width, height int
	lastPoll      time.Time
	rowFilter     rowFilter

	// Task 2.1: on-demand, cached docker inspect per container.
	inspects map[string]*docker.InspectInfo

	// Task 2.6: startup timeline tracker, observed every PollMsg.
	timeline *timeline.Tracker

	// Task 2.9: per-container stats ring buffers, fed by a separate 2s ticker.
	stats map[string]*ringBuffer

	// Doctor tab (project tab 1) state.
	doctorLoading  bool
	doctorFindings []doctor.Finding
	doctorRoot     *doctor.RootCause
	doctorFor      string
	doctorCursor   int

	// Health tab (service tab 4) probe state.
	probeLoading bool
	probeReport  *probe.Report
	probeFor     string

	// Deps tab (service tab 3) row cursor.
	depsCursor int

	// Graph tab (project tab 3) node cursor (shares graphScroll with the
	// services panel — both are project-scoped and never shown together).
	graphCursor int

	// Timeline tab (project tab 2) row cursor + scroll.
	timelineCursor int
	timelineScroll int

	actionMode      actionMode
	actionItems     []actionMenuItem
	actionCursor    int
	actionPlan      actionspkg.Plan
	actionOutput    []string
	actionErr       string
	actionCh        <-chan actionspkg.Event
	actionCancel    context.CancelFunc
	actionRunner    func(context.Context, actionspkg.Plan) <-chan actionspkg.Event
	execContainerID string
	execService     string
	execInput       string
	execActive      bool
}

// New creates the initial model and starts the Docker polling goroutine.
func New(snap *discover.Snapshot, dc *docker.Client) Model {
	ctx, cancel := context.WithCancel(context.Background())
	rows := BuildRows(snap)
	m := Model{
		snapshot:       snap,
		rows:           rows,
		cursor:         firstSelectable(rows),
		docker:         dc,
		pollCh:         dc.StartPollCh(ctx),
		cancel:         cancel,
		logFollow:      true,
		lastPoll:       time.Now(),
		inspects:       make(map[string]*docker.InspectInfo),
		timeline:       timeline.New(time.Now()),
		stats:          make(map[string]*ringBuffer),
		focusedProject: firstProjectName(rows),
	}
	return m
}

func waitForPoll(ch <-chan docker.PollMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Init starts the spinner ticker and begins listening for poll/stats updates.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), waitForPoll(m.pollCh), statsTickCmd(), m.syncSelectionStream())
}

// Update routes incoming messages, mutating model state and returning any
// follow-up commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampGraphScroll()

	case tickMsg:
		m.spinFrame = (m.spinFrame + 1) % len(spinnerFrames)
		return m, tickCmd()

	case docker.PollMsg:
		m.lastPoll = time.Now()
		rebuilt, _ := m.applySnapshot(msg.Containers)
		if m.timeline != nil {
			m.timeline.Observe(m.snapshot, time.Now())
		}
		cmds := []tea.Cmd{waitForPoll(m.pollCh)}
		if rebuilt {
			cmds = append(cmds, m.syncSelectionStream())
		}
		if id := m.selectedContainerID(); id != "" {
			cmds = append(cmds, inspectCmd(m.docker, id))
		}
		if m.logWaiting {
			if id := m.selectedContainerID(); id != "" {
				m.logWaiting = false
				m.logContainerID = id
				m.logs = nil
				m.logScroll = 0
				m.logCursor = 0
				m.logFollow = m.viewMode == viewZoom
				cmds = append(cmds, m.beginLogStream(id))
			}
		}
		return m, tea.Batch(cmds...)

	case statsTickMsg:
		ids := runningContainerIDs(m.rows)
		return m, tea.Batch(statsTickCmd(), statsSweepCmd(m.docker, ids))

	case statsMsg:
		for id, info := range msg.samples {
			ring := m.stats[id]
			if ring == nil {
				ring = newRingBuffer()
				m.stats[id] = ring
			}
			ring.push(info)
		}
		return m, nil

	case inspectMsg:
		if msg.err == nil && msg.info != nil {
			m.inspects[msg.id] = msg.info
		}
		return m, nil

	case doctorMsg:
		m.doctorLoading = false
		m.doctorFor = msg.project
		m.doctorFindings = msg.findings
		m.doctorRoot = msg.root
		if m.doctorCursor >= len(m.doctorFindings) {
			m.doctorCursor = 0
		}
		return m, nil

	case probeMsg:
		m.probeLoading = false
		m.probeFor = msg.containerID
		m.probeReport = msg.report
		return m, nil

	case logLineMsg:
		if msg.line.Err != nil {
			m.appendLogLine(fmt.Sprintf("stream error: %v", msg.line.Err))
			return m, nil
		}
		if msg.line.Line != "" {
			m.appendLogLine(msg.line.Line)
			if m.logFollow || (m.viewMode == viewDashboard && m.panelFocus == focusLeft) {
				m.logFollow = true
				m.scrollToBottom()
			}
		}
		if m.logCh != nil {
			return m, waitForLogLine(m.logCh)
		}
		return m, nil

	case logStreamDoneMsg:
		return m, nil

	case logMoreMsg:
		m.applyLogMore(msg)
		return m, nil

	case actionEventMsg:
		if msg.event.Line != "" {
			m.actionOutput = append(m.actionOutput, msg.event.Line)
		}
		if msg.event.Err != nil {
			m.actionErr = msg.event.Err.Error()
			if m.execActive {
				m.actionMode = actionModeExec
				m.execActive = false
			} else {
				m.actionMode = actionModeDone
			}
			m.actionCh = nil
			return m, nil
		}
		if msg.event.Done {
			if m.execActive {
				m.actionMode = actionModeExec
				m.execActive = false
			} else {
				m.actionMode = actionModeDone
			}
			m.actionCh = nil
			return m, nil
		}
		if m.actionCh != nil {
			return m, waitForActionEvent(m.actionCh)
		}
		return m, nil

	case tea.MouseMsg:
		leftW, _, _, compact := dashboardLayout(m.width, m.height)
		if m.viewMode == viewDashboard && !compact {
			if msg.X >= leftW {
				m.panelFocus = focusMain
				switch msg.Button {
				case tea.MouseButtonWheelUp:
					m.scrollLogLine(-1)
				case tea.MouseButtonWheelDown:
					m.scrollLogLine(1)
				}
			} else {
				m.panelFocus = focusLeft
			}
			return m, nil
		}
		if m.viewMode == viewZoom && !m.searching {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.scrollLogLine(-1)
			case tea.MouseButtonWheelDown:
				m.scrollLogLine(1)
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.searching {
			return m.updateSearch(msg)
		}
		switch m.viewMode {
		case viewZoom:
			return m.updateZoomView(msg)
		case viewHelp:
			return m.updateHelp(msg)
		default:
			return m.updateDashboard(msg)
		}
	}

	return m, nil
}

// inspectCmd fetches on-demand, cached distilled docker inspect data for one
// container (Task 2.1). The 2s cache in docker.Client makes re-firing this
// every 500ms poll for the selected container cheap.
func inspectCmd(dc *docker.Client, containerID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		info, err := dc.Inspect(ctx, containerID)
		return inspectMsg{id: containerID, info: info, err: err}
	}
}

func (m *Model) applySnapshot(containers []docker.ContainerInfo) (rebuilt bool, err error) {
	newSnap, err := discover.FromContainers(containers)
	if err != nil {
		return false, err
	}

	visible := m.visibleRows()
	prevKey := ""
	if m.cursor < len(visible) {
		prevKey = rowKey(visible[m.cursor])
	}

	if m.snapshot != nil && m.snapshot.SameStructure(newSnap) {
		m.snapshot.ApplyStatesFrom(newSnap)
		m.relocateCursor(prevKey)
		m.clampGraphScroll()
		return false, nil
	}

	m.snapshot = newSnap
	m.rows = BuildRows(newSnap)
	if m.focusedProject == "" || !projectExists(m.rows, m.focusedProject) {
		m.focusedProject = firstProjectName(m.rows)
	}
	m.relocateCursor(prevKey)
	m.clampGraphScroll()
	return true, nil
}

// relocateCursor re-locates the cursor by rowKey within the current visible
// (filtered) row set, falling back to the first selectable row.
func (m *Model) relocateCursor(prevKey string) {
	visible := m.visibleRows()
	if prevKey != "" {
		if idx := findRowByKey(visible, prevKey); idx >= 0 {
			m.cursor = idx
		} else {
			m.cursor = clampCursor(m.cursor, visible)
		}
	} else {
		m.cursor = firstSelectable(visible)
	}
}

func (m *Model) selectedContainerID() string {
	if m.selectedRowKey == "" {
		return ""
	}
	idx := findRowByKey(m.rows, m.selectedRowKey)
	if idx < 0 {
		return ""
	}
	return m.rows[idx].ContainerID
}

func (m *Model) stopLogStream() {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	m.logCh = nil
}

func (m *Model) beginLogStream(containerID string) tea.Cmd {
	m.stopLogStream()
	ctx, cancel := context.WithCancel(context.Background())
	m.logCancel = cancel
	m.logCh = m.docker.StartLogStreamCh(ctx, containerID, logTailLines)
	return waitForLogLine(m.logCh)
}

// syncSelectionStream retargets the main panel to whatever is now selected:
// starts the log stream for a service, or marks the project row selected so
// the main panel switches to the project tabs (TUI-DESIGN.md §2.7.1).
func (m *Model) syncSelectionStream() tea.Cmd {
	visible := m.visibleRows()
	if m.cursor >= len(visible) || !isSelectable(visible[m.cursor]) {
		return nil
	}
	row := visible[m.cursor]
	key := rowKey(row)
	if p := rowProject(row); p != "" {
		m.focusedProject = p
	}
	wasProject := m.selectionIsProject
	m.selectionIsProject = row.Kind == RowProjectHeader
	if m.selectionIsProject != wasProject {
		m.mainTab = 0
	}
	m.mainTab = clampMainTab(m.mainTab, m.selectionIsProject)

	if row.Kind == RowProjectHeader {
		m.selectedRowKey = key
		m.selectedSvc = row.ProjectName
		var cmd tea.Cmd
		if m.mainTab == tabDoctor {
			cmd = m.maybeFireDoctor(row)
		}
		return cmd
	}

	if key == m.selectedRowKey && (m.logCh != nil || m.logWaiting) {
		return nil
	}

	m.selectedRowKey = key
	m.selectedSvc = rowLabel(row)
	m.panelFocus = focusLeft
	m.logs = nil
	m.logScroll = 0
	m.logCursor = 0
	m.logFollow = true
	m.logNoMoreHistory = false
	m.logLoading = false
	m.logFilter = ""
	m.depsCursor = 0
	m.probeReport = nil
	m.probeFor = ""
	m.stopLogStream()

	var cmds []tea.Cmd
	if row.ContainerID != "" {
		m.logWaiting = false
		m.logContainerID = row.ContainerID
		cmds = append(cmds, m.beginLogStream(row.ContainerID))
		cmds = append(cmds, inspectCmd(m.docker, row.ContainerID))
	} else {
		m.logWaiting = true
		m.logContainerID = ""
	}
	return tea.Batch(cmds...)
}

// maybeFireDoctor runs the doctor engine for row's project, unless results
// are already loaded/loading for it (doctor never re-runs on every tab
// switch; TUI-DESIGN.md §4.5 only requires it to be fresh per selection).
func (m *Model) maybeFireDoctor(row Row) tea.Cmd {
	if row.Kind != RowProjectHeader {
		return nil
	}
	if m.doctorFor == row.ProjectName && !m.doctorLoading {
		return nil
	}
	m.doctorLoading = true
	m.doctorFindings = nil
	m.doctorRoot = nil
	m.doctorCursor = 0
	return doctorCmd(m.docker, row.ProjectName, row.Graph, row.ConfigFiles)
}

// triggerProbe fires the health probe for the currently selected service
// (Task 2.5, TUI-DESIGN.md §4.4 — `enter` on the Health tab, or the x menu's
// "run health probe").
func (m *Model) triggerProbe() tea.Cmd {
	visible := m.visibleRows()
	if m.cursor >= len(visible) {
		return nil
	}
	row := visible[m.cursor]
	if row.Kind != RowComposeNode || row.ContainerID == "" {
		return nil
	}
	var hc *docker.HealthcheckSpec
	if info := m.inspects[row.ContainerID]; info != nil {
		hc = info.Healthcheck
	}
	m.probeLoading = true
	m.probeFor = row.ContainerID
	return probeCmd(m.docker, row.ContainerID, hc, row.Node.Ports)
}

// setRowFilter switches the active row filter, preserving the current
// selection by rowKey when possible.
func (m *Model) setRowFilter(f rowFilter) {
	if m.rowFilter == f {
		return
	}
	visible := m.visibleRows()
	prevKey := ""
	if m.cursor < len(visible) {
		prevKey = rowKey(visible[m.cursor])
	}
	m.rowFilter = f
	m.relocateCursor(prevKey)
	m.clampGraphScroll()
}

func (m *Model) closeLogView() {
	m.viewMode = viewDashboard
	m.searching = false
	m.logFilter = ""
	m.logFollow = false
}

func (m *Model) focusPanel(focus panelFocus) {
	m.panelFocus = focus
	if focus == focusLeft {
		m.logFollow = true
		m.scrollToBottom()
	}
}

// jumpToService moves the left-column cursor to name (clearing any active
// filter so the target is guaranteed visible), sets the main tab, and
// re-syncs the selection stream. Used by `enter` on doctor findings, deps
// rows, and graph nodes (TUI-DESIGN.md §4.3/§4.5/§4.7).
func (m *Model) jumpToService(name string, tab int) tea.Cmd {
	m.rowFilter = filterAll
	visible := m.visibleRows()
	for i, r := range visible {
		if r.Kind == RowComposeNode && r.Node != nil && r.Node.Name == name {
			m.cursor = i
			m.clampGraphScroll()
			// syncSelectionStream resets mainTab to 0 on a project<->service
			// kind change, so it must run before the requested tab is applied.
			cmd := m.syncSelectionStream()
			m.mainTab = clampMainTab(tab, false)
			return cmd
		}
	}
	return nil
}

// jumpToProjectTab selects the focused (or first) project row and switches the
// main panel to tab — the `d`/`t` shortcuts (TUI-DESIGN.md §7).
func (m *Model) jumpToProjectTab(tab int) tea.Cmd {
	visible := m.visibleRows()
	found := false
	target := m.focusedProject
	for i, r := range visible {
		if r.Kind != RowProjectHeader {
			continue
		}
		if target == "" || r.ProjectName == target {
			m.cursor = i
			m.focusedProject = r.ProjectName
			found = true
			break
		}
	}
	if !found {
		for i, r := range visible {
			if r.Kind == RowProjectHeader {
				m.cursor = i
				m.focusedProject = r.ProjectName
				found = true
				break
			}
		}
	}
	if !found {
		return nil
	}
	// syncSelectionStream resets mainTab to 0 on a project<->service kind
	// change (and prefetches doctor when applicable), so it must run before
	// the requested tab is applied.
	cmd := m.syncSelectionStream()
	m.mainTab = clampMainTab(tab, true)
	return cmd
}

// moveMainCursor moves the tab-specific cursor when the main panel is
// focused: findings in the doctor tab, rows in the deps tab, nodes in the
// graph tab, or scrolls logs/timeline (TUI-DESIGN.md §7).
func (m *Model) moveMainCursor(delta int) {
	visible := m.visibleRows()
	if m.cursor >= len(visible) {
		return
	}
	row := visible[m.cursor]

	if m.selectionIsProject {
		switch m.mainTab {
		case tabDoctor:
			m.doctorCursor = clampIndex(m.doctorCursor+delta, len(m.doctorFindings))
		case tabGraph:
			rows := graphTabRows(row)
			m.graphCursor = clampIndex(m.graphCursor+delta, len(rows))
		case tabTimeline:
			n := 0
			if m.timeline != nil {
				spans := m.timeline.Spans(row.ProjectName, time.Now())
				n = len(timelineTreeSpans(row, spans))
			}
			m.timelineCursor = clampIndex(m.timelineCursor+delta, n)
			m.ensureTimelineCursorVisible(row, n)
		}
		return
	}

	switch m.mainTab {
	case tabDeps:
		targets := depsJumpTargets(row)
		m.depsCursor = clampIndex(m.depsCursor+delta, len(targets))
	default:
		m.scrollLogLine(delta)
	}
}

func clampIndex(idx, n int) int {
	if n <= 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= n {
		return n - 1
	}
	return idx
}

// ensureTimelineCursorVisible scrolls the timeline tab so the selected
// service row (and its expansion) stays in the main-panel viewport.
func (m *Model) ensureTimelineCursorVisible(project Row, spanCount int) {
	if m.timeline == nil || spanCount <= 0 {
		return
	}
	spans := m.timeline.Spans(project.ProjectName, time.Now())
	rows := timelineTreeSpans(project, spans)
	if len(rows) == 0 {
		return
	}
	selected := clampIndex(m.timelineCursor, len(rows))
	// Axis header occupies 3 lines (labels, ticks, now).
	line := 3
	for i := 0; i < selected && i < len(rows); i++ {
		line++ // main row
		// Non-selected rows are single-line; only selected expands — skip.
	}
	// Selected main row starts at `line`.
	_, _, panelH, _ := dashboardLayout(m.width, m.height)
	viewport := panelH - 3 // title + tabs + padding approx
	if viewport < 4 {
		viewport = 4
	}
	if line < m.timelineScroll {
		m.timelineScroll = line
	}
	// Expansion may add up to 2 lines; keep main row near top third.
	if line >= m.timelineScroll+viewport-3 {
		m.timelineScroll = line - (viewport - 3)
	}
	if m.timelineScroll < 0 {
		m.timelineScroll = 0
	}
}

// handleEnter is the context-sensitive `enter` dispatch (TUI-DESIGN.md §7):
// zoom the main panel, jump to a referenced service, or run the health
// probe, depending on which tab is active.
func (m *Model) handleEnter() tea.Cmd {
	visible := m.visibleRows()
	if m.cursor >= len(visible) || !isSelectable(visible[m.cursor]) {
		return nil
	}
	row := visible[m.cursor]

	if m.selectionIsProject {
		switch m.mainTab {
		case tabDoctor:
			if m.doctorCursor >= 0 && m.doctorCursor < len(m.doctorFindings) {
				return m.jumpToService(m.doctorFindings[m.doctorCursor].Service, tabLogs)
			}
			return nil
		case tabGraph:
			rows := graphTabRows(row)
			if m.graphCursor >= 0 && m.graphCursor < len(rows) {
				return m.jumpToService(rows[m.graphCursor].Node.Name, tabLogs)
			}
			return nil
		default:
			m.zoomMainPanel()
			return nil
		}
	}

	switch m.mainTab {
	case tabDeps:
		targets := depsJumpTargets(row)
		if m.depsCursor >= 0 && m.depsCursor < len(targets) {
			return m.jumpToService(targets[m.depsCursor], tabLogs)
		}
		return nil
	case tabHealth:
		return m.triggerProbe()
	default:
		m.zoomMainPanel()
		return nil
	}
}

func (m *Model) zoomMainPanel() {
	m.viewMode = viewZoom
	m.logFollow = true
	m.logScroll = 0
	m.scrollToBottom()
}

func (m Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	_, _, _, compact := dashboardLayout(m.width, m.height)

	switch {
	case m.actionMode == actionModeExec:
		return m, m.updateExecInput(msg)

	case m.actionMode != actionModeNone && msg.Type == tea.KeyEsc:
		m.closeAction()
		return m, nil

	case m.actionMode == actionModeMenu && key.Matches(msg, keys.Down):
		m.moveActionCursor(1)
		return m, nil

	case m.actionMode == actionModeMenu && key.Matches(msg, keys.Up):
		m.moveActionCursor(-1)
		return m, nil

	case m.actionMode == actionModeMenu && key.Matches(msg, keys.Enter):
		return m, m.selectAction()

	case m.actionMode == actionModeConfirm && key.Matches(msg, keys.Enter):
		return m, m.beginAction()

	case key.Matches(msg, keys.Quit):
		m.stopLogStream()
		m.cancel()
		return m, tea.Quit

	case isRune(msg, "q") && m.actionMode == actionModeNone:
		m.stopLogStream()
		m.cancel()
		return m, tea.Quit

	case key.Matches(msg, keys.Action):
		m.openActionMenu()
		return m, nil

	case m.actionMode == actionModeNone && key.Matches(msg, keys.Help):
		m.viewMode = viewHelp
		return m, nil

	case m.actionMode == actionModeNone && key.Matches(msg, keys.JumpDoctor):
		return m, m.jumpToProjectTab(tabDoctor)

	case m.actionMode == actionModeNone && key.Matches(msg, keys.JumpTimeline):
		return m, m.jumpToProjectTab(tabTimeline)

	case m.actionMode == actionModeNone && (key.Matches(msg, keys.Tab1) || key.Matches(msg, keys.Tab2) || key.Matches(msg, keys.Tab3) || key.Matches(msg, keys.Tab4)):
		return m, m.selectMainTabFromKey(msg)

	case m.actionMode == actionModeNone && key.Matches(msg, keys.TabPrev):
		return m, m.stepMainTab(-1)

	case m.actionMode == actionModeNone && key.Matches(msg, keys.TabNext):
		return m, m.stepMainTab(1)

	case m.actionMode == actionModeNone && key.Matches(msg, keys.Filter):
		m.setRowFilter(nextFilter(m.rowFilter))
		return m, nil

	case m.actionMode == actionModeNone && msg.Type == tea.KeyEsc:
		m.setRowFilter(filterAll)
		return m, nil

	case key.Matches(msg, keys.Tab):
		if !compact {
			if m.panelFocus == focusLeft {
				m.focusPanel(focusMain)
			} else {
				m.focusPanel(focusLeft)
			}
		}
		return m, nil

	case key.Matches(msg, keys.Right):
		if !compact {
			m.focusPanel(focusMain)
		}
		return m, nil

	case key.Matches(msg, keys.Left):
		if !compact {
			m.focusPanel(focusLeft)
		}
		return m, nil

	case key.Matches(msg, keys.Search):
		if m.panelFocus == focusMain && !m.selectionIsProject && m.mainTab == tabLogs && !m.logWaiting {
			m.searching = true
		}
		return m, nil

	case key.Matches(msg, keys.NextMatch):
		if m.panelFocus == focusMain && !m.selectionIsProject && m.mainTab == tabLogs {
			m.jumpToMatch(1)
		}
		return m, nil

	case key.Matches(msg, keys.PrevMatch):
		if m.panelFocus == focusMain && !m.selectionIsProject && m.mainTab == tabLogs {
			m.jumpToMatch(-1)
		}
		return m, nil

	case key.Matches(msg, keys.FollowEnd):
		if m.panelFocus == focusMain {
			m.logFollow = true
			m.scrollToBottom()
		}
		return m, nil

	case key.Matches(msg, keys.LoadMore):
		if m.panelFocus == focusMain && !m.selectionIsProject && m.mainTab == tabLogs &&
			!m.logWaiting && !m.logNoMoreHistory && !m.logLoading && m.logContainerID != "" {
			m.logLoading = true
			return m, fetchMoreLogsCmd(m.docker, m.logContainerID, len(m.logs))
		}
		return m, nil

	case key.Matches(msg, keys.PageUp):
		if m.panelFocus == focusMain {
			m.scrollPage(-1)
		}
		return m, nil

	case key.Matches(msg, keys.PageDown):
		if m.panelFocus == focusMain {
			m.scrollPage(1)
		}
		return m, nil

	case key.Matches(msg, keys.Down):
		if m.panelFocus == focusMain {
			m.moveMainCursor(1)
			return m, nil
		}
		m.cursor = nextSelectable(m.visibleRows(), m.cursor, 1)
		m.clampGraphScroll()
		return m, m.syncSelectionStream()

	case key.Matches(msg, keys.Up):
		if m.panelFocus == focusMain {
			m.moveMainCursor(-1)
			return m, nil
		}
		m.cursor = nextSelectable(m.visibleRows(), m.cursor, -1)
		m.clampGraphScroll()
		return m, m.syncSelectionStream()

	case key.Matches(msg, keys.Enter):
		return m, m.handleEnter()
	}
	return m, nil
}

func isRune(msg tea.KeyMsg, s string) bool {
	return msg.Type == tea.KeyRunes && string(msg.Runes) == s
}

func (m *Model) selectMainTabFromKey(msg tea.KeyMsg) tea.Cmd {
	tab := m.mainTab
	switch {
	case key.Matches(msg, keys.Tab1):
		tab = 0
	case key.Matches(msg, keys.Tab2):
		tab = 1
	case key.Matches(msg, keys.Tab3):
		tab = 2
	case key.Matches(msg, keys.Tab4):
		tab = 3
	}
	if tab >= mainTabCount(m.selectionIsProject) {
		return nil
	}
	m.mainTab = tab
	return m.onMainTabChanged()
}

func (m *Model) stepMainTab(delta int) tea.Cmd {
	n := mainTabCount(m.selectionIsProject)
	m.mainTab = ((m.mainTab+delta)%n + n) % n
	return m.onMainTabChanged()
}

func (m *Model) onMainTabChanged() tea.Cmd {
	if m.selectionIsProject && m.mainTab == tabDoctor {
		visible := m.visibleRows()
		if m.cursor < len(visible) {
			return m.maybeFireDoctor(visible[m.cursor])
		}
	}
	return nil
}

func (m Model) updateZoomView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.stopLogStream()
		m.cancel()
		return m, tea.Quit

	case msg.Type == tea.KeyEsc:
		m.closeLogView()

	case isRune(msg, "q"):
		m.closeLogView()

	case key.Matches(msg, keys.Enter):
		m.closeLogView()

	case key.Matches(msg, keys.Search):
		if !m.logWaiting {
			m.searching = true
		}

	case key.Matches(msg, keys.NextMatch):
		m.jumpToMatch(1)

	case key.Matches(msg, keys.PrevMatch):
		m.jumpToMatch(-1)

	case key.Matches(msg, keys.FollowEnd):
		m.logFollow = true
		m.scrollToBottom()

	case key.Matches(msg, keys.Home):
		return m, m.scrollHome()

	case key.Matches(msg, keys.LoadMore):
		if !m.logWaiting && !m.logNoMoreHistory && !m.logLoading && m.logContainerID != "" {
			m.logLoading = true
			return m, fetchMoreLogsCmd(m.docker, m.logContainerID, len(m.logs))
		}

	case key.Matches(msg, keys.PageUp):
		m.scrollPage(-1)

	case key.Matches(msg, keys.PageDown):
		m.scrollPage(1)
		lines := m.displayLogLines()
		if len(lines) > 0 && m.logCursor >= len(lines)-1 {
			m.logFollow = true
			m.scrollToBottom()
		}

	case key.Matches(msg, keys.Down):
		m.scrollLogLine(1)

	case key.Matches(msg, keys.Up):
		m.scrollLogLine(-1)
	}
	return m, nil
}

func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		m.viewMode = viewDashboard
	case key.Matches(msg, keys.Help):
		m.viewMode = viewDashboard
	case isRune(msg, "q"):
		m.viewMode = viewDashboard
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.searching = false
		m.logFilter = ""
	case tea.KeyEnter:
		m.searching = false
		m.clampLogCursor()
	case tea.KeyBackspace:
		if len(m.logFilter) > 0 {
			m.logFilter = m.logFilter[:len(m.logFilter)-1]
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.logFilter += string(msg.Runes)
		}
	}
	return m, nil
}

// View renders the current model state to a string for display.
func (m Model) View() string {
	switch m.viewMode {
	case viewZoom:
		return renderLogFullscreen(m)
	case viewHelp:
		return renderHelp(m)
	default:
		return renderDashboard(m)
	}
}
