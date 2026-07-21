package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/rules"
	"github.com/hyturing/compose-pulse/internal/record"
)

func cmdTestStartup(args []string) error {
	fs := flag.NewFlagSet("test-startup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dbPath := fs.String("db", "", "SQLite path (default: temp via record)")
	outJSON := fs.String("output", "", "JSON export path for the recorded run")
	failOnStr := fs.String("fail-on", "high", "Fail when findings meet confidence: high|medium|possible")
	timeout := fs.Duration("timeout", 10*time.Minute, "Overall timeout for compose up")
	project := fs.String("project", "", "Compose project name passed to docker compose (-p)")
	if err := fs.Parse(args); err != nil {
		return usageError(err.Error())
	}
	failOn, err := parseFailOn(*failOnStr)
	if err != nil {
		return usageError(err.Error())
	}

	composeArgs := fs.Args()
	if len(composeArgs) > 0 && composeArgs[0] == "--" {
		composeArgs = composeArgs[1:]
	}
	// Default: docker compose up --wait --abort-on-container-exit is too harsh;
	// use --wait so readiness is the bar.
	cmd := []string{"docker", "compose"}
	if *project != "" {
		cmd = append(cmd, "-p", *project)
	}
	cmd = append(cmd, "up", "--wait")
	cmd = append(cmd, composeArgs...)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	res, err := record.Run(ctx, record.Options{
		Command:    cmd,
		OutputJSON: *outJSON,
		DBPath:     *dbPath,
	})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return exitCodeError{code: exitTimeout, msg: "test-startup timed out"}
		}
		return exitCodeError{code: exitLaunchFail, msg: err.Error()}
	}
	if res == nil || res.Run == nil {
		return exitCodeError{code: exitLaunchFail, msg: "compose up produced no run"}
	}
	if res.ExitCode != 0 {
		// Still diagnose the recorded failure; prefer diagnosis exit over raw compose code.
		findings := engine.Diagnose(res.Run, rules.DefaultRules())
		if code := classifyRecordedFindings(findings, failOn); code != exitOK {
			_, _ = fmt.Fprintf(os.Stderr, "cpulse: test-startup failed with %d finding(s)\n", len(findings))
			return exitCodeError{code: code, msg: "startup failure diagnosed"}
		}
		return exitCodeError{code: exitLaunchFail, msg: fmt.Sprintf("compose exited %d", res.ExitCode)}
	}

	findings := engine.Diagnose(res.Run, rules.DefaultRules())
	if code := classifyRecordedFindings(findings, failOn); code != exitOK {
		_, _ = fmt.Fprintf(os.Stderr, "cpulse: test-startup unhealthy: %d finding(s)\n", len(findings))
		return exitCodeError{code: code, msg: "confirmed failure(s) at/above threshold"}
	}
	_, _ = fmt.Fprintln(os.Stderr, "cpulse: test-startup ok")
	return nil
}
