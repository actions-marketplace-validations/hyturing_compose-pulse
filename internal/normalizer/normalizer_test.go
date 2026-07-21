package normalizer_test

import (
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/collector"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/normalizer"
)

func TestNormalizeContainerList(t *testing.T) {
	exit := 1
	sig := collector.RawSignal{
		Kind:      collector.KindContainerList,
		Timestamp: time.Date(2026, 7, 22, 1, 0, 0, 0, time.UTC).UnixNano(),
		Payload: collector.ContainerListPayload{
			Containers: []docker.ContainerInfo{
				{
					ID:    "c1",
					Image: "api:1",
					Labels: map[string]string{
						"com.docker.compose.project": "demo",
						"com.docker.compose.service": "api",
					},
					State:  docker.StateHealthy,
					Status: "Up (healthy)",
					Ports:  []string{"8080:80/tcp"},
				},
				{
					ID:       "c2",
					Image:    "db:1",
					Labels:   map[string]string{"com.docker.compose.project": "demo", "com.docker.compose.service": "db"},
					State:    docker.StateExited,
					ExitCode: &exit,
					Status:   "Exited (1)",
				},
			},
		},
	}

	events, err := normalizer.Normalize("run-1", sig)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events=%d, want 2", len(events))
	}
	if events[0].Source != model.SourceContainer || events[0].Phase != model.PhaseHealthy {
		t.Fatalf("api event: %+v", events[0])
	}
	if events[0].Project != "demo" || events[0].Service != "api" {
		t.Fatalf("api identity: %+v", events[0])
	}
	if events[1].Phase != model.PhaseFailed || events[1].Severity != model.SeverityError {
		t.Fatalf("db event: %+v", events[1])
	}
}

func TestNormalizeDockerEvent(t *testing.T) {
	ts := time.Date(2026, 7, 22, 5, 0, 0, 0, time.UTC)
	events, err := normalizer.Normalize("r", collector.RawSignal{
		Kind:      collector.KindDockerEvent,
		Timestamp: ts.UnixNano(),
		Payload: collector.DockerEventPayload{Event: docker.ContainerEvent{
			Time:        ts,
			Action:      "die",
			ContainerID: "c1",
			Project:     "demo",
			Service:     "api",
			ExitCode:    "1",
			Image:       "api:dev",
		}},
	})
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%v err=%v", events, err)
	}
	if events[0].Phase != model.PhaseFailed || events[0].Type != model.EventTypeLifecycle {
		t.Fatalf("got %+v", events[0])
	}
}

func TestNormalizeInspectLogStats(t *testing.T) {
	ts := time.Now().UTC().UnixNano()

	inspectEvents, err := normalizer.Normalize("r", collector.RawSignal{
		Kind:      collector.KindInspect,
		Timestamp: ts,
		Payload: collector.InspectPayload{
			ContainerID: "abc",
			Info:        &docker.InspectInfo{RestartCount: 3, OOMKilled: true},
		},
	})
	if err != nil || len(inspectEvents) != 1 || inspectEvents[0].Type != model.EventTypeInspect {
		t.Fatalf("inspect: events=%v err=%v", inspectEvents, err)
	}

	logEvents, err := normalizer.Normalize("r", collector.RawSignal{
		Kind:      collector.KindLogLine,
		Timestamp: ts,
		Payload:   collector.LogLinePayload{ContainerID: "abc", Line: "boom"},
	})
	if err != nil || len(logEvents) != 1 || logEvents[0].Type != model.EventTypeLog || logEvents[0].Message != "boom" {
		t.Fatalf("log: events=%v err=%v", logEvents, err)
	}

	statsEvents, err := normalizer.Normalize("r", collector.RawSignal{
		Kind:      collector.KindStats,
		Timestamp: ts,
		Payload: collector.StatsPayload{Samples: map[string]*docker.StatsInfo{
			"abc": {CPUPercent: 12.5, MemUsage: 100, MemLimit: 200},
		}},
	})
	if err != nil || len(statsEvents) != 1 || statsEvents[0].Type != model.EventTypeStats {
		t.Fatalf("stats: events=%v err=%v", statsEvents, err)
	}
}
