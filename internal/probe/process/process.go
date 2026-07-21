package process

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Exec runs a command in the probe context.
type Exec func(ctx context.Context, cmd []string) (output string, exitCode int, err error)

// Listening reports whether something appears to listen on the given TCP port
// inside the target network namespace (best-effort via ss/netstat/nc).
func Listening(ctx context.Context, exec Exec, port int) (bool, error) {
	if exec == nil {
		return false, fmt.Errorf("process: nil exec")
	}
	if port <= 0 {
		return false, fmt.Errorf("process: invalid port")
	}
	p := strconv.Itoa(port)
	cmd := fmt.Sprintf(
		`if command -v ss >/dev/null 2>&1; then ss -lnt | grep -q ':%s'; `+
			`elif command -v netstat >/dev/null 2>&1; then netstat -lnt | grep -q ':%s'; `+
			`else nc -z 127.0.0.1 %s; fi`,
		p, p, p,
	)
	_, code, err := exec(ctx, []string{"sh", "-c", cmd})
	if err != nil {
		return false, err
	}
	return code == 0, nil
}

// Trim for tests / callers that want clean details.
func Trim(s string) string { return strings.TrimSpace(s) }
