package causal

import (
	"sort"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
)

// Result is the first causal failure and downstream blocked services.
type Result struct {
	FirstFailure    string
	BlockedServices []string
	// Priority maps service name → rank (0 = first causal failure).
	Priority map[string]int
	Chain    []string
}

// Analyze selects the earliest root failure in the dependency graph and
// computes services blocked behind it. Returns nil when nothing failed.
func Analyze(run *model.Run) *Result {
	if run == nil {
		return nil
	}
	deps := dependsOn(run)
	failedAt := failedServices(run)
	if len(failedAt) == 0 {
		return nil
	}

	roots := make([]string, 0, len(failedAt))
	for svc := range failedAt {
		hasFailedDep := false
		for dep := range deps[svc] {
			if _, ok := failedAt[dep]; ok {
				hasFailedDep = true
				break
			}
		}
		if !hasFailedDep {
			roots = append(roots, svc)
		}
	}
	if len(roots) == 0 {
		for svc := range failedAt {
			roots = append(roots, svc)
		}
	}
	sort.SliceStable(roots, func(i, j int) bool {
		ti, tj := failedAt[roots[i]], failedAt[roots[j]]
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return roots[i] < roots[j]
	})
	first := roots[0]

	blockedSet := map[string]bool{}
	var visit func(string)
	visit = func(svc string) {
		for _, child := range dependentsOf(deps, svc) {
			if child == first || blockedSet[child] {
				continue
			}
			blockedSet[child] = true
			visit(child)
		}
	}
	visit(first)

	blocked := make([]string, 0, len(blockedSet))
	for svc := range blockedSet {
		blocked = append(blocked, svc)
	}
	sort.Strings(blocked)

	priority := map[string]int{first: 0}
	for i, svc := range blocked {
		priority[svc] = i + 1
	}

	return &Result{
		FirstFailure:    first,
		BlockedServices: blocked,
		Priority:        priority,
		Chain:           append([]string{first}, blocked...),
	}
}

func dependsOn(run *model.Run) map[string]map[string]string {
	out := map[string]map[string]string{}
	if run.EffectiveConfig == nil {
		return out
	}
	for _, svc := range run.EffectiveConfig.Services {
		if len(svc.DependsOn) == 0 {
			continue
		}
		m := make(map[string]string, len(svc.DependsOn))
		for dep, cond := range svc.DependsOn {
			m[dep] = cond
		}
		out[svc.Name] = m
	}
	return out
}

func dependentsOf(deps map[string]map[string]string, target string) []string {
	var out []string
	for svc, d := range deps {
		if _, ok := d[target]; ok {
			out = append(out, svc)
		}
	}
	sort.Strings(out)
	return out
}

func failedServices(run *model.Run) map[string]time.Time {
	out := map[string]time.Time{}
	for _, svc := range run.ServiceList() {
		if svc.Phase == model.PhaseFailed || svc.Phase == model.PhaseExited {
			out[svc.Name] = svc.UpdatedAt
		}
	}
	for _, ev := range run.Events {
		if ev.Service == "" {
			continue
		}
		if ev.Phase != model.PhaseFailed && ev.Severity != model.SeverityError {
			continue
		}
		cur, ok := out[ev.Service]
		if !ok || ev.Timestamp.Before(cur) {
			out[ev.Service] = ev.Timestamp
		}
	}
	return out
}
