package redact_test

import (
	"strings"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/redact"
)

func TestRedactRun(t *testing.T) {
	run := model.NewRun("r", time.Now().UTC())
	run.Invocation = &model.Invocation{
		EnvNames:  []string{"DB_PASSWORD", "PATH"},
		EnvValues: map[string]string{"DB_PASSWORD": "s3cr3t"},
	}
	run.ApplyEvent(model.Event{
		Timestamp: time.Now().UTC(),
		Source:    model.SourceLog,
		Service:   "api",
		Phase:     model.PhaseFailed,
		Type:      model.EventTypeLog,
		Severity:  model.SeverityError,
		Message:   "postgres://user:hunter2@db/app",
		Data: map[string]any{
			"token": "eyJhbGciOiJIUzI1NiJ9.aaa.bbb",
			"line":  "ok",
		},
	})
	redact.Run(run)
	if run.Invocation.EnvValues["DB_PASSWORD"] != "[REDACTED]" {
		t.Fatalf("env values should be redacted: %v", run.Invocation.EnvValues)
	}
	ev := run.Events[0]
	if strings.Contains(ev.Message, "hunter2") {
		t.Fatalf("password left in message: %s", ev.Message)
	}
	if ev.Data["token"] != "[REDACTED]" {
		t.Fatalf("token not redacted: %v", ev.Data["token"])
	}
}
