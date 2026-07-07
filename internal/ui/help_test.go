package ui

import (
	"strings"
	"testing"
)

func TestHelpOverlay_OpenAndClose(t *testing.T) {
	m := actionTestModel(t)

	updated, _ := m.Update(keyMsg("?"))
	m = updated.(Model)
	if m.viewMode != viewHelp {
		t.Fatalf("viewMode = %v, want viewHelp", m.viewMode)
	}
	out := m.View()
	if !strings.Contains(out, "Navigation") || !strings.Contains(out, "Filters") {
		t.Errorf("expected help groups rendered, got:\n%s", out)
	}

	updated, _ = m.Update(keyMsg("esc"))
	m = updated.(Model)
	if m.viewMode != viewDashboard {
		t.Fatalf("viewMode after esc = %v, want viewDashboard", m.viewMode)
	}
}

func TestHelpOverlay_ClosesOnQuestionMarkOrQ(t *testing.T) {
	m := actionTestModel(t)

	updated, _ := m.Update(keyMsg("?"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("?"))
	m = updated.(Model)
	if m.viewMode != viewDashboard {
		t.Fatalf("viewMode after second ? = %v, want viewDashboard", m.viewMode)
	}

	updated, _ = m.Update(keyMsg("?"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("q"))
	m = updated.(Model)
	if m.viewMode != viewDashboard {
		t.Fatalf("viewMode after q = %v, want viewDashboard", m.viewMode)
	}
}
