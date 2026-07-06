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
)

type viewMode int

const (
	viewDashboard viewMode = iota
	viewLogFullscreen
)

type panelFocus int

const (
	focusGraph panelFocus = iota
	focusPreview
)

func firstSelectable(rows []Row) int {
	for i, r := range rows {
		if isSelectable(r) {
			return i
		}
	}
	return 0
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
	graphScroll int

	selectedSvc      string
	selectedRowKey   string
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
	filter           string
	spinFrame        int
	width, height    int

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
		snapshot:  snap,
		rows:      rows,
		cursor:    firstSelectable(rows),
		docker:    dc,
		pollCh:    dc.StartPollCh(ctx),
		cancel:    cancel,
		logFollow: true,
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

// Init starts the spinner ticker and begins listening for poll updates.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), waitForPoll(m.pollCh), m.syncSelectionStream())
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
		rebuilt, _ := m.applySnapshot(msg.Containers)
		cmds := []tea.Cmd{waitForPoll(m.pollCh)}
		if rebuilt {
			cmds = append(cmds, m.syncSelectionStream())
		}
		if m.logWaiting {
			if id := m.selectedContainerID(); id != "" {
				m.logWaiting = false
				m.logContainerID = id
				m.logs = nil
				m.logScroll = 0
				m.logCursor = 0
				m.logFollow = m.viewMode == viewLogFullscreen
				cmds = append(cmds, m.beginLogStream(id))
			}
		}
		return m, tea.Batch(cmds...)

	case logLineMsg:
		if msg.line.Err != nil {
			m.appendLogLine(fmt.Sprintf("stream error: %v", msg.line.Err))
			return m, nil
		}
		if msg.line.Line != "" {
			m.appendLogLine(msg.line.Line)
			if m.logFollow || (m.viewMode == viewDashboard && m.panelFocus == focusGraph) {
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
				m.panelFocus = focusPreview
				switch msg.Button {
				case tea.MouseButtonWheelUp:
					m.scrollLogLine(-1)
				case tea.MouseButtonWheelDown:
					m.scrollLogLine(1)
				}
			} else {
				m.panelFocus = focusGraph
			}
			return m, nil
		}
		if m.viewMode == viewLogFullscreen && !m.searching {
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
		if m.viewMode == viewLogFullscreen {
			return m.updateLogView(msg)
		}
		return m.updateDashboard(msg)
	}

	return m, nil
}

func (m *Model) applySnapshot(containers []docker.ContainerInfo) (rebuilt bool, err error) {
	newSnap, err := discover.FromContainers(containers)
	if err != nil {
		return false, err
	}

	prevKey := ""
	if m.cursor < len(m.rows) {
		prevKey = rowKey(m.rows[m.cursor])
	}

	if m.snapshot != nil && m.snapshot.SameStructure(newSnap) {
		m.snapshot.ApplyStatesFrom(newSnap)
		m.clampGraphScroll()
		return false, nil
	}

	m.snapshot = newSnap
	m.rows = BuildRows(newSnap)

	if prevKey != "" {
		if idx := findRowByKey(m.rows, prevKey); idx >= 0 {
			m.cursor = idx
		} else {
			m.cursor = clampCursor(m.cursor, m.rows)
		}
	} else {
		m.cursor = firstSelectable(m.rows)
	}
	m.clampGraphScroll()
	return true, nil
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

func (m *Model) syncSelectionStream() tea.Cmd {
	if m.cursor >= len(m.rows) || !isSelectable(m.rows[m.cursor]) {
		return nil
	}
	row := m.rows[m.cursor]
	key := rowKey(row)
	if key == m.selectedRowKey && (m.logCh != nil || m.logWaiting) {
		return nil
	}

	m.selectedRowKey = key
	m.selectedSvc = rowLabel(row)
	m.panelFocus = focusGraph
	m.logs = nil
	m.logScroll = 0
	m.logCursor = 0
	m.logFollow = true
	m.logNoMoreHistory = false
	m.logLoading = false
	m.filter = ""
	m.stopLogStream()

	if row.ContainerID != "" {
		m.logWaiting = false
		m.logContainerID = row.ContainerID
		return m.beginLogStream(row.ContainerID)
	}
	m.logWaiting = true
	m.logContainerID = ""
	return nil
}

func (m *Model) closeLogView() {
	m.viewMode = viewDashboard
	m.searching = false
	m.filter = ""
	m.logFollow = false
}

func (m *Model) focusPanel(focus panelFocus) {
	m.panelFocus = focus
	if focus == focusGraph {
		m.logFollow = true
		m.scrollToBottom()
	}
}

func (m Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	_, _, _, compact := dashboardLayout(m.width, m.height)

	switch {
	case m.actionMode == actionModeExec:
		return m, m.updateExecInput(msg)

	case m.actionMode != actionModeNone && msg.Type == tea.KeyEsc:
		m.closeAction()

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

	case key.Matches(msg, keys.Action):
		m.openActionMenu()

	case key.Matches(msg, keys.Tab):
		if !compact {
			if m.panelFocus == focusGraph {
				m.focusPanel(focusPreview)
			} else {
				m.focusPanel(focusGraph)
			}
		}

	case key.Matches(msg, keys.Right):
		if !compact {
			m.focusPanel(focusPreview)
		}

	case key.Matches(msg, keys.Left):
		if !compact {
			m.focusPanel(focusGraph)
		}

	case key.Matches(msg, keys.FollowEnd):
		if m.panelFocus == focusPreview {
			m.logFollow = true
			m.scrollToBottom()
		}

	case key.Matches(msg, keys.PageUp):
		if m.panelFocus == focusPreview {
			m.scrollPage(-1)
		}

	case key.Matches(msg, keys.PageDown):
		if m.panelFocus == focusPreview {
			m.scrollPage(1)
		}

	case key.Matches(msg, keys.Down):
		if m.panelFocus == focusPreview {
			m.scrollLogLine(1)
			return m, nil
		}
		m.cursor = nextSelectable(m.rows, m.cursor, 1)
		m.clampGraphScroll()
		return m, m.syncSelectionStream()

	case key.Matches(msg, keys.Up):
		if m.panelFocus == focusPreview {
			m.scrollLogLine(-1)
			return m, nil
		}
		m.cursor = nextSelectable(m.rows, m.cursor, -1)
		m.clampGraphScroll()
		return m, m.syncSelectionStream()

	case key.Matches(msg, keys.Enter):
		if m.cursor < len(m.rows) && isSelectable(m.rows[m.cursor]) {
			m.viewMode = viewLogFullscreen
			m.logFollow = true
			m.logScroll = 0
			m.scrollToBottom()
		}
	}
	return m, nil
}

func (m Model) updateLogView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.closeLogView()

	case key.Matches(msg, keys.Quit):
		m.stopLogStream()
		m.cancel()
		return m, tea.Quit

	case key.Matches(msg, keys.Search):
		if !m.logWaiting {
			m.searching = true
		}

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

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.searching = false
		m.filter = ""
	case tea.KeyEnter:
		m.searching = false
		m.clampLogCursor()
	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.filter += string(msg.Runes)
		}
	}
	return m, nil
}

// View renders the current model state to a string for display.
func (m Model) View() string {
	if m.viewMode == viewLogFullscreen {
		return renderLogFullscreen(m)
	}
	return renderDashboard(m)
}
