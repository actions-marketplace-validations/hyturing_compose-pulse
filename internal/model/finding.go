package model

import (
	"encoding/json"
	"fmt"
)

// Confidence is how strongly a finding is proven by evidence.
type Confidence int

// Finding confidence levels.
const (
	ConfidencePossible Confidence = iota
	ConfidenceMedium
	ConfidenceHigh
)

var confidenceNames = [...]string{
	ConfidencePossible: "possible",
	ConfidenceMedium:   "medium",
	ConfidenceHigh:     "high",
}

func (c Confidence) String() string {
	if int(c) >= 0 && int(c) < len(confidenceNames) {
		return confidenceNames[c]
	}
	return "unknown"
}

// MarshalJSON encodes confidence as its string name.
func (c Confidence) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// UnmarshalJSON decodes confidence from its string name.
func (c *Confidence) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	for i, name := range confidenceNames {
		if name == v {
			*c = Confidence(i)
			return nil
		}
	}
	return fmt.Errorf("unknown confidence %q", v)
}

// Finding is a structured diagnosis produced by the (later) diagnosis engine.
// Fields align with Phase 2; Phase 0 only defines the schema.
type Finding struct {
	RuleID          string     `json:"rule_id"`
	Service         string     `json:"service,omitempty"`
	RootCause       string     `json:"root_cause,omitempty"`
	Evidence        []string   `json:"evidence,omitempty"`
	Confidence      Confidence `json:"confidence"`
	BlockedServices []string   `json:"blocked_services,omitempty"`
	SuggestedFixes  []string   `json:"suggested_fixes,omitempty"`
}
