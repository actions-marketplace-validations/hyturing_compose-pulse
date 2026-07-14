package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSelectedLogText_Range(t *testing.T) {
	lines := []string{"a", "b", "c", "d"}
	sel := logSelection{has: true, start: 3, end: 1}
	got := selectedLogText(lines, sel)
	want := "b\nc\nd"
	if got != want {
		t.Fatalf("selectedLogText = %q, want %q", got, want)
	}
}

func TestLogSelection_ContainsSource(t *testing.T) {
	sel := logSelection{has: true, start: 2, end: 4}
	if !sel.containsSource(3) {
		t.Fatal("expected source 3 in selection")
	}
	if sel.containsSource(1) {
		t.Fatal("did not expect source 1 in selection")
	}
}

func TestRenderLogDisplayRow_Selected(t *testing.T) {
	row := logDisplayRow{text: "hello", sourceLine: 0, lineStart: true}
	line := renderLogDisplayRow(row, 40, false, true, "")
	plain := stripANSI(line)
	if !strings.Contains(plain, "hello") {
		t.Fatalf("expected text preserved, got %q", plain)
	}
}

func TestHandleCtrlC_CopiesWhenSelection(t *testing.T) {
	m := Model{
		logs:   []string{"a", "b", "c"},
		logSel: logSelection{has: true, start: 0, end: 1},
	}
	_, cmd := m.handleCtrlC()
	if cmd == nil {
		t.Fatal("expected copy cmd")
	}
	copied, ok := cmd().(logCopiedMsg)
	if !ok || copied.lines != 2 {
		t.Fatalf("got %#v", cmd())
	}
}

func TestHandleCtrlC_NoopWithoutSelection(t *testing.T) {
	m := Model{}
	_, cmd := m.handleCtrlC()
	if cmd != nil {
		t.Fatalf("expected no-op without selection, got %T", cmd())
	}
}

func TestHandleLogMouse_SelectWhileFindFocused(t *testing.T) {
	m := Model{
		viewMode:     viewZoom,
		width:        80,
		height:       24,
		logs:         []string{"one", "two", "three", "four"},
		logFind:      "two",
		logFindFocus: true,
		logFollow:    false,
		selectedSvc:  "api",
	}
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      5,
		Y:      2,
	})
	m = updated.(Model)
	if !m.logSel.has || !m.logSel.dragging {
		t.Fatalf("expected selection while find active, got %+v", m.logSel)
	}
	if m.logFindFocus {
		t.Fatal("expected find input to unfocus when starting selection")
	}
	if m.logFind != "two" {
		t.Fatalf("expected find query kept, got %q", m.logFind)
	}
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
		X:      5,
		Y:      3,
	})
	m = updated.(Model)
	lo, hi := m.logSel.normalized()
	if lo != 1 || hi != 2 {
		t.Fatalf("selection = [%d,%d], want [1,2]", lo, hi)
	}
}

func TestHandleLogMouse_SelectWithoutCopy(t *testing.T) {
	m := Model{
		viewMode:    viewZoom,
		width:       80,
		height:      24,
		logs:        []string{"one", "two", "three", "four"},
		logFollow:   false,
		logScroll:   0,
		logCursor:   0,
		selectedSvc: "api",
	}
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      5,
		Y:      2,
	})
	m = updated.(Model)
	if !m.logSel.has || !m.logSel.dragging {
		t.Fatalf("expected dragging selection, got %+v", m.logSel)
	}
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
		X:      5,
		Y:      3,
	})
	m = updated.(Model)
	updated, cmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		X:      5,
		Y:      3,
	})
	m = updated.(Model)
	if m.logSel.dragging {
		t.Fatal("expected drag to end on release")
	}
	lo, hi := m.logSel.normalized()
	if lo != 1 || hi != 2 {
		t.Fatalf("selection = [%d,%d], want [1,2]", lo, hi)
	}
	if cmd != nil {
		t.Fatalf("expected no copy on mouse release, got %T", cmd())
	}
}

func TestHandleLogMouse_ScrollBarJumpAndDrag(t *testing.T) {
	logs := make([]string, 80)
	for i := range logs {
		logs[i] = "line"
	}
	m := Model{
		viewMode:    viewZoom,
		width:       80,
		height:      24,
		logs:        logs,
		logFollow:   true,
		selectedSvc: "api",
	}
	_, y0, w, h, ok := m.logViewportRect()
	if !ok || h < 2 {
		t.Fatalf("viewport ok=%v h=%d", ok, h)
	}
	barX := w - 1

	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      barX,
		Y:      y0 + h - 1,
	})
	m = updated.(Model)
	if !m.logBarDrag.active {
		t.Fatal("expected scrollbar drag active")
	}
	if m.logFollow {
		t.Fatal("expected follow disabled after bar jump")
	}
	if m.logSel.has {
		t.Fatal("expected selection cleared on bar drag")
	}
	maxScroll := m.logMaxScroll(h)
	if m.logScroll < maxScroll/2 {
		t.Fatalf("expected jump toward bottom, scroll=%d max=%d", m.logScroll, maxScroll)
	}

	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
		X:      barX,
		Y:      y0,
	})
	m = updated.(Model)
	if m.logScroll != 0 {
		t.Fatalf("drag to top scroll=%d, want 0", m.logScroll)
	}

	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		X:      barX,
		Y:      y0,
	})
	m = updated.(Model)
	if m.logBarDrag.active {
		t.Fatal("expected bar drag ended on release")
	}
}

func TestHandleLogMouse_RightClickDoesNotCopy(t *testing.T) {
	m := Model{
		viewMode: viewZoom,
		width:    80,
		height:   24,
		logs:     []string{"one", "two", "three"},
		logSel:   logSelection{has: true, start: 0, end: 2},
	}
	_, cmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonRight,
		X:      5,
		Y:      2,
	})
	if cmd != nil {
		t.Fatalf("expected no right-click copy, got %T", cmd())
	}
}
