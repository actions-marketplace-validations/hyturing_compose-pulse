package docker

import (
	"bytes"
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

const execCaptureMaxOutput = 64 * 1024

// ExecCapture runs cmd inside containerID and returns combined stdout/stderr plus the exit code.
func (c *Client) ExecCapture(ctx context.Context, containerID string, cmd []string) (output string, exitCode int, err error) {
	created, err := c.api.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return "", -1, err
	}

	resp, err := c.api.ContainerExecAttach(ctx, created.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", -1, err
	}
	defer resp.Close()

	var buf bytes.Buffer
	writer := &cappedWriter{w: &buf, max: execCaptureMaxOutput}
	if _, err := stdcopy.StdCopy(writer, writer, resp.Reader); err != nil {
		return buf.String(), -1, err
	}

	inspect, err := c.api.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return buf.String(), -1, err
	}
	return buf.String(), inspect.ExitCode, nil
}

type cappedWriter struct {
	w   *bytes.Buffer
	max int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	remaining := w.max - w.w.Len()
	if remaining > 0 {
		if len(p) < remaining {
			remaining = len(p)
		}
		_, _ = w.w.Write(p[:remaining])
	}
	return len(p), nil
}
