package actions

import (
	"context"
	"os/exec"
	"strings"
)

// Event reports output or completion from a running action plan.
type Event struct {
	Step int
	Line string
	Done bool
	Err  error
}

// Start runs every step in a plan and emits output/final status events.
func Start(ctx context.Context, plan Plan) <-chan Event {
	ch := make(chan Event)
	go func() {
		defer close(ch)
		for i, step := range plan.Steps {
			ch <- Event{Step: i, Line: "$ " + commandString(step.Command)}
			cmd := exec.CommandContext(ctx, step.Command.Program, step.Command.Args...)
			out, err := cmd.CombinedOutput()
			for _, line := range splitLines(string(out)) {
				ch <- Event{Step: i, Line: line}
			}
			if err != nil {
				ch <- Event{Step: i, Err: err, Done: true}
				return
			}
		}
		ch <- Event{Done: true}
	}()
	return ch
}

func commandString(cmd Command) string {
	parts := append([]string{cmd.Program}, cmd.Args...)
	return strings.Join(parts, " ")
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
