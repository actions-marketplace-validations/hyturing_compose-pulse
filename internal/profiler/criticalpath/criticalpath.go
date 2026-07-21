package criticalpath

import (
	"sort"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/profiler"
)

// Segment is one contribution on the critical path.
type Segment struct {
	Service  string
	Phase    model.ServicePhase
	Duration time.Duration
}

// Path is the longest dependency-aware chain of blocking work.
type Path struct {
	Segments []Segment
	Total    time.Duration
}

// Compute returns the critical path for a run using depends_on edges and phase timings.
// For each service, its "cost" is total phase time; the path is the heaviest root→leaf chain.
func Compute(run *model.Run) *Path {
	if run == nil {
		return nil
	}
	timings := profiler.CaptureTimings(run)
	cost := map[string]time.Duration{}
	phaseCost := map[string][]profiler.PhaseTiming{}
	for _, st := range timings {
		cost[st.Service] = st.Total
		phaseCost[st.Service] = st.Phases
	}
	deps := map[string][]string{}
	services := map[string]bool{}
	if run.EffectiveConfig != nil {
		for _, svc := range run.EffectiveConfig.Services {
			services[svc.Name] = true
			for dep := range svc.DependsOn {
				deps[svc.Name] = append(deps[svc.Name], dep)
				services[dep] = true
			}
		}
	}
	for name := range cost {
		services[name] = true
	}
	if len(services) == 0 {
		return nil
	}

	memo := map[string]time.Duration{}
	pred := map[string]string{}
	var best func(string) time.Duration
	best = func(svc string) time.Duration {
		if v, ok := memo[svc]; ok {
			return v
		}
		base := cost[svc]
		var maxDep time.Duration
		var maxDepName string
		for _, dep := range deps[svc] {
			d := best(dep)
			if d >= maxDep {
				maxDep = d
				maxDepName = dep
			}
		}
		if maxDepName != "" {
			pred[svc] = maxDepName
		}
		memo[svc] = base + maxDep
		return memo[svc]
	}

	var leaf string
	var leafCost time.Duration
	names := make([]string, 0, len(services))
	for n := range services {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := best(n)
		if c > leafCost || (c == leafCost && (leaf == "" || n < leaf)) {
			leafCost = c
			leaf = n
		}
	}

	// Reconstruct chain root → leaf
	var chain []string
	for cur := leaf; cur != ""; cur = pred[cur] {
		chain = append(chain, cur)
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	p := &Path{}
	for _, svc := range chain {
		phases := phaseCost[svc]
		if len(phases) == 0 {
			p.Segments = append(p.Segments, Segment{Service: svc, Duration: cost[svc]})
			p.Total += cost[svc]
			continue
		}
		// pick longest phase for this service as the segment label
		longest := phases[0]
		for _, ph := range phases[1:] {
			if ph.Duration > longest.Duration {
				longest = ph
			}
		}
		p.Segments = append(p.Segments, Segment{
			Service:  svc,
			Phase:    longest.Phase,
			Duration: cost[svc],
		})
		p.Total += cost[svc]
	}
	return p
}
