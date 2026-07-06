package docker

import (
	"context"
	"time"
)

const pollInterval = 500 * time.Millisecond

// PollMsg is a Bubble Tea message emitted each poll cycle with full container metadata.
type PollMsg struct {
	Containers []ContainerInfo
}

// StartPollCh launches a background goroutine that polls Docker every 500 ms and
// sends a PollMsg on the returned channel. The goroutine exits when ctx is
// cancelled and the channel is then closed.
func (c *Client) StartPollCh(ctx context.Context) <-chan PollMsg {
	ch := make(chan PollMsg, 1)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				containers, err := c.ListAll(ctx)
				if err != nil {
					continue
				}
				select {
				case ch <- PollMsg{Containers: containers}:
				default: // drop frame if UI is still processing the previous one
				}
			}
		}
	}()
	return ch
}
