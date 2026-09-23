# Spike Conventions

Patterns established by the `jev-typed-decisions` spikes (001–004).

## Stack

- Go, matching the repo. Each spike program is a single `main.go` with `//go:build ignore`,
  run with `go run main.go` from the spike directory. `.planning/` starts with a dot, so
  `go test ./...` skips it anyway; the build tag keeps `go vet` and IDEs from compiling it.
- Spikes may import `github.com/seanb4t/engram/internal/...` (for example `embed`, `store`)
  to exercise real production code, not re-implementations.
- Plain `net/http` for vendor APIs in spikes, to see raw bytes. The real build should check
  for a maintained SDK first (rule `xvqj44e5mk`).

## Credentials

- Reuse engram's own env: `ENGRAM_OPENAI_BASE_URL` (OpenRouter), `ENGRAM_OPENAI_API_KEY`,
  `ENGRAM_EMBED_MODEL`. Never print key values; redact account identifiers echoed in error
  bodies before committing logs.

## Data

- The repo is **public**. A fixture holding verbatim memory-spine content gets a local
  `.gitignore` and stays uncommitted; commit code, viewers and aggregate results only.
- Build real-record fixtures read-only through engram MCP (never store, update, delete or
  supersede).

## Evaluation

- Report distributions (p50/p90/max), not single timings.
- For probability outputs, report accuracy by confidence bucket plus a Brier score, not
  accuracy alone.
- When a result is perfect, probe edge cases (no-answer, multi-answer, boundary limits)
  before calling a verdict.
- Name fixture bias in the README (who labeled it, whether they saw the targets).
