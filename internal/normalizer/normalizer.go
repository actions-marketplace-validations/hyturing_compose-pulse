package normalizer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/collector"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/model"
)

const (
	labelProject = "com.docker.compose.project"
	labelService = "com.docker.compose.service"
)

// Normalize converts a raw collector signal into zero or more model events.
// This is the only package that understands Docker wrapper response shapes.
func Normalize(runID string, sig collector.RawSignal) ([]model.Event, error) {
	switch sig.Kind {
	case collector.KindContainerList:
		return normalizeContainerList(runID, sig)
	case collector.KindInspect:
		return normalizeInspect(runID, sig)
	case collector.KindLogLine:
		return normalizeLogLine(runID, sig)
	case collector.KindStats:
		return normalizeStats(runID, sig)
	case collector.KindDockerEvent:
		return normalizeDockerEvent(runID, sig)
	default:
		return nil, fmt.Errorf("normalizer: unknown signal kind %q", sig.Kind)
	}
}

func normalizeContainerList(runID string, sig collector.RawSignal) ([]model.Event, error) {
	containers, ok := collector.ContainersOf(sig)
	if !ok {
		return nil, fmt.Errorf("normalizer: invalid container_list payload")
	}
	ts := timeFromUnixNano(sig.Timestamp)
	out := make([]model.Event, 0, len(containers))
	for _, c := range containers {
		project := c.Labels[labelProject]
		service := c.Labels[labelService]
		if service == "" {
			if len(c.Names) > 0 {
				service = trimSlash(c.Names[0])
			} else {
				service = c.ID
			}
		}
		phase := phaseFromContainerState(c.State, c.ExitCode)
		data := map[string]any{
			"image":  c.Image,
			"status": c.Status,
			"ports":  append([]string(nil), c.Ports...),
			"state":  c.State.String(),
		}
		if c.ExitCode != nil {
			data["exit_code"] = *c.ExitCode
		}
		sev := model.SeverityInfo
		if phase == model.PhaseFailed {
			sev = model.SeverityError
		}
		out = append(out, model.Event{
			RunID:       runID,
			Timestamp:   ts,
			Source:      model.SourceContainer,
			Project:     project,
			Service:     service,
			ContainerID: c.ID,
			Phase:       phase,
			Type:        model.EventTypeState,
			Severity:    sev,
			Message:     c.Status,
			Data:        data,
		})
	}
	return out, nil
}

func normalizeInspect(runID string, sig collector.RawSignal) ([]model.Event, error) {
	p, ok := sig.Payload.(collector.InspectPayload)
	if !ok {
		if ptr, ok := sig.Payload.(*collector.InspectPayload); ok && ptr != nil {
			p = *ptr
		} else {
			return nil, fmt.Errorf("normalizer: invalid inspect payload")
		}
	}
	ts := timeFromUnixNano(sig.Timestamp)
	data := map[string]any{"container_id": p.ContainerID}
	msg := "inspect"
	sev := model.SeverityInfo
	if p.Err != nil {
		data["error"] = p.Err.Error()
		msg = p.Err.Error()
		sev = model.SeverityWarn
	} else if p.Info != nil {
		data["restart_count"] = p.Info.RestartCount
		data["oom_killed"] = p.Info.OOMKilled
		data["restart_policy"] = p.Info.RestartPolicy
		data["error"] = p.Info.Error
		if p.Info.Health != nil {
			data["health_status"] = p.Info.Health.Status
			data["failing_streak"] = p.Info.Health.FailingStreak
		}
	}
	return []model.Event{{
		RunID:       runID,
		Timestamp:   ts,
		Source:      model.SourceDocker,
		ContainerID: p.ContainerID,
		Phase:       model.PhaseConfigured,
		Type:        model.EventTypeInspect,
		Severity:    sev,
		Message:     msg,
		Data:        data,
	}}, nil
}

func normalizeLogLine(runID string, sig collector.RawSignal) ([]model.Event, error) {
	p, ok := sig.Payload.(collector.LogLinePayload)
	if !ok {
		if ptr, ok := sig.Payload.(*collector.LogLinePayload); ok && ptr != nil {
			p = *ptr
		} else {
			return nil, fmt.Errorf("normalizer: invalid log_line payload")
		}
	}
	ts := timeFromUnixNano(sig.Timestamp)
	sev := model.SeverityInfo
	msg := p.Line
	data := map[string]any{"container_id": p.ContainerID}
	if p.Err != nil {
		sev = model.SeverityWarn
		msg = p.Err.Error()
		data["error"] = p.Err.Error()
	} else {
		data["line"] = p.Line
	}
	return []model.Event{{
		RunID:       runID,
		Timestamp:   ts,
		Source:      model.SourceLog,
		ContainerID: p.ContainerID,
		Phase:       model.PhaseProcessRunning,
		Type:        model.EventTypeLog,
		Severity:    sev,
		Message:     msg,
		Data:        data,
	}}, nil
}

func normalizeStats(runID string, sig collector.RawSignal) ([]model.Event, error) {
	p, ok := sig.Payload.(collector.StatsPayload)
	if !ok {
		if ptr, ok := sig.Payload.(*collector.StatsPayload); ok && ptr != nil {
			p = *ptr
		} else {
			return nil, fmt.Errorf("normalizer: invalid stats payload")
		}
	}
	ts := timeFromUnixNano(sig.Timestamp)
	out := make([]model.Event, 0, len(p.Samples))
	for id, sample := range p.Samples {
		if sample == nil {
			continue
		}
		out = append(out, model.Event{
			RunID:       runID,
			Timestamp:   ts,
			Source:      model.SourceResource,
			ContainerID: id,
			Phase:       model.PhaseProcessRunning,
			Type:        model.EventTypeStats,
			Severity:    model.SeverityInfo,
			Message:     "stats",
			Data: map[string]any{
				"cpu_percent": sample.CPUPercent,
				"mem_usage":   sample.MemUsage,
				"mem_limit":   sample.MemLimit,
				"net_rx":      sample.NetRx,
				"net_tx":      sample.NetTx,
				"pids":        sample.PIDs,
			},
		})
	}
	return out, nil
}

func phaseFromContainerState(state docker.ContainerState, exitCode *int) model.ServicePhase {
	switch state {
	case docker.StatePending:
		return model.PhaseConfigured
	case docker.StateStarting:
		return model.PhaseStarted
	case docker.StateHealthy:
		return model.PhaseHealthy
	case docker.StateUnhealthy:
		return model.PhaseFailed
	case docker.StateExited:
		if exitCode != nil && *exitCode == 0 {
			return model.PhaseExited
		}
		return model.PhaseFailed
	default:
		return model.PhaseConfigured
	}
}

func normalizeDockerEvent(runID string, sig collector.RawSignal) ([]model.Event, error) {
	p, ok := sig.Payload.(collector.DockerEventPayload)
	if !ok {
		if ptr, ok := sig.Payload.(*collector.DockerEventPayload); ok && ptr != nil {
			p = *ptr
		} else {
			return nil, fmt.Errorf("normalizer: invalid docker_event payload")
		}
	}
	ev := p.Event
	phase, sev := phaseFromDockerAction(ev)
	service := ev.Service
	if service == "" {
		service = ev.Name
	}
	data := map[string]any{
		"action": ev.Action,
		"image":  ev.Image,
		"name":   ev.Name,
	}
	if ev.ExitCode != "" {
		data["exit_code_raw"] = ev.ExitCode
		if code, err := strconv.Atoi(ev.ExitCode); err == nil {
			data["exit_code"] = code
		}
	}
	if ev.OOMKilled {
		data["oom_killed"] = true
	}
	if ev.Health != "" {
		data["health"] = ev.Health
	}
	if ev.Signal != "" {
		data["signal"] = ev.Signal
	}
	return []model.Event{{
		RunID:       runID,
		Timestamp:   ev.Time.UTC(),
		Source:      model.SourceDocker,
		Project:     ev.Project,
		Service:     service,
		ContainerID: ev.ContainerID,
		Phase:       phase,
		Type:        model.EventTypeLifecycle,
		Severity:    sev,
		Message:     ev.Action,
		Data:        data,
	}}, nil
}

func phaseFromDockerAction(ev docker.ContainerEvent) (model.ServicePhase, model.Severity) {
	action := strings.ToLower(ev.Action)
	switch {
	case strings.HasPrefix(action, "health_status: healthy"), action == "health_status: healthy":
		return model.PhaseHealthy, model.SeverityInfo
	case strings.Contains(action, "health_status") && strings.Contains(action, "unhealthy"):
		return model.PhaseFailed, model.SeverityError
	case action == "create":
		return model.PhaseCreated, model.SeverityInfo
	case action == "start":
		return model.PhaseStarted, model.SeverityInfo
	case action == "restart":
		return model.PhaseStarted, model.SeverityWarn
	case action == "oom":
		return model.PhaseFailed, model.SeverityCritical
	case action == "die", action == "kill", action == "stop", action == "destroy", action == "remove":
		if ev.ExitCode == "0" {
			return model.PhaseExited, model.SeverityInfo
		}
		return model.PhaseFailed, model.SeverityError
	default:
		return model.PhaseProcessRunning, model.SeverityInfo
	}
}

func timeFromUnixNano(n int64) time.Time {
	if n <= 0 {
		return time.Now().UTC()
	}
	return time.Unix(0, n).UTC()
}

func trimSlash(s string) string {
	for len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	return s
}
