package chain

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/probe/dns"
	"github.com/hyturing/compose-pulse/internal/probe/httpp"
	"github.com/hyturing/compose-pulse/internal/probe/process"
	"github.com/hyturing/compose-pulse/internal/probe/tcp"
	"github.com/hyturing/compose-pulse/internal/probe/tlscheck"
)

// Method identifies how the probe was executed.
type Method string

// Probe execution methods.
const (
	MethodNativeExec Method = "native_exec"
	MethodEphemeral  Method = "ephemeral"
)

// Status is the outcome of one chain step.
type Status string

// Step statuses.
const (
	StatusPass Status = "PASS"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"
	StatusWarn Status = "WARN"
)

// Step is one recorded probe-chain check.
type Step struct {
	Name     string
	Status   Status
	Detail   string
	Duration time.Duration
}

// Result is the full dependency probe chain outcome.
type Result struct {
	FromService string
	TargetHost  string
	TargetPort  int
	Method      Method
	Steps       []Step
	HardFailAt  string
}

// Exec runs a command in the probe network context.
type Exec func(ctx context.Context, cmd []string) (output string, exitCode int, err error)

// Env describes runtime facts needed before network probes.
type Env struct {
	FromService      string
	ContainerID      string
	ContainerExists  bool
	ContainerRunning bool
	SharedNetwork    bool
	TargetHost       string
	TargetPort       int
	WantTLS          bool
	WantHTTP         bool
	HTTPPath         string
}

// Runner orchestrates the dependency probe chain.
type Runner struct {
	// NativeExec runs inside the dependent container (Option A).
	NativeExec Exec
	// EphemeralExec runs from a same-network probe container (Option B).
	EphemeralExec Exec
	// HasTool reports whether a binary exists in the native context.
	// If nil, tools are assumed present when NativeExec is set.
	HasTool func(ctx context.Context, bin string) bool
	Now     func() time.Time
}

// Run executes the 10-step chain, short-circuiting on the first hard FAIL
// after recording that step. Soft SKIP/WARN do not stop the chain.
func (r *Runner) Run(ctx context.Context, env Env) *Result {
	res := &Result{
		FromService: env.FromService,
		TargetHost:  env.TargetHost,
		TargetPort:  env.TargetPort,
	}
	now := time.Now
	if r != nil && r.Now != nil {
		now = r.Now
	}

	add := func(name string, st Status, detail string, d time.Duration) {
		res.Steps = append(res.Steps, Step{Name: name, Status: st, Detail: detail, Duration: d})
		if st == StatusFail && res.HardFailAt == "" {
			res.HardFailAt = name
		}
	}

	start := now()
	if !env.ContainerExists {
		add("container_exists", StatusFail, "dependent container not found", now().Sub(start))
		return res
	}
	add("container_exists", StatusPass, env.ContainerID, now().Sub(start))

	start = now()
	if !env.ContainerRunning {
		add("container_running", StatusFail, "dependent container is not running", now().Sub(start))
		return res
	}
	add("container_running", StatusPass, "", now().Sub(start))

	start = now()
	if !env.SharedNetwork {
		add("shared_network", StatusFail, "services do not share a Docker network", now().Sub(start))
		return res
	}
	add("shared_network", StatusPass, "", now().Sub(start))

	exec, method := r.selectExec(ctx)
	res.Method = method
	if exec == nil {
		add("probe_context", StatusFail, "no probe exec available", 0)
		return res
	}
	add("probe_context", StatusPass, string(method), 0)

	ex := adapt(exec)

	start = now()
	ip, err := dns.Resolve(ctx, ex, env.TargetHost)
	if err != nil {
		add("dns", StatusFail, err.Error(), now().Sub(start))
		return res
	}
	add("dns", StatusPass, ip, now().Sub(start))

	start = now()
	add("route", StatusPass, "assumed via docker network after DNS", now().Sub(start))

	start = now()
	if err := tcp.Connect(ctx, ex, env.TargetHost, env.TargetPort); err != nil {
		add("tcp", StatusFail, err.Error(), now().Sub(start))
		// still record listening check for evidence
		start2 := now()
		listening, lerr := process.Listening(ctx, ex, env.TargetPort)
		detail := ""
		if lerr != nil {
			detail = lerr.Error()
		} else if !listening {
			detail = fmt.Sprintf("no listener on 0.0.0.0:%d", env.TargetPort)
		}
		st := StatusFail
		if listening {
			st = StatusPass
		}
		add("process_listening", st, detail, now().Sub(start2))
		return res
	}
	add("tcp", StatusPass, "", now().Sub(start))

	if env.WantTLS {
		start = now()
		if err := tlscheck.Handshake(ctx, ex, env.TargetHost, env.TargetPort); err != nil {
			add("tls", StatusFail, err.Error(), now().Sub(start))
			return res
		}
		add("tls", StatusPass, "", now().Sub(start))
	} else {
		add("tls", StatusSkip, "not requested", 0)
	}

	if env.WantHTTP {
		start = now()
		path := env.HTTPPath
		if path == "" {
			path = "/"
		}
		if err := httpp.Get(ctx, ex, env.TargetHost, env.TargetPort, path, env.WantTLS); err != nil {
			add("http", StatusFail, err.Error(), now().Sub(start))
			return res
		}
		add("http", StatusPass, path, now().Sub(start))
	} else {
		add("http", StatusSkip, "not requested", 0)
	}

	// Listening is checked inside the probe source namespace. For a remote
	// Compose service hostname, TCP success is sufficient evidence.
	if isLocalProbeTarget(env.TargetHost) {
		start = now()
		listening, err := process.Listening(ctx, ex, env.TargetPort)
		switch {
		case err != nil:
			add("process_listening", StatusWarn, err.Error(), now().Sub(start))
		case !listening:
			add("process_listening", StatusFail, fmt.Sprintf("no listener on port %d", env.TargetPort), now().Sub(start))
			return res
		default:
			add("process_listening", StatusPass, strconv.Itoa(env.TargetPort), now().Sub(start))
		}
	} else {
		add("process_listening", StatusSkip, "remote target; TCP succeeded", 0)
	}

	add("application_ready", StatusPass, "tcp reachable", 0)
	return res
}

func isLocalProbeTarget(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "", "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	default:
		return false
	}
}

func adapt(exec Exec) func(context.Context, []string) (string, int, error) {
	return func(ctx context.Context, cmd []string) (string, int, error) {
		return exec(ctx, cmd)
	}
}

func (r *Runner) selectExec(ctx context.Context) (Exec, Method) {
	if r == nil {
		return nil, ""
	}
	nativeOK := r.NativeExec != nil
	if nativeOK && r.HasTool != nil {
		nativeOK = r.HasTool(ctx, "sh")
	}
	if nativeOK {
		return r.NativeExec, MethodNativeExec
	}
	if r.EphemeralExec != nil {
		return r.EphemeralExec, MethodEphemeral
	}
	if r.NativeExec != nil {
		return r.NativeExec, MethodNativeExec
	}
	return nil, ""
}

// ToEvents converts a chain result into model probe events.
func ToEvents(runID, project string, res *Result, ts time.Time) []model.Event {
	if res == nil {
		return nil
	}
	out := make([]model.Event, 0, len(res.Steps)+1)
	target := net.JoinHostPort(res.TargetHost, strconv.Itoa(res.TargetPort))
	out = append(out, model.Event{
		RunID:     runID,
		Timestamp: ts.UTC(),
		Source:    model.SourceProbe,
		Project:   project,
		Service:   res.FromService,
		Phase:     model.PhaseProcessRunning,
		Type:      model.EventTypeProbe,
		Severity:  model.SeverityInfo,
		Message:   fmt.Sprintf("probe %s → %s via %s", res.FromService, target, res.Method),
		Data: map[string]any{
			"probe_method": string(res.Method),
			"target_host":  res.TargetHost,
			"target_port":  res.TargetPort,
			"hard_fail_at": res.HardFailAt,
		},
	})
	for i, step := range res.Steps {
		sev := model.SeverityInfo
		phase := model.PhaseProcessRunning
		switch step.Status {
		case StatusFail:
			sev = model.SeverityError
			phase = model.PhaseFailed
		case StatusWarn:
			sev = model.SeverityWarn
		}
		out = append(out, model.Event{
			RunID:     runID,
			Timestamp: ts.UTC().Add(time.Duration(i+1) * time.Millisecond),
			Source:    model.SourceProbe,
			Project:   project,
			Service:   res.FromService,
			Phase:     phase,
			Type:      model.EventTypeProbe,
			Severity:  sev,
			Message:   fmt.Sprintf("%s %s %s", step.Name, step.Status, step.Detail),
			Data: map[string]any{
				"probe_step":   step.Name,
				"probe_status": string(step.Status),
				"probe_method": string(res.Method),
				"target_host":  res.TargetHost,
				"target_port":  res.TargetPort,
				"detail":       step.Detail,
			},
		})
	}
	return out
}

// FormatTable renders the human-readable probe table.
func FormatTable(res *Result) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s → %s:%d\n", res.FromService, res.TargetHost, res.TargetPort)
	fmt.Fprintf(&b, "Method                   %s\n", res.Method)
	for _, step := range res.Steps {
		fmt.Fprintf(&b, "%-24s %-4s %s\n", step.Name, step.Status, step.Detail)
	}
	if res.HardFailAt != "" {
		fmt.Fprintf(&b, "\nFirst failure: %s\n", res.HardFailAt)
	}
	return b.String()
}
