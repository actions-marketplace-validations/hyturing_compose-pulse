# cpulse in CI

Headless diagnosis over a recorded `docker compose up` (Phase 6).

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Healthy / no findings at/above `--fail-on` |
| 1 | Confirmed failure(s) at/above threshold |
| 2 | Timeout (`test-startup --timeout`) |
| 3 | Compose failed to launch / record error |
| 4 | Usage error |

## Local / CI commands

```bash
make build

# Record then diagnose (JSON)
./bin/cpulse record --db .cpulse/ci.db -- docker compose up --wait
./bin/cpulse doctor --last --db .cpulse/ci.db --json --fail-on high

# Or diagnose a fixture run.json
./bin/cpulse doctor --file testdata/runs/phase2/config.missing_env_var.json --sarif --annotate

# One-shot headless startup check
./bin/cpulse test-startup --db .cpulse/ci.db --fail-on high
```

## GitHub Action

Use the composite action at the repo root (`action.yml`):

```yaml
- uses: ./
  with:
    compose-args: '--build'
    fail-on: high
    upload-html: true
```

See `.github/workflows/action-smoke.yml` for a smoke test against fixtures via `doctor --file`.

## Annotations

`cpulse doctor … --annotate` writes GitHub Actions workflow commands to stderr:

```text
::error title=config.missing_env_var::app: required environment variable …
```
