package model

import "time"

// Service is the per-service aggregate derived from a run's events.
type Service struct {
	Name         string            `json:"name"`
	Project      string            `json:"project,omitempty"`
	ContainerID  string            `json:"container_id,omitempty"`
	Phase        ServicePhase      `json:"phase"`
	PhaseHistory []PhaseTransition `json:"phase_history,omitempty"`
	ExitCode     *int              `json:"exit_code,omitempty"`
	Image        string            `json:"image,omitempty"`
	Status       string            `json:"status,omitempty"`
	Ports        []string          `json:"ports,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at,omitempty"`
}
