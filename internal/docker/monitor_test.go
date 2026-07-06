package docker

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
)

func TestStartPollCh(t *testing.T) {
	c := &Client{api: &mockAPI{
		containers: []container.Summary{
			{ID: "abc", State: "running", Status: "Up (healthy)"},
		},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := c.StartPollCh(ctx)
	select {
	case msg := <-ch:
		if len(msg.Containers) != 1 {
			t.Errorf("expected 1 container, got %d", len(msg.Containers))
		}
		if msg.Containers[0].ID != "abc" {
			t.Errorf("unexpected container ID %q", msg.Containers[0].ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for PollMsg")
	}
}
