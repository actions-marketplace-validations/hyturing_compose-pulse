package rules

import (
	"github.com/hyturing/compose-pulse/internal/diagnosis/confidence"
	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/evidence"
	"github.com/hyturing/compose-pulse/internal/model"
)

// probeTCPRefusedRule fires when a probe proves TCP failure while the target
// is not listening — High-confidence networking/startup evidence.
type probeTCPRefusedRule struct{}

func (probeTCPRefusedRule) ID() string { return "probe.tcp_refused_not_listening" }
func (probeTCPRefusedRule) Description() string {
	return "Probe proved TCP connection refused and target port not listening"
}

func (probeTCPRefusedRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	type key struct{ svc, host string }
	tcpFail := map[key]model.Event{}
	notListening := map[key]model.Event{}
	methods := map[key]string{}

	for _, ev := range ctx.Events() {
		if ev.Source != model.SourceProbe {
			continue
		}
		step := dataString(ev, "probe_step")
		status := dataString(ev, "probe_status")
		host := dataString(ev, "target_host")
		k := key{ev.Service, host}
		if m := dataString(ev, "probe_method"); m != "" {
			methods[k] = m
		}
		if step == "tcp" && status == "FAIL" {
			tcpFail[k] = ev
		}
		if step == "process_listening" && status == "FAIL" {
			notListening[k] = ev
		}
	}

	var out []model.Finding
	for k, tcpEv := range tcpFail {
		listenEv, ok := notListening[k]
		if !ok {
			continue
		}
		method := methods[k]
		out = append(out, model.Finding{
			RuleID:    "probe.tcp_refused_not_listening",
			Service:   k.svc,
			RootCause: k.svc + " could not reach " + k.host + " (TCP refused; port not listening)",
			Evidence: []string{
				evidence.Line("probe_tcp", tcpEv.Message),
				evidence.Line("probe_listening", listenEv.Message),
				evidence.KV("probe_method", method),
			},
			Confidence: confidence.High,
			SuggestedFixes: []string{
				"Wait for the dependency to accept connections (service_healthy / retries), or fix the process that should listen on the target port",
			},
		})
	}
	return out
}
