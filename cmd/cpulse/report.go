package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/rules"
	"github.com/hyturing/compose-pulse/internal/report"
	"github.com/hyturing/compose-pulse/internal/report/html"
	jsonreport "github.com/hyturing/compose-pulse/internal/report/json"
	"github.com/hyturing/compose-pulse/internal/report/markdown"
	"github.com/hyturing/compose-pulse/internal/report/sarif"
	"github.com/hyturing/compose-pulse/internal/store/sqlite"
)

func cmdReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	last := fs.Bool("last", false, "Use the most recent recorded run")
	format := fs.String("format", "markdown", "Output format: markdown|json|html|sarif")
	dbPath := fs.String("db", "", "SQLite path (default: .cpulse/cpulse.db)")
	project := fs.String("project", "", "Compose project filter")
	outPath := fs.String("output", "", "Write to file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*last {
		return fmt.Errorf("usage: cpulse report --last --format <markdown|json|html|sarif> [--db FILE] [--output FILE]")
	}

	path := *dbPath
	if path == "" {
		path = ".cpulse/cpulse.db"
	}
	store, err := sqlite.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	id, err := store.LastRunID(*project)
	if err != nil {
		return err
	}
	run, err := store.LoadRun(id)
	if err != nil {
		return err
	}
	findings := engine.Diagnose(run, rules.DefaultRules())
	run.Findings = findings
	rep := report.Build(run, findings)

	var body []byte
	switch strings.ToLower(*format) {
	case "markdown", "md":
		body = []byte(markdown.Render(rep))
	case "json":
		body, err = jsonreport.MarshalReport(rep)
	case "html":
		body = []byte(html.Render(rep))
	case "sarif":
		body, err = sarif.Render(rep)
	default:
		return fmt.Errorf("unknown format %q", *format)
	}
	if err != nil {
		return err
	}
	if *outPath != "" {
		return os.WriteFile(*outPath, body, 0o644)
	}
	_, err = os.Stdout.Write(body)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		_, _ = os.Stdout.Write([]byte("\n"))
	}
	return err
}
