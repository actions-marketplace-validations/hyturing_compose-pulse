package rules

import (
	"github.com/hyturing/compose-pulse/internal/diagnosis/confidence"
	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/evidence"
	"github.com/hyturing/compose-pulse/internal/model"
)

func exitCode(ev model.Event) (int, bool) {
	if ev.Data == nil {
		return 0, false
	}
	switch n := ev.Data["exit_code"].(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}

func boolData(ev model.Event, key string) bool {
	if ev.Data == nil {
		return false
	}
	v, ok := ev.Data[key].(bool)
	return ok && v
}

func serviceExitCode(ctx *engine.RunContext, name string) (int, bool) {
	svc := ctx.Service(name)
	if svc != nil && svc.ExitCode != nil {
		return *svc.ExitCode, true
	}
	for _, ev := range ctx.EventsForService(name) {
		if code, ok := exitCode(ev); ok {
			return code, true
		}
	}
	return 0, false
}

type exit126Rule struct{}

func (exit126Rule) ID() string          { return "process.exit_126" }
func (exit126Rule) Description() string { return "Process exited 126 (found but not executable)" }

func (exit126Rule) Evaluate(ctx *engine.RunContext) []model.Finding {
	var out []model.Finding
	seen := map[string]bool{}
	for _, svc := range ctx.Services() {
		code, ok := serviceExitCode(ctx, svc.Name)
		if !ok || code != 126 || seen[svc.Name] {
			continue
		}
		seen[svc.Name] = true
		logLine := firstInterestingLog(ctx, svc.Name, "permission denied", "cannot execute", "not executable")
		evs := []string{evidence.KV("exit_code", 126)}
		if logLine != "" {
			evs = append(evs, evidence.Line("log", logLine))
		}
		out = append(out, model.Finding{
			RuleID:     "process.exit_126",
			Service:    svc.Name,
			RootCause:  "command found but cannot execute (exit 126)",
			Evidence:   evs,
			Confidence: confidence.High,
			SuggestedFixes: []string{
				"Ensure the entrypoint/command is executable (chmod +x) and uses a valid interpreter shebang",
			},
		})
	}
	return out
}

type exit127Rule struct{}

func (exit127Rule) ID() string          { return "process.exit_127" }
func (exit127Rule) Description() string { return "Process exited 127 (command not found)" }

func (exit127Rule) Evaluate(ctx *engine.RunContext) []model.Finding {
	var out []model.Finding
	seen := map[string]bool{}
	for _, svc := range ctx.Services() {
		code, ok := serviceExitCode(ctx, svc.Name)
		if !ok || code != 127 || seen[svc.Name] {
			continue
		}
		seen[svc.Name] = true
		logLine := firstInterestingLog(ctx, svc.Name, "not found", "no such file", "executable file not found")
		evs := []string{evidence.KV("exit_code", 127)}
		if logLine != "" {
			evs = append(evs, evidence.Line("log", logLine))
		}
		out = append(out, model.Finding{
			RuleID:     "process.exit_127",
			Service:    svc.Name,
			RootCause:  "command not found (exit 127)",
			Evidence:   evs,
			Confidence: confidence.High,
			SuggestedFixes: []string{
				"Fix the service command/entrypoint path, or install the missing binary in the image",
			},
		})
	}
	return out
}

type oomKilledRule struct{}

func (oomKilledRule) ID() string          { return "process.oom_killed" }
func (oomKilledRule) Description() string { return "Container was OOM-killed" }

func (oomKilledRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	var out []model.Finding
	seen := map[string]bool{}
	for _, ev := range ctx.Events() {
		svc := ev.Service
		if svc == "" || seen[svc] {
			continue
		}
		oom := boolData(ev, "oom_killed") || containsAny(ev.Message, "oomkilled", "oom killed") ||
			containsAny(dataString(ev, "status"), "oomkilled", "oom")
		code, hasCode := exitCode(ev)
		if !oom {
			// exit 137 alone is not enough (could be SIGKILL); require OOM signal
			continue
		}
		seen[svc] = true
		evs := []string{evidence.Line("event", ev.Message)}
		if hasCode {
			evs = append(evs, evidence.KV("exit_code", code))
		}
		evs = append(evs, evidence.KV("oom_killed", true))
		out = append(out, model.Finding{
			RuleID:     "process.oom_killed",
			Service:    svc,
			RootCause:  "container was OOM-killed",
			Evidence:   evs,
			Confidence: confidence.High,
			SuggestedFixes: []string{
				"Raise the service memory limit, or reduce the process memory footprint",
			},
		})
	}
	return out
}

func firstInterestingLog(ctx *engine.RunContext, service string, needles ...string) string {
	for _, ev := range ctx.EventsForService(service) {
		if ev.Type != model.EventTypeLog && ev.Source != model.SourceLog {
			continue
		}
		if containsAny(ev.Message, needles...) {
			return ev.Message
		}
	}
	return ""
}
