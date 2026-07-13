# cpulse

[![CI](https://github.com/hyturing/compose-pulse/actions/workflows/ci.yml/badge.svg)](https://github.com/hyturing/compose-pulse/actions/workflows/ci.yml)
[![GitHub release](https://img.shields.io/github/v/release/hyturing/compose-pulse)](https://github.com/hyturing/compose-pulse/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**cpulse shows why your Docker Compose stack is stuck.** A terminal UI that watches every container in real time, renders the `depends_on` graph as a tree, names the root cause, and lets you probe, restart, and inspect without leaving the TUI.

![cpulse dashboard demo](docs/demo.gif)

## The problem

`docker compose up` dumps interleaved logs from every container into one stream. When something won't start, you're left guessing which service is the actual blocker, scrolling through thousands of log lines, and jumping between terminal tabs. cpulse turns that graph into something you can just look at — and tells you what's blocking what.

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
| `cpulse doctor` | Diagnose why a stack is stuck |
| `cpulse doctor --project NAME` | Limit diagnosis to one compose project |
| `cpulse --version` | Print version and exit |
| `cpulse help` | Print usage |

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
| `/` | Filter service list text, or grep logs when focused on Logs |
| `d` | Jump to Doctor tab |
| `t` | Jump to Timeline tab |
| `x` | Open the actions menu for the selection |
| `?` | Toggle the help overlay |
| `q` | Quit (in zoom: un-zoom) |
| `Ctrl+C` | Always quit |

### Logs

| Key | Action |
|---|---|
| `g` / `End` | Jump to bottom and resume following |
| `/` | Filter logs by regex |
| `n` / `N` | Next / previous match |
| `l` | Load older logs |
| `↑`/`↓`, `k`/`j`, mouse wheel | Scroll |
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
