package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/hyturing/compose-pulse/internal/docker"
)

// ContainerListPayload is the poll collector's raw payload.
type ContainerListPayload struct {
	Containers []docker.ContainerInfo
}

// Poll wraps the existing 500 ms Docker poller as a fallback collector.
// docker events (Phase 1.4) will become the primary lifecycle source.
type Poll struct {
	client *docker.Client
}

// NewPoll returns a poll collector bound to dc.
func NewPoll(dc *docker.Client) *Poll {
	return &Poll{client: dc}
}

// Name returns the collector name.
func (p *Poll) Name() string { return "poll" }

// Start streams container-list signals until ctx is cancelled.
func (p *Poll) Start(ctx context.Context, out chan<- RawSignal) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("poll collector: nil client")
	}
	ch := p.client.StartPollCh(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				sig := RawSignal{
					Kind:      KindContainerList,
					Timestamp: time.Now().UnixNano(),
					Payload:   ContainerListPayload{Containers: msg.Containers},
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

// ContainersOf extracts a container list from a poll signal.
func ContainersOf(sig RawSignal) ([]docker.ContainerInfo, bool) {
	if sig.Kind != KindContainerList {
		return nil, false
	}
	switch p := sig.Payload.(type) {
	case ContainerListPayload:
		return p.Containers, true
	case *ContainerListPayload:
		if p == nil {
			return nil, false
		}
		return p.Containers, true
	default:
		return nil, false
	}
}
