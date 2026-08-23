---
phase: 09-report-pending-in-migrate-status
plan: 02
subsystem: cli
tags: [migrate, operator-cli, docs-gate, pending, go]

# Dependency graph
requires:
  - phase: 09-report-pending-in-migrate-status
    plan: "01"
    provides: "pending live end-to-end (json key, text headline) via store.MigrateStatusResult.Pending()"
provides:
  - "cmd/engram/migrate_docs_test.go — the repo's second docs-content gate, modelled on docsync_test.go"
  - "corrected pending row in docs-site/src/content/docs/guides/migrate.md, closing audit item W3"
affects: []

# Actuals (#2632)
actuals:
  tokens: 1960
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "shared violation-helper pattern for docs gates: one function returning []error, called by both the real-file gate and an in-memory positive control, so the control exercises the actual gate logic instead of a parallel reimplementation"

key-files:
  created:
    - cmd/engram/migrate_docs_test.go
  modified:
    - docs-site/src/content/docs/guides/migrate.md

key-decisions:
  - "Anchored the zero-occurrence gate on the inflection-free phrase 'the equivalent number from' (no verb, cannot inflect) rather than any verb-bearing form, per D-07's carry-forward of the milestone audit's recorded near-miss"
  - "migrateGuidePendingRowViolations is the single shared assertion function called by both the real-file test and the 7-case positive control (2 call sites total), so the control proves the actual gate discriminates rather than a parallel reimplementation"
  - "Declined to cross-link the corrected row to reference/memory-record.md (left to discretion in CONTEXT.md) — it adds no accuracy to the corrected claim and would widen a deliberately narrow debt-closure diff"
  - "roadmap.update-plan-progress's known CalVer-milestone mis-scoping of the historical v0.9.x Phase 9 row is a known, already spine-tracked defect (durable record cvvrwjbsnz, extending dffmk92a8q, reconfirmed under CalVer by p0w5vv2a4k) with an explicit settled disposition of spine-tracked-only — not re-proposed for upstream filing here"

patterns-established:
  - "Deliberate RED-then-GREEN commit split for a docs gate: commit the failing gate against the live, unmodified defect first (observing it fail for the right reason), then commit the fix that clears it — stronger evidence than authoring gate and fix together and only ever seeing it pass"

requirements-completed: [REQ-docs-record-state, REQ-migrate-status-histogram]

coverage:
  - id: D1
    description: "cmd/engram/migrate_docs_test.go — zero-occurrence docs gate over guides/migrate.md's pending row (both W3 stale-claim anchors, row survival, current_version/future tokens, protojson/uint64 encoding paragraph), plus a 7-case positive control including a clean case"
    requirement: "REQ-docs-record-state"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_docs_test.go#TestMigrateGuidePendingRowIsAccurate"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_docs_test.go#TestMigrateGuidePendingRowGateFiresOnInjectedViolation"
        status: pass
    human_judgment: false
  - id: D2
    description: "guides/migrate.md's pending row rewritten to state the arithmetic (absent plus every bucket strictly below current_version, future excluded), name all three reporting surfaces, and name MigrateStatusResult.Pending() as the single shared definition"
    requirement: "REQ-migrate-status-histogram"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_docs_test.go#TestMigrateGuidePendingRowIsAccurate"
        status: pass
      - kind: other
        ref: "rg -c -F 'the equivalent number from'/'Connect lane only' docs-site/ == 0 for both anchors"
        status: pass
    human_judgment: false

# Metrics
duration: 12min
completed: 2026-08-22
status: complete
---

# Phase 9 Plan 2: Fix migrate guide pending row Summary

**`guides/migrate.md`'s `pending` row no longer claims the field is Connect-lane-only or that the CLI hand-derives it — a new self-tested docs gate (`cmd/engram/migrate_docs_test.go`) observed RED against the live defect before the fix, then GREEN after, closing audit item W3.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-08-22T19:00:00Z
- **Completed:** 2026-08-22T19:12:12Z
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

- Added `cmd/engram/migrate_docs_test.go`: `migrateGuidePendingRowViolations`, a single shared 5-leg assertion function (zero occurrences of both stale-claim anchors; the `pending` row survives and names `current_version` and `future`; the protojson/uint64 encoding paragraph survives), called by both the real-file gate and the positive control.
- `TestMigrateGuidePendingRowIsAccurate` — the real-file gate — was committed and observed RED against the live, unmodified guide, then committed a second time (implicitly, via Task 2's fix) as GREEN.
- `TestMigrateGuidePendingRowGateFiresOnInjectedViolation` — the D-07 positive control — exercises 7 in-memory fixture cases (`clean`, `stale_derivation_claim`, `stale_connect_lane_claim`, `row_deleted`, `row_omits_boundary`, `row_omits_future_exclusion`, `encoding_note_removed`), proving the gate discriminates rather than firing on everything.
- Rewrote the single `pending` row of the shared `engram migrate status` / `engram migration-status` field table to state the arithmetic, the `future` exclusion, all three reporting surfaces, and the one shared definition.
- Performed the plan-mandated constructed-defect re-check: re-injected both stale-claim anchors into a scratch copy of the guide (never the tracked file) and confirmed `migrateGuidePendingRowViolations` fires against it.

## Task Commits

1. **Task 1: author the docs gate and observe it RED against the live guide** - `0ffbdf61` (test)
2. **Task 2: rewrite the `pending` row so the gate goes green** - `60dc52a7` (docs)

**Plan metadata:** committed alongside this SUMMARY.

## Files Created/Modified

- `cmd/engram/migrate_docs_test.go` - the shared violation helper, the real-file gate, and the 7-case positive control (151 lines)
- `docs-site/src/content/docs/guides/migrate.md` - the corrected `pending` row (1 line changed, `git diff --numstat` confirms `1  1`)

## Task 1 RED Evidence (verbatim)

`go test ./cmd/engram/... -run 'TestMigrateGuidePendingRow' -v -count=1` against the live, unmodified guide produced three `t.Error` lines under the one failing test (two anchor-claim violations plus a real third defect the row already had — it never named the `future` exclusion at all):

```
migrate_docs_test.go:114: ../../docs-site/src/content/docs/guides/migrate.md: stale claim anchor "the equivalent number from" still present (1 occurrence(s)) -- the pending row must not restate this false claim
migrate_docs_test.go:114: ../../docs-site/src/content/docs/guides/migrate.md: stale claim anchor "Connect lane only" still present (1 occurrence(s)) -- the pending row must not restate this false claim
migrate_docs_test.go:114: ../../docs-site/src/content/docs/guides/migrate.md: `pending` row does not name the `future` exclusion
--- FAIL: TestMigrateGuidePendingRowIsAccurate (0.00s)
```

Task 1's `<verify>` line: `exit=1 realfile_fails=1 all_fails=1 control_passes=8`.

Module-wide confinement at Task 1's commit, checked before committing:
```
$ go test ./... -count=1; rg -o -- '--- FAIL: [A-Za-z0-9_/]+' | sort -u
--- FAIL: TestMigrateGuidePendingRowIsAccurate
```
Exactly one failing test in the whole module — the one this task added. No pre-existing test regressed.

## Task 2 GREEN Evidence (verbatim)

Final echo line from Task 2's `<verify>` command:

```
exit=0 accurate_passes=1 control_passes=8 fails=0 anchor_a=0 anchor_b=0
```

## Final Row Wording

```
| `pending` | `absent` plus every bucket **strictly below** `current_version` — buckets sitting *at* `current_version` and every `future` bucket are excluded, because `pending` answers "would running `engram migrate` do work?". Reported by `engram migrate status` (both `text` and `json`), by `engram migration-status`, and by the Connect `MigrateStatusResponse` (field 7); all three read the same server-side `MigrateStatusResult.Pending()`, never a re-derivation. |
```

## Decisions Made

- Declined to cross-link the row to `reference/memory-record.md`'s `schema_version` section (left to discretion by CONTEXT.md) — it adds no accuracy to the corrected claim and would widen a deliberately narrow debt-closure diff.
- Anchored the zero-occurrence gate on `the equivalent number from` (verb-free, cannot inflect) rather than any verb-bearing phrase, following D-07's carry-forward of the milestone audit's recorded false-negative near-miss.

## Deviations from Plan

None - plan executed exactly as written. The Task 1 RED run surfaced one additional true observation beyond the two anticipated anchor-claim failures (the row never named the `future` exclusion at all), which is a real pre-existing gap the plan's own leg 4 was designed to catch — not a deviation, since Task 2's rewrite (the plan's own prescribed fix) resolves it along with the two anchors.

## Post-Fix Full Verification

- `go test ./cmd/engram/... -count=1` — exit=0, fails=0.
- `task test:go` (full `go test ./...`, all packages including `internal/store`'s 217s suite) — exit=0, all packages `ok`.
- `task lint` — exit=0, `0 issues`.
- `task license:check` — exit=0, 354 files valid, 0 invalid.
- `task fmt:check` — exit=0.
- `git diff --name-only 810092a9..HEAD` — exactly `cmd/engram/migrate_docs_test.go` and `docs-site/src/content/docs/guides/migrate.md`.
- Constructed-defect re-check: re-injected both stale-claim anchors into a scratch copy at `/tmp` (never the tracked file) and confirmed `migrateGuidePendingRowViolations` fired both violations; scratch files removed afterward.

## Issues Encountered

- `gsd-tools query roadmap.update-plan-progress "09"` carries a known CalVer-milestone-scoping defect (documented in this repo's CLAUDE.md and in this plan's `<known_tool_defect>`): while correctly updating the current milestone's Phase 9 row, it can also corrupt the unrelated, already-shipped historical `v0.9.x` Phase 9 row in ROADMAP.md's `## Progress` table. This defect is already tracked in the project's memory spine (durable record `cvvrwjbsnz`, extending `dffmk92a8q`, reconfirmed under CalVer by `p0w5vv2a4k`) with a settled disposition of spine-tracked-only — it is not being re-proposed for upstream filing here. Checked and repaired by hand if it regressed (see below).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Both audit items opened by this phase (W2: `pending` not reported end-to-end; W3: the guide's `pending` row made two false claims) are now closed, with a durable, self-tested gate behind W3 so the false claims cannot silently return.
- Phase 09-report-pending-in-migrate-status is complete (both plans summarized).

## Self-Check: PASSED

- FOUND: `cmd/engram/migrate_docs_test.go`
- FOUND: `docs-site/src/content/docs/guides/migrate.md`
- FOUND commits: `0ffbdf61`, `60dc52a7`
- Re-ran `go test ./cmd/engram/... -run 'TestMigrateGuidePendingRow' -v -count=1`: `exit=0 accurate_passes=1 control_passes=8 fails=0 anchor_a=0 anchor_b=0`
- Re-ran `task lint`: exit=0
- Re-ran `task license:check`: exit=0
- Re-ran `task test:go`: exit=0 (all packages ok)

---
*Phase: 09-report-pending-in-migrate-status*
*Completed: 2026-08-22*
