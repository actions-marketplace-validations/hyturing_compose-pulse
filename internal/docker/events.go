package docker

import (
	"context"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

// ContainerEvent is a distilled docker events message for containers.
type ContainerEvent struct {
	Time        time.Time
	Action      string
	ContainerID string
	Image       string
	Name        string
	Project     string
	Service     string
	ExitCode    string
	Signal      string
	OOMKilled   bool
	Health      string
	Attributes  map[string]string
}

// StartEventsCh streams container lifecycle events until ctx is cancelled.
// Callers should keep Poll as a fallback when the events stream is unavailable.
func (c *Client) StartEventsCh(ctx context.Context) (<-chan ContainerEvent, <-chan error) {
	out := make(chan ContainerEvent, 64)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)
		f := filters.NewArgs()
		f.Add("type", "container")
		msgCh, errCh := c.api.Events(ctx, events.ListOptions{Filters: f})
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errCh:
				if !ok {
					return
				}
				if err != nil {
					select {
					case errs <- err:
					default:
					}
					return
				}
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				ev := distillContainerEvent(msg)
				select {
				case <-ctx.Done():
					return
				case out <- ev:
				}
			}
		}
	}()
	return out, errs
}

func distillContainerEvent(msg events.Message) ContainerEvent {
	attrs := msg.Actor.Attributes
	if attrs == nil {
		attrs = map[string]string{}
	}
	ts := time.Unix(msg.Time, 0).UTC()
	if msg.TimeNano > 0 {
		ts = time.Unix(0, msg.TimeNano).UTC()
	}
	action := string(msg.Action)
	id := msg.Actor.ID
	ev := ContainerEvent{
		Time:        ts,
		Action:      action,
		ContainerID: id,
		Image:       attrs["image"],
		Name:        attrs["name"],
		Project:     attrs["com.docker.compose.project"],
		Service:     attrs["com.docker.compose.service"],
		ExitCode:    attrs["exitCode"],
		Signal:      attrs["signal"],
		Health:      attrs["health"],
		Attributes:  attrs,
	}
	if attrs["oom"] == "1" || attrs["oomKilled"] == "true" || action == "oom" {
		ev.OOMKilled = true
	}
	return ev
}
