package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hyturing/compose-pulse/internal/collector"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/normalizer"
)

// RunUpdateMsg is a Bubble Tea message carrying the normalized run aggregate
// plus a discover snapshot derived outside the View/Update Docker-shape path.
type RunUpdateMsg struct {
	Run      *model.Run
	Snapshot *discover.Snapshot
}

// startPollFeed runs the poll collector → normalizer → run pipeline and emits
// RunUpdateMsg values for the TUI. The TUI never sees docker.PollMsg.
func startPollFeed(ctx context.Context, dc *docker.Client, run *model.Run) <-chan RunUpdateMsg {
	out := make(chan RunUpdateMsg, 1)
	raw := make(chan collector.RawSignal, 1)
	poll := collector.NewPoll(dc)
	if err := poll.Start(ctx, raw); err != nil {
		close(out)
		return out
	}
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case sig, ok := <-raw:
				if !ok {
					return
				}
				containers, ok := collector.ContainersOf(sig)
				if !ok {
					continue
				}
				events, err := normalizer.Normalize(run.ID, sig)
				if err != nil {
					continue
				}
				run.ApplyEvents(events)
				snap, err := discover.FromContainers(containers)
				if err != nil {
					continue
				}
				msg := RunUpdateMsg{Run: run.Clone(), Snapshot: snap}
				select {
				case <-ctx.Done():
					return
				case out <- msg:
				default: // drop frame if UI is still processing
				}
			}
		}
	}()
	return out
}

func waitForRunUpdate(ch <-chan RunUpdateMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func newLiveRun() *model.Run {
	id := fmt.Sprintf("run-%d", time.Now().UnixNano())
	return model.NewRun(id, time.Now().UTC())
}
