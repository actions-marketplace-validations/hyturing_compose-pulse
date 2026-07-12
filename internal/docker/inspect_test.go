package docker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
)

type countingInspectAPI struct {
	mockAPI
	calls atomic.Int32
	resp  container.InspectResponse
}

func (m *countingInspectAPI) ContainerInspect(_ context.Context, _ string) (container.InspectResponse, error) {
	m.calls.Add(1)
	return m.resp, nil
}

func TestDistillInspect_HealthPresent(t *testing.T) {
	start := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	end := start.Add(50 * time.Millisecond)
	raw := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:           "abc",
			RestartCount: 2,
			State: &container.State{
				OOMKilled:  false,
				Error:      "",
				StartedAt:  start.Format(time.RFC3339Nano),
				FinishedAt: "0001-01-01T00:00:00Z",
				Health: &container.Health{
					Status:        container.Unhealthy,
					FailingStreak: 5,
					Log: []*container.HealthcheckResult{{
						Start:    start,
						End:      end,
						ExitCode: 1,
						Output:   "connection refused",
					}},
				},
			},
			HostConfig: &container.HostConfig{
				RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyOnFailure},
			},
		},
		Config: &container.Config{
			Env: []string{"FOO=bar"},
			Healthcheck: &dockerspec.HealthcheckConfig{
				Test:        []string{"CMD-SHELL", "pg_isready"},
				Interval:    time.Second,
				Timeout:     time.Second,
				StartPeriod: 10 * time.Second,
				Retries:     3,
			},
		},
	}

	info := distillInspect(raw)
	if info.ID != "abc" {
		t.Fatalf("ID = %q", info.ID)
	}
	if info.RestartCount != 2 {
		t.Fatalf("RestartCount = %d", info.RestartCount)
	}
	if info.RestartPolicy != "on-failure" {
		t.Fatalf("RestartPolicy = %q", info.RestartPolicy)
	}
	if info.StartedAt.IsZero() || !info.FinishedAt.IsZero() {
		t.Fatalf("timestamps: started=%v finished=%v", info.StartedAt, info.FinishedAt)
	}
	if info.Health == nil || info.Health.FailingStreak != 5 || len(info.Health.Log) != 1 {
		t.Fatalf("health = %+v", info.Health)
	}
	if info.Healthcheck == nil || info.Healthcheck.Test[1] != "pg_isready" {
		t.Fatalf("healthcheck = %+v", info.Healthcheck)
	}
	if len(info.Env) != 1 || info.Env[0] != "FOO=bar" {
		t.Fatalf("env = %v", info.Env)
	}
}

func TestDistillInspect_NoHealth(t *testing.T) {
	raw := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID: "xyz",
			State: &container.State{
				StartedAt:  "0001-01-01T00:00:00Z",
				FinishedAt: "0001-01-01T00:00:00Z",
			},
		},
		Config: &container.Config{},
	}
	info := distillInspect(raw)
	if !info.StartedAt.IsZero() || info.Health != nil || info.Healthcheck != nil {
		t.Fatalf("expected empty health/times, got %+v", info)
	}
}

func TestInspect_CacheTTL(t *testing.T) {
	api := &countingInspectAPI{
		resp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				ID:    "c1",
				State: &container.State{StartedAt: time.Now().UTC().Format(time.RFC3339Nano)},
			},
			Config: &container.Config{},
		},
	}
	c := &Client{api: api}

	if _, err := c.Inspect(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Inspect(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	if got := api.calls.Load(); got != 1 {
		t.Fatalf("expected 1 inspect call within TTL, got %d", got)
	}

	// Force cache expiry.
	c.inspectMu.Lock()
	e := c.inspectCache["c1"]
	e.at = time.Now().Add(-3 * time.Second)
	c.inspectCache["c1"] = e
	c.inspectMu.Unlock()

	if _, err := c.Inspect(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	if got := api.calls.Load(); got != 2 {
		t.Fatalf("expected 2 inspect calls after TTL, got %d", got)
	}
}

func TestInspect_PropagatesError(t *testing.T) {
	c := &Client{api: &mockAPI{}}
	_, err := c.Inspect(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error from mock")
	}
}
