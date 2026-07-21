package sqlite

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/hyturing/compose-pulse/internal/model"
)

//go:embed schema.sql
var schemaSQL string

// Store persists runs and events in SQLite.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite database at path and applies the schema.
func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("sqlite: mkdir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// EnsureRun upserts the run header/JSON snapshot.
func (s *Store) EnsureRun(run *model.Run) error {
	if run == nil {
		return fmt.Errorf("sqlite: nil run")
	}
	if run.SchemaVersion == 0 {
		run.SchemaVersion = model.SchemaVersion
	}
	raw, err := json.Marshal(run)
	if err != nil {
		return err
	}
	var ended any
	if run.EndedAt != nil {
		ended = run.EndedAt.UTC().Format(time.RFC3339Nano)
	}
	_, err = s.db.Exec(`
INSERT INTO runs (id, schema_version, started_at, ended_at, project, run_json)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  schema_version=excluded.schema_version,
  ended_at=excluded.ended_at,
  project=excluded.project,
  run_json=excluded.run_json
`, run.ID, run.SchemaVersion, run.StartedAt.UTC().Format(time.RFC3339Nano), ended, run.Project, string(raw))
	if err != nil {
		return err
	}
	return s.syncDerivedTables(run)
}

func (s *Store) syncDerivedTables(run *model.Run) error {
	if _, err := s.db.Exec(`DELETE FROM services WHERE run_id = ?`, run.ID); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM service_phases WHERE run_id = ?`, run.ID); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM artifacts WHERE run_id = ?`, run.ID); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM diagnoses WHERE run_id = ?`, run.ID); err != nil {
		return err
	}
	for _, svc := range run.ServiceList() {
		key := svc.Project + "/" + svc.Name
		if svc.Project == "" {
			key = svc.Name
		}
		var exit any
		if svc.ExitCode != nil {
			exit = *svc.ExitCode
		}
		_, err := s.db.Exec(`
INSERT INTO services (run_id, service_key, name, project, container_id, phase, image, status, exit_code, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			run.ID, key, svc.Name, svc.Project, svc.ContainerID, svc.Phase.String(), svc.Image, svc.Status, exit,
			svc.UpdatedAt.UTC().Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		for _, tr := range svc.PhaseHistory {
			_, err := s.db.Exec(`
INSERT INTO service_phases (run_id, service_key, phase, ts, source, message)
VALUES (?, ?, ?, ?, ?, ?)`,
				run.ID, key, tr.Phase.String(), tr.Timestamp.UTC().Format(time.RFC3339Nano), tr.Source.String(), tr.Message)
			if err != nil {
				return err
			}
		}
	}
	for _, path := range run.Artifacts {
		if _, err := s.db.Exec(`INSERT INTO artifacts (run_id, path, kind) VALUES (?, ?, ?)`, run.ID, path, "file"); err != nil {
			return err
		}
	}
	for _, f := range run.Findings {
		raw, err := json.Marshal(f)
		if err != nil {
			return err
		}
		if _, err := s.db.Exec(`
INSERT INTO diagnoses (run_id, rule_id, service, root_cause, confidence, finding_json)
VALUES (?, ?, ?, ?, ?, ?)`, run.ID, f.RuleID, f.Service, f.RootCause, f.Confidence.String(), string(raw)); err != nil {
			return err
		}
	}
	return nil
}

// WriteLog appends a log line row.
func (s *Store) WriteLog(runID, containerID, service, line string, ts time.Time) error {
	_, err := s.db.Exec(`INSERT INTO logs (run_id, container_id, service, ts, line) VALUES (?, ?, ?, ?, ?)`,
		runID, containerID, service, ts.UTC().Format(time.RFC3339Nano), line)
	return err
}

// WriteResourceSample appends a resource sample row.
func (s *Store) WriteResourceSample(runID, containerID string, ts time.Time, cpu float64, memUsage, memLimit uint64) error {
	_, err := s.db.Exec(`
INSERT INTO resource_samples (run_id, container_id, ts, cpu_percent, mem_usage, mem_limit)
VALUES (?, ?, ?, ?, ?, ?)`, runID, containerID, ts.UTC().Format(time.RFC3339Nano), cpu, memUsage, memLimit)
	return err
}

// WriteProbeResult appends a probe result row.
func (s *Store) WriteProbeResult(runID, service, probeType, target, detail string, success bool, ts time.Time) error {
	suc := 0
	if success {
		suc = 1
	}
	_, err := s.db.Exec(`
INSERT INTO probe_results (run_id, service, probe_type, target, success, detail, ts)
VALUES (?, ?, ?, ?, ?, ?, ?)`, runID, service, probeType, target, suc, detail, ts.UTC().Format(time.RFC3339Nano))
	return err
}

// WriteEvent appends one event and refreshes the run snapshot.
func (s *Store) WriteEvent(run *model.Run, ev model.Event) error {
	if run == nil {
		return fmt.Errorf("sqlite: nil run")
	}
	run.ApplyEvent(ev)
	if err := s.EnsureRun(run); err != nil {
		return err
	}
	var data any
	if ev.Data != nil {
		raw, err := json.Marshal(ev.Data)
		if err != nil {
			return err
		}
		data = string(raw)
	}
	_, err := s.db.Exec(`
INSERT INTO events (run_id, ts, source, project, service, container_id, phase, type, severity, message, data_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, run.ID, ev.Timestamp.UTC().Format(time.RFC3339Nano), ev.Source.String(), ev.Project, ev.Service,
		ev.ContainerID, ev.Phase.String(), ev.Type.String(), ev.Severity.String(), ev.Message, data)
	return err
}

// ListProjectRuns returns run IDs for a project, newest first.
func (s *Store) ListProjectRuns(project string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
SELECT id FROM runs
WHERE (? = '' OR project = ?)
ORDER BY started_at DESC, rowid DESC
LIMIT ?`, project, project, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// LastRunID returns the most recent run id, optionally filtered by project.
func (s *Store) LastRunID(project string) (string, error) {
	ids, err := s.ListProjectRuns(project, 1)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("sqlite: no runs found")
	}
	return ids[0], nil
}

// SuccessfulDurations returns wall-clock durations for recent successful runs
// (EndedAt set and no service in PhaseFailed). Newest-last order for Stats.Last.
func (s *Store) SuccessfulDurations(project string, limit int) ([]time.Duration, error) {
	ids, err := s.ListProjectRuns(project, limit*3) // oversample; filter successes
	if err != nil {
		return nil, err
	}
	var out []time.Duration
	for i := len(ids) - 1; i >= 0; i-- { // oldest → newest
		run, err := s.LoadRun(ids[i])
		if err != nil || run == nil || run.EndedAt == nil {
			continue
		}
		failed := false
		for _, svc := range run.ServiceList() {
			if svc.Phase == model.PhaseFailed {
				failed = true
				break
			}
		}
		if failed {
			continue
		}
		out = append(out, run.EndedAt.Sub(run.StartedAt))
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// LoadRun loads a run by ID (from the run_json snapshot).
func (s *Store) LoadRun(runID string) (*model.Run, error) {
	var raw string
	err := s.db.QueryRow(`SELECT run_json FROM runs WHERE id = ?`, runID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sqlite: run %q not found", runID)
	}
	if err != nil {
		return nil, err
	}
	var run model.Run
	if err := json.Unmarshal([]byte(raw), &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// LoadRunFromEvents rebuilds derived state by replaying stored events in order.
func (s *Store) LoadRunFromEvents(runID string) (*model.Run, error) {
	var schemaVersion int
	var startedAt, project string
	err := s.db.QueryRow(`SELECT schema_version, started_at, project FROM runs WHERE id = ?`, runID).
		Scan(&schemaVersion, &startedAt, &project)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sqlite: run %q not found", runID)
	}
	if err != nil {
		return nil, err
	}
	started, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return nil, err
	}
	run := model.NewRun(runID, started)
	run.SchemaVersion = schemaVersion
	run.Project = project

	rows, err := s.db.Query(`
SELECT ts, source, project, service, container_id, phase, type, severity, message, data_json
FROM events WHERE run_id = ? ORDER BY id ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			ts, source, proj, service, cid, phase, typ, sev, msg string
			dataJSON                                             sql.NullString
		)
		if err := rows.Scan(&ts, &source, &proj, &service, &cid, &phase, &typ, &sev, &msg, &dataJSON); err != nil {
			return nil, err
		}
		ev, err := decodeEvent(runID, ts, source, proj, service, cid, phase, typ, sev, msg, dataJSON)
		if err != nil {
			return nil, err
		}
		run.ApplyEvent(ev)
	}
	return run, rows.Err()
}

func decodeEvent(runID, ts, source, proj, service, cid, phase, typ, sev, msg string, dataJSON sql.NullString) (model.Event, error) {
	var ev model.Event
	ev.RunID = runID
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ev, err
	}
	ev.Timestamp = t
	if err := json.Unmarshal([]byte(`"`+source+`"`), &ev.Source); err != nil {
		return ev, err
	}
	if err := json.Unmarshal([]byte(`"`+phase+`"`), &ev.Phase); err != nil {
		return ev, err
	}
	if err := json.Unmarshal([]byte(`"`+typ+`"`), &ev.Type); err != nil {
		return ev, err
	}
	if err := json.Unmarshal([]byte(`"`+sev+`"`), &ev.Severity); err != nil {
		return ev, err
	}
	ev.Project = proj
	ev.Service = service
	ev.ContainerID = cid
	ev.Message = msg
	if dataJSON.Valid && dataJSON.String != "" {
		if err := json.Unmarshal([]byte(dataJSON.String), &ev.Data); err != nil {
			return ev, err
		}
	}
	return ev, nil
}
