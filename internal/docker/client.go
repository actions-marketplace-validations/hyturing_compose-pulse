package docker

import (
	"context"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

var exitCodeRe = regexp.MustCompile(`^Exited \((\d+)\)`)

// parseExitCode extracts the exit code from a Docker status string like
// "Exited (137) 2 hours ago". Returns nil when the container is not exited.
func parseExitCode(status string) *int {
	m := exitCodeRe.FindStringSubmatch(status)
	if m == nil {
		return nil
	}
	code, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}
	return &code
}

// dockerAPI is the subset of the Docker SDK used by Client (mockable in tests).
type dockerAPI interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerExecCreate(ctx context.Context, containerID string, options container.ExecOptions) (container.ExecCreateResponse, error)
	ContainerExecAttach(ctx context.Context, execID string, options container.ExecAttachOptions) (types.HijackedResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error)
	ContainerStatsOneShot(ctx context.Context, containerID string) (container.StatsResponseReader, error)
	Close() error
}

// Client wraps the official Docker SDK client.
type Client struct {
	api          dockerAPI
	inspectMu    sync.Mutex
	inspectCache map[string]inspectCacheEntry
}

// NewClient creates a Client connected to the local Docker daemon via DOCKER_HOST / Unix socket.
func NewClient() (*Client, error) {
	dc, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}
	return &Client{api: dc}, nil
}

// Close releases the underlying Docker client.
func (c *Client) Close() error { return c.api.Close() }

// FetchStatesByID returns observed states keyed by container ID.
// IDs not found in the current list are omitted; callers keep the last known state.
func (c *Client) FetchStatesByID(ctx context.Context, ids []string) (map[string]ContainerState, error) {
	if len(ids) == 0 {
		return map[string]ContainerState{}, nil
	}

	containers, err := c.api.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	want := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		want[id] = struct{}{}
	}

	states := make(map[string]ContainerState, len(ids))
	for _, ctr := range containers {
		if _, ok := want[ctr.ID]; !ok {
			continue
		}
		states[ctr.ID] = mapContainerState(ctr)
	}
	return states, nil
}

// mapContainerState converts a Docker container summary to a ContainerState.
// ctr.Status is a human-readable string like "Up 2 minutes (healthy)" — we
// parse it with strings.Contains rather than an exact switch.
func mapContainerState(ctr container.Summary) ContainerState {
	switch ctr.State {
	case "running":
		st := ctr.Status
		switch {
		case strings.Contains(st, "(healthy)"):
			return StateHealthy
		case strings.Contains(st, "(health: starting)"):
			return StateStarting
		case strings.Contains(st, "(unhealthy)"):
			return StateUnhealthy
		default:
			// Running with no healthcheck — treat as healthy.
			return StateHealthy
		}
	case "exited":
		return StateExited
	default:
		return StateStarting
	}
}
