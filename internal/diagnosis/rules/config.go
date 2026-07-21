package rules

import (
	"strings"

	"github.com/hyturing/compose-pulse/internal/diagnosis/confidence"
	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/evidence"
	"github.com/hyturing/compose-pulse/internal/model"
)

type missingEnvVarRule struct{}

func (missingEnvVarRule) ID() string { return "config.missing_env_var" }
func (missingEnvVarRule) Description() string {
	return "Compose failed because a required environment variable is unset"
}

func (missingEnvVarRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		if dataString(ev, "error_kind") == "missing_env_var" {
			return true
		}
		return containsAny(ev.Message,
			"required variable",
			"is missing a value",
			"must be set",
			"variable is not set",
			"unset variable",
		) && containsAny(ev.Message, "interpolat", "environment", "env file", "env_file", "${")
	})
	if len(evs) == 0 {
		// also accept clear REQUIRED_VAR-style messages without interpolation keyword
		evs = eventsMatching(ctx, func(ev model.Event) bool {
			return containsAny(ev.Message, "required variable") && containsAny(ev.Message, "missing")
		})
	}
	if len(evs) == 0 {
		return nil
	}
	ev := evs[0]
	variable := dataString(ev, "variable")
	if variable == "" {
		variable = extractEnvVarName(ev.Message)
	}
	root := "required environment variable is unset"
	if variable != "" {
		root = "required environment variable " + variable + " is unset"
	}
	return []model.Finding{{
		RuleID:     "config.missing_env_var",
		Service:    ev.Service,
		RootCause:  root,
		Evidence:   []string{evidence.Line("compose_error", ev.Message), evidence.KV("variable", variable)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"Export the missing variable before `docker compose up`, or add it to an env file referenced by the compose project",
		},
	}}
}

type invalidComposeRule struct{}

func (invalidComposeRule) ID() string { return "config.invalid_compose" }
func (invalidComposeRule) Description() string {
	return "Compose config is invalid (YAML/schema/unsupported field)"
}

func (invalidComposeRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		// Ignore cpulse-internal effective-config parse failures (not Compose CLI errors).
		if dataString(ev, "stage") == "compose_config" || containsAny(ev.Message, "parse merged config") {
			return false
		}
		if dataString(ev, "error_kind") == "invalid_compose" {
			return true
		}
		return containsAny(ev.Message,
			"additional properties",
			"yaml:",
			"line ",
			"did not find expected",
			"validating ",
			"unsupported",
			"unknown field",
			"mapping values are not allowed",
		) && !containsAny(ev.Message, "required variable", "port is already", "container name")
	})
	if len(evs) == 0 {
		return nil
	}
	ev := evs[0]
	return []model.Finding{{
		RuleID:     "config.invalid_compose",
		Service:    ev.Service,
		RootCause:  "compose configuration is invalid",
		Evidence:   []string{evidence.Line("compose_error", ev.Message)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"Fix the compose file schema/YAML error reported by `docker compose config`",
		},
	}}
}

func extractEnvVarName(msg string) string {
	// Prefer TOKEN in "required variable TOKEN"
	const marker = "required variable "
	low := strings.ToLower(msg)
	if i := strings.Index(low, marker); i >= 0 {
		rest := msg[i+len(marker):]
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			return strings.Trim(fields[0], ":'\"")
		}
	}
	return ""
}
