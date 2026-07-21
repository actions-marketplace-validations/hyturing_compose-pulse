package progress

import (
	"bufio"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
)

// Event is a parsed build/pull progress signal before model conversion.
type Event struct {
	Kind      string // pull_start|pull_complete|pull_error|build_step|build_cache|build_error|context_transfer|arch_mismatch|auth_error|manifest_missing
	Image     string
	Service   string
	Step      string
	Message   string
	CacheHit  bool
	Timestamp time.Time
}

// ParseLines extracts build/pull events from Compose/BuildKit-like progress text.
func ParseLines(lines []string, now time.Time) []Event {
	var out []Event
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		low := strings.ToLower(line)
		switch {
		case strings.Contains(low, "authentication required") || strings.Contains(low, "unauthorized") || strings.Contains(low, "denied"):
			out = append(out, Event{Kind: "auth_error", Message: line, Timestamp: now})
		case strings.Contains(low, "manifest unknown") || strings.Contains(low, "not found"):
			if strings.Contains(low, "manifest") || strings.Contains(low, "pull") {
				out = append(out, Event{Kind: "manifest_missing", Message: line, Timestamp: now})
			}
		case strings.Contains(low, "no matching manifest") || strings.Contains(low, "image operating system") || strings.Contains(low, "platform"):
			if strings.Contains(low, "manifest") || strings.Contains(low, "platform") || strings.Contains(low, "operating system") {
				out = append(out, Event{Kind: "arch_mismatch", Message: line, Timestamp: now})
			}
		case strings.Contains(low, "pulling") && (strings.Contains(low, "from") || strings.Contains(low, "fs layer") || strings.HasPrefix(low, "pulling ")):
			out = append(out, Event{Kind: "pull_start", Image: extractImage(line), Message: line, Timestamp: now})
		case strings.Contains(low, "pulled") || strings.Contains(low, "download complete") || strings.Contains(low, "status: downloaded newer"):
			out = append(out, Event{Kind: "pull_complete", Image: extractImage(line), Message: line, Timestamp: now})
		case strings.Contains(low, "error") && (strings.Contains(low, "pull") || strings.Contains(low, "registry")):
			out = append(out, Event{Kind: "pull_error", Message: line, Timestamp: now})
		case strings.Contains(low, "transferring context") || strings.Contains(low, "sending tarball"):
			out = append(out, Event{Kind: "context_transfer", Message: line, Timestamp: now})
		case strings.Contains(low, "cached"):
			out = append(out, Event{Kind: "build_cache", Step: line, Message: line, CacheHit: true, Timestamp: now})
		case strings.Contains(low, "error:") || strings.Contains(low, "failed to solve"):
			out = append(out, Event{Kind: "build_error", Message: line, Timestamp: now})
		case strings.Contains(low, "=>") || strings.Contains(low, "load build definition") ||
			(strings.Contains(low, "[") && (strings.Contains(low, "from ") || strings.Contains(low, "run ") || strings.Contains(low, "copy "))):
			out = append(out, Event{Kind: "build_step", Step: line, Message: line, Timestamp: now})
		}
	}
	return out
}

// ParseReader scans an entire progress log.
func ParseReader(r interface{ Read([]byte) (int, error) }, now time.Time) ([]Event, error) {
	var lines []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return ParseLines(lines, now), sc.Err()
}

// ToModelEvents converts progress events into model.Events.
func ToModelEvents(runID string, project string, events []Event) []model.Event {
	out := make([]model.Event, 0, len(events))
	for _, e := range events {
		phase := model.PhasePulling
		sev := model.SeverityInfo
		switch e.Kind {
		case "build_step", "build_cache", "context_transfer":
			phase = model.PhaseBuilding
		case "auth_error", "manifest_missing", "arch_mismatch", "pull_error", "build_error":
			sev = model.SeverityError
			phase = model.PhaseFailed
		case "pull_complete":
			phase = model.PhasePulling
		}
		out = append(out, model.Event{
			RunID:     runID,
			Timestamp: e.Timestamp.UTC(),
			Source:    model.SourceCompose,
			Project:   project,
			Service:   e.Service,
			Phase:     phase,
			Type:      model.EventTypeLifecycle,
			Severity:  sev,
			Message:   e.Message,
			Data: map[string]any{
				"progress_kind": e.Kind,
				"image":         e.Image,
				"step":          e.Step,
				"cache_hit":     e.CacheHit,
			},
		})
	}
	return out
}

func extractImage(line string) string {
	fields := strings.Fields(line)
	for _, f := range fields {
		if strings.Contains(f, "/") || strings.Contains(f, ":") {
			return strings.Trim(f, "\"'")
		}
	}
	return ""
}
