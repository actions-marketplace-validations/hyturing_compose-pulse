package markdown

import (
	"fmt"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/report"
)

// Render returns a GitHub-friendly Markdown incident report.
func Render(r *report.Report) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# cpulse incident report\n\n")
	fmt.Fprintf(&b, "- **Run:** `%s`\n", r.RunID)
	if r.Project != "" {
		fmt.Fprintf(&b, "- **Project:** `%s`\n", r.Project)
	}
	fmt.Fprintf(&b, "- **Generated:** %s\n", r.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Summary:** %s\n\n", r.Summary)

	if r.RootCause != "" {
		fmt.Fprintf(&b, "## Root cause\n\n")
		fmt.Fprintf(&b, "%s\n\n", r.RootCause)
		if len(r.Findings) > 0 && r.Findings[0].RuleID != "" {
			fmt.Fprintf(&b, "- Rule: `%s`\n", r.Findings[0].RuleID)
		}
		fmt.Fprintf(&b, "- Service: `%s`\n", r.Service)
		fmt.Fprintf(&b, "- Confidence: **%s**\n\n", r.Confidence)
	}

	if len(r.BlockedServices) > 0 {
		fmt.Fprintf(&b, "## Blocked services\n\n")
		for _, s := range r.BlockedServices {
			fmt.Fprintf(&b, "- `%s`\n", s)
		}
		b.WriteByte('\n')
	}

	if len(r.CausalChain) > 0 {
		fmt.Fprintf(&b, "## Causal chain\n\n")
		fmt.Fprintf(&b, "%s\n\n", strings.Join(r.CausalChain, " → "))
	}

	if len(r.CriticalPath) > 0 {
		fmt.Fprintf(&b, "## Critical path\n\n")
		for _, seg := range r.CriticalPath {
			fmt.Fprintf(&b, "- `%s` %s — %s\n", seg.Service, seg.Phase, seg.Duration)
		}
		b.WriteByte('\n')
	}

	if len(r.SuggestedFixes) > 0 {
		fmt.Fprintf(&b, "## Suggested fixes\n\n")
		for _, f := range r.SuggestedFixes {
			fmt.Fprintf(&b, "- %s\n", f)
		}
		b.WriteByte('\n')
	}

	if r.Reproduction != "" {
		fmt.Fprintf(&b, "## Reproduction\n\n```bash\n%s\n```\n\n", r.Reproduction)
	}

	if len(r.LogWindows) > 0 {
		fmt.Fprintf(&b, "## Logs\n\n")
		for _, w := range r.LogWindows {
			fmt.Fprintf(&b, "<details>\n<summary>%s logs</summary>\n\n", w.Service)
			if w.Note != "" {
				fmt.Fprintf(&b, "_%s_\n\n", w.Note)
			}
			b.WriteString("```\n")
			for _, line := range w.Lines {
				b.WriteString(line)
				b.WriteByte('\n')
			}
			b.WriteString("```\n\n</details>\n\n")
		}
	}

	if len(r.Redaction) > 0 {
		fmt.Fprintf(&b, "## Redaction summary\n\n")
		fmt.Fprintf(&b, "The following secret-bearing fields/patterns were redacted (values not shown):\n\n")
		for _, item := range r.Redaction {
			fmt.Fprintf(&b, "- `%s`\n", item)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
