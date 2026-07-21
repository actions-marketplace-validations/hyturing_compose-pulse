package model

import (
	"encoding/json"
	"fmt"
)

// ServicePhase is the monotonic lifecycle phase of a Compose service.
type ServicePhase int

// Service lifecycle phases, ordered from configured through ready, plus terminals.
const (
	PhaseConfigured ServicePhase = iota
	PhasePulling
	PhaseBuilding
	PhaseCreated
	PhaseStarted
	PhaseProcessRunning
	PhasePortListening
	PhaseHealthy
	PhaseApplicationReady
	PhaseExited
	PhaseFailed
)

var servicePhaseNames = [...]string{
	PhaseConfigured:       "configured",
	PhasePulling:          "pulling",
	PhaseBuilding:         "building",
	PhaseCreated:          "created",
	PhaseStarted:          "started",
	PhaseProcessRunning:   "process_running",
	PhasePortListening:    "port_listening",
	PhaseHealthy:          "healthy",
	PhaseApplicationReady: "application_ready",
	PhaseExited:           "exited",
	PhaseFailed:           "failed",
}

func (p ServicePhase) String() string {
	if int(p) >= 0 && int(p) < len(servicePhaseNames) {
		return servicePhaseNames[p]
	}
	return "unknown"
}

// Terminal reports whether the phase is a terminal lifecycle state.
func (p ServicePhase) Terminal() bool {
	return p == PhaseExited || p == PhaseFailed
}

// MarshalJSON encodes the phase as its string name.
func (p ServicePhase) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

// UnmarshalJSON decodes a phase from its string name.
func (p *ServicePhase) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for i, name := range servicePhaseNames {
		if name == s {
			*p = ServicePhase(i)
			return nil
		}
	}
	return fmt.Errorf("unknown service phase %q", s)
}
