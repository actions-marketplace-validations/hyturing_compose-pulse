package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/doctor"
)

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	projectFilter := fs.String("project", "", "Limit diagnosis to one compose project")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dc, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer func() { _ = dc.Close() }()

	snap, err := discover.FromDocker(context.Background(), dc)
	if err != nil {
		return fmt.Errorf("discovering containers: %w", err)
	}

	exitCritical := false
	matched := false
	for i := range snap.Projects {
		proj := &snap.Projects[i]
		if *projectFilter != "" && proj.Name != *projectFilter {
			continue
		}
		matched = true
		findings, root := diagnoseProject(dc, proj)
		if err := printDoctorReport(os.Stdout, proj.Name, root, findings); err != nil {
			return err
		}
		for _, f := range findings {
			if f.Severity == doctor.SeverityCritical {
				exitCritical = true
			}
		}
	}
	if *projectFilter != "" && !matched {
		return fmt.Errorf("project %q not found", *projectFilter)
	}
	if exitCritical {
		return errCriticalFindings
	}
	return nil
}

var errCriticalFindings = fmt.Errorf("critical findings")

func diagnoseProject(dc *docker.Client, proj *discover.Project) ([]doctor.Finding, *doctor.RootCause) {
	var cfg *compose.Config
	if len(proj.ConfigFiles) > 0 {
		if parsed, err := compose.Parse(proj.ConfigFiles[0]); err == nil {
			cfg = parsed
		}
	}
	ctx := doctor.Context{
		Project: proj,
		Config:  cfg,
		Inspect: func(id string) (*docker.InspectInfo, error) {
			return dc.Inspect(context.Background(), id)
		},
		Logs: func(id string, tail int) ([]string, error) {
			return dc.FetchLogLines(context.Background(), id, tail)
		},
		Now: time.Now(),
	}
	return doctor.Run(ctx, doctor.DefaultRules()), doctor.FindRootCause(ctx)
}

func printDoctorReport(w io.Writer, project string, root *doctor.RootCause, findings []doctor.Finding) error {
	if _, err := fmt.Fprintf(w, "Project: %s\n\n", project); err != nil {
		return err
	}
	if root != nil && len(root.Culprits) > 0 {
		if _, err := fmt.Fprintln(w, "Root cause:"); err != nil {
			return err
		}
		for _, c := range root.Culprits {
			line := "  ✕ " + c
			if msg, ok := root.FirstLog[c]; ok && msg != "" {
				line += " — " + truncateStr(msg, 120)
			}
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		if len(root.CriticalPath) > 0 {
			if _, err := fmt.Fprintf(w, "  Critical path: %s\n", strings.Join(root.CriticalPath, " → ")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, "Root cause: none detected"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "Findings:"); err != nil {
		return err
	}
	if len(findings) == 0 {
		_, err := fmt.Fprintln(w, "  (none)")
		return err
	}
	for _, f := range findings {
		if _, err := fmt.Fprintf(w, "  [%s] %s · %s — %s\n", severityLabel(f.Severity), f.RuleID, f.Service, f.Title); err != nil {
			return err
		}
		for _, e := range f.Evidence {
			if _, err := fmt.Fprintf(w, "      %s\n", truncateStr(e, 200)); err != nil {
				return err
			}
		}
		for _, s := range f.Suggestion {
			if _, err := fmt.Fprintf(w, "      → %s\n", s); err != nil {
				return err
			}
		}
	}
	return nil
}

func severityLabel(s doctor.Severity) string {
	switch s {
	case doctor.SeverityCritical:
		return "critical"
	case doctor.SeverityWarn:
		return "warn"
	default:
		return "info"
	}
}

func truncateStr(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
