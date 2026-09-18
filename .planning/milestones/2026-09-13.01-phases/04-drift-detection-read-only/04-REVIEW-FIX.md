---
phase: 04-drift-detection-read-only
fixed_at: 2026-09-16T00:58:53Z
review_path: .planning/phases/04-drift-detection-read-only/04-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 04: Code Review Fix Report

**Fixed at:** 2026-09-16T00:58:53Z
**Source review:** .planning/phases/04-drift-detection-read-only/04-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2 (WR-01, WR-02 — `fix_scope: critical_warning`; IN-01 excluded by scope)
- Fixed: 2
- Skipped: 0

**Verification environment:** ran in the main checkout on branch `feat/2026-09-13.01` (no worktree — this invocation's `project_constraints` explicitly pinned the run to the main working tree; `workflow.use_worktrees` in `.planning/config.json` is `true` but this run's config block overrode it).

## Fixed Issues

### WR-01: `Result.Registered`/`Drift`/`Reason` bypass the package's own capture-size bound in the new drift-compare path

**Files modified:** `internal/setup/apply.go`, `internal/setup/apply_test.go`
**Commits:** `e303086f`, `ed9092ab`
**Applied fix:** In `execute()`'s `!mutate` branch, `res.Drift`, `res.Registered`, and `res.Reason` are now routed through the package's existing `boundCapture` (the same `maxCapturedBytes` discipline the mutate lane's `res.Registered = displayCapture(probe2.Stdout + probe2.Stderr)` already applies), closing the "flood the operator's terminal/--output json via an unbounded observed header name or URL" gap the review identified. Added `TestDriftFieldsStayBoundedAgainstOversizedProbeContent` to `apply_test.go`, which drives a scripted codex probe carrying a 100KB header name and a 100KB URL through `Preview` (never a real CLI — `fakeEnvWithRun`/`scriptedRun`) and asserts `Drift`/`Registered`/`Reason` all stay within `maxCapturedBytes + len(truncationMarker)` and never contain the full oversized strings.

A follow-up commit (`ed9092ab`) was required: the first pass wrapped `renderObservation(obs)` directly in `boundCapture(...)`, which broke `internal/keylinks`' satisfiability gate — 04-01-PLAN.md's `key_links` entry asserts the literal source pattern `res.Registered = renderObservation(obs)` (D-03: Registered is rebuilt from the parsed-and-redacted Observation, never raw probe bytes) exists verbatim in `apply.go`. Restructured so the literal D-03 rebuild assignment (`res.Registered = renderObservation(obs)`) stays intact as its own statement, with `boundCapture` applied as a separate step immediately after — both the architectural key-link and the WR-01 bound are independently visible in source. `git diff ff5a6f94 -- internal/setup/apply.go` confirms every hunk (across both commits) stays inside the `!mutate` branch; the `mutate == true` branch is byte-identical to the pinned ref, per the D-08/D-03 locked decisions in `04-CONTEXT.md`.

### WR-02: Codex's type-mismatch fallback double-reports a known field as unrecognized

**Files modified:** `internal/setup/codex.go`, `internal/setup/codex_test.go`
**Commit:** `e3a7c82f`
**Applied fix:** `codexRuntime.Observe` now records which field (if any) a `*json.UnmarshalTypeError` already named (`typeErrField`) and skips the corresponding field-rules check (`enabled`, `transport.type`) when that field was already reported by the type-error branch — implementing the review's option (b). Verified empirically (a throwaway `go run` in the scratchpad, discarded before committing) that Go's `encoding/json` renders `typeErr.Field` as the dot-joined path `"transport.type"` for the nested struct field and the bare `"enabled"` for the top-level one, matching the two guard conditions exactly. Added a table-driven test (`enabled-type-mismatch`, `transport-type-type-mismatch`) to `TestObserveCodexRegistration` in `codex_test.go`, asserting `Observation.Unrecognized` contains exactly one entry for each type-mismatched field, never two.

## Skipped Issues

None — both in-scope findings (WR-01, WR-02) were fixed. IN-01 was out of scope for this run (`fix_scope: critical_warning`; IN-01 is Info-severity) and was not attempted.

## Verification

- `go build ./...` — clean.
- `go test ./internal/setup/ ./cmd/engram/ -count=1 -shuffle=on` — pass.
- `go test ./internal/keylinks/ -count=1` — pass (confirms the WR-01 follow-up commit restored key-link satisfiability).
- `go test ./internal/setup/ -run TestSetupPackageIsStdlibOnlyLeaf` — pass (no new imports introduced).
- `task lint` — clean (golangci-lint, yamlfmt, actionlint, rumdl, ruff all pass).
- `task test:go` (full `go test ./...`, including `internal/store` against Docker/Qdrant) — pass, no environmental skips encountered.

---

_Fixed: 2026-09-16T00:58:53Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
