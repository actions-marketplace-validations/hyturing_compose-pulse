package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/ui"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		var exitErr exitCodeError
		if errors.As(err, &exitErr) {
			if exitErr.msg != "" {
				fmt.Fprintf(os.Stderr, "cpulse: %v\n", err)
			}
			os.Exit(exitErr.code)
		}
		fmt.Fprintf(os.Stderr, "cpulse: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "doctor":
			return cmdDoctor(os.Args[2:])
		case "replay":
			return cmdReplay(os.Args[2:])
		case "record":
			return cmdRecord(os.Args[2:])
		case "up":
			return cmdUp(os.Args[2:])
		case "probe":
			return cmdProbe(os.Args[2:])
		case "compare":
			return cmdCompare(os.Args[2:])
		case "report":
			return cmdReport(os.Args[2:])
		case "test-startup":
			return cmdTestStartup(os.Args[2:])
		case "help", "-h", "--help":
			printUsage()
			return nil
		}
	}

	ver := flag.Bool("version", false, "Print version and exit")
	flag.Usage = printUsage
	flag.Parse()

	if *ver {
		fmt.Printf("cpulse %s\n", version)
		return nil
	}

	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("error connecting to Docker daemon: %w", err)
	}
	defer func() { _ = dockerClient.Close() }()

	snapshot, err := discover.FromDocker(context.Background(), dockerClient)
	if err != nil {
		return fmt.Errorf("error discovering containers: %w", err)
	}

	model := ui.New(snapshot, dockerClient)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `cpulse %s — Docker Compose startup debugger

Usage:
  cpulse              Launch the TUI dashboard
  cpulse doctor       Live diagnose (default) or headless over a recorded run
  cpulse record -- <cmd>
                      Record a Compose invocation (flight recorder)
  cpulse up [args]    Alias for: cpulse record -- docker compose up [args]
  cpulse replay FILE  Replay a recorded run.json through the run model
  cpulse probe <svc> <host:port>
                      Run dependency probe chain from a service's network
  cpulse compare --last successful
                      Compare current run critical path to baseline
  cpulse report --last --format <md|json|html|sarif>
                      Shareable incident report from last recorded run
  cpulse test-startup [--] [compose-up-args]
                      Headless record+diagnose of docker compose up --wait
  cpulse --version    Print version

Doctor flags:
  --project NAME      Limit to one compose project (live mode)
  --json / --sarif    Headless report formats over a recorded run
  --last / --run ID   Select run from SQLite
  --file PATH         Diagnose a run.json fixture
  --db FILE           SQLite path (default: .cpulse/cpulse.db)
  --fail-on LEVEL     high|medium|possible (default high)
  --annotate          Emit GitHub Actions annotations on stderr

Record flags:
  --output FILE       JSON export path
  --db FILE           SQLite path
  --include-env-values
                      Persist env values (secrets still redacted)

Probe flags:
  --project NAME      Limit to one compose project
  --tls               Require TLS handshake
  --http PATH         Also perform HTTP GET on PATH

Exit codes (doctor/test-startup CI paths):
  0 healthy · 1 confirmed failure · 2 timeout · 3 launch fail · 4 usage

`, version)
}
