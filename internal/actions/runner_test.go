package actions

import (
	"context"
	"strings"
	"testing"
)

func TestStartEmitsOutputAndDone(t *testing.T) {
	plan := Plan{
		Title: "test",
		Steps: []Step{{
			Label: "print",
			Command: Command{
				Program: "sh",
				Args:    []string{"-c", "printf 'hello\\n'"},
			},
		}},
	}

	var events []Event
	for event := range Start(context.Background(), plan) {
		events = append(events, event)
	}

	if !events[len(events)-1].Done {
		t.Fatalf("last event should be done, got %#v", events[len(events)-1])
	}
	if !containsEventLine(events, "hello") {
		t.Fatalf("expected output line, got %#v", events)
	}
}

func TestStartStopsAfterFailedStep(t *testing.T) {
	plan := Plan{
		Title: "test",
		Steps: []Step{
			{
				Label: "fail",
				Command: Command{
					Program: "sh",
					Args:    []string{"-c", "printf 'bad\\n'; exit 7"},
				},
			},
			{
				Label: "skip",
				Command: Command{
					Program: "sh",
					Args:    []string{"-c", "printf 'should-not-run\\n'"},
				},
			},
		},
	}

	var events []Event
	for event := range Start(context.Background(), plan) {
		events = append(events, event)
	}

	last := events[len(events)-1]
	if last.Err == nil {
		t.Fatalf("expected failure event, got %#v", last)
	}
	if containsEventLine(events, "should-not-run") {
		t.Fatalf("runner did not stop after failure: %#v", events)
	}
}

func containsEventLine(events []Event, want string) bool {
	for _, event := range events {
		if strings.Contains(event.Line, want) {
			return true
		}
	}
	return false
}
