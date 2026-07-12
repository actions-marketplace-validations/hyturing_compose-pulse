package docker

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

func TestExecCaptureReturnsCombinedOutputAndExitCode(t *testing.T) {
	api := &execMockAPI{
		attachReader: framedOutput(t, "ready\n", "warn\n"),
		inspect:      container.ExecInspect{ExitCode: 7},
	}
	client := &Client{api: api}

	output, exitCode, err := client.ExecCapture(context.Background(), "ctr", []string{"sh", "-c", "echo ready"})
	if err != nil {
		t.Fatal(err)
	}
	if output != "ready\nwarn\n" {
		t.Fatalf("output = %q, want combined stdout/stderr", output)
	}
	if exitCode != 7 {
		t.Fatalf("exitCode = %d, want 7", exitCode)
	}
	if api.containerID != "ctr" {
		t.Fatalf("containerID = %q, want ctr", api.containerID)
	}
	if strings.Join(api.cmd, " ") != "sh -c echo ready" {
		t.Fatalf("cmd = %v, want healthcheck command", api.cmd)
	}
}

func TestExecCaptureCapsOutput(t *testing.T) {
	api := &execMockAPI{
		attachReader: framedOutput(t, strings.Repeat("x", execCaptureMaxOutput+128), ""),
		inspect:      container.ExecInspect{ExitCode: 0},
	}
	client := &Client{api: api}

	output, _, err := client.ExecCapture(context.Background(), "ctr", []string{"spam"})
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != execCaptureMaxOutput {
		t.Fatalf("output length = %d, want %d", len(output), execCaptureMaxOutput)
	}
}

type execMockAPI struct {
	attachReader io.Reader
	inspect      container.ExecInspect
	containerID  string
	cmd          []string
}

func (m *execMockAPI) ContainerList(context.Context, container.ListOptions) ([]container.Summary, error) {
	return nil, errors.New("not implemented")
}

func (m *execMockAPI) ContainerLogs(context.Context, string, container.LogsOptions) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (m *execMockAPI) ContainerExecCreate(_ context.Context, containerID string, opts container.ExecOptions) (container.ExecCreateResponse, error) {
	m.containerID = containerID
	m.cmd = opts.Cmd
	return container.ExecCreateResponse{ID: "exec-1"}, nil
}

func (m *execMockAPI) ContainerExecAttach(context.Context, string, container.ExecAttachOptions) (types.HijackedResponse, error) {
	clientConn, serverConn := net.Pipe()
	_ = serverConn.Close()
	return types.HijackedResponse{
		Conn:   clientConn,
		Reader: bufio.NewReader(m.attachReader),
	}, nil
}

func (m *execMockAPI) ContainerExecInspect(context.Context, string) (container.ExecInspect, error) {
	return m.inspect, nil
}

func (m *execMockAPI) ContainerInspect(context.Context, string) (container.InspectResponse, error) {
	return container.InspectResponse{}, errors.New("not implemented")
}

func (m *execMockAPI) ContainerStatsOneShot(context.Context, string) (container.StatsResponseReader, error) {
	return container.StatsResponseReader{}, errors.New("not implemented")
}

func (m *execMockAPI) Close() error { return nil }

func framedOutput(t *testing.T, stdout, stderr string) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	if stdout != "" {
		w := stdcopy.NewStdWriter(&buf, stdcopy.Stdout)
		if _, err := w.Write([]byte(stdout)); err != nil {
			t.Fatal(err)
		}
	}
	if stderr != "" {
		w := stdcopy.NewStdWriter(&buf, stdcopy.Stderr)
		if _, err := w.Write([]byte(stderr)); err != nil {
			t.Fatal(err)
		}
	}
	return bytes.NewReader(buf.Bytes())
}
