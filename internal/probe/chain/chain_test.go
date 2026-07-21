package chain_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/probe/chain"
)

type script map[string]struct {
	out  string
	code int
	err  error
}

func (s script) exec(ctx context.Context, cmd []string) (string, int, error) {
	key := strings.Join(cmd, " ")
	for pattern, res := range s {
		if strings.Contains(key, pattern) {
			return res.out, res.code, res.err
		}
	}
	return "", 1, nil
}

func TestRun_TCPFailRecordsListeningAndStops(t *testing.T) {
	r := &chain.Runner{
		NativeExec: script{
			"getent hosts": {out: "172.24.0.3\n", code: 0},
			"nc -z":        {code: 1, out: "connection refused"},
			"ss -lnt":      {code: 1},
			"netstat":      {code: 1},
		}.exec,
		HasTool: func(context.Context, string) bool { return true },
		Now:     func() time.Time { return time.Unix(0, 0).UTC() },
	}
	res := r.Run(context.Background(), chain.Env{
		FromService:      "api",
		ContainerID:      "capi",
		ContainerExists:  true,
		ContainerRunning: true,
		SharedNetwork:    true,
		TargetHost:       "postgres",
		TargetPort:       5432,
	})
	if res.Method != chain.MethodNativeExec {
		t.Fatalf("method = %s", res.Method)
	}
	if res.HardFailAt != "tcp" {
		t.Fatalf("HardFailAt = %q, want tcp", res.HardFailAt)
	}
	assertStep(t, res, "dns", chain.StatusPass)
	assertStep(t, res, "tcp", chain.StatusFail)
	assertStep(t, res, "process_listening", chain.StatusFail)
	for _, s := range res.Steps {
		if s.Name == "application_ready" {
			t.Fatal("should short-circuit before application_ready")
		}
	}
}

func TestRun_FallsBackToEphemeralWhenNoShell(t *testing.T) {
	r := &chain.Runner{
		NativeExec: script{}.exec,
		HasTool:    func(context.Context, string) bool { return false },
		EphemeralExec: script{
			"getent hosts": {out: "10.0.0.2\n", code: 0},
			"nc -z":        {code: 0},
			"ss -lnt":      {code: 0},
		}.exec,
	}
	res := r.Run(context.Background(), chain.Env{
		FromService:      "api",
		ContainerExists:  true,
		ContainerRunning: true,
		SharedNetwork:    true,
		TargetHost:       "db",
		TargetPort:       5432,
	})
	if res.Method != chain.MethodEphemeral {
		t.Fatalf("method = %s, want ephemeral", res.Method)
	}
	if res.HardFailAt != "" {
		t.Fatalf("unexpected hard fail %q", res.HardFailAt)
	}
	assertStep(t, res, "probe_context", chain.StatusPass)
	assertStep(t, res, "tcp", chain.StatusPass)
	assertStep(t, res, "application_ready", chain.StatusPass)
}

func TestToEvents_IncludesMethodAndSteps(t *testing.T) {
	res := &chain.Result{
		FromService: "api",
		TargetHost:  "postgres",
		TargetPort:  5432,
		Method:      chain.MethodNativeExec,
		HardFailAt:  "tcp",
		Steps: []chain.Step{
			{Name: "tcp", Status: chain.StatusFail, Detail: "connection refused"},
		},
	}
	evs := chain.ToEvents("run1", "demo", res, time.Unix(1, 0).UTC())
	if len(evs) < 2 {
		t.Fatalf("events = %d", len(evs))
	}
	if evs[0].Data["probe_method"] != "native_exec" {
		t.Fatalf("method data = %#v", evs[0].Data)
	}
}

func assertStep(t *testing.T, res *chain.Result, name string, want chain.Status) {
	t.Helper()
	for _, s := range res.Steps {
		if s.Name == name {
			if s.Status != want {
				t.Fatalf("step %s status = %s, want %s", name, s.Status, want)
			}
			return
		}
	}
	t.Fatalf("missing step %s in %#v", name, res.Steps)
}

func TestRun_RemoteTCPOkSkipsLocalListeningCheck(t *testing.T) {
	r := &chain.Runner{
		NativeExec: script{
			"getent hosts": {out: "172.24.0.3\n", code: 0},
			"nc -z":        {code: 0},
			"ss -lnt":      {code: 1},
			"netstat":      {code: 1},
		}.exec,
		HasTool: func(context.Context, string) bool { return true },
		Now:     func() time.Time { return time.Unix(0, 0).UTC() },
	}
	res := r.Run(context.Background(), chain.Env{
		FromService:      "client",
		ContainerID:      "cclient",
		ContainerExists:  true,
		ContainerRunning: true,
		SharedNetwork:    true,
		TargetHost:       "web",
		TargetPort:       80,
	})
	if res.HardFailAt != "" {
		t.Fatalf("HardFailAt = %q, want none", res.HardFailAt)
	}
	assertStep(t, res, "tcp", chain.StatusPass)
	assertStep(t, res, "process_listening", chain.StatusSkip)
	assertStep(t, res, "application_ready", chain.StatusPass)
}
