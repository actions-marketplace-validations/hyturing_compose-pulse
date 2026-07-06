package docker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// LogLineMsg carries one line from a streaming log follow, or anaconda terminal error.
type LogLineMsg struct {
	Line string
	Err  error
}

// StartLogStreamCh launches a goroutine that tails container logs with Follow enabled.
// The channel closes when ctx is cancelled, the stream ends, or an error occurs.
func (c *Client) StartLogStreamCh(ctx context.Context, containerID string, tail int) <-chan LogLineMsg {
	ch := make(chan LogLineMsg, 64)
	go func() {
		defer close(ch)
		tailStr := "200"
		if tail > 0 {
			tailStr = fmt.Sprintf("%d", tail)
		}
		opts := container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Tail:       tailStr,
		}
		reader, err := c.api.ContainerLogs(ctx, containerID, opts)
		if err != nil {
			select {
			case ch <- LogLineMsg{Err: err}:
			case <-ctx.Done():
			}
			return
		}
		defer func() { _ = reader.Close() }()

		pr, pw := io.Pipe()
		go func() {
			_, _ = stdcopy.StdCopy(pw, pw, reader)
			_ = pw.Close()
		}()

		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case ch <- LogLineMsg{Line: scanner.Text()}:
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			select {
			case ch <- LogLineMsg{Err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return ch
}

// Logs returns the last n lines of stdout+stderr for containerID.
func (c *Client) Logs(ctx context.Context, containerID string, lines int) (string, error) {
	tail := "200"
	if lines > 0 {
		tail = fmt.Sprintf("%d", lines)
	}
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	}
	reader, err := c.api.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()

	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, reader); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// FetchLogLines returns the last tail lines of stdout+stderr as a slice (oldest first).
func (c *Client) FetchLogLines(ctx context.Context, containerID string, tail int) ([]string, error) {
	raw, err := c.Logs(ctx, containerID, tail)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}
