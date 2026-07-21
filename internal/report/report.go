package report

import (
	"time"

	"github.com/hyturing/compose-pulse/internal/graph/causal"
	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/profiler/criticalpath"
	"github.com/hyturing/compose-pulse/internal/report/logs"
)

// SchemaVersion is the report document version.
const SchemaVersion = 1

// Report is a format-agnostic incident report.
type Report struct {
	SchemaVersion   int                   `json:"schema_version"`
	GeneratedAt     time.Time             `json:"generated_at"`
	RunID           string                `json:"run_id"`
	Project         string                `json:"project,omitempty"`
	Summary         string                `json:"summary"`
	RootCause       string                `json:"root_cause,omitempty"`
	Confidence      string                `json:"confidence,omitempty"`
	Service         string                `json:"service,omitempty"`
	BlockedServices []string              `json:"blocked_services,omitempty"`
	CausalChain     []string              `json:"causal_chain,omitempty"`
	Findings        []model.Finding       `json:"findings,omitempty"`
	CriticalPath    []CriticalPathSegment `json:"critical_path,omitempty"`
	Timeline        []TimelineEntry       `json:"timeline,omitempty"`
	LogWindows      []logs.Window         `json:"log_windows,omitempty"`
	Redaction       []string              `json:"redaction_summary,omitempty"`
	Reproduction    string                `json:"reproduction,omitempty"`
	SuggestedFixes  []string              `json:"suggested_fixes,omitempty"`
	Versions        map[string]string     `json:"versions,omitempty"`
}

// CriticalPathSegment is JSON-friendly critical path data.
type CriticalPathSegment struct {
	Service  string `json:"service"`
	Phase    string `json:"phase,omitempty"`
	Duration string `json:"duration"`
}

// TimelineEntry is a compact service phase marker.
type TimelineEntry struct {
	Service   string    `json:"service"`
	Phase     string    `json:"phase"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
}

// Build constructs a Report from a recorded run (findings should already be attached).
func Build(run *model.Run, findings []model.Finding) *Report {
	if run == nil {
		return nil
	}
	if findings == nil {
		findings = run.Findings
	}
	rep := &Report{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		RunID:         run.ID,
		Project:       run.Project,
		Findings:      append([]model.Finding(nil), findings...),
		Versions:      map[string]string{},
	}
	if run.Invocation != nil {
		rep.Reproduction = reproductionCmd(run.Invocation)
		if run.Invocation.ComposeVersion != "" {
			rep.Versions["compose"] = run.Invocation.ComposeVersion
		}
		if run.Invocation.DockerVersion != "" {
			rep.Versions["docker"] = run.Invocation.DockerVersion
		}
	}

	if len(findings) > 0 {
		f := findings[0]
		rep.RootCause = f.RootCause
		rep.Confidence = f.Confidence.String()
		rep.Service = f.Service
		rep.BlockedServices = append([]string(nil), f.BlockedServices...)
		rep.SuggestedFixes = append([]string(nil), f.SuggestedFixes...)
		rep.Summary = "Root cause: " + f.RootCause
	} else {
		rep.Summary = "No high-confidence findings"
	}

	if causalRes := causal.Analyze(run); causalRes != nil {
		if len(rep.BlockedServices) == 0 {
			rep.BlockedServices = append([]string(nil), causalRes.BlockedServices...)
		}
		rep.CausalChain = append([]string(nil), causalRes.Chain...)
		if rep.Service == "" {
			rep.Service = causalRes.FirstFailure
		}
	}

	if path := criticalpath.Compute(run); path != nil {
		for _, seg := range path.Segments {
			rep.CriticalPath = append(rep.CriticalPath, CriticalPathSegment{
				Service:  seg.Service,
				Phase:    seg.Phase.String(),
				Duration: seg.Duration.String(),
			})
		}
	}

	for _, ev := range run.Events {
		if ev.Service == "" {
			continue
		}
		if ev.Type == model.EventTypeState || ev.Type == model.EventTypeLifecycle {
			rep.Timeline = append(rep.Timeline, TimelineEntry{
				Service:   ev.Service,
				Phase:     ev.Phase.String(),
				Timestamp: ev.Timestamp.UTC(),
				Message:   ev.Message,
			})
		}
	}

	reduced, redactions := logs.Reduce(run)
	rep.LogWindows = reduced
	rep.Redaction = redactions
	return rep
}

func reproductionCmd(inv *model.Invocation) string {
	if inv == nil || len(inv.Command) == 0 {
		return "cpulse record -- docker compose up"
	}
	out := "cpulse record --"
	for _, c := range inv.Command {
		out += " " + c
	}
	return out
}
