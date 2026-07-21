package engine

import (
	"github.com/hyturing/compose-pulse/internal/graph/causal"
	"github.com/hyturing/compose-pulse/internal/model"
)

// Diagnose runs rules with causal priority applied and annotates the
// first-failure finding with BlockedServices when available.
func Diagnose(run *model.Run, rules []Rule) []model.Finding {
	ctx := NewRunContext(run)
	if res := causal.Analyze(run); res != nil {
		ctx.SetCausalPriority(res.Priority)
		findings := Evaluate(ctx, rules)
		for i := range findings {
			if findings[i].Service == res.FirstFailure && len(findings[i].BlockedServices) == 0 {
				findings[i].BlockedServices = append([]string(nil), res.BlockedServices...)
			}
		}
		return findings
	}
	return Evaluate(ctx, rules)
}
