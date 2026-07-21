package docker

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
)

type mockAPI struct {
	containers []container.Summary
}

func (m *mockAPI) ContainerList(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
	return m.containers, nil
}

func (m *mockAPI) ContainerLogs(_ context.Context, _ string, _ container.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(nil), nil
}

func (m *mockAPI) ContainerExecCreate(_ context.Context, _ string, _ container.ExecOptions) (container.ExecCreateResponse, error) {
	return container.ExecCreateResponse{}, errors.New("not implemented")
}

func (m *mockAPI) ContainerExecAttach(_ context.Context, _ string, _ container.ExecAttachOptions) (types.HijackedResponse, error) {
	return types.HijackedResponse{}, errors.New("not implemented")
}

func (m *mockAPI) ContainerExecInspect(_ context.Context, _ string) (container.ExecInspect, error) {
	return container.ExecInspect{}, errors.New("not implemented")
}

func (m *mockAPI) ContainerInspect(_ context.Context, _ string) (container.InspectResponse, error) {
	return container.InspectResponse{}, errors.New("not implemented")
}

func (m *mockAPI) ContainerStatsOneShot(_ context.Context, _ string) (container.StatsResponseReader, error) {
	return container.StatsResponseReader{}, errors.New("not implemented")
}

func (m *mockAPI) Events(ctx context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
	msgs := make(chan events.Message)
	errs := make(chan error, 1)
	go func() {
		defer close(msgs)
		defer close(errs)
		<-ctx.Done()
	}()
	return msgs, errs
}

func (m *mockAPI) Close() error { return nil }

func TestListAll(t *testing.T) {
	c := &Client{api: &mockAPI{
		containers: []container.Summary{
			{
				ID:     "abc123",
				Names:  []string{"/myapp-api-1"},
				Image:  "nginx:alpine",
				State:  "running",
				Status: "Up 2 minutes (healthy)",
				Labels: map[string]string{
					"com.docker.compose.project": "myapp",
					"com.docker.compose.service": "api",
				},
			},
			{
				ID:     "def456",
				Names:  []string{"/stray"},
				Image:  "redis:7",
				State:  "running",
				Status: "Up 1 minute",
				Labels: map[string]string{},
			},
		},
	}}

	infos, err := c.ListAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(infos))
	}

	compose := infos[0]
	if compose.Labels["com.docker.compose.service"] != "api" {
		t.Errorf("expected compose service api, got %q", compose.Labels["com.docker.compose.service"])
	}
	if compose.State != StateHealthy {
		t.Errorf("expected healthy state, got %v", compose.State)
	}

	standalone := infos[1]
	if _, ok := standalone.Labels["com.docker.compose.service"]; ok {
		t.Error("expected standalone container without compose labels")
	}
	if standalone.State != StateHealthy {
		t.Errorf("expected healthy state for running container, got %v", standalone.State)
	}
}

func TestFetchStatesByID(t *testing.T) {
	c := &Client{api: &mockAPI{
		containers: []container.Summary{
			{ID: "abc", State: "running", Status: "Up (healthy)"},
			{ID: "def", State: "exited", Status: "Exited (1)"},
		},
	}}

	states, err := c.FetchStatesByID(context.Background(), []string{"abc", "def", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if states["abc"] != StateHealthy {
		t.Errorf("abc: got %v, want healthy", states["abc"])
	}
	if states["def"] != StateExited {
		t.Errorf("def: got %v, want exited", states["def"])
	}
	if _, ok := states["missing"]; ok {
		t.Error("missing ID should be omitted")
	}
}
