package sqlite_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
	"github.com/hyturing/compose-pulse/internal/store/sqlite"
)

func TestSuccessfulDurations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	start := time.Unix(0, 0).UTC()
	endOK := start.Add(10 * time.Second)
	ok := model.NewRun("ok", start)
	ok.Project = "p"
	ok.EndedAt = &endOK
	ok.Services["api"] = &model.Service{Name: "api", Phase: model.PhaseHealthy}
	if err := store.EnsureRun(ok); err != nil {
		t.Fatal(err)
	}

	endFail := start.Add(20 * time.Second)
	bad := model.NewRun("bad", start.Add(time.Minute))
	bad.Project = "p"
	bad.EndedAt = &endFail
	bad.Services["api"] = &model.Service{Name: "api", Phase: model.PhaseFailed}
	if err := store.EnsureRun(bad); err != nil {
		t.Fatal(err)
	}

	durs, err := store.SuccessfulDurations("p", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(durs) != 1 || durs[0] != 10*time.Second {
		t.Fatalf("durs = %v", durs)
	}
}

func TestLastRunID_TieBreaksByRowID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tie.db")
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	ts := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	a := model.NewRun("older-insert", ts)
	a.Project = "p"
	b := model.NewRun("newer-insert", ts)
	b.Project = "p"
	if err := store.EnsureRun(a); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureRun(b); err != nil {
		t.Fatal(err)
	}
	id, err := store.LastRunID("p")
	if err != nil {
		t.Fatal(err)
	}
	if id != "newer-insert" {
		t.Fatalf("LastRunID = %q, want newer-insert", id)
	}
}
