package doctor

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestRun_SortsBySeverityServiceThenRule(t *testing.T) {
	ctx := testContext(t, map[string]compose.Service{
		"api": {},
		"db":  {},
	})
	rules := []Rule{
		staticRule{id: "z-rule", findings: []Finding{{RuleID: "z-rule", Service: "db", Severity: SeverityWarn}}},
		staticRule{id: "a-rule", findings: []Finding{{RuleID: "a-rule", Service: "api", Severity: SeverityCritical}}},
		staticRule{id: "b-rule", findings: []Finding{{RuleID: "b-rule", Service: "api", Severity: SeverityCritical}}},
		staticRule{id: "info", findings: []Finding{{RuleID: "info", Service: "api", Severity: SeverityInfo}}},
	}

	got := Run(ctx, rules)

	var order []string
	for _, finding := range got {
		order = append(order, finding.RuleID+":"+finding.Service)
	}
	want := []string{"a-rule:api", "b-rule:api", "z-rule:db", "info:api"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestInterestingLogLines_ReturnsLatestInterestingLinesInOriginalOrder(t *testing.T) {
	lines := []string{
		"booting",
		"fatal: database missing",
		"still booting",
		"permission denied opening config",
		"ready",
		"panic: crashed",
	}

	got := InterestingLogLines(lines, 2)

	want := []string{"permission denied opening config", "panic: crashed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InterestingLogLines() = %v, want %v", got, want)
	}
}

func TestRules_FireAndStaySilent(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	longOutput := strings.Repeat("x", 350)

	tests := []struct {
		ruleID string
		fire   func(t *testing.T) Context
		silent func(t *testing.T) Context
		check  func(t *testing.T, findings []Finding)
	}{
		{
			ruleID: "missing-dependency",
			fire: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{
					"db-init": {},
					"django":  {DependsOn: compose.DependsOn{"db-init": {Condition: "service_completed_successfully"}}},
				})
				setNode(ctx, "django", "django1", docker.StateHealthy, nil)
				setNode(ctx, "db-init", "", docker.StatePending, nil)
				return ctx
			},
			silent: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{
					"db-init": {},
					"django":  {DependsOn: compose.DependsOn{"db-init": {Condition: "service_completed_successfully"}}},
				})
				setNode(ctx, "db-init", "init1", docker.StateExited, intPtr(0))
				setNode(ctx, "django", "django1", docker.StateHealthy, nil)
				return ctx
			},
		},
		{
			ruleID: "blocked-by-unhealthy",
			fire: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{
					"db":  {},
					"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
				})
				setNode(ctx, "db", "db1", docker.StateUnhealthy, nil)
				return ctx
			},
			silent: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{
					"db":  {},
					"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
				})
				setNode(ctx, "db", "db1", docker.StateStarting, nil)
				return ctx
			},
		},
		{
			ruleID: "unhealthy-service",
			fire: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"db": {}})
				setNode(ctx, "db", "db1", docker.StateUnhealthy, nil)
				ctx.Inspect = inspectMap(map[string]*docker.InspectInfo{
					"db1": {Health: &docker.HealthInfo{Log: []docker.ProbeResult{{Output: longOutput}}}},
				})
				return ctx
			},
			silent: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"db": {}})
				setNode(ctx, "db", "db1", docker.StateHealthy, nil)
				return ctx
			},
			check: func(t *testing.T, findings []Finding) {
				if len(findings) != 1 {
					t.Fatalf("findings = %v, want one", findings)
				}
				if len(findings[0].Evidence) == 0 || len(findings[0].Evidence[0]) != 300 {
					t.Fatalf("evidence length = %d, want 300", len(findings[0].Evidence[0]))
				}
			},
		},
		{
			ruleID: "init-failed",
			fire: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{
					"migrate": {},
					"api":     {DependsOn: compose.DependsOn{"migrate": {Condition: "service_completed_successfully"}}},
				})
				setNode(ctx, "migrate", "m1", docker.StateExited, intPtr(1))
				return ctx
			},
			silent: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"migrate": {}})
				setNode(ctx, "migrate", "m1", docker.StateExited, intPtr(1))
				return ctx
			},
		},
		{
			ruleID: "failed-service",
			fire: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"job": {}})
				setNode(ctx, "job", "j1", docker.StateExited, intPtr(1))
				return ctx
			},
			silent: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{
					"job": {},
					"api": {DependsOn: compose.DependsOn{"job": {Condition: "service_completed_successfully"}}},
				})
				setNode(ctx, "job", "j1", docker.StateExited, intPtr(1))
				return ctx
			},
		},
		{
			ruleID: "exited-init-success",
			fire: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{
					"migrate": {},
					"api":     {DependsOn: compose.DependsOn{"migrate": {Condition: "service_completed_successfully"}}},
				})
				setNode(ctx, "migrate", "m1", docker.StateExited, intPtr(0))
				return ctx
			},
			silent: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"migrate": {}})
				setNode(ctx, "migrate", "m1", docker.StateExited, intPtr(0))
				return ctx
			},
		},
		{
			ruleID: "missing-healthcheck",
			fire: func(t *testing.T) Context {
				return testContext(t, map[string]compose.Service{
					"db":  {},
					"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
				})
			},
			silent: func(t *testing.T) Context {
				return testContext(t, map[string]compose.Service{
					"db":  {Healthcheck: &compose.Healthcheck{Test: []string{"CMD", "pg_isready"}}},
					"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
				})
			},
		},
		{
			ruleID: "short-depends-on",
			fire: func(t *testing.T) Context {
				return testContext(t, map[string]compose.Service{
					"db":  {Healthcheck: &compose.Healthcheck{Test: []string{"CMD", "pg_isready"}}},
					"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_started"}}},
				})
			},
			silent: func(t *testing.T) Context {
				return testContext(t, map[string]compose.Service{
					"db":  {Healthcheck: &compose.Healthcheck{Test: []string{"CMD", "pg_isready"}}},
					"api": {DependsOn: compose.DependsOn{"db": {Condition: "service_healthy"}}},
				})
			},
		},
		{
			ruleID: "healthcheck-cmd-not-found",
			fire: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"db": {}})
				setNode(ctx, "db", "db1", docker.StateUnhealthy, nil)
				ctx.Inspect = inspectMap(map[string]*docker.InspectInfo{
					"db1": {Health: &docker.HealthInfo{Log: []docker.ProbeResult{{Output: "sh: pg_isready: not found"}}}},
				})
				return ctx
			},
			silent: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"db": {}})
				setNode(ctx, "db", "db1", docker.StateUnhealthy, nil)
				ctx.Inspect = inspectMap(map[string]*docker.InspectInfo{
					"db1": {Health: &docker.HealthInfo{Log: []docker.ProbeResult{{Output: "connection refused"}}}},
				})
				return ctx
			},
		},
		{
			ruleID: "localhost-host-ip-misuse",
			fire: func(t *testing.T) Context {
				return testContext(t, map[string]compose.Service{
					"api": {Healthcheck: &compose.Healthcheck{Test: []string{"CMD-SHELL", "curl http://192.168.65.2:8080/health"}}},
				})
			},
			silent: func(t *testing.T) Context {
				return testContext(t, map[string]compose.Service{
					"api": {Healthcheck: &compose.Healthcheck{Test: []string{"CMD-SHELL", "curl http://127.0.0.1:8080/health"}}},
				})
			},
		},
		{
			ruleID: "slow-startup",
			fire: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"api": {}})
				ctx.Now = now
				setNode(ctx, "api", "api1", docker.StateHealthy, nil)
				ctx.Inspect = inspectMap(map[string]*docker.InspectInfo{
					"api1": {
						StartedAt:   now.Add(-45 * time.Second),
						Healthcheck: &docker.HealthcheckSpec{},
						Health:      &docker.HealthInfo{Status: "healthy", Log: []docker.ProbeResult{{End: now}}},
					},
				})
				return ctx
			},
			silent: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"api": {}})
				ctx.Now = now
				setNode(ctx, "api", "api1", docker.StateHealthy, nil)
				ctx.Inspect = inspectMap(map[string]*docker.InspectInfo{
					"api1": {
						StartedAt:   now.Add(-10 * time.Second),
						Healthcheck: &docker.HealthcheckSpec{},
						Health:      &docker.HealthInfo{Status: "healthy", Log: []docker.ProbeResult{{End: now}}},
					},
				})
				return ctx
			},
		},
		{
			ruleID: "restart-loop",
			fire: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"api": {}})
				setNode(ctx, "api", "api1", docker.StateStarting, nil)
				ctx.Inspect = inspectMap(map[string]*docker.InspectInfo{"api1": {RestartCount: 3}})
				return ctx
			},
			silent: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"api": {}})
				setNode(ctx, "api", "api1", docker.StateHealthy, nil)
				ctx.Inspect = inspectMap(map[string]*docker.InspectInfo{"api1": {RestartCount: 3}})
				return ctx
			},
		},
		{
			ruleID: "oom-killed",
			fire: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"api": {}})
				setNode(ctx, "api", "api1", docker.StateExited, intPtr(137))
				ctx.Inspect = inspectMap(map[string]*docker.InspectInfo{"api1": {OOMKilled: true}})
				return ctx
			},
			silent: func(t *testing.T) Context {
				ctx := testContext(t, map[string]compose.Service{"api": {}})
				setNode(ctx, "api", "api1", docker.StateExited, intPtr(1))
				ctx.Inspect = inspectMap(map[string]*docker.InspectInfo{"api1": {OOMKilled: false}})
				return ctx
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.ruleID+"/fires", func(t *testing.T) {
			findings := ruleByID(t, tt.ruleID).Check(tt.fire(t))
			if len(findings) == 0 {
				t.Fatalf("expected %s to fire", tt.ruleID)
			}
			for _, finding := range findings {
				if finding.RuleID != tt.ruleID {
					t.Fatalf("RuleID = %q, want %q", finding.RuleID, tt.ruleID)
				}
			}
			if tt.check != nil {
				tt.check(t, findings)
			}
		})
		t.Run(tt.ruleID+"/silent", func(t *testing.T) {
			if findings := ruleByID(t, tt.ruleID).Check(tt.silent(t)); len(findings) != 0 {
				t.Fatalf("expected %s silent, got %v", tt.ruleID, findings)
			}
		})
	}
}

func TestRules_DefensiveNilAndFailingCallbacks(t *testing.T) {
	ctx := Context{
		Project: nil,
		Config:  nil,
		Inspect: func(string) (*docker.InspectInfo, error) {
			return nil, errors.New("inspect failed")
		},
		Logs: func(string, int) ([]string, error) {
			return nil, errors.New("logs failed")
		},
	}

	if findings := Run(ctx, DefaultRules()); len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

type staticRule struct {
	id       string
	findings []Finding
}

func (r staticRule) ID() string { return r.id }
func (r staticRule) Check(Context) []Finding {
	return append([]Finding(nil), r.findings...)
}

func testContext(t *testing.T, services map[string]compose.Service) Context {
	t.Helper()
	cfg := &compose.Config{Services: services}
	g, err := dag.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Ordered {
		n.ContainerID = n.Name + "1"
		n.State = docker.StateHealthy
	}
	return Context{
		Project: &discover.Project{
			Name:  "test",
			Graph: g,
		},
		Config: cfg,
		Inspect: func(string) (*docker.InspectInfo, error) {
			return nil, nil
		},
		Logs: func(string, int) ([]string, error) {
			return nil, nil
		},
		Now: time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC),
	}
}

func setNode(ctx Context, service, containerID string, state docker.ContainerState, exitCode *int) {
	n := ctx.Project.Graph.ByName[service]
	n.ContainerID = containerID
	n.State = state
	n.ExitCode = exitCode
}

func intPtr(i int) *int {
	return &i
}

func inspectMap(values map[string]*docker.InspectInfo) func(string) (*docker.InspectInfo, error) {
	return func(containerID string) (*docker.InspectInfo, error) {
		return values[containerID], nil
	}
}

func ruleByID(t *testing.T, id string) Rule {
	t.Helper()
	for _, rule := range DefaultRules() {
		if rule.ID() == id {
			return rule
		}
	}
	t.Fatalf("rule %q not found", id)
	return nil
}
