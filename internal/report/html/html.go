package html

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/report"
)

// Render returns a self-contained HTML report (no external assets).
func Render(r *report.Report) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8">`)
	b.WriteString(`<title>cpulse incident report</title>`)
	b.WriteString(`<style>
body{font-family:ui-sans-serif,system-ui,sans-serif;margin:2rem;max-width:900px;line-height:1.45;color:#122}
h1,h2{margin-top:1.6rem} code,pre{background:#f4f6f8;padding:.15rem .35rem;border-radius:4px}
pre{padding:1rem;overflow:auto} .muted{color:#556} .card{border:1px solid #d8dee6;border-radius:8px;padding:1rem;margin:1rem 0}
</style></head><body>`)
	fmt.Fprintf(&b, `<h1>cpulse incident report</h1>`)
	fmt.Fprintf(&b, `<p class="muted">Run <code>%s</code> · %s</p>`, html.EscapeString(r.RunID), html.EscapeString(r.GeneratedAt.Format(time.RFC3339)))
	fmt.Fprintf(&b, `<div class="card"><strong>Summary</strong><p>%s</p>`, html.EscapeString(r.Summary))
	if r.RootCause != "" {
		fmt.Fprintf(&b, `<p><strong>Root cause:</strong> %s<br><strong>Service:</strong> %s · <strong>Confidence:</strong> %s</p>`,
			html.EscapeString(r.RootCause), html.EscapeString(r.Service), html.EscapeString(r.Confidence))
	}
	b.WriteString(`</div>`)

	if len(r.BlockedServices) > 0 {
		b.WriteString(`<h2>Blocked services</h2><ul>`)
		for _, s := range r.BlockedServices {
			fmt.Fprintf(&b, `<li><code>%s</code></li>`, html.EscapeString(s))
		}
		b.WriteString(`</ul>`)
	}
	if len(r.CriticalPath) > 0 {
		b.WriteString(`<h2>Critical path</h2><ul>`)
		for _, seg := range r.CriticalPath {
			fmt.Fprintf(&b, `<li><code>%s</code> %s — %s</li>`, html.EscapeString(seg.Service), html.EscapeString(seg.Phase), html.EscapeString(seg.Duration))
		}
		b.WriteString(`</ul>`)
	}
	if len(r.SuggestedFixes) > 0 {
		b.WriteString(`<h2>Suggested fixes</h2><ul>`)
		for _, f := range r.SuggestedFixes {
			fmt.Fprintf(&b, `<li>%s</li>`, html.EscapeString(f))
		}
		b.WriteString(`</ul>`)
	}
	if r.Reproduction != "" {
		fmt.Fprintf(&b, `<h2>Reproduction</h2><pre>%s</pre>`, html.EscapeString(r.Reproduction))
	}
	for _, w := range r.LogWindows {
		fmt.Fprintf(&b, `<h2>%s logs</h2>`, html.EscapeString(w.Service))
		if w.Note != "" {
			fmt.Fprintf(&b, `<p class="muted">%s</p>`, html.EscapeString(w.Note))
		}
		b.WriteString(`<pre>`)
		for _, line := range w.Lines {
			b.WriteString(html.EscapeString(line))
			b.WriteByte('\n')
		}
		b.WriteString(`</pre>`)
	}
	if len(r.Redaction) > 0 {
		b.WriteString(`<h2>Redaction summary</h2><ul>`)
		for _, item := range r.Redaction {
			fmt.Fprintf(&b, `<li><code>%s</code></li>`, html.EscapeString(item))
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`</body></html>`)
	return b.String()
}
