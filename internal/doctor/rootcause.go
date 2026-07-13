package doctor

import (
	"sort"

	"github.com/hyturing/compose-pulse/internal/dag"
)

// RootCause summarizes the likely service(s) causing startup blockage.
type RootCause struct {
	Culprits     []string
	Chains       map[string][]string // blocked -> path to culprit
	CriticalPath []string
	FirstLog     map[string]string
}

// FindRootCause traces blocked services back to failed, unhealthy, or
// restart-looping dependency endpoints. It returns nil when the graph has no
// blocked or broken services to explain.
func FindRootCause(ctx Context) *RootCause {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}

	displays := make(map[string]dag.DisplayState, len(g.Ordered))
	waiting := make(map[string][]string, len(g.Ordered))
	broken := make(map[string]bool, len(g.Ordered))
	var brokenOrdered []string
	var blockedOrdered []string

	for _, n := range g.Ordered {
		display, waitingOn := dag.Display(n, g)
		displays[n.Name] = display
		waiting[n.Name] = waitingOn
		if display == dag.DisplayFailed || display == dag.DisplayUnhealthy || display == dag.DisplayMissing || restartLooping(ctx, n.Name) {
			broken[n.Name] = true
			brokenOrdered = append(brokenOrdered, n.Name)
		}
	}
	for _, n := range g.Ordered {
		if displays[n.Name] != dag.DisplayBlocked {
			for _, depName := range n.Deps {
				if displays[depName] == dag.DisplayBlocked {
					displays[n.Name] = dag.DisplayBlocked
					waiting[n.Name] = append(waiting[n.Name], depName)
				}
			}
		}
		if displays[n.Name] == dag.DisplayBlocked {
			blockedOrdered = append(blockedOrdered, n.Name)
		}
	}

	if len(blockedOrdered) == 0 && len(brokenOrdered) == 0 {
		return nil
	}

	rc := &RootCause{
		Chains:   map[string][]string{},
		FirstLog: map[string]string{},
	}

	culpritSeen := map[string]struct{}{}
	addCulprit := func(name string) {
		if _, ok := culpritSeen[name]; ok {
			return
		}
		culpritSeen[name] = struct{}{}
		rc.Culprits = append(rc.Culprits, name)
	}
	for _, name := range brokenOrdered {
		addCulprit(name)
	}

	var longest []string
	for _, blockedName := range blockedOrdered {
		chain := findBrokenChain(blockedName, waiting, broken)
		if len(chain) == 0 {
			continue
		}
		rc.Chains[blockedName] = chain
		endpoint := chain[len(chain)-1]
		addCulprit(endpoint)
		if len(chain) > len(longest) {
			longest = chain
		}
	}

	if len(longest) > 0 {
		rc.CriticalPath = reverseStrings(longest)
	} else if len(rc.Culprits) > 0 {
		rc.CriticalPath = []string{rc.Culprits[0]}
	}

	for _, culprit := range rc.Culprits {
		if line := firstLogLine(ctx, culprit, displays[culprit]); line != "" {
			rc.FirstLog[culprit] = line
		}
	}

	if len(rc.Culprits) == 0 && len(rc.Chains) == 0 {
		return nil
	}
	return rc
}

func restartLooping(ctx Context, service string) bool {
	g := graphFrom(ctx)
	if g == nil {
		return false
	}
	n := g.ByName[service]
	info := inspectNode(ctx, n)
	return info != nil && info.RestartCount >= 3
}

func findBrokenChain(blocked string, waiting map[string][]string, broken map[string]bool) []string {
	var best []string
	seen := map[string]bool{}
	var dfs func(name string, path []string)
	dfs = func(name string, path []string) {
		if seen[name] {
			return
		}
		seen[name] = true
		path = append(path, name)
		defer func() { seen[name] = false }()

		if broken[name] {
			if len(path) > len(best) || (len(path) == len(best) && chainLess(path, best)) {
				best = append([]string(nil), path...)
			}
			return
		}

		deps := append([]string(nil), waiting[name]...)
		sort.Strings(deps)
		for _, dep := range deps {
			dfs(dep, path)
		}
	}
	dfs(blocked, nil)
	return best
}

func chainLess(a, b []string) bool {
	if len(b) == 0 {
		return true
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func reverseStrings(in []string) []string {
	out := make([]string, len(in))
	for i := range in {
		out[len(in)-1-i] = in[i]
	}
	return out
}

func firstLogLine(ctx Context, service string, display dag.DisplayState) string {
	g := graphFrom(ctx)
	if g == nil {
		return ""
	}
	n := g.ByName[service]
	if n == nil {
		return ""
	}
	if ctx.Logs != nil && n.ContainerID != "" {
		lines, err := ctx.Logs(n.ContainerID, 200)
		if err == nil {
			interesting := InterestingLogLines(lines, 1)
			if len(interesting) > 0 {
				return interesting[0]
			}
		}
	}
	if display == dag.DisplayUnhealthy {
		info := inspectNode(ctx, n)
		if info != nil && info.Health != nil && len(info.Health.Log) > 0 {
			return info.Health.Log[len(info.Health.Log)-1].Output
		}
	}
	return ""
}
