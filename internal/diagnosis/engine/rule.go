package engine

import "github.com/hyturing/compose-pulse/internal/model"

// Rule evaluates a recorded run and returns zero or more findings.
// Rules never touch Docker; they reason only over RunContext.
type Rule interface {
	ID() string
	Description() string
	Evaluate(ctx *RunContext) []model.Finding
}
