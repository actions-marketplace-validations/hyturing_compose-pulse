package sarif

import (
	"encoding/json"

	"github.com/hyturing/compose-pulse/internal/report"
)

// Document is a minimal SARIF 2.1.0 document for findings.
type Document struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []Run  `json:"runs"`
}

// Run is one SARIF run.
type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

// Tool describes cpulse.
type Tool struct {
	Driver Driver `json:"driver"`
}

// Driver is the tool driver metadata.
type Driver struct {
	Name           string `json:"name"`
	InformationURI string `json:"informationUri,omitempty"`
	Rules          []Rule `json:"rules,omitempty"`
}

// Rule is a SARIF reporting descriptor.
type Rule struct {
	ID               string  `json:"id"`
	ShortDescription Message `json:"shortDescription"`
}

// Result is one finding.
type Result struct {
	RuleID  string  `json:"ruleId"`
	Level   string  `json:"level"`
	Message Message `json:"message"`
}

// Message is SARIF text.
type Message struct {
	Text string `json:"text"`
}

// Render builds SARIF JSON for the report findings.
func Render(r *report.Report) ([]byte, error) {
	doc := Document{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []Run{{
			Tool: Tool{Driver: Driver{
				Name:           "cpulse",
				InformationURI: "https://github.com/hyturing/compose-pulse",
			}},
		}},
	}
	if r == nil {
		return json.MarshalIndent(doc, "", "  ")
	}
	seen := map[string]bool{}
	for _, f := range r.Findings {
		if !seen[f.RuleID] {
			seen[f.RuleID] = true
			doc.Runs[0].Tool.Driver.Rules = append(doc.Runs[0].Tool.Driver.Rules, Rule{
				ID:               f.RuleID,
				ShortDescription: Message{Text: f.RootCause},
			})
		}
		level := "warning"
		if f.Confidence.String() == "high" {
			level = "error"
		}
		text := f.RootCause
		if f.Service != "" {
			text = f.Service + ": " + text
		}
		doc.Runs[0].Results = append(doc.Runs[0].Results, Result{
			RuleID:  f.RuleID,
			Level:   level,
			Message: Message{Text: text},
		})
	}
	return json.MarshalIndent(doc, "", "  ")
}
