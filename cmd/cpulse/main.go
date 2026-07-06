package main

import (
	"context"
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
		fmt.Fprintf(os.Stderr, "cpulse: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ver := flag.Bool("version", false, "Print version and exit")
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
