package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hyturing/compose-pulse/internal/profiler/criticalpath"
	"github.com/hyturing/compose-pulse/internal/profiler/regression"
	"github.com/hyturing/compose-pulse/internal/store/sqlite"
)

func cmdCompare(args []string) error {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dbPath := fs.String("db", "", "SQLite path (default: .cpulse/cpulse.db)")
	project := fs.String("project", "", "Compose project filter")
	// String (not bool) so `--last successful --db FILE` keeps parsing flags after the value.
	last := fs.String("last", "", `Baseline selector (use "successful")`)
	if err := fs.Parse(args); err != nil {
		return err
	}
	mode := compareBaselineMode(*last, fs.Args())
	if mode != "successful" {
		return fmt.Errorf("usage: cpulse compare --last successful [--db FILE] [--project NAME]")
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

	curID, err := store.LastRunID(*project)
	if err != nil {
		return err
	}
	cur, err := store.LoadRun(curID)
	if err != nil {
		return err
	}
	pathRes := criticalpath.Compute(cur)
	var currentTotal time.Duration
	if pathRes != nil {
		currentTotal = pathRes.Total
	} else if cur.EndedAt != nil {
		currentTotal = cur.EndedAt.Sub(cur.StartedAt)
	}

	durs, err := store.SuccessfulDurations(*project, 20)
	if err != nil {
		return err
	}
	stats := regression.Stats(durs)
	baseline := stats.Median
	if baseline == 0 && stats.Last > 0 {
		baseline = stats.Last
	}

	res := regression.Compare(currentTotal, baseline, pathRes)
	_, _ = fmt.Fprintf(os.Stdout, "Current run: %s\n", cur.ID)
	_, _ = fmt.Fprintf(os.Stdout, "Stack critical path: %s\n", currentTotal)
	_, _ = fmt.Fprintf(os.Stdout, "Previous median:     %s (n=%d)\n", baseline, stats.Count)
	if res.IsRegression {
		_, _ = fmt.Fprintf(os.Stdout, "Regression:          %s\n", res.Explanation)
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "Regression:          none\n")
	}
	if pathRes != nil {
		_, _ = fmt.Fprintln(os.Stdout, "\nCRITICAL PATH")
		for _, seg := range pathRes.Segments {
			_, _ = fmt.Fprintf(os.Stdout, "%-20s %s  %s\n", seg.Service, seg.Phase, seg.Duration)
		}
	}
	return nil
}

func compareBaselineMode(lastFlag string, positional []string) string {
	if lastFlag != "" {
		return lastFlag
	}
	if len(positional) == 1 {
		return positional[0]
	}
	return ""
}
