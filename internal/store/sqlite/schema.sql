CREATE TABLE IF NOT EXISTS runs (
    id TEXT PRIMARY KEY,
    schema_version INTEGER NOT NULL,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    project TEXT,
    run_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS services (
    run_id TEXT NOT NULL,
    service_key TEXT NOT NULL,
    name TEXT NOT NULL,
    project TEXT,
    container_id TEXT,
    phase TEXT NOT NULL,
    image TEXT,
    status TEXT,
    exit_code INTEGER,
    updated_at TEXT,
    PRIMARY KEY (run_id, service_key),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE TABLE IF NOT EXISTS service_phases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    service_key TEXT NOT NULL,
    phase TEXT NOT NULL,
    ts TEXT NOT NULL,
    source TEXT,
    message TEXT,
    FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    ts TEXT NOT NULL,
    source TEXT NOT NULL,
    project TEXT,
    service TEXT,
    container_id TEXT,
    phase TEXT NOT NULL,
    type TEXT NOT NULL,
    severity TEXT NOT NULL,
    message TEXT,
    data_json TEXT,
    FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    container_id TEXT,
    service TEXT,
    ts TEXT,
    line TEXT NOT NULL,
    FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE TABLE IF NOT EXISTS probe_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    service TEXT,
    probe_type TEXT,
    target TEXT,
    success INTEGER,
    detail TEXT,
    ts TEXT,
    FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE TABLE IF NOT EXISTS resource_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    container_id TEXT,
    ts TEXT,
    cpu_percent REAL,
    mem_usage INTEGER,
    mem_limit INTEGER,
    FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE TABLE IF NOT EXISTS diagnoses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    rule_id TEXT,
    service TEXT,
    root_cause TEXT,
    confidence TEXT,
    finding_json TEXT,
    FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE TABLE IF NOT EXISTS artifacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    path TEXT NOT NULL,
    kind TEXT,
    FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE INDEX IF NOT EXISTS idx_events_run_id ON events(run_id);
CREATE INDEX IF NOT EXISTS idx_service_phases_run_id ON service_phases(run_id);
CREATE INDEX IF NOT EXISTS idx_logs_run_id ON logs(run_id);
