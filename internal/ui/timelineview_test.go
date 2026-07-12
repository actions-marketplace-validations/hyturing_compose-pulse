package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/timeline"
)

func TestTimelineBars_ScaledAndStaggered(t *testing.T) {
	// Tests run without a TTY; force color so background fills emit ANSI.
	lipgloss.SetColorProfile(termenv.TrueColor)

	spans := []timeline.Span{
		{
			Service:  "redis",
			Final:    dag.DisplayHealthy,
			Duration: 2 * time.Second,
			Segments: []timeline.Segment{
				{State: dag.DisplayStarting, Dur: 500 * time.Millisecond},
				{State: dag.DisplayHealthy, Dur: 1500 * time.Millisecond},
			},
		},
		{
			Service:  "postgres",
			Final:    dag.DisplayUnhealthy,
			Duration: 10 * time.Second,
			Segments: []timeline.Segment{
				{State: dag.DisplayPending, Dur: 2 * time.Second},
				{State: dag.DisplayStarting, Dur: 1 * time.Second},
				{State: dag.DisplayUnhealthy, Dur: 7 * time.Second},
			},
		},
		{
			Service:  "api",
			Final:    dag.DisplayBlocked,
			Duration: 10 * time.Second,
			WaitsOn:  "postgres:healthy",
			Segments: []timeline.Segment{
				{State: dag.DisplayPending, Dur: 3 * time.Second},
				{State: dag.DisplayBlocked, Dur: 7 * time.Second},
			},
		},
	}

	rows := flatTimelineRows(spans)
	lines := renderTimelineBars(rows, 10*time.Second, 8, 40, -1)
	joined := strings.Join(lines, "\n")
	plain := stripANSI(joined)
	if !strings.Contains(plain, "0s") || !strings.Contains(plain, "─") || !strings.Contains(plain, "┬") {
		t.Fatalf("expected horizontal axis with ticks, got:\n%s", plain)
	}
	if !strings.Contains(plain, "now") {
		t.Fatalf("expected live now cursor, got:\n%s", plain)
	}
	for _, glyph := range []rune{'░', '▒', '█', '▄', '┄'} {
		if strings.ContainsRune(plain, glyph) {
			t.Fatalf("bars still use glyph fill %q:\n%s", glyph, plain)
		}
	}
	if !strings.ContainsRune(plain, '▬') {
		t.Fatalf("expected mid-cell bar fills, got:\n%s", plain)
	}
	// Stagger: measure plain (non-ANSI) leading spaces on the bar itself.
	pxPerSec := float64(40) / 10.0
	redisLead := countPlainBarLeading(renderTimelineBar(spans[0], 10*time.Second, 40, pxPerSec))
	postgresLead := countPlainBarLeading(renderTimelineBar(spans[1], 10*time.Second, 40, pxPerSec))
	if redisLead <= postgresLead {
		t.Fatalf("expected redis more staggered than postgres: redis=%d postgres=%d\n%s", redisLead, postgresLead, plain)
	}
}

func flatTimelineRows(spans []timeline.Span) []timelineTreeSpan {
	out := make([]timelineTreeSpan, len(spans))
	for i, s := range spans {
		out[i] = timelineTreeSpan{Span: s}
	}
	return out
}

func countPlainBarLeading(bar string) int {
	n := 0
	for i := 0; i < len(bar); i++ {
		if bar[i] == ' ' {
			n++
			continue
		}
		// ANSI segment fill or end marker starts here.
		return n
	}
	return n
}

func TestTimelineBars_SelectedExpansionAndWaitsOn(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	spans := []timeline.Span{
		{
			Service:  "api",
			Final:    dag.DisplayBlocked,
			Duration: 10 * time.Second,
			WaitsOn:  "postgres:healthy",
			Segments: []timeline.Segment{
				{State: dag.DisplayBlocked, Dur: 10 * time.Second},
			},
		},
		{
			Service:  "redis",
			Final:    dag.DisplayHealthy,
			Duration: 4 * time.Second,
			Segments: []timeline.Segment{
				{State: dag.DisplayHealthy, Dur: 4 * time.Second},
			},
		},
	}
	lines := renderTimelineBars(flatTimelineRows(spans), 10*time.Second, 8, 40, 0)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "▸") {
		t.Fatalf("expected selection marker, got:\n%s", joined)
	}
	if !strings.Contains(joined, "blocked") || !strings.Contains(joined, "10s") {
		t.Fatalf("expected segment duration labels, got:\n%s", joined)
	}
	if !strings.Contains(joined, "↳ waiting for postgres:healthy") {
		t.Fatalf("expected waits-on expansion, got:\n%s", joined)
	}
}

func TestTimelineBars_AttachedLate(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	spans := []timeline.Span{
		{
			Service:      "postgres",
			Final:        dag.DisplayHealthy,
			Duration:     5 * time.Second,
			AttachedLate: true,
			Segments: []timeline.Segment{
				{State: dag.DisplayHealthy, Dur: 5 * time.Second},
			},
		},
	}
	lines := renderTimelineBars(flatTimelineRows(spans), 5*time.Second, 10, 30, 0)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "◀") {
		t.Fatalf("expected attached-late marker, got:\n%s", joined)
	}
	if !strings.Contains(joined, "already ready when cpulse attached") {
		t.Fatalf("expected attached-late expansion text, got:\n%s", joined)
	}
}

func TestTimelineTab_DependencyTreeOrderAndColoredLegend(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"postgres": {},
			"api":      {DependsOn: compose.DependsOn{"postgres": {Condition: "service_healthy"}}},
			"worker":   {DependsOn: compose.DependsOn{"api": {Condition: "service_started"}}},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	snap := &discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	}
	rows := BuildRows(snap)
	projIdx := findRowByKey(rows, "project:app")
	proj := rows[projIdx]

	start := time.Unix(1_000, 0)
	tr := timeline.New(start)
	// Give every service a container so Display yields starting/healthy/blocked
	// rather than pending-only — enough for Spans to exist for all three.
	graph.ByName["postgres"].ContainerID = "p1"
	graph.ByName["postgres"].State = docker.StateHealthy
	graph.ByName["api"].ContainerID = "a1"
	graph.ByName["api"].State = docker.StateStarting
	graph.ByName["worker"].State = docker.StatePending
	tr.Observe(snap, start)
	tr.Observe(snap, start.Add(4*time.Second))

	m := Model{
		rows:               rows,
		cursor:             projIdx,
		selectionIsProject: true,
		mainTab:            tabTimeline,
		timeline:           tr,
	}
	out := renderTimelineTab(m, proj, 100)
	plain := stripANSI(out)

	pi := strings.Index(plain, "postgres")
	ai := strings.Index(plain, "api")
	wi := strings.Index(plain, "worker")
	if pi < 0 || ai < 0 || wi < 0 {
		t.Fatalf("missing services in timeline:\n%s", plain)
	}
	if pi >= ai || ai >= wi {
		t.Fatalf("expected dependency tree order postgres→api→worker (not alpha), got indexes p=%d a=%d w=%d:\n%s", pi, ai, wi, plain)
	}
	if !strings.Contains(plain, "└─") && !strings.Contains(plain, "├─") {
		t.Fatalf("expected pstree branch glyphs on timeline rows, got:\n%s", plain)
	}
	// Legend uses background swatches (48;…) so chips share the text baseline.
	if !strings.Contains(out, "48;") {
		t.Fatalf("expected background-colored legend swatches, got:\n%s", out)
	}
	if !strings.Contains(plain, "blocked") || !strings.Contains(plain, "pending") ||
		!strings.Contains(plain, "starting") || !strings.Contains(plain, "healthy") ||
		!strings.Contains(plain, "failed") {
		t.Fatalf("expected colored legend labels, got:\n%s", plain)
	}
	if strings.Contains(plain, "healthy/failed") {
		t.Fatalf("legend should list healthy and failed separately, got:\n%s", plain)
	}
}

func TestGraphTab_ShowsTreeAndEdgeConditions(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"postgres": {},
			"api":      {DependsOn: compose.DependsOn{"postgres": {Condition: "service_healthy"}}},
			"worker":   {DependsOn: compose.DependsOn{"api": {Condition: "service_started"}}},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["postgres"].ContainerID = "p1"
	graph.ByName["postgres"].State = docker.StateUnhealthy
	rows := BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "app", Graph: graph}},
	})
	projIdx := findRowByKey(rows, "project:app")
	m := Model{rows: rows, cursor: projIdx, selectionIsProject: true, mainTab: tabGraph}
	out := stripANSI(renderGraphTab(m, rows[projIdx], 80))
	if !strings.Contains(out, "└─") && !strings.Contains(out, "├─") {
		t.Fatalf("expected pstree branch glyphs, got:\n%s", out)
	}
	if !strings.Contains(out, "postgres") || !strings.Contains(out, "api") {
		t.Fatalf("expected services, got:\n%s", out)
	}
	if !strings.Contains(out, "healthy") {
		t.Fatalf("expected edge condition text, got:\n%s", out)
	}
}

func TestSparkline_ScalesAndEmpty(t *testing.T) {
	if got := sparkline(nil, 100); got != "" {
		t.Fatalf("nil sparkline = %q, want empty", got)
	}
	vals := []float64{0, 25, 50, 75, 100}
	out := stripANSI(sparkline(vals, 100))
	if !strings.ContainsAny(out, "▁▂▃▄▅▆▇█") {
		t.Fatalf("expected sparkline glyphs, got %q", out)
	}
	if len([]rune(out)) != len(vals) {
		t.Fatalf("sparkline len = %d, want %d (%q)", len([]rune(out)), len(vals), out)
	}
}

func TestListStatsColumns_DashWhenMissing(t *testing.T) {
	m := Model{stats: map[string]*ringBuffer{}}
	row := Row{Kind: RowComposeNode, ContainerID: "c1", Node: &dag.Node{Name: "api", State: docker.StateHealthy, ContainerID: "c1"}}
	cpu, mem := listStatsColumns(m, row)
	if cpu != "--" || mem != "--" {
		t.Fatalf("expected dashes, got %q %q", cpu, mem)
	}
}

func TestKeyNav_TabFocusAndDoctorJump(t *testing.T) {
	m := actionTestModel(t)
	updated, _ := m.Update(keyMsg("tab"))
	m = updated.(Model)
	if m.panelFocus != focusMain {
		t.Fatalf("after tab focus = %v, want focusMain", m.panelFocus)
	}

	updated, _ = m.Update(keyMsg("d"))
	m = updated.(Model)
	if !m.selectionIsProject {
		t.Fatal("d should select project row")
	}
	if m.mainTab != tabDoctor {
		t.Fatalf("mainTab = %d, want doctor", m.mainTab)
	}

	updated, _ = m.Update(keyMsg("t"))
	m = updated.(Model)
	if m.mainTab != tabTimeline {
		t.Fatalf("mainTab = %d, want timeline", m.mainTab)
	}
}

func TestKeyNav_EnterZoomsAndEscRestores(t *testing.T) {
	m := actionTestModel(t)
	updated, _ := m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.viewMode != viewZoom {
		t.Fatalf("viewMode = %v, want viewZoom", m.viewMode)
	}
	updated, _ = m.Update(keyMsg("esc"))
	m = updated.(Model)
	if m.viewMode != viewDashboard {
		t.Fatalf("viewMode after esc = %v, want viewDashboard", m.viewMode)
	}
}

func TestKeyNav_XOpensMenu(t *testing.T) {
	m := actionTestModel(t)
	updated, _ := m.Update(keyMsg("x"))
	m = updated.(Model)
	if m.actionMode != actionModeMenu {
		t.Fatalf("actionMode = %v, want menu", m.actionMode)
	}
	if len(m.actionItems) == 0 {
		t.Fatal("expected menu items")
	}
}

func TestRingBuffer_Wrap(t *testing.T) {
	r := newRingBuffer()
	// Shrink for a fast wrap test by pushing past capacity manually via a tiny buffer.
	r.samples = make([]*docker.StatsInfo, 3)
	r.push(&docker.StatsInfo{CPUPercent: 1})
	r.push(&docker.StatsInfo{CPUPercent: 2})
	r.push(&docker.StatsInfo{CPUPercent: 3})
	r.push(&docker.StatsInfo{CPUPercent: 4})
	vals := r.ordered()
	if len(vals) != 3 || vals[0].CPUPercent != 2 || vals[2].CPUPercent != 4 {
		t.Fatalf("ordered = %v, want CPU [2 3 4]", vals)
	}
}
