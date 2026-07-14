package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hyturing/compose-pulse/internal/discover"
)

func TestMatchingLineIndexes_CaseInsensitive(t *testing.T) {
	lines := []string{
		`SELECT * FROM t WHERE id=1`,
		`select * from t where id=2`,
		`no match here`,
		`Where clause`,
	}
	idxs := matchingLineIndexes(lines, "where")
	if len(idxs) != 3 {
		t.Fatalf("matches = %v, want 3 indexes", idxs)
	}
	if idxs[0] != 0 || idxs[1] != 1 || idxs[2] != 3 {
		t.Fatalf("indexes = %v, want [0 1 3]", idxs)
	}
}

func TestHighlightFindMatches_SubstringOnly(t *testing.T) {
	const text = `FROM "t" WHERE id=1`
	re := compileFindPattern("where")
	if re == nil {
		t.Fatal("expected pattern")
	}
	idxs := re.FindAllStringIndex(text, -1)
	if len(idxs) != 1 || text[idxs[0][0]:idxs[0][1]] != "WHERE" {
		t.Fatalf("expected single WHERE span, got %v", idxs)
	}
	out := highlightFindMatches(text, "where")
	if stripANSI(out) != text {
		t.Fatalf("plain text changed: %q", stripANSI(out))
	}
	row := logDisplayRow{text: text, sourceLine: 0, lineStart: true}
	rendered := renderLogDisplayRow(row, 80, false, false, "where")
	plain := stripANSI(rendered)
	if !strings.Contains(plain, `FROM "t"`) || !strings.Contains(plain, "WHERE") {
		t.Fatalf("expected full row text preserved, got %q", plain)
	}
}

func TestLogFind_ScrollKeepsFocus(t *testing.T) {
	m := Model{
		viewMode:     viewZoom,
		width:        80,
		height:       24,
		logs:         []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
		logFind:      "c",
		logFindFocus: true,
		logFollow:    false,
		logScroll:    0,
		selectedSvc:  "api",
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if !m.logFindFocus {
		t.Fatal("expected find to stay focused while scrolling")
	}
	if m.logScroll < 1 && m.logCursor < 1 {
		t.Fatal("expected scroll or cursor to advance")
	}
}

func TestLogFind_EnterCyclesMatches(t *testing.T) {
	m := Model{
		viewMode:     viewZoom,
		width:        80,
		height:       24,
		logs:         []string{"alpha", "error one", "beta", "error two", "gamma"},
		logFindFocus: true,
		selectedSvc:  "api",
	}
	m.setLogFind("error")
	if got := m.logFindCounter(); got != "1/2" {
		t.Fatalf("counter = %q, want 1/2", got)
	}
	src, ok := m.currentLogFindSource()
	if !ok || src != 1 {
		t.Fatalf("first match source = %d ok=%v, want 1", src, ok)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if got := m.logFindCounter(); got != "2/2" {
		t.Fatalf("after enter counter = %q, want 2/2", got)
	}
	src, ok = m.currentLogFindSource()
	if !ok || src != 3 {
		t.Fatalf("second match source = %d ok=%v, want 3", src, ok)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if got := m.logFindCounter(); got != "1/2" {
		t.Fatalf("wrap counter = %q, want 1/2", got)
	}
}

func TestLogFind_StepPrevious(t *testing.T) {
	m := Model{
		viewMode:    viewZoom,
		width:       80,
		height:      24,
		logs:        []string{"a", "x", "b", "x", "c"},
		logFind:     "x",
		logFindIdx:  1,
		selectedSvc: "api",
	}
	m.jumpToLogFind()
	m.logFindStep(-1)
	if got := m.logFindCounter(); got != "1/2" {
		t.Fatalf("prev counter = %q, want 1/2", got)
	}
}

func TestLogFind_KeepsAllLinesVisible(t *testing.T) {
	m := Model{
		viewMode:    viewZoom,
		width:       80,
		height:      24,
		logs:        []string{"keep me", "error here", "also keep"},
		logFind:     "error",
		logFindIdx:  0,
		selectedSvc: "api",
	}
	m.jumpToLogFind()
	out := stripANSI(renderLogFullscreen(m))
	if !strings.Contains(out, "keep me") || !strings.Contains(out, "also keep") {
		t.Fatalf("expected surrounding lines visible, got:\n%s", out)
	}
	if !strings.Contains(out, "1/1") {
		t.Fatalf("expected counter 1/1 in footer, got:\n%s", out)
	}
}

func TestLogFind_EscClearsThenUnfocuses(t *testing.T) {
	m := Model{
		viewMode:     viewZoom,
		width:        80,
		height:       24,
		logs:         []string{"error"},
		logFind:      "error",
		logFindFocus: true,
		selectedSvc:  "api",
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.logFind != "" {
		t.Fatalf("expected clear query, got find=%q", m.logFind)
	}
	if !m.logFindFocus {
		t.Fatal("expected still focused after clearing query")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.logFindFocus {
		t.Fatal("expected unfocus on second esc")
	}
}

func TestLogFind_BoxOnStatusBar(t *testing.T) {
	m := Model{
		viewMode:   viewDashboard,
		panelFocus: focusMain,
		mainTab:    tabLogs,
		width:      120,
		height:     40,
		logs:       []string{"hello error world"},
		logFind:    "error",
		logFindIdx: 0,
		rows: []Row{{
			Kind:       RowStandalone,
			Standalone: &discover.Standalone{Name: "api", ID: "c1"},
		}},
		cursor: 0,
	}
	out := stripANSI(renderDashboard(m))
	lines := strings.Split(out, "\n")
	status := lines[len(lines)-1]
	if !strings.Contains(status, "error") || !strings.Contains(status, "1/1") {
		t.Fatalf("expected find box on status bar, got %q", status)
	}
	if !strings.Contains(status, "[") || !strings.Contains(status, "]") {
		t.Fatalf("expected boxed find field, got %q", status)
	}
}
