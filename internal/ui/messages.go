package ui

import (
	actionspkg "github.com/hyturing/compose-pulse/internal/actions"
	"github.com/hyturing/compose-pulse/internal/docker"
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
