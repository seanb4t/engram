---
phase: 05-operator-correctness
plan: 03
subsystem: keylinks
tags: [keylinks, satisfiability-gate, tdd, parser]

# Dependency graph
requires: []
provides:
  - "ParsePlanKeyLinks skips fieldless key_links list items (#502), matching its doc comment"
  - "parsePlanKeyLinkItems: the unexported raw walk over every list item, fieldless included, now shared by ParsePlanKeyLinks (filtered) and ScanPlansWithStats (unfiltered)"
affects: []

# Actuals (#2632)
actuals:
  tokens: 2672
  tasks: 1
  commits: 1
plan_head_before: 63d83fadf9ae2d17f5aba64ad547cbf4527f93f2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Parser split: an unexported *Items walk returns every raw entry (for a gate that must see malformed input); the exported function filters it (for a caller that wants only well-formed results). Keeps one walk, two views, rather than duplicating the state machine."

key-files:
  created: []
  modified:
    - internal/keylinks/keylinks.go
    - internal/keylinks/keylinks_test.go

key-decisions:
  - "D-04 implemented as written: ParsePlanKeyLinks skips fieldless items; ScanPlansWithStats keeps reading every item (fieldless included) via the new parsePlanKeyLinkItems, so ShapeMalformed reporting for a prose/fieldless entry is unchanged. The #502 issue's literal flush()-drop fix was NOT used — it would have dropped the entry from the scanner too, silently passing a prose-authored plan through the satisfiability gate."
  - "isFieldlessKeyLink is a small unexported predicate (all four fields empty), not an inline four-way comparison repeated at the call site."

requirements-completed: [OPS-03]

coverage:
  - id: D1
    description: "ParsePlanKeyLinks returns no KeyLink for a fieldless key_links item (bare prose, or only unrecognized keys); well-formed items parse identically to before (same From/To/Via/Pattern and Line); the satisfiability scanner still reports every fieldless item as ShapeMalformed at its own line"
    requirement: OPS-03
    verification:
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestParsePlanKeyLinksSkipsFieldlessItems"
        status: pass
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestMalformedKeyLinkEntry (unchanged assertions, still green)"
        status: pass
      - kind: unit
        ref: "internal/keylinks/gate_test.go#TestActiveMilestoneKeyLinksSatisfiable"
        status: pass
    human_judgment: false

# Metrics
duration: 6min
completed: 2026-09-24
status: complete
---

# Phase 5 Plan 3: ParsePlanKeyLinks Skips Fieldless key_links Items Summary

**`ParsePlanKeyLinks` now honors its own doc comment — no empty `KeyLink` for a prose/fieldless `key_links` item — while the satisfiability gate still reports such an item as `ShapeMalformed` at its own line (#502, OPS-03).**

## Performance

- **Duration:** 6 min
- **Started:** 2026-09-24T16:43:00Z
- **Completed:** 2026-09-24T16:49:04Z
- **Tasks:** 1 completed
- **Files modified:** 2

## Accomplishments

- Split the frontmatter walk: the existing state machine moved byte-for-byte into a new unexported `parsePlanKeyLinkItems(path string) ([]KeyLink, error)`, returning one `KeyLink` per `- ` list item, fieldless ones included, each seeded with its own `- ` line.
- `ParsePlanKeyLinks` now calls `parsePlanKeyLinkItems` and filters through a new unexported predicate `isFieldlessKeyLink` (true when From, To, Via, and Pattern are all empty), returning only items that set at least one field. Order is preserved; a nil slice is returned when nothing remains.
- `ScanPlansWithStats` now calls `parsePlanKeyLinkItems` directly instead of `ParsePlanKeyLinks`, so the satisfiability gate keeps seeing every item — the change is invisible to `stats.KeyLinks`, `CheckWellFormed`, `ValidatePattern`, and `CheckSatisfiable`, all unchanged.
- Doc comments reconciled: `ParsePlanKeyLinks` now documents the filter and points at `parsePlanKeyLinkItems` for the scanner's unfiltered view; the new function's own doc explains why the scanner needs it; `ShapeMalformed`'s doc and `TestMalformedKeyLinkEntry`'s doc comment (test code itself untouched) no longer attribute the all-empty entry to `ParsePlanKeyLinks`.
- `TestParsePlanKeyLinksSkipsFieldlessItems` (new, 3 subtests) proved RED against the unmodified parser before the fix, then GREEN after. `go test ./internal/keylinks/ -count=1` is green including `TestActiveMilestoneKeyLinksSatisfiable`; `golangci-lint run ./internal/keylinks/...` and `gofmt -l internal/keylinks` are both clean.

## Task Commits

Each task was committed atomically:

1. **Task 1: ParsePlanKeyLinks skips fieldless items; the scanner keeps reporting them as malformed (D-04, #502)** - `ddbae718` (fix, tracer)

_Commit carries a `Closes #502` trailer per D-02._

## Files Created/Modified

- `internal/keylinks/keylinks.go` - `parsePlanKeyLinkItems` (unexported, the raw walk); `ParsePlanKeyLinks` (exported, filters via `isFieldlessKeyLink`); `ScanPlansWithStats` reads `parsePlanKeyLinkItems`; doc comments on `ParsePlanKeyLinks`, `parsePlanKeyLinkItems`, and `ShapeMalformed` reconciled
- `internal/keylinks/keylinks_test.go` - `TestParsePlanKeyLinksSkipsFieldlessItems` (new, 3 subtests: mixed-order fieldless-skipping, all-fieldless-yields-nothing, scanner-still-reports-malformed); `TestMalformedKeyLinkEntry`'s doc comment updated (code and assertions untouched)

## Decisions Made

- D-04 implemented exactly as scoped in the plan's objective: kept the gate's malformed-reporting behavior via the item-level parse rather than taking #502's literal flush()-drop fix, which predates `ShapeMalformed` and would silently reopen the no-op-gate hole `TestMalformedKeyLinkEntry` exists to close.
- "Fieldless" is defined as none of From/To/Via/Pattern set — an item with only `via:` is kept by `ParsePlanKeyLinks` (it is a partial mapping worth returning) even though the scanner still flags it `ShapeMalformed` for missing from/to/pattern. Both behaviors are pinned by the same new test's first and third subtests.

## Deviations from Plan

None - plan executed exactly as written.

## TDD Gate Compliance

**Task 1 (`type="tracer"`, no `tdd` attribute — production-quality per its own type, RED required by the plan's own action step):**

RED — ran `go test ./internal/keylinks/ -count=1 -run '^TestParsePlanKeyLinksSkipsFieldlessItems$' -v` against the unmodified parser:

```
keylinks_test.go:500: expected exactly 3 links (fieldless items (b) and (c) skipped), got 5: [...]
keylinks_test.go:524: expected zero links, got 2: [...]
--- FAIL: TestParsePlanKeyLinksSkipsFieldlessItems (0.00s)
    --- FAIL: TestParsePlanKeyLinksSkipsFieldlessItems/fieldless_items_are_skipped_and_the_rest_keep_document_order_and_lines (0.00s)
    --- FAIL: TestParsePlanKeyLinksSkipsFieldlessItems/a_block_of_only_fieldless_items_yields_no_links (0.00s)
    --- PASS: TestParsePlanKeyLinksSkipsFieldlessItems/the_satisfiability_scanner_still_reports_each_fieldless_item_as_malformed (0.00s)
FAIL
```

Exactly as the plan predicted: the first two subtests failed returning 5 and 2 links (want 3 and 0); the third subtest — which exercises only the scanner's unchanged `ShapeMalformed` reporting — already passed, since that path was not yet touched.

GREEN — implemented the parser split and filter (`parsePlanKeyLinkItems` + `ParsePlanKeyLinks` + `isFieldlessKeyLink`), repointed `ScanPlansWithStats`, reconciled doc comments. Re-ran the same command: all 3 subtests PASS. Then ran the plan's full `<verify>` (`go test ./internal/keylinks/ -count=1 -v`, checking for the new test's 3 PASS subtests, the unchanged prose-entry malformed subtest, and `TestActiveMilestoneKeyLinksSatisfiable`) — all green. `golangci-lint run ./internal/keylinks/...` reports 0 issues; `gofmt -l internal/keylinks` prints nothing.

**Tracer feedback gate:** this plan's only task carries no `gate="blocking-human"` attribute. `workflow._auto_chain_active` is `false` and `workflow.auto_advance` is unset (not auto mode); `workflow.human_verify_mode` defaults to `end-of-phase`, and the task's `<verify>` carries only an `<automated>` block (no `<human-check>`). Per protocol the gate re-ran `<verify>` end-to-end (same command as above) — passed. There is no expansion task after Task 1 (this plan has exactly one task), so the gate closes the plan rather than unblocking a second task; no checkpoint was synthesized.

## Issues Encountered

None. Precondition (`go test ./internal/keylinks/ -count=1` green before any edit) verified before the first edit.

## User Setup Required

None - no external service configuration required; this plan touches only an internal Go package.

## Next Phase Readiness

- OPS-03 marked complete in REQUIREMENTS.md.
- `go build ./...` unaffected (stdlib-only leaf, no new imports); `go test ./internal/keylinks/ -count=1`, `golangci-lint run ./internal/keylinks/...`, and `gofmt -l internal/keylinks` all pass clean.
- No blockers for the remaining Phase 5 plans (05-04, 05-05).

---
*Phase: 05-operator-correctness*
*Completed: 2026-09-24*

## Self-Check: PASSED

Both modified files (`internal/keylinks/keylinks.go`, `internal/keylinks/keylinks_test.go`) and this SUMMARY.md verified present on disk. Task commit hash `ddbae718` verified present in `git log --oneline --all`. Plan-level `<verification>` re-run and passing at SUMMARY time: `go test ./internal/keylinks/ -count=1` exits 0; `golangci-lint run ./internal/keylinks/...` reports 0 issues.
