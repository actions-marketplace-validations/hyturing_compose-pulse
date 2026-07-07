# cpulse

[![CI](https://github.com/hyturing/compose-pulse/actions/workflows/ci.yml/badge.svg)](https://github.com/hyturing/compose-pulse/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hyturing/compose-pulse)](https://goreportcard.com/report/github.com/hyturing/compose-pulse)
[![GitHub release](https://img.shields.io/github/v/release/hyturing/compose-pulse)](https://github.com/hyturing/compose-pulse/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**cpulse shows why your Docker Compose stack is stuck.** A terminal UI that watches every container in real time, renders the `depends_on` graph as a tree, and tells you in plain language what's healthy, what failed, and what's blocked on what.

![cpulse dashboard demo](docs/demo.gif)

## The problem

`docker compose up` dumps interleaved logs from every container into one stream. When something won't start, you're left guessing which service is the actual blocker, scrolling through thousands of log lines, and jumping between terminal tabs. cpulse turns that graph into something you can just look at.

## What it shows you

Every service gets one of these states, derived from its container state, exit code, and dependency graph — not just raw Docker status:

| Glyph | State | Meaning |
|---|---|---|
| `●` green | `healthy` | Running, healthcheck passing (or none defined) |
| `◐` yellow (spinning) | `starting` | Container exists, healthcheck in its start period |
| `○` gray | `blocked` | Waiting on a `depends_on` condition that isn't satisfied yet |
| `○` gray | `pending` | Not blocked, just doesn't have a container yet |
| `✓` green | `completed` | Exited `0` — a migration/init job that did its job |
| `✕` red | `failed` | Exited non-zero, or exited with no known code |
| `●` red | `unhealthy` | Running, but the healthcheck is failing |

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

Compose-managed containers appear as dependency trees grouped by project (`COMPOSE · projectname`). Everything else shows up in a flat **OTHER CONTAINERS** section below. New stacks and containers appear automatically — no restart needed.

Bring up stacks in other terminals as usual:

```sh
docker compose up
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `--version` | — | Print version and exit |

## Interface

- **Top bar** — project name, a live count of services per state, and how long ago the last poll landed.
- **Services panel** (left) — the dependency tree. Each row shows a state glyph, the service name, its state label, and a short hint (`exit 1`, `postgres:healthy`, `+2 deps`).
- **Inspector panel** (right) — the selected service, in three tabs:
  - **Overview** — status, exit code, image, ports, how long ago it was created, what it's blocked by (if anything), and its last few log lines.
  - **Logs** — the full live log stream, filterable and scrollable; press `Enter` to go full-screen.
  - **Deps** — what it's waiting on and why, its full dependency list with satisfied/unsatisfied state, and its direct dependents.

## Keyboard shortcuts

### Dashboard

| Key | Action |
|---|---|
| `↑`/`k`, `↓`/`j` | Move selection |
| `Tab`, `←`/`→` | Switch between Services and Inspector panels |
| `Enter` | Open full-screen logs for the selected service |
| `1` / `2` / `3` | Inspector tab: Overview / Logs / Deps |
| `f` | Toggle a filter showing only failed + unhealthy services |
| `b` | Toggle a filter showing only blocked services |
| `Esc` | Clear the active filter |
| `a` | Open the actions menu (restart, rebuild, exec) for the selected service |
| `?` | Toggle the help overlay |
| `q` / `Ctrl+C` | Quit |

### Full-screen logs

| Key | Action |
|---|---|
| `q` / `Esc` | Back to dashboard |
| `↑`/`↓`, `k`/`j`, mouse wheel | Scroll |
| `PgUp`/`PgDn`, `Ctrl+U`/`Ctrl+D` | Page scroll |
| `Home` | Jump to top; press again to load older logs |
| `g` / `End` | Jump to bottom and resume following |
| `l` | Load older logs |
| `/` | Filter logs by regex |
| `Ctrl+C` | Quit |

### Actions menu (`a`)

Restart or rebuild the selected service, cascade a restart to everything that depends on it, or drop into an in-TUI exec shell — without leaving cpulse or touching another terminal.

## Requirements

- Docker Desktop or Docker Engine running locally
- macOS or Linux

## Contributing

Pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and guidelines.

## License

[MIT](LICENSE) — © 2026 hyturing
