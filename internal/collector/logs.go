package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/hyturing/compose-pulse/internal/docker"
)

// LogLinePayload is one streamed log line (or stream error).
type LogLinePayload struct {
	ContainerID string
	Line        string
	Err         error
}

// Logs wraps docker log follow as a collector.
type Logs struct {
	client      *docker.Client
	containerID string
	tail        int
}

// NewLogs returns a log-follow collector.
func NewLogs(dc *docker.Client, containerID string, tail int) *Logs {
	return &Logs{client: dc, containerID: containerID, tail: tail}
}

// Name returns the collector name.
func (l *Logs) Name() string { return "logs" }

// Start streams log-line signals until ctx is cancelled or the stream ends.
func (l *Logs) Start(ctx context.Context, out chan<- RawSignal) error {
	if l == nil || l.client == nil {
		return fmt.Errorf("logs collector: nil client")
	}
	ch := l.client.StartLogStreamCh(ctx, l.containerID, l.tail)
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
					Kind:      KindLogLine,
					Timestamp: time.Now().UnixNano(),
					Payload: LogLinePayload{
						ContainerID: l.containerID,
						Line:        msg.Line,
						Err:         msg.Err,
					},
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
