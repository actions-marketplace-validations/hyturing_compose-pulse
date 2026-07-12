package ui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

// Filtered failed/waiting views are where CPU/MEM misalignment shows most:
// short detail text + empty stats. Headers must share column starts with values.
func TestCPUMemColumns_AlignedInFailedFilter(t *testing.T) {
	cfg := &compose.Config{
		Services: map[string]compose.Service{
			"crasher":                             {},
			"dte-ray-deeptech-engineer-very-long": {},
		},
	}
	graph, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	graph.ByName["crasher"].ContainerID = "c1"
	graph.ByName["crasher"].State = docker.StateExited
	graph.ByName["crasher"].ExitCode = intPtr(1)
	graph.ByName["dte-ray-deeptech-engineer-very-long"].ContainerID = "d1"
	graph.ByName["dte-ray-deeptech-engineer-very-long"].State = docker.StateExited
	graph.ByName["dte-ray-deeptech-engineer-very-long"].ExitCode = intPtr(137)

	rows := filterRows(BuildRows(&discover.Snapshot{
		Projects: []discover.Project{{Name: "demo-broken", Graph: graph}},
	}), filterFailed)

	const panelW = 58
	cols := computeGraphColumns(rows, panelW)
	m := Model{}
	panel := renderPanel(
		formatGraphColumnHeader(cols, filterFailed),
		renderGraphContent(m, rows, 0, 0, panelW, 20),
		panelW+2, 12, true, false,
	)

	var title, body string
	for _, l := range strings.Split(panel, "\n") {
		plain := stripANSI(l)
		plain = strings.TrimPrefix(plain, "│")
		plain = strings.TrimSuffix(plain, "│")
		if title == "" && strings.Contains(plain, "CPU") && strings.Contains(plain, "MEM") {
			title = plain
			continue
		}
		if strings.Contains(plain, "crasher") && strings.Contains(plain, "exit") {
			body = plain
			break
		}
	}
	if title == "" || body == "" {
		t.Fatalf("missing title/body\n%s", stripANSI(panel))
	}

	cpuStart := cols.nameW + graphColGap + cols.stateW + graphColGap + cols.detailW + graphColGap
	memStart := cpuStart + graphCPUColWidth + 1

	// Headers and values are both right-aligned in their columns.
	titleCPU := indexVis(title, "CPU")
	titleMEM := indexVis(title, "MEM")
	wantCPULetters := cpuStart + graphCPUColWidth - len("CPU")
	wantMEMLetters := memStart + graphMEMColWidth - len("MEM")
	if titleCPU != wantCPULetters {
		t.Fatalf("CPU header at %d, want right-aligned %d\ntitle=%q", titleCPU, wantCPULetters, title)
	}
	if titleMEM != wantMEMLetters {
		t.Fatalf("MEM header at %d, want right-aligned %d\ntitle=%q", titleMEM, wantMEMLetters, title)
	}

	cpuCell := sliceVis(body, cpuStart, graphCPUColWidth)
	memCell := sliceVis(body, memStart, graphMEMColWidth)
	if strings.TrimSpace(cpuCell) != "--" {
		t.Fatalf("cpu cell %q, want --", cpuCell)
	}
	if strings.TrimSpace(memCell) != "--" {
		t.Fatalf("mem cell %q, want --", memCell)
	}
	if indexVis(body, "--") != cpuStart+graphCPUColWidth-2 {
		t.Fatalf("cpu -- at %d, want right-aligned %d\nbody=%q", indexVis(body, "--"), cpuStart+graphCPUColWidth-2, body)
	}
}

func sliceVis(s string, start, width int) string {
	var b strings.Builder
	col := 0
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if col >= start+width {
			break
		}
		if col >= start {
			b.WriteRune(r)
		}
		col += w
	}
	return b.String()
}

func indexVis(s, substr string) int {
	byteIdx := strings.Index(s, substr)
	if byteIdx < 0 {
		return -1
	}
	return runewidth.StringWidth(s[:byteIdx])
}
