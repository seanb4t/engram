---
phase: 01-interface-enforceability
plan: 01
subsystem: testing
tags: [cli, cobra, pflag, exit-codes, table-driven-test, regression-baseline]

# Dependency graph
requires: []
provides:
  - "D-09 before-table (`cmd/engram/exitcode_baseline_test.go`): 24 rows, every client verb x
    failure mode and every operator command's early-guard/unreachable-backend case, with its
    currently-observed exit code committed against unchanged production code"
  - "`resetCommandFlagState` (`cmd/engram/clienttest_test.go`): clears pflag's `Changed` latch
    (and restores `DefValue`) for a command's own flags and the root's persistent flags"
  - "`resetEveryCommandFlagState` (test-local helper): resets flag state across the whole
    (flat) command tree — rootCmd plus every direct subcommand — before each table row"
affects: [01-02, 01-03, 01-04, 01-05, 01-06, 01-07, 01-08, 01-09]

# Actuals (#2632)
actuals:
  tokens: 3668
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Table-driven exit-code regression baseline: before/after/changes/landed/introduced row
      shape, with a structural claims test (distinct-where-changed, identical-where-not)
      separate from the observation test that actually drives the command"
    - "Flag-state reset for a shared package-level cobra command tree: pflag's Changed latch
      and the underlying Go var must both be reset per-row for a table test to be order-
      independent under -shuffle=on"

key-files:
  created:
    - cmd/engram/exitcode_baseline_test.go
  modified:
    - cmd/engram/clienttest_test.go

key-decisions:
  - "Every row's hand-derived 'before' value (from reading root.go/client_common.go/each
    command's early-guard sites) matched the actual observed exit code on the first run — no
    row needed correcting against the plan's authored expectations."
  - "resetCommandFlagState alone (called only against rootCmd, as the plan's task action
    prose specifies) does not protect subcommand-local flags, since rootCmd's own flag set is
    empty at this commit and cobra's tree is flat (every command is a direct rootCmd child,
    never nested). Discovered as a real state leak (store/missing-required observed exit 5
    instead of 2 under full-package `task test` run order) and fixed with a test-local
    resetEveryCommandFlagState that resets rootCmd plus every direct subcommand before each
    row (Rule 1 auto-fix — see Deviations)."

patterns-established:
  - "A future baseline-style table test in this package should reset flag state for every
    command a row might invoke, not just the command object handed to the harness — the
    per-row reset must cover the full command tree, not just root."

requirements-completed: [REQ-exit-code-migration-safe]

coverage:
  - id: D1
    description: "D-09 before-table lands green against unchanged production code, proving the
      pre-change exit-code contract was observed and committed, not reconstructed later"
    requirement: "REQ-exit-code-migration-safe"
    verification:
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaseline"
        status: pass
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaselineClaims"
        status: pass
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaselineRowCount"
        status: pass
    human_judgment: false
  - id: D2
    description: "resetCommandFlagState clears pflag's Changed latch between table rows so a
      multi-row table over a shared rootCmd is correct"
    requirement: "REQ-exit-code-migration-safe"
    verification:
      - kind: unit
        ref: "cmd/engram/clienttest_test.go#TestResetCommandFlagState"
        status: pass
    human_judgment: false
  - id: D3
    description: "Commit touches zero production .go files — the before-table is the
      third-party-verifiable proof the pre-change state was observed"
    verification:
      - kind: other
        ref: "git diff --stat d7c9db45..HEAD -- . ':!.planning' (only cmd/engram/*_test.go)"
        status: pass
    human_judgment: false

duration: ~15min
completed: 2026-08-03
status: complete
---

# Phase 01 Plan 01: D-09 Before-Table Summary

**24-row table-driven regression baseline pinning every `engram` client/operator command x
failure-mode exit code as observed against unchanged production code, plus a pflag-latch
reset helper that makes the multi-row table order-independent.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-08-03T18:35Z
- **Completed:** 2026-08-03T18:42Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments

- `resetCommandFlagState` in `cmd/engram/clienttest_test.go`: clears pflag's `Changed` latch
  (which never self-clears across a shared package-level `*testing.T` binary) and restores
  each flag's `DefValue`, for both a command's own flags and the root's persistent flags.
- `exitCodeBaselineCase` + `exitCodeBaseline` in the new `cmd/engram/exitcode_baseline_test.go`:
  a typed row shape (`before`/`after`/`changes`/`landed`/`introduced`) with `TestExitCodeBaselineClaims`
  proving the claim itself (distinct where a row says it changes, identical where it says it
  doesn't — memory `nczgrtfec2` discipline) independent of any command execution.
- `TestExitCodeBaseline` populated with 24 rows spanning `list`/`search`/`store`, root
  self-describe/unknown-subcommand/legacy-env, and all six operator commands (`reindex`,
  `prune-expired`, `summarize-missing`, `backfill-short-ids`, `migrate-remap-owner`,
  `migrate-set-owner`), green against production code with zero non-test changes, and stable
  under `-shuffle=on -count=2`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add resetCommandFlagState** - `0ebdd7b2` (test)
2. **Task 2: Author the before-table structure and claim assertions** - `e7fdf6c6` (test)
3. **Task 3: Populate every row and prove it green on unchanged code** - `0e9b0ad5` (test)

**Plan metadata:** committed alongside this summary.

## Files Created/Modified

- `cmd/engram/exitcode_baseline_test.go` - new file: `exitCodeBaselineCase` struct,
  `exitCodeBaseline` table (24 rows), `TestExitCodeBaselineClaims`,
  `TestExitCodeBaselineRowCount`, `TestExitCodeBaseline`, `resetEveryCommandFlagState`
- `cmd/engram/clienttest_test.go` - added `resetCommandFlagState` + `TestResetCommandFlagState`

## Decisions Made

- Every row's `before` value, hand-derived by reading `root.go`, `client_common.go`, each
  client command's guard order, and each operator command's early-return sites (per
  `01-PATTERNS.md`'s site inventory table), matched the actual observed exit code on the
  first `go test` run — no row required correction against this plan's authored expectations.
  No divergence to reconcile.
- Used `http://127.0.0.1:1` / `127.0.0.1:1` (nothing listens there) for every row needing a
  dial to fail fast with connection-refused, rather than a mock server, keeping the table's
  dependency surface to stdlib networking only.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed flag-state leakage across table rows/tests (Task 3)**
- **Found during:** Task 3, running `task test` (full-package suite) after the table passed
  in isolation (`go test ./cmd/engram/... -run TestExitCodeBaseline`).
- **Issue:** The plan's Task 3 action prose calls for `resetCommandFlagState(t, rootCmd)`
  before each row. `resetCommandFlagState` only walks the *handed* command's own `Flags()`
  plus the root's `PersistentFlags()`. At this commit `rootCmd` has neither — every flag
  (`--content`, `--offset`, `--target`, etc.) lives on a subcommand (`storeCmd`, `listCmd`,
  `reindexCmd`, ...), and cobra's command tree here is flat (every command is a direct
  `rootCmd` child, confirmed via `rg '\.AddCommand\(' cmd/engram/*.go`). Calling the helper
  with only `rootCmd` therefore reset nothing useful. Under the full-package test binary's
  run order, the `store/missing-required` row observed `exitUnavailable` (5) instead of the
  expected `exitUsage` (2): an earlier test in the package had left `storeContent` non-empty,
  so `store`'s own `--content is required` early guard never fired and the command dialed the
  dead server instead.
- **Fix:** Added a test-local `resetEveryCommandFlagState(t, root)` that resets `root` itself
  plus every one of `root.Commands()` (one level is exhaustive for this flat tree), and
  switched `TestExitCodeBaseline`'s per-row call to it.
- **Files modified:** `cmd/engram/exitcode_baseline_test.go`
- **Verification:** `go test ./cmd/engram/... -run TestExitCodeBaseline -shuffle=on -count=2`
  and `task test` (full suite, real run order) both green after the fix; re-ran `task test`
  three additional times with `-shuffle=on -count=1` on the package alone with no flake.
- **Committed in:** `0e9b0ad5` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary for the table's own correctness claim (order-independence under
`-shuffle=on`) — this is exactly the kind of defect the phase's own D-09 before-table exists
to prevent from hiding inside "it passed once." No scope creep: the fix is confined to test
infrastructure in the same file the plan already scoped for Task 3, and no production `.go`
file was touched.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The before-table is committed and green; plans 01-02 through 01-09 each flip a subset of
  rows' `landed` field to `true` as their own behavior lands, and `TestExitCodeBaseline`
  will then compare against `after` instead of `before` for those rows.
- `resetCommandFlagState` / `resetEveryCommandFlagState` are available to later plans' own
  table-driven tests (e.g. `flaggroup_test.go`, `timeout_test.go` named in this plan's
  "Created later in this phase" table) — no new helper needed there.
- No blockers.

---
*Phase: 01-interface-enforceability*
*Completed: 2026-08-03*
