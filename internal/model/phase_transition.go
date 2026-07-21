package model

import "time"

// PhaseTransition records when a service entered a phase.
type PhaseTransition struct {
	Phase     ServicePhase `json:"phase"`
	Timestamp time.Time    `json:"timestamp"`
	Source    EventSource  `json:"source,omitempty"`
	Message   string       `json:"message,omitempty"`
}
