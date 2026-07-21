package rules

import (
	"regexp"

	"github.com/hyturing/compose-pulse/internal/diagnosis/confidence"
	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/evidence"
	"github.com/hyturing/compose-pulse/internal/model"
)

var hostPortRe = regexp.MustCompile(`(?i)(?:bind for [^:]+:|0\.0\.0\.0:|:::|port )(\d+)`)

type hostPortOccupiedRule struct{}

func (hostPortOccupiedRule) ID() string { return "port.host_occupied" }
func (hostPortOccupiedRule) Description() string {
	return "Host port publish failed because the port is already allocated"
}

func (hostPortOccupiedRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		if dataString(ev, "error_kind") == "port_conflict" {
			return true
		}
		if ev.Severity != model.SeverityError && ev.Phase != model.PhaseFailed {
			return false
		}
		return containsAny(ev.Message,
			"port is already allocated",
			"address already in use",
			"bind for",
			"failed to bind host port",
		)
	})
	if len(evs) == 0 {
		return nil
	}
	ev := evs[0]
	port := dataString(ev, "host_port")
	if port == "" {
		if m := hostPortRe.FindStringSubmatch(ev.Message); len(m) > 1 {
			port = m[1]
		}
	}
	root := "host port is already allocated"
	if port != "" {
		root = "host port " + port + " is already allocated"
	}
	return []model.Finding{{
		RuleID:     "port.host_occupied",
		Service:    ev.Service,
		RootCause:  root,
		Evidence:   []string{evidence.Line("compose_error", ev.Message), evidence.KV("host_port", port)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"Change the host port mapping for the failing service, or stop the process/container that owns the port",
		},
	}}
}

type containerNameConflictRule struct{}

func (containerNameConflictRule) ID() string { return "name.container_conflict" }
func (containerNameConflictRule) Description() string {
	return "Container name is already in use by another container"
}

func (containerNameConflictRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		if dataString(ev, "error_kind") == "container_name_conflict" {
			return true
		}
		return containsAny(ev.Message,
			"container name",
			"is already in use by container",
			"conflict. the container name",
		)
	})
	if len(evs) == 0 {
		return nil
	}
	ev := evs[0]
	name := dataString(ev, "container_name")
	root := "container name is already in use"
	if name != "" {
		root = "container name " + name + " is already in use"
	}
	return []model.Finding{{
		RuleID:     "name.container_conflict",
		Service:    ev.Service,
		RootCause:  root,
		Evidence:   []string{evidence.Line("compose_error", ev.Message)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"Remove or rename the existing container, or drop `container_name` so Compose can assign a project-scoped name",
		},
	}}
}
