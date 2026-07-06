# cpulse

[![CI](https://github.com/hyturing/compose-pulse/actions/workflows/ci.yml/badge.svg)](https://github.com/hyturing/compose-pulse/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hyturing/compose-pulse)](https://goreportcard.com/report/github.com/hyturing/compose-pulse)
[![GitHub release](https://img.shields.io/github/v/release/hyturing/compose-pulse)](https://github.com/hyturing/compose-pulse/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A real-time terminal UI that visualizes your Docker Compose startup sequence as an interactive dependency tree — so you can see exactly which service is blocking everything else.

```
  ● api-gateway
  ├─ ● auth-service
  │  └─ ● postgres
  ├─ ◐ payment-service        ← still starting
  │  └─ ● redis
  └─ ✕ worker                 ← exited with error
     └─ ● rabbitmq
```

Press `Enter` on any service for full-screen logs. The dashboard shows a live log preview in the right panel.

---

## Interface

cpulse uses a **lazydocker-style split dashboard**:

- **Left panel `[1] Services`** — compose dependency trees grouped by project, plus standalone containers
- **Right panel `[2] Details`** — selected service metadata and a live tail of the last ~15 log lines
- **Enter** — open full-screen log view (entire terminal)
- **q / Esc** (in log view) — return to the dashboard
- **Mouse wheel** — scroll logs in full-screen view

## The Problem

`docker compose up` dumps interleaved logs from every container into one stream. When a service fails, you stop everything, scroll through thousands of lines, open multiple terminal tabs, and lose 10 minutes hunting the root cause.

**cpulse** makes the dependency graph visual and live — healthy services stay green, failing ones turn red immediately, and their logs are one keypress away.

---

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

Requires Go 1.22+.

```sh
git clone https://github.com/hyturing/compose-pulse.git
cd compose-pulse
make build
# binary at ./bin/cpulse
```

---

## Usage

Run `cpulse` anywhere — it auto-discovers all containers on your local Docker daemon:

```sh
cpulse
```

Compose-managed containers appear as dependency trees grouped by project (`COMPOSE · projectname`). All other containers appear in a flat **OTHER CONTAINERS** section below.

New stacks and containers appear automatically — no restart needed. Service status reflects `depends_on` conditions (`service_healthy`, `service_started`).

Bring up stacks in other terminals as usual:

```sh
docker compose up
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `--version` | — | Print version and exit |

---

## Keyboard Shortcuts

### Dashboard

| Key | Action |
|---|---|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` | Open full-screen logs for selected service |
| `q` / `Ctrl+C` | Quit |

### Full-screen logs

| Key | Action |
|---|---|
| `q` / `Esc` | Back to dashboard |
| `↑`/`↓` or `k`/`j` or mouse wheel | Scroll logs |
| `PgUp`/`PgDn` or `Ctrl+U`/`Ctrl+D` | Page scroll |
| `Home` | Jump to top; press again to load older logs |
| `g` / `End` | Jump to bottom and resume follow |
| `l` | Load older logs |
| `/` | Filter logs by regex |
| `Ctrl+C` | Quit |

---

## Status Indicators

| Symbol | Color | Meaning |
|---|---|---|
| `●` | Green | Running and healthy |
| `◐` | Yellow | Starting / health check running |
| `●` | Red | Health check failed / unhealthy |
| `○` | Gray | Waiting on a dependency or not yet started |
| `✕` | Red | Exited with error |

---

## Requirements

- Docker Desktop or Docker Engine running locally
- macOS or Linux

---

## Roadmap

- **v0.2** — Switch from polling to `docker events` streaming (lower latency)
- **v0.3** — Full `jq`-style log filtering via `gojq`
- **v0.4** — Real graph canvas with box-drawing edges
- **v0.5** — Container lifecycle actions (restart, stop)

---

## Contributing

Pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and guidelines.

## License

[MIT](LICENSE) — © 2026 hyturing
