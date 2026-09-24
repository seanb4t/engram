---
phase: 05-operator-correctness
plan: 01
subsystem: testing
tags: [cli, cobra, pflag, exit-codes, golden-tests]

# Dependency graph
requires:
  - phase: 04-nyquist-reconciliation
    provides: a clean cmd/engram tree with no pending changes to golden_test.go or exitcode_baseline_test.go
provides:
  - "neutralizeEnvDerivedFlagDefaults(t): the single place an env-derived pflag default is neutralized for tests, shared by the help/catalog goldens and TestExitCodeBaseline"
  - "TestExitCodeBaseline isolated from ambient ENGRAM_REINDEX_TARGET / ENGRAM_MIGRATE_OWNER (#476)"
affects: [operator-correctness, cli-testing]

actuals:
  tokens: 1344
  tasks: 1
  commits: 1

tech-stack:
  added: []
  patterns:
    - "DefValue neutralization (blank pflag.Flag.DefValue before a flag reset copies it into the bound Go variable) is the fix for an init()-time os.Getenv flag default, since t.Setenv cannot retroactively affect a value already captured at init()."

key-files:
  created: []
  modified:
    - cmd/engram/golden_test.go
    - cmd/engram/exitcode_baseline_test.go

key-decisions:
  - "D-03 implemented as written: isolation lives in the test harness only (neutralizeEnvDerivedFlagDefaults), no production file touched, both affected rows' expected exit code unchanged."
  - "Extracted the existing withGoldenDeterminism DefValue-blanking loop into a standalone helper rather than writing a second copy, so the goldens and the baseline can never drift apart on which env-derived flags they neutralize."

patterns-established:
  - "A new entry added to envDerivedFlagDefaults is automatically neutralized in both the golden tests and TestExitCodeBaseline, since both call the same helper."

requirements-completed: [OPS-01]

coverage:
  - id: D1
    description: "TestExitCodeBaseline/reindex/missing-target and TestExitCodeBaseline/migrate-set-owner/missing-owner pass with ENGRAM_REINDEX_TARGET and ENGRAM_MIGRATE_OWNER set in the environment"
    requirement: OPS-01
    verification:
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaseline (with ENGRAM_REINDEX_TARGET=ambient ENGRAM_MIGRATE_OWNER=ambient)"
        status: pass
      - kind: unit
        ref: "cmd/engram/golden_test.go#TestHelpGolden and TestCatalogGolden (same env)"
        status: pass
      - kind: unit
        ref: "go test ./cmd/engram/ -count=1 -shuffle=on (same env, #476's discovery condition)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Neutralization covers every envDerivedFlagDefaults entry (adjacency: setup's --runtime/--header) and the unset/empty case (baseline condition) both still pass"
    requirement: OPS-01
    verification:
      - kind: unit
        ref: "go test ./cmd/engram/ -run '^TestExitCodeBaseline' with all four ENGRAM_REINDEX_TARGET/ENGRAM_MIGRATE_OWNER/ENGRAM_RUNTIME/ENGRAM_HEADERS set"
        status: pass
      - kind: unit
        ref: "go test ./cmd/engram/ -run 'TestExitCodeBaseline|TestHelpGolden|TestCatalogGolden' with both target/owner vars explicitly unset"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-09-24
status: complete
---

# Phase 5 Plan 1: Isolate exit-code baseline from ambient env Summary

`TestExitCodeBaseline` no longer leaks a contributor's `ENGRAM_REINDEX_TARGET` / `ENGRAM_MIGRATE_OWNER` shell variables into the "missing" rows, via a shared `DefValue`-neutralization helper extracted from the golden-test determinism setup.

## Performance

- **Duration:** 12 min
- **Started:** 2026-09-24T16:33:00Z
- **Completed:** 2026-09-24T16:35:22Z
- **Tasks:** 1 completed
- **Files modified:** 2

## Accomplishments

- Extracted `neutralizeEnvDerivedFlagDefaults(t *testing.T)` out of `withGoldenDeterminism` in `cmd/engram/golden_test.go` — the single place an env-derived pflag default is neutralized for tests, with a doc comment stating its scope (blanks only `DefValue`; callers needing the bound Go variable blank must still run a flag reset afterward) and its two callers.
- `TestExitCodeBaseline` (`cmd/engram/exitcode_baseline_test.go`) now calls the helper on every row, after the row's `t.Setenv` loop and before `resetClientFlags`/`resetEveryCommandFlagState` — the ordering that matters, since the reset is what copies `DefValue` into the bound variable.
- `envDerivedFlagDefaults`'s doc comment extended with one sentence: a new entry added to the map is neutralized automatically in both the goldens and the exit-code baseline.

## Task Commits

Each task was committed atomically:

1. **Task 1: Neutralize env-derived flag defaults in every exit-code baseline row (D-03, #476)** - `99719b2f` (test)

**Plan metadata:** (this commit, pending)

## Files Created/Modified

- `cmd/engram/golden_test.go` — extracted `neutralizeEnvDerivedFlagDefaults`; `withGoldenDeterminism` now delegates to it; doc comment extended.
- `cmd/engram/exitcode_baseline_test.go` — `TestExitCodeBaseline` calls the helper on every row before the flag resets; no row's `args`/`env`/`before`/`after`/`changes`/`landed`/`introduced` field changed.

## Decisions Made

D-03 implemented exactly as CONTEXT.md specifies: isolation lives in the test harness (the new helper), no production `.go` file touched, and both affected rows' expected exit codes (`exitUsage`) are unchanged. `t.Setenv` (D-03's example mechanism) does not work here because the flag default is captured at package `init()` time, before any test runs — `DefValue` neutralization, the mechanism the goldens already used, is the correct fix, matching the plan's stated deviation from D-03's literal example.

## Deviations from Plan

None - plan executed exactly as written.

### RED evidence (step 1, pre-fix tree)

Command: `env ENGRAM_REINDEX_TARGET=ambient ENGRAM_MIGRATE_OWNER=ambient go test ./cmd/engram/ -count=1 -run '^TestExitCodeBaseline$' -v`

Exactly two failures, nothing else:

```
--- FAIL: TestExitCodeBaseline (0.69s)
...
    --- FAIL: TestExitCodeBaseline/reindex/missing-target (0.03s)
...
    --- FAIL: TestExitCodeBaseline/migrate-set-owner/missing-owner (0.02s)
```

```
exitcode_baseline_test.go:633: row "reindex/missing-target": exitCodeFromError(err) = 5, want 2 (err=reindex: check source "mem_eval": CollectionExists() failed: mem_eval: rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp 127.0.0.1:6334: connect: connection refused")
exitcode_baseline_test.go:633: row "migrate-set-owner/missing-owner": exitCodeFromError(err) = 5, want 2 (err=EnsureCollection: CollectionExists() failed: mem_eval: rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp 127.0.0.1:6334: connect: connection refused")
```

### Post-fix verification

- Re-run of the RED command: both rows PASS, everything else still PASS (`ok github.com/seanb4t/engram/cmd/engram 0.912s`).
- `env ENGRAM_REINDEX_TARGET=ambient ENGRAM_MIGRATE_OWNER=ambient go test ./cmd/engram/ -count=1 -shuffle=on`: exits 0 (#476's discovery condition).
- Adjacency probe (`ENGRAM_RUNTIME`/`ENGRAM_HEADERS` also set): exits 0.
- Empty probe (both vars explicitly unset): exits 0.
- `golangci-lint run ./cmd/engram/...`: 0 issues.
- `gofmt -l cmd/engram/golden_test.go cmd/engram/exitcode_baseline_test.go`: no output.
- `go test ./cmd/engram/... -count=1`: ok.
- `go test ./internal/keylinks/ -count=1`: ok (this milestone's key_links gate stays satisfiable — 05-CONTEXT.md carry-forward).

### Acceptance criteria (all run and passed)

- `rg -o -F 'neutralizeEnvDerivedFlagDefaults(t)' cmd/engram/golden_test.go cmd/engram/exitcode_baseline_test.go | wc -l` → `2`
- `rg -o -F 'f.DefValue = ""' cmd/engram/golden_test.go | wc -l` → `1`
- Commit touches only test files: `0` non-`_test.go` lines
- No baseline row line removed: `0` removed lines in `exitcode_baseline_test.go`'s diff
- Commit message carries `Closes #476`: `1`

## Authentication Gates

None encountered.

## Known Stubs

None.

## Threat Flags

None — this plan's threat register (T-05-01, T-05-02) covered its own scope; no new security-relevant surface was introduced.

## Next Phase Readiness

More plans remain in Phase 5 (OPS-02 through OPS-05). Ready for `05-02-PLAN.md`.

## Self-Check: PASSED

- `cmd/engram/golden_test.go` — FOUND
- `cmd/engram/exitcode_baseline_test.go` — FOUND
- Commit `99719b2f` — FOUND (`git log --oneline --all | grep 99719b2f` matches)
