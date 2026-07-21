package tlscheck

import (
	"context"
	"fmt"
	"strings"
)

// Exec runs a command in the probe context.
type Exec func(ctx context.Context, cmd []string) (output string, exitCode int, err error)

// Handshake attempts a TLS handshake using openssl s_client when available.
func Handshake(ctx context.Context, exec Exec, host string, port int) error {
	if exec == nil {
		return fmt.Errorf("tls: nil exec")
	}
	cmd := fmt.Sprintf(
		"if command -v openssl >/dev/null 2>&1; then echo | openssl s_client -connect %s:%d -servername %s 2>/dev/null | grep -q 'Verify return code'; else exit 127; fi",
		host, port, shellQuote(host),
	)
	out, code, err := exec(ctx, []string{"sh", "-c", cmd})
	if code == 127 {
		return fmt.Errorf("openssl not available for TLS probe")
	}
	if err != nil || code != 0 {
		detail := strings.TrimSpace(out)
		if detail == "" && err != nil {
			detail = err.Error()
		}
		if detail == "" {
			detail = "tls handshake failed"
		}
		return fmt.Errorf("%s", detail)
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
