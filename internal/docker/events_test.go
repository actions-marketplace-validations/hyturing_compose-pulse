package docker

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/events"
)

type eventsMockAPI struct {
	mockAPI
	messages []events.Message
}

func (m *eventsMockAPI) Events(ctx context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
	msgs := make(chan events.Message, len(m.messages))
	errs := make(chan error, 1)
	go func() {
		defer close(msgs)
		defer close(errs)
		for _, msg := range m.messages {
			select {
			case <-ctx.Done():
				return
			case msgs <- msg:
			}
		}
		<-ctx.Done()
	}()
	return msgs, errs
}

func TestDistillAndStreamEvents(t *testing.T) {
	msg := events.Message{
		Type:   events.ContainerEventType,
		Action: events.ActionDie,
		Actor: events.Actor{
			ID: "abc123",
			Attributes: map[string]string{
				"image":                      "api:dev",
				"name":                       "demo-api-1",
				"com.docker.compose.project": "demo",
				"com.docker.compose.service": "api",
				"exitCode":                   "1",
			},
		},
		TimeNano: time.Date(2026, 7, 22, 5, 0, 0, 0, time.UTC).UnixNano(),
	}
	ev := distillContainerEvent(msg)
	if ev.Service != "api" || ev.Project != "demo" || ev.ExitCode != "1" || ev.Action != "die" {
		t.Fatalf("distill: %+v", ev)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &Client{api: &eventsMockAPI{messages: []events.Message{msg}}}
	ch, errCh := c.StartEventsCh(ctx)
	select {
	case got := <-ch:
		if got.ContainerID != "abc123" || got.Service != "api" {
			t.Fatalf("streamed: %+v", got)
		}
	case err := <-errCh:
		t.Fatalf("err: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
