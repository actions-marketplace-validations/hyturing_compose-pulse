package rules

import (
	"strings"

	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/model"
)

func eventsMatching(ctx *engine.RunContext, pred func(model.Event) bool) []model.Event {
	if ctx == nil {
		return nil
	}
	var out []model.Event
	for _, ev := range ctx.Events() {
		if pred(ev) {
			out = append(out, ev)
		}
	}
	return out
}

func containsAny(s string, needles ...string) bool {
	low := strings.ToLower(s)
	for _, n := range needles {
		if strings.Contains(low, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

func dataString(ev model.Event, key string) string {
	if ev.Data == nil {
		return ""
	}
	v, ok := ev.Data[key].(string)
	if !ok {
		return ""
	}
	return v
}
