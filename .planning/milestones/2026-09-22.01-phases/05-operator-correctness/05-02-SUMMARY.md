---
phase: 05-operator-correctness
plan: 02
subsystem: cli
tags: [operator-view, testing, coverage, sanitization]

# Dependency graph
requires:
  - phase: 03-curation-verdicts
    provides: "03-06: registerRowFieldRenderer + the generic sanitizing nested flatten (flattenNested/flattenObject/flattenArray) in operator_view.go, which the bare nested-object branch this plan tests shares"
provides:
  - "TestViewFieldsBareNestedObject: a direct, named test pinning viewFields' top-level `case '{':` branch (populated object, hostile-leaf sanitization, array-of-arrays sibling element), cited as OPS-02's coverage per D-01"
  - "A fixed defect that test's own coverage exposed: an empty nested object now renders zero rows instead of a whitespace-only four-space line"
affects: []

# Actuals (#2632)
actuals:
  tokens: 2069
  tasks: 2
  commits: 2
plan_head_before: 3344cf0516cb231c84cf6471fa9787cb8f81b168

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "viewFields' bare nested-object branch (case '{':) mirrors the empty-array precedent: an object-valued key whose rendering collapses to the empty string now gets a non-nil, zero-length Rows slice, never a one-element slice holding an empty string."

key-files:
  created: []
  modified:
    - cmd/engram/operator_view_test.go
    - cmd/engram/operator_view.go

key-decisions:
  - "D-01 resolved to keep-and-pin, not remove: the bare nested-object branch is reached by construction (viewFields takes `any`), even though no shipped report field reaches it today. Verified via a before/after coverage profile rather than assumed."
  - "The RED-exposed empty-object defect is fixed at the same branch the pin test covers (two-line change inside case '{':), not treated as a separate architectural change — it stays consistent with the array branch's existing empty-container-renders-zero-rows behavior."

requirements-completed: [OPS-02]

coverage:
  - id: D1
    description: "viewFields' bare nested-object branch (case '{':) and its array-of-arrays sibling element branch are reached and pinned by a named direct test: a populated object renders one sanitized flattened row, a hostile control-character leaf inside it is sanitized, and an array element that is itself an array renders one flattened row per outer element"
    requirement: OPS-02
    verification:
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestViewFieldsBareNestedObject"
        status: pass
      - kind: other
        ref: "coverage profile of TestViewFieldsBareNestedObject alone shows operator_view.go:103.4,103.21 count 1 (unreached before this plan, count 0, at the same line)"
        status: pass
    human_judgment: false
  - id: D2
    description: "An object-valued top-level field whose row renders empty (`{}`, or an object holding only an empty object) contributes zero rows under its label, matching the empty-array precedent, so renderOperatorView never emits a whitespace-only line"
    requirement: OPS-02
    verification:
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestViewFieldsEmptyNestedObjectRendersNoRows"
        status: pass
    human_judgment: false

# Metrics
duration: 6min
completed: 2026-09-24
status: complete
---

# Phase 5 Plan 2: Operator View Bare Nested-Object Coverage Summary

**Pinned `viewFields`' previously-uncovered bare nested-object branch with a direct test, then fixed the whitespace-only-line defect that test exposed (#504, OPS-02).**

## Performance

- **Duration:** 6 min
- **Started:** 2026-09-24T12:36:57-04:00
- **Completed:** 2026-09-24T12:41:42-04:00
- **Tasks:** 2 completed
- **Files modified:** 2

## Accomplishments

- `TestViewFieldsBareNestedObject` (Task 1, tracer) pins `viewFields`' top-level `case '{':` block — unreached by any shipped operator report today (`viewFields(doc any)` makes it reachable by construction for a future struct/map-typed field) — through three subtests: a populated nested object renders one flattened, sanitized row (`count=3 label=x inner.flag=true tags[0]=a tags[1]=b`); a hostile control-character leaf inside it is sanitized (no unsanitized control rune, exact predicted newline count); and the sibling array-of-arrays element branch renders one flattened row per outer element. Coverage evidence: `operator_view.go:103.4,104.18 2 0` before this plan, `operator_view.go:103.4,103.21 1 1` after (line renumbered by the Task 2 fix but the same branch, now reached).
- Task 1's tracer feedback gate re-ran its `<verify>` block end-to-end before expansion (`HUMAN_VERIFY_MODE=end-of-phase`, `<verify>` carries only `<automated>` blocks) — passed, no checkpoint synthesized.
- `TestViewFieldsEmptyNestedObjectRendersNoRows` (Task 2, TDD) proved RED against the unmodified renderer: an empty nested object (`{}` directly, or nested one level inside a wrapper object) rendered as a single row holding the empty string, which `renderOperatorView` printed as a whitespace-only four-space line, breaking its own documented "never a trailing blank line" contract.
- GREEN fix: `viewFields`' `case '{':` block now sets `field.Rows` to a non-nil empty slice when `viewRow`'s returned row is the empty string, matching the existing empty-array precedent (`TestOperatorViewEmptyShapes`). The sibling array-element branch is untouched. `viewField`'s doc comment updated to describe the new zero-rows case.
- `go test ./cmd/engram/ -count=1` and `golangci-lint run ./cmd/engram/...` are clean; `go test ./internal/keylinks/` (this milestone's active key_links gate) still passes — no repoint needed.

## Task Commits

Each task was committed atomically:

1. **Task 1: Pin viewFields' bare nested-object branch with a direct test (D-01, #504)** - `a1a4a40f` (test, tracer)
2. **Task 2: An empty nested object renders zero rows, never a whitespace-only line (#504)** - `3ab6c86f` (fix, tdd)

_Both commits carry a `Closes #504` trailer per D-02._

## Files Created/Modified

- `cmd/engram/operator_view_test.go` - `TestViewFieldsBareNestedObject` (Task 1) and `TestViewFieldsEmptyNestedObjectRendersNoRows` (Task 2), both using throwaway struct types declared inside the test functions
- `cmd/engram/operator_view.go` - `viewFields`' `case '{':` block now yields a non-nil empty `Rows` slice for an empty-rendering row instead of `[]string{""}`; `viewField`'s doc comment updated

## Decisions Made

- D-01 (verify-then-pin, not remove) confirmed live: the branch is unreached by any of the 14 shipped operator report doc types but reachable by construction through `viewFields(doc any)`, so it is kept and pinned rather than removed.
- The empty-object defect Task 1's own coverage work surfaced is fixed in the same branch it tests, as a minimal two-line change, rather than deferred or treated as an architectural change (Rule 1: auto-fix bugs).

## Deviations from Plan

None - plan executed exactly as written.

## TDD Gate Compliance

**Task 1 (tracer, no `tdd` attribute — production-quality per its own type):** `TestViewFieldsBareNestedObject` is a pin test, not a red/green cycle: it passed on the FIRST run against the unmodified renderer (the plan's own instruction: "this task pins the shipped behavior of the populated path"). Ran the coverage-before check first (D-01 verification): `go test ./cmd/engram/ -count=1 -run 'View|Operator|Consolidate' -coverprofile=...` showed `operator_view.go:103.4,104.18 2 0` (unreached). Then added the test, confirmed it PASSED immediately, and confirmed the coverage profile of the test alone showed the branch reached (`operator_view.go:103.4,104.18 2 1`).

**Tracer feedback gate:** re-ran Task 1's full `<verify>` block (both automated commands) immediately after commit, before starting Task 2 — both passed. `workflow.human_verify_mode` defaults to `end-of-phase` and the tracer's `<verify>` carries only `<automated>` blocks, so expansion continued without a checkpoint.

**Task 2 (`tdd="true"`, explicit RED-first):** Added `TestViewFieldsEmptyNestedObjectRendersNoRows` against the unmodified `viewFields` and ran it — confirmed RED with 4 failures:

```
operator_view_test.go:805: empty.Rows = [], want zero rows
operator_view_test.go:817: rendered output "headline\n\n  Name    n\n  Empty\n    \n" contains a whitespace-only line "    "
operator_view_test.go:821: rendered output "headline\n\n  Name    n\n  Empty\n    \n" does not end with the Empty label line followed by exactly one newline
operator_view_test.go:843: wrapper.Rows = [], want zero rows
--- FAIL: TestViewFieldsEmptyNestedObjectRendersNoRows (0.00s)
    --- FAIL: TestViewFieldsEmptyNestedObjectRendersNoRows/last_field_is_an_empty_struct (0.00s)
    --- FAIL: TestViewFieldsEmptyNestedObjectRendersNoRows/wrapper_whose_only_member_is_an_empty_struct (0.00s)
```

Then applied the two-line `viewFields` fix (empty-row check inside `case '{':`) and the doc-comment update. Ran the full target set (`TestViewFieldsEmptyNestedObjectRendersNoRows`, `TestViewFieldsBareNestedObject`, `TestOperatorViewEmptyShapes`, `TestOperatorViewFixturesHaveNoUnsanitizedNesting`, `TestOperatorViewIdentityAcrossEveryOperatorCommand`) and confirmed GREEN, then `go test ./cmd/engram/ -count=1` clean across the whole package, `golangci-lint run ./cmd/engram/...` clean, `gofmt -l` clean on both files.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required; this plan adds no new config surface.

## Next Phase Readiness

- OPS-02 marked complete in REQUIREMENTS.md.
- `go build ./...`, `go test ./cmd/engram/ -count=1`, `golangci-lint run ./cmd/engram/...`, and `go test ./internal/keylinks/` (this milestone's key_links gate) all pass with no regressions.
- No blockers for 05-03 (OPS-03, `ParsePlanKeyLinks`) or the remaining Phase 5 plans.

---
*Phase: 05-operator-correctness*
*Completed: 2026-09-24*

## Self-Check: PASSED

Both modified files (`cmd/engram/operator_view_test.go`, `cmd/engram/operator_view.go`) and this SUMMARY.md verified present on disk; both task commit hashes (`a1a4a40f`, `3ab6c86f`) verified present in `git log --oneline --all`. Plan-level `<verification>` re-run and passing at SUMMARY time: `go test ./cmd/engram/ -count=1` exits 0; the coverage profile of `TestViewFieldsBareNestedObject` alone shows `operator_view.go:103.4,103.21 1 1` (reached); `golangci-lint run ./cmd/engram/...` reports 0 issues. `go test ./internal/keylinks/` also passes (no key_links repoint needed).
