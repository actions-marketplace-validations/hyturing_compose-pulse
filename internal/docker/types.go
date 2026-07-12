package docker

import "time"

// ContainerState represents the observed lifecycle state of a container.
type ContainerState int

// The set of observable container lifecycle states, ordered from not-yet-started
// to terminal.
const (
	StatePending   ContainerState = iota // waiting on a dependency; not yet started
	StateStarting                        // container exists, health check running
	StateHealthy                         // running and health check passing (or no healthcheck)
	StateUnhealthy                       // health check explicitly failing
	StateExited                          // stopped with non-zero exit code
)

func (s ContainerState) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateStarting:
		return "starting"
	case StateHealthy:
		return "healthy"
	case StateUnhealthy:
		return "unhealthy"
	case StateExited:
		return "exited"
	default:
		return "unknown"
	}
}

// InspectInfo is the normalized subset of docker inspect used by the doctor
// package. It intentionally avoids exposing Docker SDK structs to callers.
type InspectInfo struct {
	ID, RestartPolicy, Error string
	StartedAt, FinishedAt    time.Time
	RestartCount             int
	OOMKilled                bool
	Env                      []string
	Health                   *HealthInfo
	Healthcheck              *HealthcheckSpec
}

// HealthInfo captures the runtime health state and probe history.
type HealthInfo struct {
	Status        string // starting|healthy|unhealthy
	FailingStreak int
	Log           []ProbeResult
}

// ProbeResult is one healthcheck probe result from docker inspect.
type ProbeResult struct {
	Start, End time.Time
	ExitCode   int
	Output     string
}

// HealthcheckSpec captures the configured healthcheck from docker inspect.
type HealthcheckSpec struct {
	Test                           []string
	Interval, Timeout, StartPeriod time.Duration
	Retries                        int
}
