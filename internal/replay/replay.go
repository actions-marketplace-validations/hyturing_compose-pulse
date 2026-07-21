package replay

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hyturing/compose-pulse/internal/model"
)

// LoadRunJSON reads a serialized run and rebuilds derived state by replaying events.
func LoadRunJSON(path string) (*model.Run, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("replay: read: %w", err)
	}
	var recorded model.Run
	if err := json.Unmarshal(raw, &recorded); err != nil {
		return nil, fmt.Errorf("replay: decode: %w", err)
	}
	if recorded.SchemaVersion != 0 && recorded.SchemaVersion != model.SchemaVersion && recorded.SchemaVersion != 1 {
		return nil, fmt.Errorf("replay: unsupported schema_version %d (supported: 1, %d)", recorded.SchemaVersion, model.SchemaVersion)
	}
	run := model.NewRun(recorded.ID, recorded.StartedAt)
	run.Project = recorded.Project
	run.EndedAt = recorded.EndedAt
	run.Invocation = recorded.Invocation
	run.EffectiveConfig = recorded.EffectiveConfig
	run.Findings = append([]model.Finding(nil), recorded.Findings...)
	run.ApplyEvents(recorded.Events)
	return run, nil
}
