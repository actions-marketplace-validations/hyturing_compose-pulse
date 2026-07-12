package ui

import (
	actionspkg "github.com/hyturing/compose-pulse/internal/actions"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/doctor"
	"github.com/hyturing/compose-pulse/internal/probe"
)

// tickMsg is sent by the spinner animation ticker.
type tickMsg struct{}

// logLineMsg wraps a streaming log line or error from the Docker log follow.
type logLineMsg struct {
	line docker.LogLineMsg
}

// logStreamDoneMsg signals the log stream channel closed.
type logStreamDoneMsg struct{}

// logMoreMsg carries older log lines fetched from Docker.
type logMoreMsg struct {
	lines   []string
	prevLen int
	err     error
}

type actionEventMsg struct {
	event actionspkg.Event
}

// inspectMsg carries the result of an on-demand, cached docker inspect for
// one container (Task 2.1).
type inspectMsg struct {
	id   string
	info *docker.InspectInfo
	err  error
}

// doctorMsg carries the result of running the doctor engine + root-cause
// analysis for one project (Task 2.3/2.4), fired from the doctor tab.
type doctorMsg struct {
	project  string
	findings []doctor.Finding
	root     *doctor.RootCause
}

// probeMsg carries the result of running the health probe for one container
// (Task 2.5), fired from the health tab's enter key.
type probeMsg struct {
	containerID string
	report      *probe.Report
}

// statsMsg carries one batch of one-shot stats samples for currently running
// containers (Task 2.9), fired from the separate 2s stats ticker.
type statsMsg struct {
	samples map[string]*docker.StatsInfo
}

// statsTickMsg drives the 2s stats sweep, independent of the 500ms poll.
type statsTickMsg struct{}
