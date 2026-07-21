# cpulse

[![CI](https://github.com/hyturing/compose-pulse/actions/workflows/ci.yml/badge.svg)](https://github.com/hyturing/compose-pulse/actions/workflows/ci.yml)
[![GitHub release](https://img.shields.io/github/v/release/hyturing/compose-pulse)](https://github.com/hyturing/compose-pulse/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**cpulse shows why your Docker Compose stack is stuck.** A terminal UI that watches every container in real time, renders the `depends_on` graph as a tree, names the root cause, and lets you probe, restart, and inspect without leaving the TUI. Beyond the dashboard: record startups, diagnose offline, probe dependencies, compare critical paths, and ship findings to CI.

![cpulse dashboard demo](docs/demo.gif)

## The problem

`docker compose up` dumps interleaved logs from every container into one stream. When something won't start, you're left guessing which service is the actual blocker, scrolling through thousands of log lines, and jumping between terminal tabs. cpulse turns that graph into something you can just look at — and tells you what's blocking what.

## Beyond the TUI

The same root-cause engine powers headless workflows:

| Feature | What you get |
|---|---|
| **Flight recorder** | Wrap `docker compose up` — capture events, inspect, logs, and config into SQLite / JSON |
| **Offline doctor** | Diagnose a recorded run (`--file`, `--last`, `--json`, `--sarif`) with confidence-gated exit codes |
| **Dependency probes** | DNS → TCP → TLS/HTTP from inside a service's network namespace |
| **Critical path** | Profile startup phases and compare against previous successful baselines |
| **Incident reports** | Shareable markdown, JSON, HTML, or SARIF from the last recorded run |
| **CI one-shot** | `cpulse test-startup` + a GitHub Action that annotates failures in PRs |

See [Record, diagnose & CI](#record-diagnose--ci) for commands and examples.

## What it shows you

Every service gets one of these states, derived from its container state, exit code, restart count, and dependency graph — not just raw Docker status:

| Glyph | State | Meaning |
|---|---|---|
| `●` green | `healthy` | Running, healthcheck passing (or none defined) |
| `◐` yellow (spinning) | `starting` | Container exists, healthcheck in its start period |
| `┄` gray | `blocked` | Waiting on a `depends_on` condition that isn't satisfied yet |
| `┄` gray | `pending` | Not blocked, just doesn't have a container yet |
| `✓` green | `completed` | Exited `0` — a migration/init job that did its job |
| `✕` red | `failed` | Exited non-zero, or exited with no known code |
| `✕` red | `unhealthy` | Running, but the healthcheck is failing |
| `⚠` yellow | `degraded` | Running, but restarted 3+ times (restart loop) |

An init container that finishes with `exit 0` looks nothing like one that crashed — that distinction alone used to require reading logs by hand.

## Installation

### Homebrew (macOS / Linux)

```sh
brew tap hyturing/cpulse
brew trust hyturing/cpulse   # required once for third-party taps (Homebrew 4.6+)
brew install cpulse
```

Verify: `cpulse --version`

To upgrade later: `brew update && brew upgrade cpulse`

### Download a binary

Grab the latest release for your platform from [GitHub Releases](https://github.com/hyturing/compose-pulse/releases).

```sh
# macOS (Apple Silicon)
curl -L https://github.com/hyturing/compose-pulse/releases/latest/download/cpulse_darwin_arm64.tar.gz | tar xz
sudo mv cpulse /usr/local/bin/
```

### Build from source

Requires Go 1.25+.

```sh
git clone https://github.com/hyturing/compose-pulse.git
cd compose-pulse
make build
# binary at ./bin/cpulse
```

## Usage

Run `cpulse` anywhere — it auto-discovers every container on your local Docker daemon:

```sh
cpulse
```

Compose-managed containers appear as dependency trees grouped by project. Everything else shows up in a flat **OTHER CONTAINERS** section below. New stacks and containers appear automatically — no restart needed.

Bring up stacks in other terminals as usual:

```sh
docker compose up
```

### Headless doctor

Diagnose without the TUI — prints root cause + findings, exits `1` if any critical finding:

```sh
cpulse doctor
cpulse doctor --project myapp
```

### Commands & flags

| Command / flag | Description |
|---|---|
| `cpulse` | Launch the TUI dashboard |
| `cpulse doctor` | Live diagnose, or headless over a recorded run |
| `cpulse doctor --project NAME` | Limit diagnosis to one compose project |
| `cpulse record -- <cmd>` | Record a Compose invocation (flight recorder) |
| `cpulse up [args]` | Alias for `cpulse record -- docker compose up [args]` |
| `cpulse replay FILE` | Replay a recorded `run.json` through the run model |
| `cpulse probe <svc> <host:port>` | Run dependency probe chain from a service's network |
| `cpulse compare --last successful` | Compare current run critical path to baseline |
| `cpulse report --last --format <md\|json\|html\|sarif>` | Shareable incident report from last recorded run |
| `cpulse test-startup` | Headless record + diagnose of `docker compose up --wait` |
| `cpulse --version` | Print version and exit |
| `cpulse help` | Print usage |

## Record, diagnose & CI

Beyond the TUI, cpulse can **record** a Compose startup, **diagnose** the run with a root-cause engine, **probe** dependencies from inside a service's network, **compare** critical-path timings, and emit **shareable reports** for CI.

### Flight recorder

Wrap any Compose command to capture events, inspect snapshots, logs, and effective config into SQLite (default `.cpulse/cpulse.db`) and an optional JSON export:

```sh
cpulse record -- docker compose up --wait
cpulse record --db .cpulse/ci.db --output run.json -- docker compose up --wait
cpulse up --wait   # same as: cpulse record -- docker compose up --wait
```

Secrets in env values are redacted. Pass `--include-env-values` only when you need values persisted (names are stored by default).

Replay a fixture or export without Docker:

```sh
cpulse replay path/to/run.json
```

### Doctor over recorded runs

Live `cpulse doctor` still works against the daemon. Against a recorded run you can select by file, run ID, or “last”, and emit machine-readable output:

```sh
cpulse doctor --file testdata/runs/phase2/config.missing_env_var.json
cpulse doctor --last --db .cpulse/ci.db --json --fail-on high
cpulse doctor --run <id> --sarif --annotate
```

| Flag | Description |
|---|---|
| `--project NAME` | Limit to one compose project (live mode) |
| `--file PATH` | Diagnose a `run.json` fixture |
| `--last` / `--run ID` | Select a run from SQLite |
| `--db FILE` | SQLite path (default: `.cpulse/cpulse.db`) |
| `--json` / `--sarif` | Headless report formats over a recorded run |
| `--fail-on LEVEL` | `high` \| `medium` \| `possible` (default `high`) |
| `--annotate` | Emit GitHub Actions annotations on stderr |

### Dependency probes

Run the probe chain (DNS → TCP → optional TLS/HTTP) from a service container's network namespace:

```sh
cpulse probe api db:5432
cpulse probe --project myapp --tls --http /ready api backend:8443
```

### Critical path compare

Compare the latest recorded run's stack critical path against previous successful baselines:

```sh
cpulse compare --last successful
cpulse compare --last successful --project myapp --db .cpulse/ci.db
```

### Incident reports

Generate a shareable report from the last recorded run (markdown, JSON, HTML, or SARIF):

```sh
cpulse report --last --format markdown
cpulse report --last --format html --output incident.html
cpulse report --last --format sarif --project myapp
```

### CI one-shot

Record `docker compose up --wait`, diagnose, and exit with a CI-friendly code:

```sh
cpulse test-startup --fail-on high
cpulse test-startup --db .cpulse/ci.db --timeout 10m -- --build
```

| Exit code | Meaning |
|---|---|
| `0` | Healthy / no findings at or above `--fail-on` |
| `1` | Confirmed failure(s) at or above threshold |
| `2` | Timeout (`test-startup --timeout`) |
| `3` | Compose failed to launch / record error |
| `4` | Usage error |

A composite GitHub Action at the repo root records compose up, runs doctor with SARIF + annotations, and can upload an HTML report:

```yaml
- uses: hyturing/compose-pulse@main
  with:
    compose-args: '--build'
    fail-on: high
    upload-html: true
```

More detail: [docs/ci.md](docs/ci.md).

## Interface

Two-pane layout: select on the left, inspect on the right.

- **Top bar** — live count of services per state, and how long ago the last poll landed.
- **Left column** — project rows + nested dependency tree. Each service row shows a state glyph, name, state label, short hint (`exit 1`, `←2 deps`, waiting-since), and live CPU/MEM columns.
- **Main panel** (right) — tabs follow what you selected:
  - **Service selected** — `1` Logs · `2` Stats (CPU/MEM sparklines) · `3` Deps (waits-on, blocks, restart order) · `4` Health (inspect healthcheck; `Enter` runs a probe inside the container)
  - **Project selected** — `1` Doctor (root cause + findings) · `2` Timeline (scaled startup Gantt) · `3` Graph (roomy pstree with edge conditions)
- **`Enter`** zooms the main panel (or jumps to a service / runs a health probe, depending on context). **`Esc`** always goes back — never quits.

## Keyboard shortcuts

### Dashboard

| Key | Action |
|---|---|
| `↑`/`k`, `↓`/`j` | Move selection |
| `Tab`, `←`/`→` | Switch between left column and main panel |
| `1`–`4` / `[` `]` | Switch tabs (service: logs/stats/deps/health · project: doctor/timeline/graph) |
| `Enter` | Zoom · jump to service · run health probe |
| `Esc` | Back / un-zoom / clear filter — never quits |
| `f` | Cycle filter: all → failed → waiting |
| `Ctrl+F` | Focus the log find box (bottom-right; or click it) |
| `d` | Jump to Doctor tab |
| `t` | Jump to Timeline tab |
| `x` | Open the actions menu for the selection |
| `?` | Toggle the help overlay |
| `q` | Quit (in zoom: un-zoom) |

### Logs

| Key | Action |
|---|---|
| `g` / `End` | Jump to bottom and resume following |
| Find box (bottom-right) | Always visible on Logs — click it or `Ctrl+F`, then type |
| `Enter` / `Shift+Enter` | Next / previous match (counter shows `4/36`; lines stay visible) |
| `Esc` | Clear query; press again to leave the find box |
| `l` | Load older logs |
| `↑`/`↓`, `k`/`j`, mouse wheel | Scroll |
| drag in logs | Select log lines |
| `Ctrl+C` | Copy selection (Control on macOS, not Cmd; quit with `q`) |
| `PgUp`/`PgDn`, `Ctrl+U`/`Ctrl+D` | Page scroll |
| `Home` | Jump to top; press again to load older logs |
| `Enter` | Full-screen zoom |
| `Esc` / `q` (in zoom) | Back to dashboard |

### Actions menu (`x`)

Restart, stop, start, rebuild, cascade a restart to dependents, run a health probe, or drop into an in-TUI exec shell — without leaving cpulse.

## Requirements

- Docker Desktop or Docker Engine running locally
- macOS or Linux

## Contributing

Pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and guidelines.

## License

[MIT](LICENSE) — © 2026 hyturing
