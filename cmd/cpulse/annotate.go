package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/hyturing/compose-pulse/internal/model"
)

// writeGitHubAnnotations emits Actions workflow commands for findings.
func writeGitHubAnnotations(w io.Writer, findings []model.Finding) {
	for _, f := range findings {
		level := "warning"
		if f.Confidence == model.ConfidenceHigh {
			level = "error"
		}
		title := f.RuleID
		if title == "" {
			title = "cpulse"
		}
		msg := f.RootCause
		if f.Service != "" {
			msg = f.Service + ": " + msg
		}
		msg = strings.ReplaceAll(msg, "\n", " ")
		msg = strings.ReplaceAll(msg, "%", "%25")
		msg = strings.ReplaceAll(msg, "\r", "")
		_, _ = fmt.Fprintf(w, "::%s title=%s::%s\n", level, escapeProp(title), msg)
	}
}

func escapeProp(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}
