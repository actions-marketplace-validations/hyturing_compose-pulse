package doctor

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/docker"
)

// DefaultRules returns the built-in Phase 2 doctor rule set.
func DefaultRules() []Rule {
	return []Rule{
		missingDependencyRule{},
		blockedByUnhealthyRule{},
		unhealthyServiceRule{},
		initFailedRule{},
		failedServiceRule{},
		exitedInitSuccessRule{},
		missingHealthcheckRule{},
		shortDependsOnRule{},
		healthcheckCmdNotFoundRule{},
		localhostHostIPMisuseRule{},
		slowStartupRule{},
		restartLoopRule{},
		oomKilledRule{},
	}
}

type missingDependencyRule struct{}

func (missingDependencyRule) ID() string { return "missing-dependency" }
func (r missingDependencyRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		display, waitingOn := dag.Display(n, g)
		if display != dag.DisplayBlocked {
			continue
		}
		for _, depName := range waitingOn {
			dep := g.ByName[depName]
			if dep == nil || dep.ContainerID != "" {
				continue
			}
			findings = append(findings, Finding{
				RuleID:   r.ID(),
				Service:  n.Name,
				Title:    "Blocked by missing dependency",
				Detail:   fmt.Sprintf("%s is waiting on %s, which has no container.", n.Name, depName),
				Severity: SeverityCritical,
				Evidence: []string{fmt.Sprintf("waiting_on=%s", depName), "container_id="},
				Suggestion: []string{
					fmt.Sprintf("Recreate %s (e.g. docker compose up %s) or remove the depends_on edge.", depName, depName),
				},
			})
		}
	}
	return findings
}

type blockedByUnhealthyRule struct{}

func (blockedByUnhealthyRule) ID() string { return "blocked-by-unhealthy" }
func (r blockedByUnhealthyRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		display, waitingOn := dag.Display(n, g)
		if display != dag.DisplayBlocked {
			continue
		}
		for _, depName := range waitingOn {
			dep := g.ByName[depName]
			if dep == nil {
				continue
			}
			depDisplay, _ := dag.Display(dep, g)
			if depDisplay != dag.DisplayUnhealthy && depDisplay != dag.DisplayFailed {
				continue
			}
			findings = append(findings, Finding{
				RuleID:   r.ID(),
				Service:  n.Name,
				Title:    "Blocked by unhealthy dependency",
				Detail:   fmt.Sprintf("%s is waiting on %s, which is %s.", n.Name, dep.Name, depDisplay),
				Severity: SeverityCritical,
				Evidence: []string{fmt.Sprintf("waiting_on=%s", dep.Name), fmt.Sprintf("%s display=%s", dep.Name, depDisplay)},
				Suggestion: []string{
					fmt.Sprintf("Fix %s first; dependent services cannot become ready until it satisfies depends_on.", dep.Name),
				},
			})
		}
	}
	return findings
}

type unhealthyServiceRule struct{}

func (unhealthyServiceRule) ID() string { return "unhealthy-service" }
func (r unhealthyServiceRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		display, _ := dag.Display(n, g)
		if display != dag.DisplayUnhealthy {
			continue
		}
		info := inspectNode(ctx, n)
		var evidence []string
		if info != nil && info.Health != nil && len(info.Health.Log) > 0 {
			output := info.Health.Log[len(info.Health.Log)-1].Output
			evidence = append(evidence, truncate(output, 300))
		}
		evidence = append(evidence, "display=unhealthy")
		findings = append(findings, Finding{
			RuleID:     r.ID(),
			Service:    n.Name,
			Title:      "Service healthcheck is failing",
			Detail:     fmt.Sprintf("%s is reporting an unhealthy status.", n.Name),
			Severity:   SeverityCritical,
			Evidence:   evidence,
			Suggestion: []string{"Inspect the healthcheck output and container logs for the failing probe."},
		})
	}
	return findings
}

type initFailedRule struct{}

func (initFailedRule) ID() string { return "init-failed" }
func (r initFailedRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		display, _ := dag.Display(n, g)
		if display != dag.DisplayFailed || len(n.Children) == 0 {
			continue
		}
		findings = append(findings, Finding{
			RuleID:     r.ID(),
			Service:    n.Name,
			Title:      "Init service failed",
			Detail:     fmt.Sprintf("%s failed before dependent services could start.", n.Name),
			Severity:   SeverityCritical,
			Evidence:   exitEvidence(n),
			Suggestion: []string{"Fix the init job or migration command, then recreate dependent services."},
		})
	}
	return findings
}

type failedServiceRule struct{}

func (failedServiceRule) ID() string { return "failed-service" }
func (r failedServiceRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		display, _ := dag.Display(n, g)
		if display != dag.DisplayFailed || len(n.Children) != 0 {
			continue
		}
		findings = append(findings, Finding{
			RuleID:     r.ID(),
			Service:    n.Name,
			Title:      "Service exited with failure",
			Detail:     fmt.Sprintf("%s exited unsuccessfully.", n.Name),
			Severity:   SeverityWarn,
			Evidence:   exitEvidence(n),
			Suggestion: []string{"Check the container logs for the failing command or process."},
		})
	}
	return findings
}

type exitedInitSuccessRule struct{}

func (exitedInitSuccessRule) ID() string { return "exited-init-success" }
func (r exitedInitSuccessRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		display, _ := dag.Display(n, g)
		if display != dag.DisplayCompleted || len(n.Children) == 0 {
			continue
		}
		findings = append(findings, Finding{
			RuleID:     r.ID(),
			Service:    n.Name,
			Title:      "Init service completed successfully",
			Detail:     fmt.Sprintf("%s completed and can unblock dependents using service_completed_successfully.", n.Name),
			Severity:   SeverityInfo,
			Evidence:   exitEvidence(n),
			Suggestion: []string{"This is expected for one-shot init or migration services."},
		})
	}
	return findings
}

type missingHealthcheckRule struct{}

func (missingHealthcheckRule) ID() string { return "missing-healthcheck" }
func (r missingHealthcheckRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		if len(n.Children) == 0 || hasHealthcheck(ctx, n) {
			continue
		}
		findings = append(findings, Finding{
			RuleID:   r.ID(),
			Service:  n.Name,
			Title:    "Dependency has no healthcheck",
			Detail:   fmt.Sprintf("%s has dependents but no healthcheck to signal readiness.", n.Name),
			Severity: SeverityWarn,
			Evidence: []string{fmt.Sprintf("dependents=%d", len(n.Children))},
			Suggestion: []string{
				"Add a healthcheck and use condition: service_healthy for services that depend on it.",
			},
		})
	}
	return findings
}

type shortDependsOnRule struct{}

func (shortDependsOnRule) ID() string { return "short-depends-on" }
func (r shortDependsOnRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		for _, depName := range n.Deps {
			condition := n.DepConditions[depName]
			if condition == "" {
				condition = "service_started"
			}
			if condition != "service_started" {
				continue
			}
			dep := g.ByName[depName]
			if dep == nil || !hasHealthcheck(ctx, dep) {
				continue
			}
			findings = append(findings, Finding{
				RuleID:   r.ID(),
				Service:  n.Name,
				Title:    "Dependency only waits for container start",
				Detail:   fmt.Sprintf("%s waits for %s to start, but %s has a healthcheck.", n.Name, depName, depName),
				Severity: SeverityWarn,
				Evidence: []string{fmt.Sprintf("%s depends_on %s condition=service_started", n.Name, depName)},
				Suggestion: []string{
					fmt.Sprintf("Change %s -> %s to condition: service_healthy.", n.Name, depName),
				},
			})
		}
	}
	return findings
}

type healthcheckCmdNotFoundRule struct{}

func (healthcheckCmdNotFoundRule) ID() string { return "healthcheck-cmd-not-found" }
func (r healthcheckCmdNotFoundRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		info := inspectNode(ctx, n)
		if info == nil || info.Health == nil {
			continue
		}
		for _, probe := range info.Health.Log {
			output := strings.ToLower(probe.Output)
			if !strings.Contains(output, "not found") && !strings.Contains(output, "executable file not found") {
				continue
			}
			findings = append(findings, Finding{
				RuleID:   r.ID(),
				Service:  n.Name,
				Title:    "Healthcheck command was not found",
				Detail:   fmt.Sprintf("%s healthcheck references a command missing from the image.", n.Name),
				Severity: SeverityCritical,
				Evidence: []string{truncate(probe.Output, 300)},
				Suggestion: []string{
					"Install the probe command in the image or change the healthcheck to use an available command.",
				},
			})
			break
		}
	}
	return findings
}

type localhostHostIPMisuseRule struct{}

func (localhostHostIPMisuseRule) ID() string { return "localhost-host-ip-misuse" }
func (r localhostHostIPMisuseRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		test := healthcheckTest(ctx, n)
		if len(test) == 0 {
			continue
		}
		for _, ip := range nonLoopbackIPv4s(strings.Join(test, " ")) {
			findings = append(findings, Finding{
				RuleID:   r.ID(),
				Service:  n.Name,
				Title:    "Healthcheck targets a host IP",
				Detail:   fmt.Sprintf("%s healthcheck uses non-loopback IPv4 %s.", n.Name, ip),
				Severity: SeverityWarn,
				Evidence: []string{strings.Join(test, " ")},
				Suggestion: []string{
					"Use localhost/127.0.0.1 for checks inside the same container, or a service DNS name for another container.",
				},
			})
			break
		}
	}
	return findings
}

type slowStartupRule struct{}

func (slowStartupRule) ID() string { return "slow-startup" }
func (r slowStartupRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		display, _ := dag.Display(n, g)
		if display != dag.DisplayHealthy {
			continue
		}
		info := inspectNode(ctx, n)
		if info == nil || info.StartedAt.IsZero() || info.Health == nil {
			continue
		}
		healthyAt, ok := firstHealthyProbeEnd(info.Health.Log)
		if !ok {
			continue
		}
		threshold := 30 * time.Second
		if info.Healthcheck != nil && info.Healthcheck.StartPeriod > 0 {
			threshold = info.Healthcheck.StartPeriod
		}
		elapsed := healthyAt.Sub(info.StartedAt)
		if elapsed <= threshold {
			continue
		}
		findings = append(findings, Finding{
			RuleID:   r.ID(),
			Service:  n.Name,
			Title:    "Service startup is slow",
			Detail:   fmt.Sprintf("%s took %s to become healthy.", n.Name, elapsed.Round(time.Second)),
			Severity: SeverityInfo,
			Evidence: []string{
				fmt.Sprintf("started_at=%s", info.StartedAt.Format(time.RFC3339)),
				fmt.Sprintf("healthy_at=%s", healthyAt.Format(time.RFC3339)),
				fmt.Sprintf("threshold=%s", threshold),
			},
			Suggestion: []string{"Consider increasing start_period or optimizing startup work before the healthcheck can pass."},
		})
	}
	return findings
}

type restartLoopRule struct{}

func (restartLoopRule) ID() string { return "restart-loop" }
func (r restartLoopRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		info := inspectNode(ctx, n)
		if info == nil || info.RestartCount < 3 || stablyRunning(n, g, info) {
			continue
		}
		findings = append(findings, Finding{
			RuleID:     r.ID(),
			Service:    n.Name,
			Title:      "Service is restarting repeatedly",
			Detail:     fmt.Sprintf("%s has restarted %d times and is not stable.", n.Name, info.RestartCount),
			Severity:   SeverityCritical,
			Evidence:   []string{fmt.Sprintf("restart_count=%d", info.RestartCount)},
			Suggestion: []string{"Inspect recent logs and the restart policy to identify the crash loop cause."},
		})
	}
	return findings
}

type oomKilledRule struct{}

func (oomKilledRule) ID() string { return "oom-killed" }
func (r oomKilledRule) Check(ctx Context) []Finding {
	g := graphFrom(ctx)
	if g == nil {
		return nil
	}
	var findings []Finding
	for _, n := range g.Ordered {
		info := inspectNode(ctx, n)
		if info == nil || !info.OOMKilled {
			continue
		}
		findings = append(findings, Finding{
			RuleID:   r.ID(),
			Service:  n.Name,
			Title:    "Service was killed by the OOM killer",
			Detail:   fmt.Sprintf("%s exceeded its memory limit or host memory pressure killed it.", n.Name),
			Severity: SeverityCritical,
			Evidence: []string{"oom_killed=true"},
			Suggestion: []string{
				"Increase the memory limit, reduce memory usage, or inspect host memory pressure during startup.",
			},
		})
	}
	return findings
}

func graphFrom(ctx Context) *dag.Graph {
	if ctx.Project == nil {
		return nil
	}
	return ctx.Project.Graph
}

func inspectNode(ctx Context, n *dag.Node) *docker.InspectInfo {
	if n == nil || n.ContainerID == "" || ctx.Inspect == nil {
		return nil
	}
	info, err := ctx.Inspect(n.ContainerID)
	if err != nil {
		return nil
	}
	return info
}

func hasHealthcheck(ctx Context, n *dag.Node) bool {
	if n == nil {
		return false
	}
	if ctx.Config != nil {
		svc, ok := ctx.Config.Services[n.Name]
		return ok && svc.Healthcheck != nil
	}
	info := inspectNode(ctx, n)
	return info != nil && info.Healthcheck != nil
}

func healthcheckTest(ctx Context, n *dag.Node) []string {
	if n == nil {
		return nil
	}
	if ctx.Config != nil {
		svc, ok := ctx.Config.Services[n.Name]
		if ok && svc.Healthcheck != nil {
			return svc.Healthcheck.Test
		}
		return nil
	}
	info := inspectNode(ctx, n)
	if info != nil && info.Healthcheck != nil {
		return info.Healthcheck.Test
	}
	return nil
}

func exitEvidence(n *dag.Node) []string {
	if n.ExitCode == nil {
		return []string{"exit_code=unknown"}
	}
	return []string{fmt.Sprintf("exit_code=%d", *n.ExitCode)}
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

var ipv4Re = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

func nonLoopbackIPv4s(s string) []string {
	matches := ipv4Re.FindAllString(s, -1)
	out := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
	for _, match := range matches {
		ip := net.ParseIP(match)
		if ip == nil || ip.To4() == nil || ip.IsLoopback() {
			continue
		}
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		out = append(out, match)
	}
	return out
}

func firstHealthyProbeEnd(log []docker.ProbeResult) (time.Time, bool) {
	for _, probe := range log {
		if probe.ExitCode == 0 && !probe.End.IsZero() {
			return probe.End, true
		}
	}
	return time.Time{}, false
}

func stablyRunning(n *dag.Node, g *dag.Graph, info *docker.InspectInfo) bool {
	display, _ := dag.Display(n, g)
	if display != dag.DisplayHealthy {
		return false
	}
	return info.Health == nil || info.Health.Status == "" || info.Health.Status == "healthy"
}
