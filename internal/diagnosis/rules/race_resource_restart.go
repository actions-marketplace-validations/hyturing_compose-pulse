package rules

import (
	"strings"

	"github.com/hyturing/compose-pulse/internal/diagnosis/confidence"
	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/evidence"
	"github.com/hyturing/compose-pulse/internal/model"
)

type dependsOnStartedRaceRule struct{}

func (dependsOnStartedRaceRule) ID() string { return "race.depends_on_started" }
func (dependsOnStartedRaceRule) Description() string {
	return "Dependent connected before dependency was ready (depends_on: service_started)"
}

func (dependsOnStartedRaceRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	cfg := ctx.EffectiveConfig()
	if cfg == nil {
		return nil
	}
	var out []model.Finding
	for _, svc := range cfg.Services {
		for dep, cond := range svc.DependsOn {
			if !strings.EqualFold(cond, "service_started") {
				continue
			}
			if !hasConnectionRefusedTo(ctx, svc.Name, dep) {
				continue
			}
			depSvc := ctx.Service(dep)
			// Strong signal: dependency later becomes healthy, or never left started while dependent failed.
			if depSvc == nil {
				continue
			}
			dependentFailed := false
			if s := ctx.Service(svc.Name); s != nil && (s.Phase == model.PhaseFailed || s.Phase == model.PhaseExited) {
				dependentFailed = true
			}
			if !dependentFailed && !hasServiceFailureEvent(ctx, svc.Name) {
				continue
			}
			out = append(out, model.Finding{
				RuleID:    "race.depends_on_started",
				Service:   svc.Name,
				RootCause: svc.Name + " connected before " + dep + " was ready",
				Evidence: []string{
					evidence.KV("depends_on", dep+"=service_started"),
					evidence.Line("log", firstConnectionRefused(ctx, svc.Name)),
					evidence.KV("dependency_phase", depSvc.Phase.String()),
				},
				Confidence: confidence.High,
				SuggestedFixes: []string{
					"Change depends_on condition to service_healthy (and add a healthcheck), or add application-level retry/backoff",
				},
			})
		}
	}
	return out
}

type resourceOOMRule struct{}

func (resourceOOMRule) ID() string          { return "resource.oom" }
func (resourceOOMRule) Description() string { return "Service hit its memory limit / cgroup OOM" }

func (resourceOOMRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		if dataString(ev, "error_kind") == "memory_limit" {
			return true
		}
		return ev.Source == model.SourceResource && containsAny(ev.Message, "memory limit", "out of memory", "cgroup")
	})
	if len(evs) == 0 {
		return nil
	}
	ev := evs[0]
	return []model.Finding{{
		RuleID:     "resource.oom",
		Service:    ev.Service,
		RootCause:  "service reached its memory limit",
		Evidence:   []string{evidence.Line("resource", ev.Message)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"Increase deploy/resources memory limits for the service, or reduce memory usage",
		},
	}}
}

type identicalExitLoopRule struct{}

func (identicalExitLoopRule) ID() string { return "restart.identical_exit_loop" }
func (identicalExitLoopRule) Description() string {
	return "Service is restart-looping with the same exit code"
}

func (identicalExitLoopRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	type sig struct {
		code  int
		count int
	}
	bySvc := map[string]*sig{}
	restarts := map[string]int{}
	for _, ev := range ctx.Events() {
		if ev.Service == "" {
			continue
		}
		if containsAny(ev.Message, "restart") || dataString(ev, "status") == "restart" {
			restarts[ev.Service]++
		}
		if code, ok := exitCode(ev); ok {
			s := bySvc[ev.Service]
			if s == nil {
				bySvc[ev.Service] = &sig{code: code, count: 1}
				continue
			}
			if s.code == code {
				s.count++
			}
		}
	}
	var out []model.Finding
	for svc, s := range bySvc {
		if s.count < 3 || restarts[svc] < 2 {
			continue
		}
		logLine := firstInterestingLog(ctx, svc, "fatal", "error", "panic")
		evs := []string{evidence.KV("exit_code", s.code), evidence.KV("identical_exits", s.count), evidence.KV("restarts", restarts[svc])}
		if logLine != "" {
			evs = append(evs, evidence.Line("log", logLine))
		}
		out = append(out, model.Finding{
			RuleID:     "restart.identical_exit_loop",
			Service:    svc,
			RootCause:  "service restart loop with identical exit signature",
			Evidence:   evs,
			Confidence: confidence.High,
			SuggestedFixes: []string{
				"Fix the recurring application error causing the exit; avoid relying on restart policy to hide the first failure",
			},
		})
	}
	return out
}

func hasConnectionRefusedTo(ctx *engine.RunContext, service, dep string) bool {
	for _, ev := range ctx.EventsForService(service) {
		if !containsAny(ev.Message, "connection refused", "connect: connection refused") {
			continue
		}
		if containsAny(ev.Message, dep) || containsAny(ev.Message, "5432") {
			return true
		}
	}
	return false
}

func firstConnectionRefused(ctx *engine.RunContext, service string) string {
	for _, ev := range ctx.EventsForService(service) {
		if containsAny(ev.Message, "connection refused") {
			return ev.Message
		}
	}
	return ""
}

func hasServiceFailureEvent(ctx *engine.RunContext, service string) bool {
	for _, ev := range ctx.EventsForService(service) {
		if ev.Phase == model.PhaseFailed || ev.Severity == model.SeverityError {
			return true
		}
	}
	return false
}
