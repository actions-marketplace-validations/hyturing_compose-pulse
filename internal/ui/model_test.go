package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	actionspkg "github.com/hyturing/compose-pulse/internal/actions"
	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestDashboardLayout_Split(t *testing.T) {
	left, right, panelH, compact := dashboardLayout(100, 30)
	if compact {
		t.Fatal("expected split layout at 100x30")
	}
	// 45% → 45, minLeft 58; main gets 42.
	if left != 58 {
		t.Errorf("leftW = %d, want 58", left)
	}
	if right != 42 {
		t.Errorf("rightW = %d, want 42", right)
	}
	if panelH != 28 {
		t.Errorf("panelH = %d, want 28", panelH)
	}
}

func TestDashboardLayout_Compact(t *testing.T) {
	_, right, _, compact := dashboardLayout(60, 15)
	if !compact {
		t.Fatal("expected compact layout")
	}
	if right != 0 {
		t.Errorf("rightW = %d, want 0 in compact mode", right)
	}
}

func TestRenderDashboard(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"api": {},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["api"].ContainerID = "c1"
	graph.ByName["api"].State = docker.StateHealthy

	snap := &discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	}
	rows := BuildRows(snap)
	idx := findRowByKey(rows, "compose:app:api")
	if idx < 0 {
		t.Fatal("api row not found")
	}
	m := Model{
		snapshot: snap,
		rows:     rows,
		cursor:   idx,
		width:    100,
		height:   30,
		lastPoll: time.Now(),
	}
	out := renderDashboard(m)
	if !strings.Contains(out, "STATUS") || !strings.Contains(out, "DETAIL") {
		t.Error("expected column headers in left panel title")
	}
	if strings.Contains(out, "Dependency graph") {
		t.Error("did not expect Dependency graph title")
	}
	if !strings.Contains(out, "app") {
		t.Error("expected compose project name in left panel")
	}
	if !strings.Contains(out, "services") {
		t.Error("expected project summary in right panel")
	}
	if !strings.Contains(out, "api · healthy") {
		t.Error("expected main panel title with selected service name and state")
	}
	if !strings.Contains(out, "1 logs") || !strings.Contains(out, "2 stats") || !strings.Contains(out, "3 deps") {
		t.Errorf("expected service tab strip, got:\n%s", stripANSI(out))
	}
	if !strings.Contains(out, "enter zoom") {
		t.Error("expected status bar zoom hint")
	}
	if !strings.Contains(out, "esc back") {
		t.Error("expected esc back hint in status bar")
	}
	if !strings.Contains(out, "f filter") {
		t.Error("expected f filter hint in status bar")
	}
	if !strings.Contains(out, "tab/←→") {
		t.Error("expected tab and side-key panel switching hint")
	}
	if !strings.Contains(out, "logs/stats/deps/health") {
		t.Error("expected named service tab hints in status bar")
	}
	if !strings.Contains(out, "app") || !strings.Contains(out, "1 services") || !strings.Contains(out, "updated") {
		t.Errorf("expected summary bar with project/service counts, got:\n%s", out)
	}
	for i, line := range strings.Split(out, "\n") {
		plain := stripANSI(line)
		if w := runewidth.StringWidth(plain); w > m.width {
			t.Fatalf("line %d width = %d, want <= %d: %q", i, w, m.width, plain)
		}
		if i > 0 && i < m.height-1 && !hasRightPanelEdge(plain) {
			t.Fatalf("panel line %d missing right border: %q", i, plain)
		}
	}
}

func TestDashboardPanelFocus_LeftRightKeys(t *testing.T) {
	m := actionTestModel(t)

	updated, _ := m.Update(keyMsg("right"))
	m = updated.(Model)
	if m.panelFocus != focusMain {
		t.Fatalf("panelFocus after right = %v, want preview", m.panelFocus)
	}

	updated, _ = m.Update(keyMsg("left"))
	m = updated.(Model)
	if m.panelFocus != focusLeft {
		t.Fatalf("panelFocus after left = %v, want graph", m.panelFocus)
	}
}

func TestRenderGraphContent_SelectedRowFullWidth(t *testing.T) {
	row := Row{
		Kind:       RowComposeNode,
		Node:       &dag.Node{Name: "api", ContainerID: "c1", State: docker.StateHealthy},
		linePrefix: "  ",
	}

	m := Model{spinFrame: 0}
	out := renderGraphContent(m, []Row{row}, 0, 0, 40, 1)
	plain := stripANSI(out)
	if w := runewidth.StringWidth(plain); w != 40 {
		t.Fatalf("selected row width = %d, want 40: %q", w, plain)
	}
	if !strings.Contains(plain, "api") || !strings.Contains(plain, "healthy") {
		t.Fatalf("expected full selected row content: %q", plain)
	}
}

func TestActionMenu_OpenAndRender(t *testing.T) {
	m := actionTestModel(t)

	updated, _ := m.Update(keyMsg("x"))
	got := updated.(Model)

	if got.actionMode != actionModeMenu {
		t.Fatalf("actionMode = %v, want menu", got.actionMode)
	}
	out := renderActionView(got, 60)
	if !strings.Contains(out, "Actions · api") {
		t.Fatalf("expected action menu title, got:\n%s", out)
	}
	if !strings.Contains(out, "restart") {
		t.Fatalf("expected restart action, got:\n%s", out)
	}
}

func TestActionMenu_DependentRestartRequiresConfirmation(t *testing.T) {
	m := actionTestModel(t)
	updated, _ := m.Update(keyMsg("x"))
	m = updated.(Model)

	// Move from restart selected to restart dependents.
	updated, _ = m.Update(keyMsg("down"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)

	if m.actionMode != actionModeConfirm {
		t.Fatalf("actionMode = %v, want confirm", m.actionMode)
	}
	out := renderActionView(m, 60)
	if !strings.Contains(out, "Restart dependents") {
		t.Fatalf("expected confirm view, got:\n%s", out)
	}
	if !strings.Contains(out, "restart worker") {
		t.Fatalf("expected dependent step, got:\n%s", out)
	}
}

func TestActionSelectedStartsRunnerImmediatelyAndShowsOutput(t *testing.T) {
	m := actionTestModel(t)
	events := make(chan actionspkg.Event, 2)
	events <- actionspkg.Event{Line: "restarted api"}
	events <- actionspkg.Event{Done: true}
	close(events)
	m.actionRunner = func(context.Context, actionspkg.Plan) <-chan actionspkg.Event {
		return events
	}

	updated, _ := m.Update(keyMsg("x"))
	m = updated.(Model)
	updated, cmd := m.Update(keyMsg("enter"))
	m = updated.(Model)

	if m.actionMode != actionModeRunning {
		t.Fatalf("actionMode = %v, want running", m.actionMode)
	}
	if cmd == nil {
		t.Fatal("expected immediate action runner command")
	}
	msg := cmd()
	updated, cmd = m.Update(msg)
	m = updated.(Model)
	if !strings.Contains(strings.Join(m.actionOutput, "\n"), "restarted api") {
		t.Fatalf("expected action output, got %#v", m.actionOutput)
	}
	msg = cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)
	if m.actionMode != actionModeDone {
		t.Fatalf("actionMode = %v, want done", m.actionMode)
	}
}

func TestActionFailureStopsAndRendersError(t *testing.T) {
	m := actionTestModel(t)
	events := make(chan actionspkg.Event, 1)
	events <- actionspkg.Event{Err: fmt.Errorf("boom"), Done: true}
	close(events)
	m.actionRunner = func(context.Context, actionspkg.Plan) <-chan actionspkg.Event {
		return events
	}

	updated, _ := m.Update(keyMsg("x"))
	m = updated.(Model)
	updated, cmd := m.Update(keyMsg("enter"))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected immediate action runner command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)

	if m.actionMode != actionModeDone {
		t.Fatalf("actionMode = %v, want done", m.actionMode)
	}
	out := renderActionView(m, 60)
	if !strings.Contains(out, "error: boom") {
		t.Fatalf("expected rendered error, got:\n%s", out)
	}
}

func TestActionMenu_EscapeCloses(t *testing.T) {
	m := actionTestModel(t)

	updated, _ := m.Update(keyMsg("x"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("esc"))
	m = updated.(Model)

	if m.actionMode != actionModeNone {
		t.Fatalf("actionMode = %v, want none", m.actionMode)
	}
}

func TestExecActionOpensInTUIExecPrompt(t *testing.T) {
	m := openExecAction(t)

	if m.actionMode != actionModeExec {
		t.Fatalf("actionMode = %v, want exec", m.actionMode)
	}
	out := renderActionView(m, 60)
	if !strings.Contains(out, "Exec · api") {
		t.Fatalf("expected exec view title, got:\n%s", out)
	}
	if !strings.Contains(out, "esc: exit exec") {
		t.Fatalf("expected exec exit hint, got:\n%s", out)
	}
}

func TestExecActionRunsLineWithoutLeavingTUI(t *testing.T) {
	m := openExecAction(t)
	events := make(chan actionspkg.Event, 2)
	events <- actionspkg.Event{Line: "/app"}
	events <- actionspkg.Event{Done: true}
	close(events)
	var got actionspkg.Plan
	m.actionRunner = func(_ context.Context, plan actionspkg.Plan) <-chan actionspkg.Event {
		got = plan
		return events
	}

	updated, _ := m.Update(keyMsg("p"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("w"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("d"))
	m = updated.(Model)
	updated, cmd := m.Update(keyMsg("enter"))
	m = updated.(Model)

	if m.actionMode != actionModeRunning {
		t.Fatalf("actionMode = %v, want running", m.actionMode)
	}
	want := []string{"exec", "api-id", "sh", "-lc", "pwd"}
	if len(got.Steps) != 1 || !reflect.DeepEqual(got.Steps[0].Command.Args, want) {
		t.Fatalf("exec args = %#v, want %#v", got.Steps, want)
	}
	updated, cmd = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.actionMode != actionModeExec {
		t.Fatalf("actionMode after command = %v, want exec", m.actionMode)
	}
	if !strings.Contains(strings.Join(m.actionOutput, "\n"), "/app") {
		t.Fatalf("expected exec output, got %#v", m.actionOutput)
	}
}

func TestExecActionExitClosesExecMode(t *testing.T) {
	m := openExecAction(t)

	for _, key := range []string{"e", "x", "i", "t", "enter"} {
		updated, _ := m.Update(keyMsg(key))
		m = updated.(Model)
	}

	if m.actionMode != actionModeNone {
		t.Fatalf("actionMode = %v, want none", m.actionMode)
	}
}

func hasRightPanelEdge(line string) bool {
	return strings.HasSuffix(line, "│") ||
		strings.HasSuffix(line, "┐") ||
		strings.HasSuffix(line, "┘")
}

func actionTestModel(t *testing.T) Model {
	t.Helper()
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db":     {},
			"api":    {DependsOn: compose.DependsOn{"db": {}}},
			"worker": {DependsOn: compose.DependsOn{"api": {}}},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["api"].ContainerID = "api-id"
	graph.ByName["api"].State = docker.StateHealthy
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{
			Name:        "app",
			Graph:       graph,
			ConfigFiles: []string{"/tmp/compose.yml"},
		}},
	})
	idx := findRowByKey(rows, "compose:app:api")
	if idx < 0 {
		t.Fatal("api row not found")
	}
	return Model{
		rows:      rows,
		cursor:    idx,
		width:     100,
		height:    30,
		logs:      []string{"hello"},
		logFollow: true,
	}
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func openExecAction(t *testing.T) Model {
	t.Helper()
	m := actionTestModel(t)
	updated, _ := m.Update(keyMsg("x"))
	m = updated.(Model)
	found := false
	for i, item := range m.actionItems {
		if item.Label == "exec shell" {
			m.actionCursor = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("exec shell action not found")
	}
	updated, _ = m.Update(keyMsg("enter"))
	return updated.(Model)
}

func TestRenderLogFullscreen(t *testing.T) {
	m := Model{
		selectedSvc: "api",
		logs:        []string{"line one", "line two"},
		logFollow:   true,
		width:       80,
		height:      24,
	}
	out := renderLogFullscreen(m)
	if strings.Contains(out, strings.Repeat("\n", 5)) && len(out) < 80 {
		t.Error("expected full-width output without overlay padding")
	}
	if !strings.Contains(out, "cpulse · api") {
		t.Error("expected header with service name")
	}
	if !strings.Contains(out, "q back") {
		t.Error("expected footer with back hint")
	}
}

func TestRenderView_Snapshot(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db":  {},
			"app": {DependsOn: compose.DependsOn{"db": {}}},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["db"].State = docker.StateHealthy
	graph.ByName["app"].State = docker.StateStarting

	snap := &discover.Snapshot{
		Projects: []discover.Project{{
			Name:  "myapp",
			Graph: graph,
		}},
		Standalone: []discover.Standalone{{
			ID:    "x1",
			Name:  "stray",
			Image: "nginx:alpine",
			State: docker.StateHealthy,
		}},
	}

	rows := BuildRows(snap)
	out := renderView(rows, firstSelectable(rows), 80)

	// Project headers and pstrees share the left panel (renderView).
	if !strings.Contains(out, "OTHER CONTAINERS") {
		t.Error("expected standalone section header")
	}
	if !strings.Contains(out, "stray") {
		t.Error("expected standalone container name")
	}
	if !strings.Contains(out, "db") || !strings.Contains(out, "app") {
		t.Error("expected compose services in tree")
	}
}

func TestFirstSelectable_IncludesProjectHeader(t *testing.T) {
	rows := []Row{
		{Kind: RowProjectHeader, Label: "COMPOSE · app", ProjectName: "app"},
		{Kind: RowComposeNode, Node: &dag.Node{Name: "web"}},
	}
	if got := firstSelectable(rows); got != 0 {
		t.Errorf("firstSelectable = %d, want 0 (project row is selectable)", got)
	}
}

func TestRowKey_CursorPreservation(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"web": {},
			"api": {},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["web"].ContainerID = "w1"
	graph.ByName["api"].ContainerID = "a1"

	snap1 := &discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	}
	rows1 := BuildRows(snap1)
	key := "compose:app:web"
	if rowKey(rows1[findRowByKey(rows1, key)]) != key {
		t.Fatalf("web row not found in initial rows")
	}

	snap2 := &discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
		Standalone: []discover.Standalone{{
			ID: "x1", Name: "stray", Image: "nginx", State: docker.StateHealthy,
		}},
	}
	rows2 := BuildRows(snap2)
	idx := findRowByKey(rows2, key)
	if idx < 0 {
		t.Fatalf("row key %q not found after rebuild", key)
	}
	if rows2[idx].Node.Name != "web" {
		t.Errorf("expected web node, got %q", rows2[idx].Node.Name)
	}
}

func TestApplySnapshot_RebuildSyncsSelectedRowKeyWithCursor(t *testing.T) {
	composePath := filepath.Join(t.TempDir(), "compose.yml")
	composeYAML := "services:\n  db:\n    image: postgres\n  api:\n    image: api\n    depends_on:\n      - db\n"
	if err := os.WriteFile(composePath, []byte(composeYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db":  {},
			"api": {DependsOn: compose.DependsOn{"db": {}}},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["db"].ContainerID = "db1"
	graph.ByName["db"].State = docker.StateHealthy

	snap := &discover.Snapshot{
		Projects: []discover.Project{{
			Name:        "app",
			Graph:       graph,
			ConfigFiles: []string{composePath},
		}},
	}
	rows := BuildRows(snap)
	apiIdx := findRowByKey(rows, "compose:app:api")
	dbIdx := findRowByKey(rows, "compose:app:db")
	if apiIdx < 0 || dbIdx < 0 {
		t.Fatal("expected api and db rows")
	}

	m := Model{
		snapshot:       snap,
		rows:           rows,
		cursor:         apiIdx,
		selectedRowKey: rowKey(rows[dbIdx]), // stale: cursor on api, selection still db
		selectedSvc:    "db",
		logWaiting:     true,
	}

	containers := []docker.ContainerInfo{
		{
			ID: "db1", Labels: map[string]string{
				"com.docker.compose.project":              "app",
				"com.docker.compose.service":              "db",
				"com.docker.compose.project.config_files": composePath,
			}, State: docker.StateHealthy,
		},
		{
			ID: "x1", Names: []string{"/stray"}, Image: "nginx:alpine",
			Labels: map[string]string{}, State: docker.StateHealthy,
		},
	}

	rebuilt, err := m.applySnapshot(containers)
	if err != nil {
		t.Fatal(err)
	}
	if !rebuilt {
		t.Fatal("expected topology rebuild when standalone container appears")
	}
	if m.selectedRowKey == rowKey(m.rows[m.cursor]) {
		t.Fatal("applySnapshot alone must not fix stale selectedRowKey")
	}

	_ = m.syncSelectionStream()

	wantKey := rowKey(m.rows[m.cursor])
	if m.selectedRowKey != wantKey {
		t.Fatalf("selectedRowKey = %q, want %q (cursor service)", m.selectedRowKey, wantKey)
	}
	if m.selectedSvc != "api" {
		t.Fatalf("selectedSvc = %q, want api", m.selectedSvc)
	}
	if !m.logWaiting {
		t.Fatal("expected logWaiting for pending api after selection sync")
	}
	if m.selectedContainerID() != "" {
		t.Fatalf("selectedContainerID = %q, want empty for pending api", m.selectedContainerID())
	}
}

func TestBuildWaitingContent_BlockedBy(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"postgres": {},
			"api": {
				DependsOn: compose.DependsOn{
					"postgres": {Condition: "service_healthy"},
				},
			},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["postgres"].ContainerID = "pg"
	graph.ByName["postgres"].State = docker.StateStarting

	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	apiRow := rows[2]

	m := Model{
		selectedRowKey: rowKey(apiRow),
		rows:           rows,
	}
	content := buildWaitingContent(m)
	if !strings.Contains(content, "Blocked by:") {
		t.Error("expected blocked-by message")
	}
	if !strings.Contains(content, "postgres") {
		t.Error("expected postgres in blocked-by list")
	}
}

func TestPrependOlderLogs(t *testing.T) {
	current := []string{"line201", "line202", "line203"}
	fetched := make([]string, 203)
	for i := range fetched {
		fetched[i] = fmt.Sprintf("line%d", i+1)
	}
	merged, added, noMore := prependOlderLogs(current, fetched)
	if noMore {
		t.Fatal("expected more history")
	}
	if added != 200 {
		t.Errorf("added = %d, want 200", added)
	}
	if len(merged) != 203 {
		t.Errorf("merged len = %d, want 203", len(merged))
	}

	_, _, noMore = prependOlderLogs(current, current)
	if !noMore {
		t.Error("expected no more history when fetch size equals current")
	}
}

func TestLogTitleSuffix(t *testing.T) {
	m := Model{
		logs:      []string{"a", "b", "c"},
		logCursor: 1,
		logFollow: false,
	}
	if s := logTitleSuffix(m); !strings.Contains(s, "line 2/3 · paused") {
		t.Errorf("unexpected suffix: %q", s)
	}
	m.logFollow = true
	if s := logTitleSuffix(m); !strings.Contains(s, "following") {
		t.Errorf("expected following suffix, got %q", s)
	}
}

func TestEffectiveState_InRender(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"db":  {},
			"app": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["db"].ContainerID = "d1"
	graph.ByName["db"].State = docker.StateStarting

	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	out := renderView(rows, firstSelectable(rows), 80)
	if !strings.Contains(out, "blocked") {
		t.Error("expected blocked label for app service waiting on db:healthy")
	}
}

// TestUpdateDashboard_JumpTimelineDirectlyFromService is a regression test:
// jumping straight to 't' from a service selection must not let the
// project<->service kind-change reset inside syncSelectionStream clobber the
// requested tab back to 0 (doctor).
func TestUpdateDashboard_JumpTimelineDirectlyFromService(t *testing.T) {
	m := actionTestModel(t)
	if m.selectionIsProject {
		t.Fatal("fixture should start on a service selection")
	}

	updated, _ := m.Update(keyMsg("t"))
	m = updated.(Model)
	if !m.selectionIsProject {
		t.Fatal("expected project row selected after 't'")
	}
	if m.mainTab != tabTimeline {
		t.Fatalf("mainTab = %d, want tabTimeline (must survive the kind-change reset)", m.mainTab)
	}
}

func TestUpdateDashboard_JumpDoctorAndTimeline(t *testing.T) {
	m := actionTestModel(t)

	updated, _ := m.Update(keyMsg("d"))
	m = updated.(Model)
	if !m.selectionIsProject || m.mainTab != tabDoctor {
		t.Fatalf("expected project selected on doctor tab, got selectionIsProject=%v mainTab=%d", m.selectionIsProject, m.mainTab)
	}

	updated, _ = m.Update(keyMsg("t"))
	m = updated.(Model)
	if !m.selectionIsProject || m.mainTab != tabTimeline {
		t.Fatalf("expected project selected on timeline tab, got selectionIsProject=%v mainTab=%d", m.selectionIsProject, m.mainTab)
	}
}

func TestUpdateDashboard_MainTabNumberKeysAndBrackets(t *testing.T) {
	m := actionTestModel(t)

	updated, _ := m.Update(keyMsg("2"))
	m = updated.(Model)
	if m.mainTab != tabStats {
		t.Fatalf("mainTab after '2' = %d, want tabStats", m.mainTab)
	}

	updated, _ = m.Update(keyMsg("4"))
	m = updated.(Model)
	if m.mainTab != tabHealth {
		t.Fatalf("mainTab after '4' = %d, want tabHealth", m.mainTab)
	}

	updated, _ = m.Update(keyMsg("["))
	m = updated.(Model)
	if m.mainTab != tabDeps {
		t.Fatalf("mainTab after '[' = %d, want tabDeps", m.mainTab)
	}

	updated, _ = m.Update(keyMsg("]"))
	m = updated.(Model)
	if m.mainTab != tabHealth {
		t.Fatalf("mainTab after ']' = %d, want tabHealth", m.mainTab)
	}
	updated, _ = m.Update(keyMsg("]"))
	m = updated.(Model)
	if m.mainTab != tabLogs {
		t.Fatalf("mainTab after wrap ']' = %d, want tabLogs", m.mainTab)
	}
}

func TestUpdateDashboard_ProjectSelectionSwitchesToProjectTabs(t *testing.T) {
	m := actionTestModel(t)
	projectIdx := findRowByKey(m.rows, "project:app")
	if projectIdx < 0 {
		t.Fatal("expected project row")
	}
	m.cursor = projectIdx
	m.syncSelectionStream()
	if !m.selectionIsProject {
		t.Fatal("expected selectionIsProject after selecting project row")
	}
	if m.mainTab != tabDoctor {
		t.Fatalf("mainTab = %d, want tabDoctor (reset on selection kind change)", m.mainTab)
	}

	updated, _ := m.Update(keyMsg("2"))
	m = updated.(Model)
	if m.mainTab != tabTimeline {
		t.Fatalf("mainTab after '2' on project = %d, want tabTimeline", m.mainTab)
	}
	updated, _ = m.Update(keyMsg("3"))
	m = updated.(Model)
	if m.mainTab != tabGraph {
		t.Fatalf("mainTab after '3' on project = %d, want tabGraph", m.mainTab)
	}
}
