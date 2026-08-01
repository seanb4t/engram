---
phase: 03-cross-spine-memory-recall
plan: 01
subsystem: auth
tags: [qdrant, authorization, testing, testcontainers]

# Dependency graph
requires:
  - phase: 02-authz-foundation
    provides: ownerOrSharedCondition, ownerScopeFilter, the store-level Subject authz model
provides:
  - "TestCrossSpineAuthzIsolation: a non-vacuous two-owner isolation proof against real Qdrant, pinning the cross-spine-shaped filter (authz clause only, no scope element) never leaks a private record across owners"
  - "A pasted PASS/FAIL/PASS transcript (03-RED-TRANSCRIPT.md) proving the isolation test is sensitive to the authz clause being dropped"
  - "03-AUTHZ-GATE.md amended to cover listFilter (D-06), extending the closed gate's verdict to list_memory's filter builder"
affects: [03-02-search-cross-spine, 03-03-list-cross-spine]

actuals:
  tokens: 3053
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Isolation proofs for authz filter composition are built directly against *qdrant.Filter and scrolled via s.client.Scroll — never routed through Store.Search/Store.List when the scope value under test has different semantics today than it will post-feature (avoids the vacuous-green trap)."
    - "RED-by-mutation transcripts as a durable artifact: a fail-first test's rigor is demonstrated with pasted go test -v output (PASS -> FAIL -> PASS), not asserted in prose."

key-files:
  created:
    - .planning/phases/03-cross-spine-memory-recall/03-RED-TRANSCRIPT.md
  modified:
    - internal/store/store_test.go
    - .planning/phases/03-cross-spine-memory-recall/03-AUTHZ-GATE.md

key-decisions:
  - "D-15 mutation chosen: empty the Must slice (drop ownerOrSharedCondition entirely) rather than any other mutation variant — matches 03-AUTHZ-GATE.md's own wording ('delete ownerOrSharedCondition from the Must slice')."
  - "Zero production code touched — internal/store/store.go is unmodified by this plan, confirmed via git log --oneline -- internal/store/store.go showing no commit from this plan."

patterns-established:
  - "Pattern: isolation-proof-before-feature — for any authz-composition change, land a real-Qdrant test proving the pre-widening filter shape is correctly scoped, observe its RED by mutating the authz clause, THEN make the widening edit in a later, separate commit."

requirements-completed: [REQ-cross-spine-authz-verified]

coverage:
  - id: D1
    description: "TestCrossSpineAuthzIsolation proves a cross-spine-shaped filter (authz clause only, no scope) never returns owner B's private record over an overlapping scope name, and does return the records it should, over a demonstrably non-truncated page."
    requirement: "REQ-cross-spine-authz-verified"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestCrossSpineAuthzIsolation"
        status: pass
    human_judgment: false
  - id: D2
    description: "The isolation test's green is proven non-vacuous: RED is observed by mutating the authz clause (emptying the Must slice), the failure is confirmed to be the leaked-owner-B assertion specifically, and the test file is restored byte-exact before landing."
    requirement: "REQ-cross-spine-authz-verified"
    verification:
      - kind: manual_procedural
        ref: ".planning/phases/03-cross-spine-memory-recall/03-RED-TRANSCRIPT.md"
        status: pass
    human_judgment: false
  - id: D3
    description: "03-AUTHZ-GATE.md amended to cover listFilter (store.go:1054-1077) on the strength of a live reading, extending Evidence 1 and Evidence 2 of the closed gate; Status line remains CLOSED."
    requirement: "REQ-cross-spine-authz-verified"
    verification:
      - kind: manual_procedural
        ref: ".planning/phases/03-cross-spine-memory-recall/03-AUTHZ-GATE.md (Amendment section)"
        status: pass
    human_judgment: false

duration: 9min
completed: 2026-08-01
status: complete
---

# Phase 3 Plan 1: Cross-Spine Authz Isolation Proof Summary

**Landed a non-vacuous two-owner isolation test against real Qdrant, pinning the cross-spine-shaped
filter's authz clause before a single line of the cross-spine feature exists — plus its RED-by-mutation
evidence and the listFilter extension of the closed authz gate.**

## Performance

- **Duration:** ~9 min
- **Completed:** 2026-08-01
- **Tasks:** 3/3
- **Files modified:** 3 (1 test file, 1 new doc, 1 amended doc)

## Accomplishments

- `TestCrossSpineAuthzIsolation` added to `internal/store/store_test.go`, built directly against a
  `*qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(...)}}` (no scope element) and
  scrolled via `s.client.Scroll` against real Qdrant — deliberately not driven through
  `Store.Search`/`Store.List`, since `scope==""` currently matches essentially nothing there and
  would produce a vacuous green.
- RED observed by mutation: emptied the `Must` slice (dropping the authz clause entirely), confirmed
  the test fails specifically on the leaked-owner-B assertion (not a compile error, not the
  truncation guard), restored the file byte-exact, confirmed green again. All three `go test -v`
  runs pasted verbatim in `03-RED-TRANSCRIPT.md`.
- `03-AUTHZ-GATE.md` amended with a new section reading `listFilter` (`store.go:1054-1077`) live
  against the tree, confirming its `Must` slice opens with the same two separate, unconditional
  elements (`scope` match at index 0, `ownerOrSharedCondition(subj)` at index 1) as `ownerScopeFilter`
  — Evidence 1 and Evidence 2 transfer without qualification. Status line remains CLOSED.
- Zero production code touched. `internal/store/store.go` was not modified by any commit in this
  plan, confirmed via `git log --oneline -- internal/store/store.go`.

## Task Commits

Each task was committed atomically:

1. **Task 1: TestCrossSpineAuthzIsolation** - `737178e2` (test)
2. **Task 2: RED-by-mutation transcript** - `17ddc1cf` (docs)
3. **Task 3: listFilter reading amendment (D-06)** - `4db3cec9` (docs)

**Plan metadata:** committed below.

## Files Created/Modified

- `internal/store/store_test.go` - Added `TestCrossSpineAuthzIsolation`, placed immediately after
  `TestBulkFilterOrderIndependent` per D-17.
- `.planning/phases/03-cross-spine-memory-recall/03-RED-TRANSCRIPT.md` - New. The pasted
  PASS -> FAIL -> PASS transcript proving the test is sensitive to the authz clause.
- `.planning/phases/03-cross-spine-memory-recall/03-AUTHZ-GATE.md` - Amended with a new
  "Amendment (D-06) — the reading extends to listFilter" section; Status line unchanged (CLOSED).

## Decisions Made

- **D-15 mutation:** chosen to empty the `Must` slice (drop `ownerOrSharedCondition` entirely) —
  matches the gate document's own wording ("delete `ownerOrSharedCondition` from the `Must` slice")
  over the alternative variant in RESEARCH.md Assumption A2.
- No other deviations from the plan's decisions (D-01 through D-18 in `03-CONTEXT.md` were premises
  this plan operated under, not decisions this plan made).

## Deviations from Plan

None - plan executed exactly as written. No Rule 1/2/3 auto-fixes were needed; the plan's premises
about `ListScopes`'s live filter shape (store.go:1396-1401), `ownerOrSharedCondition`
(store.go:680-698), and `listFilter` (store.go:1054-1077) all matched the live tree exactly as
described in `03-CONTEXT.md` and `03-AUTHZ-GATE.md`.

## Issues Encountered

None. `gofmt -w` was applied once to the new test (alignment of `mk(...)` call arguments after
adding trailing comments) — routine formatting, not a deviation.

## User Setup Required

None - no external service configuration required. Docker/testcontainers Qdrant was already running
and reachable.

## Next Phase Readiness

- `TestCrossSpineAuthzIsolation` is green on a pre-feature tree, giving plan 03-02 a real regression
  guard the moment `ownerScopeFilter` gains its `if scope != ""` conditional (D-05).
- `03-AUTHZ-GATE.md` now covers both filter builders (`ownerScopeFilter` and `listFilter`) that
  03-02 and 03-03 will edit, so neither plan needs to re-derive the authz argument.
- Ordering gate satisfied: `git log --oneline -- internal/store/store.go` shows no commit from this
  plan touching `store.go` at all — the isolation test genuinely precedes the feature.
- No blockers for 03-02.

---
*Phase: 03-cross-spine-memory-recall*
*Completed: 2026-08-01*

## Self-Check: PASSED

- FOUND: `internal/store/store_test.go`
- FOUND: `.planning/phases/03-cross-spine-memory-recall/03-RED-TRANSCRIPT.md`
- FOUND: `.planning/phases/03-cross-spine-memory-recall/03-AUTHZ-GATE.md`
- FOUND commits: `737178e2`, `17ddc1cf`, `4db3cec9` (all present in `git log --oneline --all`)
