package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
	"github.com/hyturing/compose-pulse/internal/probe/chain"
	"github.com/hyturing/compose-pulse/internal/probe/tcp"
)

func cmdProbe(args []string) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Compose project name (optional)")
	tls := fs.Bool("tls", false, "Require TLS handshake")
	httpPath := fs.String("http", "", "If set, perform HTTP GET on this path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("usage: cpulse probe [--project NAME] [--tls] [--http PATH] <service> <host:port>")
	}
	service := fs.Arg(0)
	host, port, err := tcp.ParseHostPort(fs.Arg(1))
	if err != nil {
		return err
	}

	dc, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer func() { _ = dc.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	snap, err := discover.FromDocker(ctx, dc)
	if err != nil {
		return err
	}
	containerID, running, exists := findServiceContainer(snap, *project, service)

	native := func(ctx context.Context, cmd []string) (string, int, error) {
		if containerID == "" {
			return "", 1, fmt.Errorf("no container")
		}
		return dc.ExecCapture(ctx, containerID, cmd)
	}
	hasTool := func(ctx context.Context, bin string) bool {
		if !exists || !running {
			return false
		}
		out, code, err := dc.ExecCapture(ctx, containerID, []string{"sh", "-c", "command -v " + bin})
		return err == nil && code == 0 && strings.TrimSpace(out) != ""
	}

	r := &chain.Runner{
		NativeExec: native,
		HasTool:    hasTool,
	}

	res := r.Run(ctx, chain.Env{
		FromService:      service,
		ContainerID:      containerID,
		ContainerExists:  exists,
		ContainerRunning: running,
		SharedNetwork:    exists, // discovery does not yet expose networks; assume compose-shared when container exists
		TargetHost:       host,
		TargetPort:       port,
		WantTLS:          *tls,
		WantHTTP:         *httpPath != "",
		HTTPPath:         *httpPath,
	})
	_, _ = fmt.Fprint(os.Stdout, chain.FormatTable(res))
	if res.HardFailAt != "" {
		return exitCodeError{code: 1}
	}
	return nil
}

func findServiceContainer(snap *discover.Snapshot, project, service string) (id string, running, exists bool) {
	if snap == nil {
		return "", false, false
	}
	for _, p := range snap.Projects {
		if project != "" && p.Name != project {
			continue
		}
		if p.Graph == nil {
			continue
		}
		for _, n := range p.Graph.Ordered {
			if n.Name != service {
				continue
			}
			exists = n.ContainerID != ""
			running = n.State == docker.StateHealthy || n.State == docker.StateStarting || n.State == docker.StateUnhealthy
			return n.ContainerID, running, exists
		}
	}
	return "", false, false
}
