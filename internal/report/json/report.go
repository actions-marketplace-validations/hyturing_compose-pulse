package jsonreport

import (
	"encoding/json"

	"github.com/hyturing/compose-pulse/internal/report"
)

// MarshalReport encodes the Phase 5 report document (versioned schema).
func MarshalReport(r *report.Report) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
