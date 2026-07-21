package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/hyturing/compose-pulse/internal/docker"
)

// InspectPayload is one on-demand inspect result.
type InspectPayload struct {
	ContainerID string
	Info        *docker.InspectInfo
	Err         error
}

// Inspect is a one-shot collector for a single container inspect.
type Inspect struct {
	client      *docker.Client
	containerID string
}

// NewInspect returns an inspect collector for containerID.
func NewInspect(dc *docker.Client, containerID string) *Inspect {
	return &Inspect{client: dc, containerID: containerID}
}

// Name returns the collector name.
func (i *Inspect) Name() string { return "inspect" }

// Start emits one inspect signal then returns.
func (i *Inspect) Start(ctx context.Context, out chan<- RawSignal) error {
	if i == nil || i.client == nil {
		return fmt.Errorf("inspect collector: nil client")
	}
	info, err := i.client.Inspect(ctx, i.containerID)
	sig := RawSignal{
		Kind:      KindInspect,
		Timestamp: time.Now().UnixNano(),
		Payload: InspectPayload{
			ContainerID: i.containerID,
			Info:        info,
			Err:         err,
		},
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- sig:
		return nil
	}
}
