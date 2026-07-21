package tcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Exec runs a command in the probe context.
type Exec func(ctx context.Context, cmd []string) (output string, exitCode int, err error)

// Connect checks TCP reachability to host:port via nc or /dev/tcp.
func Connect(ctx context.Context, exec Exec, host string, port int) error {
	if exec == nil {
		return fmt.Errorf("tcp: nil exec")
	}
	if port <= 0 {
		return fmt.Errorf("tcp: invalid port %d", port)
	}
	cmd := fmt.Sprintf(
		"if command -v nc >/dev/null 2>&1; then nc -z -w 2 %s %d; else timeout 2 bash -c 'echo > /dev/tcp/%s/%d'; fi",
		shellQuote(host), port, host, port,
	)
	out, code, err := exec(ctx, []string{"sh", "-c", cmd})
	if err != nil {
		return fmt.Errorf("tcp connect %s:%d: %w", host, port, err)
	}
	if code != 0 {
		detail := strings.TrimSpace(out)
		if detail == "" {
			detail = "connection refused"
		}
		return fmt.Errorf("%s", detail)
	}
	return nil
}

// ParseHostPort splits "host:port" (IPv6 bracket form supported loosely).
func ParseHostPort(target string) (host string, port int, err error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", 0, fmt.Errorf("empty target")
	}
	host, portStr, ok := strings.Cut(target, ":")
	if !ok || host == "" || portStr == "" {
		return "", 0, fmt.Errorf("want host:port, got %q", target)
	}
	port, err = strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return "", 0, fmt.Errorf("invalid port in %q", target)
	}
	return host, port, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
