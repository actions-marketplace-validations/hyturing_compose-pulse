package httpp

import (
	"context"
	"fmt"
	"strings"
)

// Exec runs a command in the probe context.
type Exec func(ctx context.Context, cmd []string) (output string, exitCode int, err error)

// Get performs an HTTP GET using curl or wget inside the probe context.
func Get(ctx context.Context, exec Exec, host string, port int, path string, tls bool) error {
	if exec == nil {
		return fmt.Errorf("http: nil exec")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	scheme := "http"
	if tls {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d%s", scheme, host, port, path)
	cmd := fmt.Sprintf(
		`if command -v curl >/dev/null 2>&1; then curl -fsS -o /dev/null -w '%%{http_code}' --max-time 3 %s; `+
			`elif command -v wget >/dev/null 2>&1; then wget -q -O /dev/null %s && echo 200; else exit 127; fi`,
		shellQuote(url), shellQuote(url),
	)
	out, code, err := exec(ctx, []string{"sh", "-c", cmd})
	if code == 127 {
		return fmt.Errorf("curl/wget not available for HTTP probe")
	}
	if err != nil || code != 0 {
		detail := strings.TrimSpace(out)
		if detail == "" && err != nil {
			detail = err.Error()
		}
		if detail == "" {
			detail = "http request failed"
		}
		return fmt.Errorf("%s", detail)
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
