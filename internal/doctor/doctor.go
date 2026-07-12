package doctor

import (
	"regexp"
	"sort"
	"time"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/discover"
	"github.com/hyturing/compose-pulse/internal/docker"
)

// Severity is the urgency level of a doctor finding.
type Severity int

// Finding severity levels, ordered from least to most urgent.
const (
	SeverityInfo Severity = iota
	SeverityWarn
	SeverityCritical
)

// Finding is one actionable diagnosis from a doctor rule.
type Finding struct {
	RuleID, Service, Title, Detail string
	Severity                       Severity
	Evidence, Suggestion           []string
}

// Context provides the static and runtime inputs used by doctor rules.
type Context struct {
	Project *discover.Project
	Config  *compose.Config // MAY BE NIL
	Inspect func(containerID string) (*docker.InspectInfo, error)
	Logs    func(containerID string, tail int) ([]string, error)
	Now     time.Time
}

// Rule checks the doctor context and returns zero or more findings.
type Rule interface {
	ID() string
	Check(ctx Context) []Finding
}

// Run executes rules and returns findings in deterministic priority order.
func Run(ctx Context, rules []Rule) []Finding {
	var findings []Finding
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		findings = append(findings, rule.Check(ctx)...)
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity > findings[j].Severity
		}
		if findings[i].Service != findings[j].Service {
			return findings[i].Service < findings[j].Service
		}
		return findings[i].RuleID < findings[j].RuleID
	})
	return findings
}

var errLineRe = regexp.MustCompile(`(?i)\b(fatal|error|panic|refused|denied|failed|cannot|unable|exception|traceback)\b`)

// InterestingLogLines scans newest-to-oldest for error-like lines and returns
// up to max matches in original chronological order.
func InterestingLogLines(lines []string, max int) []string {
	if max <= 0 {
		return nil
	}
	out := make([]string, 0, max)
	for i := len(lines) - 1; i >= 0 && len(out) < max; i-- {
		if errLineRe.MatchString(lines[i]) {
			out = append(out, lines[i])
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
