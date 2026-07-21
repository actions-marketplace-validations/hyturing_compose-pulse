package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/hyturing/compose-pulse/internal/replay"
)

func cmdReplay(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: cpulse replay <run.json>")
	}
	path := fs.Arg(0)
	run, err := replay.LoadRunJSON(path)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(run)
}
