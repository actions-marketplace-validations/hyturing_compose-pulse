package collector

import (
	"context"
	"fmt"

	"github.com/hyturing/compose-pulse/internal/docker"
)

// KindDockerEvent is emitted by the docker-events collector.
const KindDockerEvent Kind = "docker_event"

// DockerEventPayload wraps a distilled container event.
type DockerEventPayload struct {
	Event docker.ContainerEvent
}

// Events streams docker events as raw signals (primary lifecycle source).
type Events struct {
	client *docker.Client
}

// NewEvents returns a docker-events collector.
func NewEvents(dc *docker.Client) *Events {
	return &Events{client: dc}
}

// Name returns the collector name.
func (e *Events) Name() string { return "docker-events" }

// Start streams docker event signals until ctx is cancelled.
func (e *Events) Start(ctx context.Context, out chan<- RawSignal) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("events collector: nil client")
	}
	ch, errCh := e.client.StartEventsCh(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errCh:
				if !ok {
					errCh = nil
					continue
				}
				if err != nil {
					return
				}
			case ev, ok := <-ch:
				if !ok {
					return
				}
				sig := RawSignal{
					Kind:      KindDockerEvent,
					Timestamp: ev.Time.UnixNano(),
					Payload:   DockerEventPayload{Event: ev},
				}
				select {
				case <-ctx.Done():
					return
				case out <- sig:
				}
			}
		}
	}()
	return nil
}
