package timeline

import (
	"sort"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
)

const maxTransitionsPerService = 256

// Tracker records display-state transitions for services over time.
type Tracker struct {
	start        time.Time
	entries      map[string][]Transition
	waits        map[string]string
	attachedLate map[string]bool
}

// Transition captures a service display-state change at a point in time.
type Transition struct {
	State dag.DisplayState
	At    time.Time
}

// New creates a tracker starting at now.
func New(now time.Time) *Tracker {
	return &Tracker{
		start:        now,
		entries:      make(map[string][]Transition),
		waits:        make(map[string]string),
		attachedLate: make(map[string]bool),
	}
}

// alreadyReady reports whether a first-seen display state means the service
// was already up when cpulse attached (no invented startup history).
func alreadyReady(state dag.DisplayState) bool {
	switch state {
	case dag.DisplayHealthy, dag.DisplayCompleted:
		return true
	default:
		return false
	}
}

// Observe records the current display state of each service in a snapshot.
func (t *Tracker) Observe(snap *discover.Snapshot, now time.Time) {
	if t == nil || snap == nil {
		return
	}
	for _, project := range snap.Projects {
		if project.Graph == nil {
			continue
		}
		for _, node := range project.Graph.Ordered {
			if node == nil {
				continue
			}
			state, waitingOn := dag.Display(node, project.Graph)
			key := project.Name + "/" + node.Name
			t.observeState(key, state, now)
			t.waits[key] = waitsOn(node, waitingOn)
		}
	}
}

func (t *Tracker) observeState(key string, state dag.DisplayState, now time.Time) {
	transitions := t.entries[key]
	if len(transitions) == 0 {
		t.attachedLate[key] = alreadyReady(state)
	} else if transitions[len(transitions)-1].State == state {
		return
	}
	transitions = append(transitions, Transition{State: state, At: now})
	if len(transitions) > maxTransitionsPerService {
		transitions = transitions[len(transitions)-maxTransitionsPerService:]
	}
	t.entries[key] = transitions
}

// Spans returns render-ready timeline spans for one compose project.
func (t *Tracker) Spans(project string, now time.Time) []Span {
	if t == nil {
		return nil
	}
	prefix := project + "/"
	keys := make([]string, 0)
	for key := range t.entries {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	spans := make([]Span, 0, len(keys))
	for _, key := range keys {
		transitions := t.entries[key]
		if len(transitions) == 0 {
			continue
		}
		spans = append(spans, Span{
			Service:      strings.TrimPrefix(key, prefix),
			Segments:     segments(transitions, now),
			Final:        transitions[len(transitions)-1].State,
			Duration:     nonNegative(now.Sub(transitions[0].At)),
			WaitsOn:      t.waits[key],
			AttachedLate: t.attachedLate[key],
		})
	}
	return spans
}

// Span is the timeline history for one service.
type Span struct {
	Service      string
	Segments     []Segment
	Final        dag.DisplayState
	Duration     time.Duration
	WaitsOn      string
	AttachedLate bool // true when first observed already healthy/completed
}

// Segment is one contiguous interval spent in a display state.
type Segment struct {
	State dag.DisplayState
	Dur   time.Duration
}

func segments(transitions []Transition, now time.Time) []Segment {
	out := make([]Segment, 0, len(transitions))
	for i, transition := range transitions {
		end := now
		if i+1 < len(transitions) {
			end = transitions[i+1].At
		}
		out = append(out, Segment{
			State: transition.State,
			Dur:   nonNegative(end.Sub(transition.At)),
		})
	}
	return out
}

func waitsOn(node *dag.Node, waitingOn []string) string {
	if len(waitingOn) == 0 {
		return ""
	}
	parts := make([]string, 0, len(waitingOn))
	for _, dep := range waitingOn {
		parts = append(parts, dep+":"+conditionLabel(node.DepConditions[dep]))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func conditionLabel(condition string) string {
	switch condition {
	case "service_healthy":
		return "healthy"
	case "service_completed_successfully":
		return "completed"
	default:
		return "started"
	}
}

func nonNegative(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}
