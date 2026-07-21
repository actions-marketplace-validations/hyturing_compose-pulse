package record

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hyturing/compose-pulse/internal/collector"
	"github.com/hyturing/compose-pulse/internal/compose/config"
	"github.com/hyturing/compose-pulse/internal/compose/invocation"
	"github.com/hyturing/compose-pulse/internal/compose/progress"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/normalizer"
	"github.com/hyturing/compose-pulse/internal/redact"
	jsonreport "github.com/hyturing/compose-pulse/internal/report/json"
	"github.com/hyturing/compose-pulse/internal/store/sqlite"
)

// Options configures a recording session.
type Options struct {
	Command        []string
	IncludeEnvVals bool
	OutputJSON     string
	DBPath         string
	ConfigRunner   config.Runner
	SkipDocker     bool // for tests / environments without a daemon
}

// Result is the outcome of a recording session.
type Result struct {
	Run      *model.Run
	ExitCode int
	JSONPath string
}

// Run executes the wrapped command while recording the Compose lifecycle.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if len(opts.Command) == 0 {
		return nil, fmt.Errorf("record: empty command")
	}
	now := time.Now().UTC()
	runID := fmt.Sprintf("run-%d", now.UnixNano())
	run := model.NewRun(runID, now)

	inv, err := invocation.Capture(invocation.Options{
		Command:        opts.Command,
		IncludeEnvVals: opts.IncludeEnvVals,
		Now:            now,
	})
	if err != nil {
		return nil, err
	}
	run.Invocation = inv
	if inv.ProjectName != "" {
		run.Project = inv.ProjectName
	}

	composeArgs := composeCLIArgs(opts.Command)
	if len(inv.ComposeFiles) > 0 || looksLikeCompose(opts.Command) {
		eff, err := config.Capture(inv.WorkingDir, inv.ComposeFiles, composeArgs, opts.ConfigRunner)
		if err != nil {
			run.ApplyEvent(model.Event{
				Timestamp: time.Now().UTC(),
				Source:    model.SourceCompose,
				Project:   run.Project,
				Phase:     model.PhaseFailed,
				Type:      model.EventTypeLifecycle,
				Severity:  model.SeverityError,
				Message:   err.Error(),
				Data:      map[string]any{"stage": "compose_config"},
			})
		} else {
			run.EffectiveConfig = eff
			for _, svc := range eff.Services {
				run.ApplyEvent(model.Event{
					Timestamp: time.Now().UTC(),
					Source:    model.SourceCompose,
					Project:   run.Project,
					Service:   svc.Name,
					Phase:     model.PhaseConfigured,
					Type:      model.EventTypeLifecycle,
					Severity:  model.SeverityInfo,
					Message:   "configured",
					Data:      map[string]any{"image": svc.Image},
				})
			}
		}
	}

	dbPath := opts.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(os.TempDir(), "cpulse-runs", runID+".db")
	}
	store, err := sqlite.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	// Redact before the first write so secrets never reach SQLite pages.
	redact.Run(run)
	if err := store.EnsureRun(run); err != nil {
		return nil, err
	}

	recCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		dc    *docker.Client
		rawCh chan collector.RawSignal
	)
	applySignal := func(sig collector.RawSignal) {
		events, err := normalizer.Normalize(run.ID, sig)
		if err != nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		for _, ev := range events {
			_ = store.WriteEvent(run, ev)
		}
	}

	if !opts.SkipDocker {
		if c, err := docker.NewClient(); err == nil {
			dc = c
			rawCh = make(chan collector.RawSignal, 128)
			_ = collector.NewEvents(dc).Start(recCtx, rawCh)
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-recCtx.Done():
						return
					case sig, ok := <-rawCh:
						if !ok {
							return
						}
						applySignal(sig)
					}
				}
			}()
		}
	}

	exitCode, progressLines, _ := runCommand(recCtx, opts.Command, inv.WorkingDir)
	progEvents := progress.ParseLines(progressLines, time.Now().UTC())
	mu.Lock()
	for _, ev := range progress.ToModelEvents(run.ID, run.Project, progEvents) {
		_ = store.WriteEvent(run, ev)
	}
	mu.Unlock()

	cancel()
	wg.Wait()
	if dc != nil {
		_ = dc.Close()
	}

	ended := time.Now().UTC()
	run.EndedAt = &ended
	if exitCode != 0 {
		hasFailure := false
		for _, ev := range run.Events {
			if ev.Phase == model.PhaseFailed || ev.Severity >= model.SeverityError {
				hasFailure = true
				break
			}
		}
		if !hasFailure {
			_ = store.WriteEvent(run, model.Event{
				Timestamp: ended,
				Source:    model.SourceCompose,
				Project:   run.Project,
				Phase:     model.PhaseFailed,
				Type:      model.EventTypeLifecycle,
				Severity:  model.SeverityError,
				Message:   fmt.Sprintf("command exited %d", exitCode),
				Data:      map[string]any{"exit_code": exitCode},
			})
		}
	}

	redact.Run(run)
	jsonPath := opts.OutputJSON
	if jsonPath == "" {
		jsonPath = filepath.Join(filepath.Dir(dbPath), runID+".json")
	}
	if err := jsonreport.Export(jsonPath, run); err != nil {
		return nil, err
	}
	run.Artifacts = append(run.Artifacts, jsonPath, dbPath)
	if err := store.EnsureRun(run); err != nil {
		return nil, err
	}

	result := &Result{Run: run, ExitCode: exitCode, JSONPath: jsonPath}
	// runCommand discards its own error, so a command killed by a deadline
	// (e.g. `test-startup --timeout`) would otherwise look like an ordinary
	// non-zero exit. Surface deadline-exceeded specifically; plain
	// cancellation (Ctrl+C) keeps its existing nil-error, exit-code contract.
	if ctxErr := ctx.Err(); errors.Is(ctxErr, context.DeadlineExceeded) {
		return result, ctxErr
	}
	return result, nil
}

func runCommand(ctx context.Context, command []string, cwd string) (exitCode int, progressLines []string, err error) {
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		return 1, nil, pipeErr
	}
	stderr, pipeErr := cmd.StderrPipe()
	if pipeErr != nil {
		return 1, nil, pipeErr
	}
	if err := cmd.Start(); err != nil {
		return 1, nil, err
	}
	var mu sync.Mutex
	collect := func(r io.Reader, w io.Writer) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			_, _ = fmt.Fprintln(w, line)
			mu.Lock()
			progressLines = append(progressLines, line)
			mu.Unlock()
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); collect(stdout, os.Stdout) }()
	go func() { defer wg.Done(); collect(stderr, os.Stderr) }()
	wg.Wait()
	err = cmd.Wait()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), progressLines, err
		}
		return 1, progressLines, err
	}
	return 0, progressLines, nil
}

func looksLikeCompose(cmd []string) bool {
	joined := strings.Join(cmd, " ")
	return strings.Contains(joined, "docker compose") || strings.Contains(joined, "docker-compose")
}

func composeCLIArgs(cmd []string) []string {
	idx := -1
	for i, a := range cmd {
		if a == "compose" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	rest := cmd[idx+1:]
	verbs := map[string]bool{
		"up": true, "down": true, "build": true, "pull": true, "run": true,
		"config": true, "ps": true, "logs": true, "start": true, "stop": true, "restart": true,
	}
	var out []string
	for i := 0; i < len(rest); i++ {
		if verbs[rest[i]] {
			break
		}
		out = append(out, rest[i])
		if (rest[i] == "-f" || rest[i] == "--file" || rest[i] == "--profile" || rest[i] == "-p" || rest[i] == "--project-name") && i+1 < len(rest) {
			out = append(out, rest[i+1])
			i++
		}
	}
	return out
}
