package ui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	actionspkg "github.com/hyturing/compose-pulse/internal/actions"
)

type actionMode int

const (
	actionModeNone actionMode = iota
	actionModeMenu
	actionModeConfirm
	actionModeRunning
	actionModeDone
	actionModeExec
)

type actionMenuItem struct {
	Label string
	Plan  actionspkg.Plan
}

func (m *Model) openActionMenu() {
	row, ok := m.selectedActionRow()
	if !ok {
		return
	}
	project := actionspkg.Project{Name: row.ProjectName, ConfigFiles: row.ConfigFiles}
	service := row.Node.Name
	m.actionItems = []actionMenuItem{
		{Label: "restart selected", Plan: actionspkg.PlanRestartSelected(project, service)},
		{Label: "restart dependents", Plan: actionspkg.PlanRestartDependents(project, row.Graph, service)},
		{Label: "rebuild selected", Plan: actionspkg.PlanRebuildSelected(project, service)},
		{Label: "rebuild + restart dependents", Plan: actionspkg.PlanRebuildAndRestartDependents(project, row.Graph, service)},
	}
	if row.ContainerID != "" {
		m.actionItems = append(m.actionItems, actionMenuItem{
			Label: "exec shell",
			Plan:  actionspkg.PlanExecShell(row.ContainerID),
		})
	}
	m.actionMode = actionModeMenu
	m.actionCursor = 0
	m.actionPlan = actionspkg.Plan{}
	m.actionOutput = nil
	m.actionErr = ""
}

func (m Model) selectedActionRow() (Row, bool) {
	if m.cursor >= len(m.rows) || m.cursor < 0 {
		return Row{}, false
	}
	row := m.rows[m.cursor]
	return row, row.Kind == RowComposeNode && row.Node != nil
}

func (m *Model) closeAction() {
	if m.actionCancel != nil {
		m.actionCancel()
		m.actionCancel = nil
	}
	m.actionMode = actionModeNone
	m.actionCursor = 0
	m.actionPlan = actionspkg.Plan{}
	m.actionOutput = nil
	m.actionErr = ""
	m.actionCh = nil
	m.execContainerID = ""
	m.execService = ""
	m.execInput = ""
	m.execActive = false
}

func (m *Model) moveActionCursor(delta int) {
	if len(m.actionItems) == 0 {
		return
	}
	m.actionCursor += delta
	if m.actionCursor < 0 {
		m.actionCursor = 0
	}
	if m.actionCursor >= len(m.actionItems) {
		m.actionCursor = len(m.actionItems) - 1
	}
}

func (m *Model) selectAction() tea.Cmd {
	if len(m.actionItems) == 0 || m.actionCursor >= len(m.actionItems) {
		return nil
	}
	plan := m.actionItems[m.actionCursor].Plan
	m.actionPlan = plan
	if plan.Title == "Exec shell" && len(plan.Steps) > 0 {
		m.actionMode = actionModeExec
		m.execContainerID = plan.Steps[0].Service
		m.execService = rowLabel(m.rows[m.cursor])
		m.execInput = ""
		m.actionOutput = []string{"in-TUI exec started; type a command and press enter"}
		m.actionErr = ""
		return nil
	}
	if plan.RequiresConfirm {
		m.actionMode = actionModeConfirm
		return nil
	}
	return m.beginAction()
}

func (m *Model) beginAction() tea.Cmd {
	if len(m.actionPlan.Steps) == 0 {
		return nil
	}
	if m.actionCancel != nil {
		m.actionCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.actionCancel = cancel
	runner := m.actionRunner
	if runner == nil {
		runner = actionspkg.Start
	}
	m.actionCh = runner(ctx, m.actionPlan)
	m.execActive = m.actionMode == actionModeExec
	m.actionMode = actionModeRunning
	if !m.execActive {
		m.actionOutput = nil
	}
	m.actionErr = ""
	return waitForActionEvent(m.actionCh)
}

func waitForActionEvent(ch <-chan actionspkg.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return actionEventMsg{event: actionspkg.Event{Done: true}}
		}
		return actionEventMsg{event: event}
	}
}

func renderActionView(m Model, width int) string {
	switch m.actionMode {
	case actionModeMenu:
		return renderActionMenu(m, width)
	case actionModeExec:
		return renderExecView(m, width)
	case actionModeConfirm, actionModeRunning, actionModeDone:
		return renderActionPlan(m, width)
	default:
		return ""
	}
}

func (m *Model) updateExecInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.closeAction()
	case tea.KeyBackspace:
		if len(m.execInput) > 0 {
			m.execInput = m.execInput[:len(m.execInput)-1]
		}
	case tea.KeyEnter:
		cmd := strings.TrimSpace(m.execInput)
		m.execInput = ""
		if cmd == "" {
			return nil
		}
		if cmd == "exit" || cmd == "quit" {
			m.closeAction()
			return nil
		}
		m.actionPlan = actionspkg.PlanExecCommand(m.execContainerID, cmd)
		m.actionOutput = append(m.actionOutput, "$ "+cmd)
		return m.beginAction()
	default:
		if msg.Type == tea.KeyRunes {
			m.execInput += string(msg.Runes)
		}
	}
	return nil
}

func renderExecView(m Model, width int) string {
	var b strings.Builder
	title := "Exec"
	if m.execService != "" {
		title += " · " + m.execService
	}
	b.WriteString(padMetaLine(title, width))
	b.WriteString("\n")
	b.WriteString(padMetaLine(strings.Repeat("─", maxInt(8, width-scrollBarWidth-4)), width))
	for _, line := range m.actionOutput {
		b.WriteString("\n")
		b.WriteString(padMetaLine(line, width))
	}
	if m.actionErr != "" {
		b.WriteString("\n")
		b.WriteString(padMetaLine("error: "+m.actionErr, width))
	}
	b.WriteString("\n")
	b.WriteString(padMetaLine("> "+m.execInput, width))
	b.WriteString("\n")
	b.WriteString(padMetaLine("enter: run   esc: exit exec   type exit to close", width))
	return b.String()
}

func renderActionMenu(m Model, width int) string {
	var b strings.Builder
	b.WriteString(padMetaLine("Actions · "+rowLabel(m.rows[m.cursor]), width))
	b.WriteString("\n")
	b.WriteString(padMetaLine(strings.Repeat("─", maxInt(8, width-scrollBarWidth-4)), width))
	for i, item := range m.actionItems {
		b.WriteString("\n")
		prefix := "  "
		if i == m.actionCursor {
			prefix = styleLogMarker.Render("▸") + " "
		}
		b.WriteString(padMetaLine(prefix+item.Label, width))
	}
	b.WriteString("\n")
	b.WriteString(padMetaLine("enter: choose   esc: cancel", width))
	return b.String()
}

func renderActionPlan(m Model, width int) string {
	var b strings.Builder
	title := m.actionPlan.Title
	if title == "" {
		title = "Action"
	}
	b.WriteString(padMetaLine(title, width))
	if m.actionPlan.ConfirmText != "" {
		b.WriteString("\n")
		b.WriteString(padMetaLine(m.actionPlan.ConfirmText, width))
	}
	for i, step := range m.actionPlan.Steps {
		b.WriteString("\n")
		b.WriteString(padMetaLine(fmt.Sprintf("%d. %s", i+1, step.Label), width))
	}
	if len(m.actionOutput) > 0 {
		b.WriteString("\n")
		b.WriteString(padMetaLine(strings.Repeat("─", maxInt(8, width-scrollBarWidth-4)), width))
		for _, line := range m.actionOutput {
			b.WriteString("\n")
			b.WriteString(padMetaLine(line, width))
		}
	}
	if m.actionErr != "" {
		b.WriteString("\n")
		b.WriteString(padMetaLine("error: "+m.actionErr, width))
	}
	b.WriteString("\n")
	b.WriteString(padMetaLine("enter: run   esc: cancel", width))
	return b.String()
}
