package rules

import (
	"github.com/hyturing/compose-pulse/internal/diagnosis/confidence"
	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/evidence"
	"github.com/hyturing/compose-pulse/internal/model"
)

type healthMissingExecutableRule struct{}

func (healthMissingExecutableRule) ID() string { return "health.missing_executable" }
func (healthMissingExecutableRule) Description() string {
	return "Healthcheck executable is missing inside the container"
}

func (healthMissingExecutableRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		if dataString(ev, "error_kind") == "healthcheck_missing_executable" {
			return true
		}
		if ev.Source != model.SourceHealthcheck && !containsAny(ev.Message, "healthcheck", "health check") {
			return false
		}
		code := 0
		if ev.Data != nil {
			switch n := ev.Data["healthcheck_exit_code"].(type) {
			case float64:
				code = int(n)
			case int:
				code = n
			}
		}
		return code == 127 || containsAny(ev.Message, "not found", "no such file", "executable file not found")
	})
	if len(evs) == 0 {
		return nil
	}
	ev := evs[0]
	return []model.Finding{{
		RuleID:     "health.missing_executable",
		Service:    ev.Service,
		RootCause:  "healthcheck executable is missing in the container",
		Evidence:   []string{evidence.Line("healthcheck", ev.Message), evidence.KV("healthcheck_exit_code", 127)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"Install the healthcheck binary in the image, or change the healthcheck to a command that exists (e.g. wget/busybox)",
		},
	}}
}

type localhostInContainerRule struct{}

func (localhostInContainerRule) ID() string { return "net.localhost_in_container" }
func (localhostInContainerRule) Description() string {
	return "Service tried to reach a dependency via localhost inside the container network"
}

func (localhostInContainerRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		return containsAny(ev.Message, "127.0.0.1", "localhost") &&
			containsAny(ev.Message, "connection refused", "connect: connection refused", "dial tcp")
	})
	if len(evs) == 0 {
		return nil
	}
	// Prefer cases where another compose service exists (dependency likely mis-addressed).
	if cfg := ctx.EffectiveConfig(); cfg != nil && len(cfg.Services) < 2 {
		return nil
	}
	ev := evs[0]
	return []model.Finding{{
		RuleID:     "net.localhost_in_container",
		Service:    ev.Service,
		RootCause:  "service used localhost/127.0.0.1 to reach another container",
		Evidence:   []string{evidence.Line("log", ev.Message)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"Use the Compose service hostname (e.g. db) instead of localhost inside the container network",
		},
	}}
}
