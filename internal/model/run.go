package model

import (
	"encoding/json"
	"sort"
	"time"
)

// SchemaVersion is embedded in every serialized Run.
// Bump when the on-disk / JSON shape of Run or Event changes incompatibly.
const SchemaVersion = 2

// Run is the aggregate state of one Compose recording session.
type Run struct {
	SchemaVersion   int                 `json:"schema_version"`
	ID              string              `json:"id"`
	StartedAt       time.Time           `json:"started_at"`
	EndedAt         *time.Time          `json:"ended_at,omitempty"`
	Project         string              `json:"project,omitempty"`
	Invocation      *Invocation         `json:"invocation,omitempty"`
	EffectiveConfig *EffectiveConfig    `json:"effective_config,omitempty"`
	Services        map[string]*Service `json:"services,omitempty"`
	Events          []Event             `json:"events,omitempty"`
	Findings        []Finding           `json:"findings,omitempty"`
	Artifacts       []string            `json:"artifacts,omitempty"`
}

// NewRun creates an empty run with the current schema version.
func NewRun(id string, startedAt time.Time) *Run {
	return &Run{
		SchemaVersion: SchemaVersion,
		ID:            id,
		StartedAt:     startedAt.UTC(),
		Services:      make(map[string]*Service),
	}
}

// ApplyEvent appends an event and updates derived service state.
func (r *Run) ApplyEvent(ev Event) {
	if r.Services == nil {
		r.Services = make(map[string]*Service)
	}
	ev.RunID = r.ID
	r.Events = append(r.Events, ev)

	if ev.Service == "" {
		return
	}
	key := serviceKey(ev.Project, ev.Service)
	svc := r.Services[key]
	if svc == nil {
		svc = &Service{Name: ev.Service, Project: ev.Project}
		r.Services[key] = svc
	}
	if ev.ContainerID != "" {
		svc.ContainerID = ev.ContainerID
	}
	prevPhase := svc.Phase
	// Monotonic forward for non-terminal phases; terminal always applies.
	if ev.Phase.Terminal() {
		svc.Phase = ev.Phase
	} else if !svc.Phase.Terminal() && ev.Phase >= svc.Phase {
		svc.Phase = ev.Phase
	}
	if len(svc.PhaseHistory) == 0 || svc.Phase != prevPhase {
		if len(svc.PhaseHistory) == 0 || svc.PhaseHistory[len(svc.PhaseHistory)-1].Phase != svc.Phase {
			svc.PhaseHistory = append(svc.PhaseHistory, PhaseTransition{
				Phase:     svc.Phase,
				Timestamp: ev.Timestamp.UTC(),
				Source:    ev.Source,
				Message:   ev.Message,
			})
		}
	}
	svc.UpdatedAt = ev.Timestamp.UTC()
	if ev.Data != nil {
		if img, ok := ev.Data["image"].(string); ok {
			svc.Image = img
		}
		if status, ok := ev.Data["status"].(string); ok {
			svc.Status = status
		}
		if ports, ok := ev.Data["ports"].([]string); ok {
			svc.Ports = append([]string(nil), ports...)
		} else if rawPorts, ok := ev.Data["ports"].([]any); ok {
			svc.Ports = make([]string, 0, len(rawPorts))
			for _, p := range rawPorts {
				if s, ok := p.(string); ok {
					svc.Ports = append(svc.Ports, s)
				}
			}
		}
		if code, ok := asInt(ev.Data["exit_code"]); ok {
			svc.ExitCode = &code
		}
	}
}

// ApplyEvents applies events in order.
func (r *Run) ApplyEvents(events []Event) {
	for _, ev := range events {
		r.ApplyEvent(ev)
	}
}

// Clone returns a deep copy suitable for concurrent readers.
func (r *Run) Clone() *Run {
	if r == nil {
		return nil
	}
	out := *r
	if r.EndedAt != nil {
		t := *r.EndedAt
		out.EndedAt = &t
	}
	if r.Services != nil {
		out.Services = make(map[string]*Service, len(r.Services))
		for k, svc := range r.Services {
			cp := *svc
			if svc.ExitCode != nil {
				code := *svc.ExitCode
				cp.ExitCode = &code
			}
			if svc.Ports != nil {
				cp.Ports = append([]string(nil), svc.Ports...)
			}
			if svc.PhaseHistory != nil {
				cp.PhaseHistory = append([]PhaseTransition(nil), svc.PhaseHistory...)
			}
			out.Services[k] = &cp
		}
	}
	if r.Events != nil {
		out.Events = append([]Event(nil), r.Events...)
		for i := range out.Events {
			if r.Events[i].Data != nil {
				out.Events[i].Data = copyData(r.Events[i].Data)
			}
		}
	}
	if r.Findings != nil {
		out.Findings = append([]Finding(nil), r.Findings...)
	}
	if r.Artifacts != nil {
		out.Artifacts = append([]string(nil), r.Artifacts...)
	}
	if r.Invocation != nil {
		inv := *r.Invocation
		inv.Command = append([]string(nil), r.Invocation.Command...)
		inv.ComposeFiles = append([]string(nil), r.Invocation.ComposeFiles...)
		inv.Profiles = append([]string(nil), r.Invocation.Profiles...)
		inv.EnvNames = append([]string(nil), r.Invocation.EnvNames...)
		if r.Invocation.EnvValues != nil {
			inv.EnvValues = copyStringMap(r.Invocation.EnvValues)
		}
		out.Invocation = &inv
	}
	if r.EffectiveConfig != nil {
		cfg := *r.EffectiveConfig
		cfg.Services = append([]EffectiveService(nil), r.EffectiveConfig.Services...)
		cfg.Overrides = append([]OverrideNote(nil), r.EffectiveConfig.Overrides...)
		cfg.SourceFiles = append([]string(nil), r.EffectiveConfig.SourceFiles...)
		out.EffectiveConfig = &cfg
	}
	return &out
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// ServiceList returns services sorted by project then name (stable for tests).
func (r *Run) ServiceList() []*Service {
	if r == nil || len(r.Services) == 0 {
		return nil
	}
	out := make([]*Service, 0, len(r.Services))
	for _, svc := range r.Services {
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// MarshalJSON ensures schema_version is always present.
func (r *Run) MarshalJSON() ([]byte, error) {
	type alias Run
	if r.SchemaVersion == 0 {
		r.SchemaVersion = SchemaVersion
	}
	return json.Marshal((*alias)(r))
}

func serviceKey(project, service string) string {
	if project == "" {
		return service
	}
	return project + "/" + service
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func copyData(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
