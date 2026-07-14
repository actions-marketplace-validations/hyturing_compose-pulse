package ui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestStatusHintContext_LeftService(t *testing.T) {
	m := Model{viewMode: viewDashboard, panelFocus: focusLeft, selectionIsProject: false}
	ctx := statusHintContext(m)
	if !strings.Contains(ctx, "logs/stats/deps/health") || !strings.Contains(ctx, "f filter") || !strings.Contains(ctx, "enter zoom") {
		t.Fatalf("ctx=%q", ctx)
	}
}

func TestStatusHintContext_LeftProject(t *testing.T) {
	m := Model{viewMode: viewDashboard, panelFocus: focusLeft, selectionIsProject: true}
	ctx := statusHintContext(m)
	if !strings.Contains(ctx, "doctor/timeline/graph") || !strings.Contains(ctx, "d/t") {
		t.Fatalf("ctx=%q", ctx)
	}
}

func TestStatusHintContext_MainLogs(t *testing.T) {
	m := Model{viewMode: viewDashboard, panelFocus: focusMain, selectionIsProject: false, mainTab: tabLogs}
	ctx := statusHintContext(m)
	if !strings.Contains(ctx, "drag") || !strings.Contains(ctx, "^F find") {
		t.Fatalf("ctx=%q", ctx)
	}
}

func TestStatusHintContext_MainHealth(t *testing.T) {
	m := Model{viewMode: viewDashboard, panelFocus: focusMain, selectionIsProject: false, mainTab: tabHealth}
	ctx := statusHintContext(m)
	if ctx != "enter run probe" {
		t.Fatalf("ctx=%q", ctx)
	}
}

func TestStatusHintContext_MainDoctor(t *testing.T) {
	m := Model{viewMode: viewDashboard, panelFocus: focusMain, selectionIsProject: true, mainTab: tabDoctor}
	ctx := statusHintContext(m)
	if !strings.Contains(ctx, "enter jump") {
		t.Fatalf("ctx=%q", ctx)
	}
}

func TestStatusHintContext_Zoom(t *testing.T) {
	m := Model{viewMode: viewZoom}
	ctx := statusHintContext(m)
	if !strings.Contains(ctx, "drag") || !strings.Contains(ctx, "^F find") {
		t.Fatalf("ctx=%q", ctx)
	}
}

func TestStatusHintContext_ActionMenu(t *testing.T) {
	m := Model{viewMode: viewDashboard, actionMode: actionModeMenu}
	ctx := statusHintContext(m)
	if !strings.Contains(ctx, "enter") {
		t.Fatalf("ctx=%q", ctx)
	}
}

func TestFormatStatusHints_DropsContextWhenNarrow(t *testing.T) {
	ctx := "1-4 logs/stats/deps/health · f filter · / · enter zoom"
	wide := formatStatusHints(200, ctx)
	if !strings.Contains(wide, "enter zoom") {
		t.Fatalf("wide should keep context: %q", wide)
	}
	coreW := runewidth.StringWidth(" " + statusHintCore)
	narrow := formatStatusHints(coreW+5, ctx)
	if strings.Contains(narrow, "enter zoom") {
		t.Fatalf("narrow should drop context: %q", narrow)
	}
	if !strings.Contains(narrow, "esc back") {
		t.Fatalf("narrow should keep core: %q", narrow)
	}
}

func TestFormatStatusHints_FitsCommonWidth(t *testing.T) {
	ctx := statusHintContext(Model{viewMode: viewDashboard, panelFocus: focusLeft})
	got := formatStatusHints(100, ctx)
	if !strings.Contains(got, "logs/stats/deps/health") {
		t.Fatalf("100-col terminal should show service tab context: %q", got)
	}
}
