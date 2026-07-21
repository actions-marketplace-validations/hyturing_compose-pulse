package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/hyturing/compose-pulse/internal/record"
)

func cmdRecord(args []string) error {
	fs := flag.NewFlagSet("record", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("output", "", "JSON export path (default: temp dir)")
	db := fs.String("db", "", "SQLite path (default: temp dir)")
	includeEnv := fs.Bool("include-env-values", false, "Persist env values (redacted secrets); default is names only")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cmd := fs.Args()
	// Support: cpulse record -- docker compose up
	if len(cmd) > 0 && cmd[0] == "--" {
		cmd = cmd[1:]
	}
	if len(cmd) == 0 {
		return fmt.Errorf("usage: cpulse record [--output FILE] [--db FILE] -- <command...>")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	res, err := record.Run(ctx, record.Options{
		Command:        cmd,
		IncludeEnvVals: *includeEnv,
		OutputJSON:     *out,
		DBPath:         *db,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "cpulse: recorded run %s → %s\n", res.Run.ID, res.JSONPath)
	if res.ExitCode != 0 {
		stop()
		return exitCodeError{code: res.ExitCode}
	}
	return nil
}

type exitCodeError struct {
	code int
	msg  string
}

func (e exitCodeError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("command exited %d", e.code)
}

func cmdUp(args []string) error {
	// cpulse up [compose up args...] → record -- docker compose up ...
	cmd := append([]string{"docker", "compose", "up"}, args...)
	return cmdRecord(append([]string{"--"}, cmd...))
}
