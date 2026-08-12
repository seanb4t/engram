---
phase: 02-interface-discoverability
fixed_at: 2026-08-05T19:50:00Z
review_path: .planning/phases/02-interface-discoverability/02-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: partial
---

# Phase 02: Code Review Fix Report

**Fixed at:** 2026-08-05T19:50:00Z
**Source review:** .planning/phases/02-interface-discoverability/02-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2 (CR-01, WR-01 — the two findings explicitly assigned for this fix pass)
- Fixed: 2
- Skipped: 0

`status: partial` (not `all_fixed`) because two findings from the source
review — WR-02 (AST sweep evasion) and WR-03 (no dedicated anchor.go test
file) — were out of scope for this pass and were not attempted as fix
targets. WR-03 is nonetheless now resolved as a side effect: CR-01's fix
required adding `internal/surfaces/anchor_test.go`, which did not exist
before, and that file directly satisfies WR-03's ask. WR-02 remains
completely unaddressed.

## Fixed Issues

### CR-01: `WriteRegion` silently corrupts, and `ReadRegion` panics on, a reversed-order same-line anchor pair

**Files modified:** `internal/surfaces/anchor.go`, `internal/surfaces/anchor_test.go` (new)
**Commit:** fbc72232
**Applied fix:** In `scanAnchors`'s same-line branch, when an end anchor is
found on the same line as a still-pending start anchor, compare byte
offsets: if the end anchor's span starts at or before the start anchor's
span ends, return the same malformed-pairing error already used for the
cross-line cases, instead of recording the pair. `ReadRegion` and
`WriteRegion` both go through `scanAnchors`, so both now refuse cleanly
(non-nil error, no panic, no write) on the exact reversed-inline fixture
from the review's repro. Added `internal/surfaces/anchor_test.go` (none
existed) covering: the reversed-same-line malformed case (read + write,
plus a byte-identical-file-after-refused-write assertion), start-with-no-end,
end-with-no-start, two-starts-before-an-end, a well-formed inline
(same-line, markdown-table-cell) round-trip, a well-formed multi-line
round-trip including a line-count-changing replacement body, the
multi-pair-per-rule-ID (proto) rewrite case, and an OS-level write failure
(read-only directory) leaving the original file untouched.

### WR-01: `discovery-not-schedulable` rule text composed onto tools/flags that don't enforce it

**Files modified:** `internal/surfaces/rules.go`, `internal/surfaces/rules_test.go`,
`internal/surfaces/normalize.go`, `internal/surfaces/normalize_test.go`,
`internal/surfaces/conformance_test.go`, `internal/server/tools.go`,
`internal/server/surfaces_test.go`, `cmd/engram/client_store.go`,
`cmd/engram/testdata/catalog.golden`, `cmd/engram/testdata/help.golden`
**Commit:** 6a5550c5
**Applied fix:** Added `ConditionalRule.SurfaceFields []string` — when
declared, `ApplicableSurfaces` (and the hand-duplicated equivalents in
`internal/server/surfaces_test.go`'s jsonschema-tag/MCP-description checks,
via the new exported `SurfaceApplicabilityFields` helper) derive which
surfaces the rule composes onto from `SurfaceFields` instead of `Fields`,
falling back to `Fields` when `SurfaceFields` is empty (every other rule is
unaffected, zero registry churn). Declared
`SurfaceFields: []string{"category", "not_before", "not_after"}` on
`discovery-not-schedulable` — the field combination only
`scheduleArgs`/`ScheduleMemoryRequest` expose — while leaving
`Fields: []string{"category"}` untouched so `conditionalErrf`'s error
envelope still reports `field=category` (chosen resolution: option (a) from
the review's fix menu, extended with the Fields/SurfaceFields split rather
than replacing Fields outright).

Removed the now-inapplicable composition and restored prior wording:
`store_memory`'s Description, `supersede_memory`'s Description, and
`storeArgs.Category`'s jsonschema tag no longer state the rule (the sentence
was reachable there only because `Category` is promoted via Go embedding
onto `scheduleArgs`/`supersedeArgs`, not because either tool enforces it).
`engram store --category`'s cobra Usage string also reverted to its prior
wording; that surface's own conformance check (`cmd/engram/surfaces_test.go`)
calls `surfaces.ApplicableSurfaces` directly, so it picked up the
`SurfaceFields` fix automatically with no test-file edit needed. Moved the
jsonschema-tag-surface composition onto `scheduleArgs.NotBefore`/`NotAfter` —
fields declared directly on `scheduleArgs`, not promoted via embedding, so
the text can't leak into `store_memory`'s/`supersede_memory`'s own generated
schemas. Regenerated `cmd/engram/testdata/{catalog,help}.golden` (the only
observable text change: `--category`'s Usage string). Added
`TestDiscoveryNotSchedulableExcludesCategoryOnlySurfaces` (proves the rule
resolves empty on a category-only exposed set and non-empty on a
category+window exposed set) and `TestValidateRulesCatchesEmptySurfaceFieldsEntry`
(pins `ValidateRules`' new `SurfaceFields` entry validation).

Verified `go run ./internal/surfacesgen && git diff --exit-code -- proto/
docs-site/ skill/` is clean both before and after — no rule `Sentence` text
changed, only which surfaces compose it, so the generator's anchored-region
output is byte-identical.

## Verification

Run from the repo working tree (no worktree isolation — `workflow.use_worktrees`
effectively off for this shared-session task per the caller's explicit
instruction):

- `go clean -testcache && task test` (go + python unit tests): **PASS**, all
  packages green, including `internal/e2e` (rebuilt binary, not a stale
  cache hit).
- `task lint:go` (golangci-lint): **0 issues**. `task lint` (the full
  aggregate) fails only on `rumdl` markdown lint findings inside the
  untracked, pre-existing `.gsd/` session-state directory (present before
  this fix pass began, unrelated to either finding) — not a regression from
  this change.
- `go run ./internal/surfacesgen && git diff --exit-code -- proto/
  docs-site/ skill/`: clean, no drift.
- `go test ./cmd/engram -count=1 -shuffle=1` and `-shuffle=42`: both **PASS**.
- `task license:check`: `valid: 269, invalid: 0`.
- `go build ./...` and `go vet ./...`: clean.

## Not In Scope (Not Attempted)

### WR-02: `TestNoUnregisteredConditionalRejection`'s AST sweep is trivially evaded by one level of indirection

Not assigned to this fix pass (the task explicitly scoped work to CR-01 and
WR-01 only). `internal/server/conditionalsweep_test.go`'s
`scanFileForUnregisteredConditionalRejections` still only matches a bare
`*ast.Ident` hint argument by name; a local-variable indirection (or a
`HintCode("ordering")` conversion) still defeats it. Left untouched.

### WR-03: `internal/surfaces/anchor.go` has zero direct test coverage

Resolved as a byproduct of the CR-01 fix, not attempted independently:
`internal/surfaces/anchor_test.go` now exists and covers every case WR-03's
own **Fix:** section names (absent-start/absent-end/malformed-pairing error
paths including the same-line case, single- and multi-pair `WriteRegion`
rewrites with a line-count-changing body, inline vs. multi-line
indentation, and a failed write leaving the original file untouched).

---

_Fixed: 2026-08-05T19:50:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
