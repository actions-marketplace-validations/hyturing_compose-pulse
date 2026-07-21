package dns

import (
	"context"
	"fmt"
	"strings"
)

// Exec runs a command in the probe context.
type Exec func(ctx context.Context, cmd []string) (output string, exitCode int, err error)

// Resolve looks up host using getent/nslookup/ping-style fallbacks inside the container.
func Resolve(ctx context.Context, exec Exec, host string) (string, error) {
	if exec == nil {
		return "", fmt.Errorf("dns: nil exec")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("dns: empty host")
	}
	out, code, err := exec(ctx, []string{"sh", "-c", "getent hosts " + shellQuote(host) + " | awk '{print $1; exit}'"})
	if err == nil && code == 0 {
		ip := firstLine(out)
		if ip != "" {
			return ip, nil
		}
	}
	out, code, err = exec(ctx, []string{"sh", "-c", "nslookup " + shellQuote(host) + " 2>/dev/null | awk '/^Address: /{print $2; exit}'"})
	if err == nil && code == 0 {
		ip := firstLine(out)
		if ip != "" {
			return ip, nil
		}
	}
	return "", fmt.Errorf("dns resolution failed for %s", host)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
