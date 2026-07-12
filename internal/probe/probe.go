package probe

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/hyturing/compose-pulse/internal/docker"
)

// StepStatus describes the outcome of one diagnostic probe step.
type StepStatus int

// Probe step statuses.
const (
	StepPass StepStatus = iota
	StepFail
	StepWarn
	StepSkip
)

// Step is a single diagnostic check performed during a probe run.
type Step struct {
	Label  string
	Status StepStatus
	Output string
}

// Report contains the full result of probing one service container.
type Report struct {
	Service     string
	Healthcheck *docker.HealthcheckSpec
	Steps       []Step
	Suggestions []string
}

// Exec runs a command in the probed container and returns its captured result.
type Exec func(ctx context.Context, cmd []string) (output string, exitCode int, err error)

// Run executes a lightweight, one-shot startup probe for a service container.
func Run(ctx context.Context, service string, hc *docker.HealthcheckSpec, ports []string, exec Exec) *Report {
	report := &Report{Service: service, Healthcheck: hc}
	if hc == nil || isHealthcheckDisabled(hc.Test) {
		report.addStep("healthcheck", StepWarn, "no healthcheck configured")
		report.Suggestions = append(report.Suggestions, "Add a healthcheck so cpulse can explain startup readiness.")
		return report
	}

	kind, command, bin, ok := parseHealthcheck(hc.Test)
	if !ok {
		report.addStep("healthcheck", StepWarn, "unsupported or empty healthcheck command")
		report.Suggestions = append(report.Suggestions, "Use CMD or CMD-SHELL healthchecks for probe diagnostics.")
		return report
	}
	if exec == nil {
		report.addStep("healthcheck command", StepSkip, "no exec function available")
		return report
	}

	out, exitCode, err := exec(ctx, []string{"sh", "-c", "command -v " + shellQuote(bin)})
	if exitCode == 126 || exitCode == 127 {
		report.addStep("container shell", StepWarn, combineOutput(out, err))
		report.Suggestions = append(report.Suggestions, "Install /bin/sh in the image or use an image with shell support for diagnostics.")
		return report
	}
	if err != nil || exitCode != 0 {
		report.addStep("healthcheck binary", StepFail, combineOutput(out, err))
		report.Suggestions = append(report.Suggestions, fmt.Sprintf("Install %q in the image or update the healthcheck command.", bin))
		return report
	}
	report.addStep("healthcheck binary", StepPass, out)

	runCmd := command
	if kind == "CMD-SHELL" {
		runCmd = []string{"sh", "-c", command[0]}
	}
	out, exitCode, err = exec(ctx, runCmd)
	if err != nil || exitCode != 0 {
		report.addStep("healthcheck command", StepFail, combineOutput(out, err))
	} else {
		report.addStep("healthcheck command", StepPass, out)
	}

	report.addURLHostWarnings(command)
	report.checkPorts(ctx, ports, exec)
	return report
}

func (r *Report) addStep(label string, status StepStatus, output string) {
	r.Steps = append(r.Steps, Step{Label: label, Status: status, Output: strings.TrimSpace(output)})
}

func isHealthcheckDisabled(test []string) bool {
	return len(test) > 0 && strings.EqualFold(test[0], "NONE")
}

func parseHealthcheck(test []string) (kind string, command []string, bin string, ok bool) {
	if len(test) == 0 {
		return "", nil, "", false
	}
	switch strings.ToUpper(test[0]) {
	case "CMD":
		if len(test) < 2 {
			return "", nil, "", false
		}
		return "CMD", append([]string(nil), test[1:]...), test[1], true
	case "CMD-SHELL":
		if len(test) < 2 || strings.TrimSpace(test[1]) == "" {
			return "", nil, "", false
		}
		fields := strings.Fields(test[1])
		if len(fields) == 0 {
			return "", nil, "", false
		}
		return "CMD-SHELL", []string{test[1]}, fields[0], true
	default:
		// Docker also permits omitting CMD and treating the whole slice as exec form.
		return "CMD", append([]string(nil), test...), test[0], true
	}
}

func combineOutput(output string, err error) string {
	if err == nil {
		return output
	}
	if output == "" {
		return err.Error()
	}
	return strings.TrimRight(output, "\n") + "\n" + err.Error()
}

func (r *Report) addURLHostWarnings(command []string) {
	for _, part := range command {
		for _, token := range strings.Fields(part) {
			u, err := url.Parse(token)
			if err != nil || u.Hostname() == "" {
				continue
			}
			if isLocalHost(u.Hostname()) {
				continue
			}
			r.addStep("healthcheck URL host", StepWarn, fmt.Sprintf("healthcheck targets non-local host %s", u.Hostname()))
			r.Suggestions = append(r.Suggestions, "Prefer localhost or 127.0.0.1 for container-internal healthcheck URLs.")
			return
		}
	}
}

func isLocalHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsUnspecified()
}

func (r *Report) checkPorts(ctx context.Context, ports []string, exec Exec) {
	for _, port := range privatePorts(ports) {
		out, exitCode, err := exec(ctx, []string{"sh", "-c", "nc -z 127.0.0.1 " + strconv.Itoa(port)})
		label := fmt.Sprintf("port %d/tcp", port)
		if err != nil || exitCode != 0 {
			r.addStep(label, StepFail, combineOutput(out, err))
			r.Suggestions = append(r.Suggestions, fmt.Sprintf("Check whether the service is listening on container port %d.", port))
			continue
		}
		r.addStep(label, StepPass, out)
	}
}

func privatePorts(ports []string) []int {
	seen := map[int]struct{}{}
	var out []int
	for _, port := range ports {
		beforeSlash, _, _ := strings.Cut(port, "/")
		private := beforeSlash
		if idx := strings.LastIndex(private, ":"); idx >= 0 {
			private = private[idx+1:]
		}
		n, err := strconv.Atoi(private)
		if err != nil || n <= 0 {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func shellQuote(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\n'\"\\$`;&|()<>*?[]{}!") {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	}
	return s
}
