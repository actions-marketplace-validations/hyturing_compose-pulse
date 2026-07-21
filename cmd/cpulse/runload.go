package main

import (
	"fmt"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/replay"
	"github.com/hyturing/compose-pulse/internal/store/sqlite"
)

func defaultDBPath(db string) string {
	if db == "" {
		return ".cpulse/cpulse.db"
	}
	return db
}

// loadRecordedRun loads a run from --file JSON, or from SQLite via --run / --last.
func loadRecordedRun(dbPath, project, runID, file string, last bool) (*model.Run, error) {
	if file != "" {
		return replay.LoadRunJSON(file)
	}
	if !last && runID == "" {
		return nil, fmt.Errorf("need --last, --run ID, or --file PATH")
	}
	store, err := sqlite.Open(defaultDBPath(dbPath))
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()

	id := runID
	if last || id == "" {
		id, err = store.LastRunID(project)
		if err != nil {
			return nil, err
		}
	}
	return store.LoadRun(id)
}
