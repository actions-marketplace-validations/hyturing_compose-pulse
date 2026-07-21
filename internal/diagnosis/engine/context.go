package engine

import "github.com/hyturing/compose-pulse/internal/model"

// RunContext is a read-only view over a recorded model.Run for diagnosis rules.
type RunContext struct {
	run            *model.Run
	causalPriority map[string]int // lower = earlier causal failure; missing = last
}

// NewRunContext wraps run for rule evaluation. run may be nil (empty context).
func NewRunContext(run *model.Run) *RunContext {
	return &RunContext{run: run}
}

// SetCausalPriority sets per-service causal ranks used when sorting findings.
// Lower values surface first. Services absent from the map sort after ranked ones.
func (c *RunContext) SetCausalPriority(priority map[string]int) {
	if c == nil {
		return
	}
	c.causalPriority = priority
}

// Run returns the underlying run (may be nil).
func (c *RunContext) Run() *model.Run {
	if c == nil {
		return nil
	}
	return c.run
}

// Services returns services sorted stably (project, name).
func (c *RunContext) Services() []*model.Service {
	if c == nil || c.run == nil {
		return nil
	}
	return c.run.ServiceList()
}

// Service returns the first service with the given name, or nil.
func (c *RunContext) Service(name string) *model.Service {
	if c == nil || c.run == nil || name == "" {
		return nil
	}
	for _, svc := range c.run.ServiceList() {
		if svc.Name == name {
			return svc
		}
	}
	return nil
}

// Events returns all events on the run.
func (c *RunContext) Events() []model.Event {
	if c == nil || c.run == nil {
		return nil
	}
	return c.run.Events
}

// EventsForService returns events whose Service field matches name.
func (c *RunContext) EventsForService(name string) []model.Event {
	if c == nil || c.run == nil || name == "" {
		return nil
	}
	var out []model.Event
	for _, ev := range c.run.Events {
		if ev.Service == name {
			out = append(out, ev)
		}
	}
	return out
}

// EffectiveConfig returns the run's effective config, if any.
func (c *RunContext) EffectiveConfig() *model.EffectiveConfig {
	if c == nil || c.run == nil {
		return nil
	}
	return c.run.EffectiveConfig
}

// Invocation returns the run's invocation context, if any.
func (c *RunContext) Invocation() *model.Invocation {
	if c == nil || c.run == nil {
		return nil
	}
	return c.run.Invocation
}

func (c *RunContext) causalRank(service string) int {
	if c == nil || c.causalPriority == nil {
		return 1 << 30
	}
	if rank, ok := c.causalPriority[service]; ok {
		return rank
	}
	return 1 << 30
}
