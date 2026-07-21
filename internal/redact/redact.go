package redact

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/hyturing/compose-pulse/internal/model"
)

var (
	secretKeyRe = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|auth|credential|bearer)`)
	jwtRe       = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
	pemRe       = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]+?-----END [A-Z ]*PRIVATE KEY-----`)
)

const redacted = "[REDACTED]"

// IsSecretKey reports whether an env/config key name looks secret-bearing.
func IsSecretKey(name string) bool {
	return secretKeyRe.MatchString(name)
}

// Run redacts secret values in place on a recorded run.
func Run(run *model.Run) {
	if run == nil {
		return
	}
	if run.Invocation != nil {
		for k, v := range run.Invocation.EnvValues {
			if secretKeyRe.MatchString(k) {
				run.Invocation.EnvValues[k] = redacted
			} else {
				run.Invocation.EnvValues[k] = redactString(v)
			}
		}
	}
	if run.EffectiveConfig != nil {
		// `docker compose config` expands environment values. Parsed fields retain
		// the useful structure without persisting that secret-bearing raw output.
		run.EffectiveConfig.RawYAML = ""
	}
	for i := range run.Events {
		run.Events[i].Message = redactString(run.Events[i].Message)
		run.Events[i].Data = redactData(run.Events[i].Data)
	}
	for _, svc := range run.Services {
		if svc == nil {
			continue
		}
		svc.Status = redactString(svc.Status)
		for i := range svc.PhaseHistory {
			svc.PhaseHistory[i].Message = redactString(svc.PhaseHistory[i].Message)
		}
	}
}

func redactData(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	out := make(map[string]any, len(data))
	for k, v := range data {
		if secretKeyRe.MatchString(k) {
			out[k] = redacted
			continue
		}
		switch t := v.(type) {
		case string:
			out[k] = redactString(t)
		case map[string]any:
			out[k] = redactData(t)
		default:
			out[k] = v
		}
	}
	return out
}

func redactString(s string) string {
	if s == "" {
		return s
	}
	if i := strings.IndexByte(s, '='); i > 0 && secretKeyRe.MatchString(s[:i]) {
		return s[:i+1] + redacted
	}
	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.User != nil {
		user := u.User.Username()
		u.User = url.UserPassword(user, "REDACTED")
		return u.String()
	}
	s = jwtRe.ReplaceAllString(s, redacted)
	s = pemRe.ReplaceAllString(s, redacted)
	return s
}
