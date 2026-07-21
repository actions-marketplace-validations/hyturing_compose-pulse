package jsonreport

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/redact"
)

// Export writes a stable JSON document for a run (shared with Phase 5 report --format json).
func Export(path string, run *model.Run) error {
	if run == nil {
		return fmt.Errorf("json export: nil run")
	}
	cp := run.Clone()
	redact.Run(cp)
	raw, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}

// Marshal returns redacted JSON bytes for a run.
func Marshal(run *model.Run) ([]byte, error) {
	if run == nil {
		return nil, fmt.Errorf("json export: nil run")
	}
	cp := run.Clone()
	redact.Run(cp)
	return json.MarshalIndent(cp, "", "  ")
}
