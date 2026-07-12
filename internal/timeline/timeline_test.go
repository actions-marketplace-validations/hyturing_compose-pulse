package timeline

import (
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestTrackerSpansTransitions(t *testing.T) {
	start := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	tracker := New(start)
	snap := snapshotWithGraph(t, "demo", map[string]compose.Service{
		"api": {},
	})
	api := snap.Projects[0].Graph.ByName["api"]

	tracker.Observe(snap, start)
	api.ContainerID = "abc"
	api.State = docker.StateStarting
	tracker.Observe(snap, start.Add(2*time.Second))
	api.State = docker.StateHealthy
	tracker.Observe(snap, start.Add(5*time.Second))

	spans := tracker.Spans("demo", start.Add(8*time.Second))
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(spans))
	}
	span := spans[0]
	if span.Service != "api" {
		t.Fatalf("service = %q, want api", span.Service)
	}
	if span.Final != dag.DisplayHealthy {
		t.Fatalf("final = %s, want healthy", span.Final)
	}
	if span.Duration != 8*time.Second {
		t.Fatalf("duration = %s, want 8s", span.Duration)
	}
	assertSegments(t, span.Segments, []Segment{
		{State: dag.DisplayPending, Dur: 2 * time.Second},
		{State: dag.DisplayStarting, Dur: 3 * time.Second},
		{State: dag.DisplayHealthy, Dur: 3 * time.Second},
	})
}

func TestTrackerAttachedLateWhenAlreadyHealthy(t *testing.T) {
	start := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	tracker := New(start)
	snap := snapshotWithGraph(t, "demo", map[string]compose.Service{
		"redis": {},
		"api":   {},
	})
	redis := snap.Projects[0].Graph.ByName["redis"]
	api := snap.Projects[0].Graph.ByName["api"]

	redis.ContainerID = "r1"
	redis.State = docker.StateHealthy
	api.State = docker.StatePending
	tracker.Observe(snap, start)
	tracker.Observe(snap, start.Add(3*time.Second))

	spans := tracker.Spans("demo", start.Add(3*time.Second))
	redisSpan := findSpan(t, spans, "redis")
	if !redisSpan.AttachedLate {
		t.Fatalf("redis AttachedLate = false, want true")
	}
	apiSpan := findSpan(t, spans, "api")
	if apiSpan.AttachedLate {
		t.Fatalf("api AttachedLate = true, want false")
	}
}

func TestTrackerSpansBlockedWithWaitsOnCondition(t *testing.T) {
	start := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	tracker := New(start)
	snap := snapshotWithGraph(t, "demo", map[string]compose.Service{
		"postgres": {},
		"api": {
			DependsOn: compose.DependsOn{
				"postgres": {Condition: "service_healthy"},
			},
		},
	})

	tracker.Observe(snap, start)
	tracker.Observe(snap, start.Add(4*time.Second))

	spans := tracker.Spans("demo", start.Add(6*time.Second))
	api := findSpan(t, spans, "api")
	if api.Final != dag.DisplayBlocked {
		t.Fatalf("final = %s, want blocked", api.Final)
	}
	if api.WaitsOn != "postgres:healthy" {
		t.Fatalf("waitsOn = %q, want postgres:healthy", api.WaitsOn)
	}
	assertSegments(t, api.Segments, []Segment{
		{State: dag.DisplayBlocked, Dur: 6 * time.Second},
	})
}

func snapshotWithGraph(t *testing.T, project string, services map[string]compose.Service) *discover.Snapshot {
	t.Helper()
	g, err := dag.Build(&compose.Config{Services: services})
	if err != nil {
		t.Fatal(err)
	}
	return &discover.Snapshot{
		Projects: []discover.Project{{
			Name:  project,
			Graph: g,
		}},
	}
}

func findSpan(t *testing.T, spans []Span, service string) Span {
	t.Helper()
	for _, span := range spans {
		if span.Service == service {
			return span
		}
	}
	t.Fatalf("missing span for %q in %#v", service, spans)
	return Span{}
}

func assertSegments(t *testing.T, got, want []Segment) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("segments = %#v, want %#v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("segment %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}
