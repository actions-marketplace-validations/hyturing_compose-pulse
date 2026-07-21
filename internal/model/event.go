package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventSource identifies which collector/producer emitted an event.
type EventSource int

// Event sources in the normalized schema.
const (
	SourceCompose EventSource = iota
	SourceDocker
	SourceContainer
	SourceHealthcheck
	SourceProbe
	SourceProcess
	SourceNetwork
	SourceResource
	SourceLog
	SourceDiagnosis
)

var eventSourceNames = [...]string{
	SourceCompose:     "compose",
	SourceDocker:      "docker",
	SourceContainer:   "container",
	SourceHealthcheck: "healthcheck",
	SourceProbe:       "probe",
	SourceProcess:     "process",
	SourceNetwork:     "network",
	SourceResource:    "resource",
	SourceLog:         "log",
	SourceDiagnosis:   "diagnosis",
}

func (s EventSource) String() string {
	if int(s) >= 0 && int(s) < len(eventSourceNames) {
		return eventSourceNames[s]
	}
	return "unknown"
}

// MarshalJSON encodes the source as its string name.
func (s EventSource) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON decodes a source from its string name.
func (s *EventSource) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	for i, name := range eventSourceNames {
		if name == v {
			*s = EventSource(i)
			return nil
		}
	}
	return fmt.Errorf("unknown event source %q", v)
}

// EventType is the kind of normalized lifecycle/diagnostic signal.
type EventType int

// Normalized event types.
const (
	EventTypeState EventType = iota
	EventTypeLog
	EventTypeStats
	EventTypeInspect
	EventTypeProbe
	EventTypeDiagnosis
	EventTypeLifecycle
)

var eventTypeNames = [...]string{
	EventTypeState:     "state",
	EventTypeLog:       "log",
	EventTypeStats:     "stats",
	EventTypeInspect:   "inspect",
	EventTypeProbe:     "probe",
	EventTypeDiagnosis: "diagnosis",
	EventTypeLifecycle: "lifecycle",
}

func (t EventType) String() string {
	if int(t) >= 0 && int(t) < len(eventTypeNames) {
		return eventTypeNames[t]
	}
	return "unknown"
}

// MarshalJSON encodes the type as its string name.
func (t EventType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// UnmarshalJSON decodes a type from its string name.
func (t *EventType) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	for i, name := range eventTypeNames {
		if name == v {
			*t = EventType(i)
			return nil
		}
	}
	return fmt.Errorf("unknown event type %q", v)
}

// Severity is the urgency of an event or finding.
type Severity int

// Event/finding severity levels.
const (
	SeverityInfo Severity = iota
	SeverityWarn
	SeverityError
	SeverityCritical
)

var severityNames = [...]string{
	SeverityInfo:     "info",
	SeverityWarn:     "warn",
	SeverityError:    "error",
	SeverityCritical: "critical",
}

func (s Severity) String() string {
	if int(s) >= 0 && int(s) < len(severityNames) {
		return severityNames[s]
	}
	return "unknown"
}

// MarshalJSON encodes the severity as its string name.
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON decodes a severity from its string name.
func (s *Severity) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	for i, name := range severityNames {
		if name == v {
			*s = Severity(i)
			return nil
		}
	}
	return fmt.Errorf("unknown severity %q", v)
}

// Event is one normalized signal in a Compose run.
type Event struct {
	RunID       string         `json:"run_id"`
	Timestamp   time.Time      `json:"timestamp"`
	Source      EventSource    `json:"source"`
	Project     string         `json:"project,omitempty"`
	Service     string         `json:"service,omitempty"`
	ContainerID string         `json:"container_id,omitempty"`
	Phase       ServicePhase   `json:"phase"`
	Type        EventType      `json:"type"`
	Severity    Severity       `json:"severity"`
	Message     string         `json:"message,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}
