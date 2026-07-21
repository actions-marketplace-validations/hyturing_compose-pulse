# cpulse incident report

- **Run:** `demo-failed`
- **Project:** `demo`
- **Generated:** 2026-07-22T00:00:00Z
- **Summary:** Root cause: command not found (exit 127)

## Root cause

command not found (exit 127)

- Rule: `process.exit_127`
- Service: `db`
- Confidence: **high**

## Blocked services

- `api`

## Causal chain

db → api

## Critical path

- `db` failed — 0s
- `api` failed — 3s

## Suggested fixes

- Fix the entrypoint

## Reproduction

```bash
cpulse record -- docker compose up
```

## Logs

<details>
<summary>api logs</summary>

_collapsed repeated log signatures (max repeats=3); showing first occurrence per signature and last lines_

```
[REDACTED] connection refused
…
— last lines —
[REDACTED] connection refused
[REDACTED] connection refused
[REDACTED] connection refused
```

</details>

## Redaction summary

The following secret-bearing fields/patterns were redacted (values not shown):

- `pattern:(?i)(password|passwd|secret|token|api[_-]?key)\s*[:=]\s*\S+`
- `env:DB_PASSWORD`

