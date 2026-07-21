package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/hyturing/compose-pulse/internal/docker"
)

// StatsPayload is one batch of one-shot stats samples.
type StatsPayload struct {
	Samples map[string]*docker.StatsInfo
}

// Stats collects one-shot stats for a set of containers.
type Stats struct {
	client       *docker.Client
	containerIDs []string
}

// NewStats returns a one-shot stats collector.
func NewStats(dc *docker.Client, containerIDs []string) *Stats {
	return &Stats{client: dc, containerIDs: containerIDs}
}

// Name returns the collector name.
func (s *Stats) Name() string { return "stats" }

// Start emits one stats batch signal then returns.
func (s *Stats) Start(ctx context.Context, out chan<- RawSignal) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("stats collector: nil client")
	}
	samples := make(map[string]*docker.StatsInfo, len(s.containerIDs))
	for _, id := range s.containerIDs {
		if id == "" {
			continue
		}
		info, err := s.client.Stats(ctx, id)
		if err != nil || info == nil {
			continue
		}
		samples[id] = info
	}
	sig := RawSignal{
		Kind:      KindStats,
		Timestamp: time.Now().UnixNano(),
		Payload:   StatsPayload{Samples: samples},
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- sig:
		return nil
	}
}
