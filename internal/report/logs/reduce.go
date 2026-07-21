package logs

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/redact"
)

// Window is a reduced log excerpt for one service.
type Window struct {
	Service string   `json:"service"`
	Note    string   `json:"note,omitempty"`
	Lines   []string `json:"lines"`
	Count   int      `json:"repeat_count,omitempty"`
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key)\s*[:=]\s*\S+`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`),
	regexp.MustCompile(`(?i)postgres(?:ql)?://[^:\s]+:[^@\s]+@`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
}

// Reduce collapses repeated log signatures and returns windows + redaction notes.
func Reduce(run *model.Run) ([]Window, []string) {
	if run == nil {
		return nil, nil
	}
	bySvc := map[string][]string{}
	order := []string{}
	for _, ev := range run.Events {
		if ev.Type != model.EventTypeLog && ev.Source != model.SourceLog {
			continue
		}
		if ev.Service == "" {
			continue
		}
		if _, ok := bySvc[ev.Service]; !ok {
			order = append(order, ev.Service)
		}
		bySvc[ev.Service] = append(bySvc[ev.Service], ev.Message)
	}

	var windows []Window
	var redactions []string
	seenRedact := map[string]bool{}

	for _, svc := range order {
		lines := bySvc[svc]
		sigCount := map[string]int{}
		var first []string
		for _, line := range lines {
			sig := signature(line)
			sigCount[sig]++
			if sigCount[sig] == 1 {
				first = append(first, line)
			}
		}
		note := ""
		maxRepeat := 0
		for _, c := range sigCount {
			if c > maxRepeat {
				maxRepeat = c
			}
		}
		if maxRepeat > 1 {
			note = fmt.Sprintf("collapsed repeated log signatures (max repeats=%d); showing first occurrence per signature and last lines", maxRepeat)
		}
		last := lines
		if len(last) > 20 {
			last = last[len(last)-20:]
		}
		merged := append([]string{}, first...)
		if len(first) < len(lines) {
			merged = append(merged, "…", "— last lines —")
			merged = append(merged, last...)
		}
		outLines := make([]string, 0, len(merged))
		for _, line := range merged {
			clean, notes := redactLine(line)
			outLines = append(outLines, clean)
			for _, n := range notes {
				if !seenRedact[n] {
					seenRedact[n] = true
					redactions = append(redactions, n)
				}
			}
		}
		windows = append(windows, Window{
			Service: svc,
			Note:    note,
			Lines:   outLines,
			Count:   maxRepeat,
		})
	}

	if run.Invocation != nil {
		for _, name := range run.Invocation.EnvNames {
			if redact.IsSecretKey(name) && !seenRedact["env:"+name] {
				seenRedact["env:"+name] = true
				redactions = append(redactions, "env:"+name)
			}
		}
	}
	return windows, redactions
}

// ContainsSecrets reports whether text still has known secret values (not redacted).
func ContainsSecrets(text string) bool {
	for _, re := range secretPatterns {
		for _, m := range re.FindAllString(text, -1) {
			upper := strings.ToUpper(m)
			if strings.Contains(upper, "REDACTED") {
				continue
			}
			return true
		}
	}
	return false
}

func signature(line string) string {
	line = strings.TrimSpace(line)
	fields := strings.Fields(line)
	if len(fields) > 3 {
		return strings.Join(fields[len(fields)-3:], " ")
	}
	return line
}

func redactLine(line string) (string, []string) {
	var notes []string
	out := line
	for _, re := range secretPatterns {
		if re.MatchString(out) {
			notes = append(notes, "pattern:"+re.String())
			out = re.ReplaceAllString(out, "[REDACTED]")
		}
	}
	return out, notes
}
