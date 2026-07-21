package engine

import (
	"sort"

	"github.com/hyturing/compose-pulse/internal/model"
)

// Evaluate runs all rules over ctx, dedupes, and returns stably ordered findings.
// Sort order: causal priority (lower first) → confidence (High→Possible) → RuleID → Service.
func Evaluate(ctx *RunContext, rules []Rule) []model.Finding {
	if ctx == nil {
		ctx = NewRunContext(nil)
	}

	var findings []model.Finding
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		for _, f := range rule.Evaluate(ctx) {
			if f.RuleID == "" {
				f.RuleID = rule.ID()
			}
			findings = append(findings, f)
		}
	}

	findings = dedupe(findings)
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		ra, rb := ctx.causalRank(a.Service), ctx.causalRank(b.Service)
		if ra != rb {
			return ra < rb
		}
		if a.Confidence != b.Confidence {
			return a.Confidence > b.Confidence // High > Medium > Possible
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.Service < b.Service
	})
	return findings
}

func dedupe(in []model.Finding) []model.Finding {
	type key struct{ rule, service string }
	best := make(map[key]model.Finding, len(in))
	order := make([]key, 0, len(in))
	for _, f := range in {
		k := key{f.RuleID, f.Service}
		prev, ok := best[k]
		if !ok {
			best[k] = f
			order = append(order, k)
			continue
		}
		if f.Confidence > prev.Confidence {
			best[k] = f
		}
	}
	out := make([]model.Finding, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out
}
