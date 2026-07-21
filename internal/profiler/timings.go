package profiler

import (
	"sort"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
)

// PhaseTiming is the duration a service spent in/at a phase transition.
type PhaseTiming struct {
	Service  string
	Phase    model.ServicePhase
	Start    time.Time
	End      time.Time
	Duration time.Duration
}

// ServiceTimings aggregates per-service phase durations for a run.
type ServiceTimings struct {
	Service string
	Phases  []PhaseTiming
	Total   time.Duration
}

// CaptureTimings derives per-phase timings from a recorded run's phase history.
func CaptureTimings(run *model.Run) []ServiceTimings {
	if run == nil {
		return nil
	}
	var out []ServiceTimings
	for _, svc := range run.ServiceList() {
		st := ServiceTimings{Service: svc.Name}
		hist := svc.PhaseHistory
		for i := 0; i < len(hist); i++ {
			start := hist[i].Timestamp
			end := start
			switch {
			case i+1 < len(hist):
				end = hist[i+1].Timestamp
			case !svc.UpdatedAt.IsZero() && !svc.UpdatedAt.Before(start):
				// Prefer per-service update time so the terminal phase isn't
				// stretched to the whole run end for every service.
				end = svc.UpdatedAt
			case run.EndedAt != nil:
				end = *run.EndedAt
			}
			if end.Before(start) {
				end = start
			}
			d := end.Sub(start)
			st.Phases = append(st.Phases, PhaseTiming{
				Service:  svc.Name,
				Phase:    hist[i].Phase,
				Start:    start,
				End:      end,
				Duration: d,
			})
			st.Total += d
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out
}
