---
phase: 01-gate-ci-integrity
plan: 01
subsystem: testing
tags: [go, regexp, re2, conformance-gate, gsd-key-links]

# Dependency graph
requires: []
provides:
  - "internal/keylinks: a stdlib-only leaf package that validates a plan's key-link pattern: field for both silent-no-op shapes (escaping, unsatisfiability) and returns the corrected form"
  - "ScanPlans(repoRoot, roots, mode) — the single matcher entry point plan 01-02 (recurring repo-wide gate) and plan 01-03 (one-time v0.13.x reassessment sweep) both call"
affects: [01-gate-ci-integrity/01-02, 01-gate-ci-integrity/01-03]

# Actuals (#2632)
actuals:
  tokens: 8362
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "stdlib-only leaf package (internal/surfaces/internal/openaiurl precedent): zero repo-internal imports, error-value convention (fmt.Errorf wrapped, package-prefixed), no panics"
    - "committed known-good/known-corrupted testdata fixture pair proving a gate's fail-first direction as a permanent, re-running artifact rather than a one-time execution-time observation"
    - "escaping check runs unconditionally before compile attempt — a construct that compiles cleanly with the wrong semantics is not caught by 'compile succeeded' (Pitfall 1)"

key-files:
  created:
    - internal/keylinks/keylinks.go
    - internal/keylinks/keylinks_test.go
    - internal/keylinks/testdata/good_key_links.md
    - internal/keylinks/testdata/bad_key_links.md
  modified:
    - .licenserc.yaml

key-decisions:
  - "Backreference patterns (\\1) necessarily contain a backslash, so D-02's unconditional escaping check (which must run first, per Pitfall 1) catches them as ShapeEscaping rather than letting them reach regexp.Compile for ShapeCompileError. Task 2's <behavior> bullet names ShapeCompileError for all four JS-only constructs, but the must_haves.truths (the authoritative gated contract) only require 'any backslash is rejected' and 'compile failure is rejected' — both hold regardless of which of the two shapes a backreference lands in. Implemented the flat, unconditional rule as D-02 and Pitfall 1 explicitly require; the backreference-rejected subtest accepts either shape."
  - "ShapeUnsupportedSyntax is declared (Task 1) but never assigned by production code. RE2's compile-error messages for lookahead/negative-lookahead ('unsupported Perl syntax') vs. lookbehind ('invalid named capture') are not uniform enough to substring-match without a bespoke per-construct detector — exactly the duplication 01-RESEARCH.md's Don't Hand-Roll guidance warns against. Task 2's own acceptance criteria only name four subtest shapes (escaping, named-group, compile-error, unsatisfiable); ShapeUnsupportedSyntax is not among them. All four RE2-rejected JS-only constructs surface as ShapeCompileError carrying RE2's verbatim message."
  - "Added internal/keylinks/testdata/*.md to .licenserc.yaml's paths-ignore. task license:check flagged both committed fixtures (internal/** is doublestar-recursive over all extensions, with no existing testdata carve-out); the plan explicitly forbids an SPDX header on these files since they mimic real plans' `---`-on-line-1 frontmatter contract. Rule 3 auto-fix (blocking issue) — mirrors the existing .planning/** carve-out's rationale."

requirements-completed: [REQ-keylink-pattern-matchable]

coverage:
  - id: D1
    description: "ValidatePattern rejects any pattern whose raw string contains a backslash, independently of whether it compiles (D-02)"
    requirement: "REQ-keylink-pattern-matchable"
    verification:
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestFixturePairEscaping"
        status: pass
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestFixturePairSubsetAndSatisfiability/backreference-rejected"
        status: pass
    human_judgment: false
  - id: D2
    description: "ValidatePattern rejects a compile-error pattern and a named capture group in either Go or JavaScript syntax (D-08)"
    requirement: "REQ-keylink-pattern-matchable"
    verification:
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestFixturePairSubsetAndSatisfiability/named-group-go-syntax"
        status: pass
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestFixturePairSubsetAndSatisfiability/named-group-js-syntax"
        status: pass
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestFixturePairSubsetAndSatisfiability/compile-error-lookahead"
        status: pass
    human_judgment: false
  - id: D3
    description: "CheckSatisfiable reports an offender only when the from file exists and neither from nor to matches; a missing from file is silent"
    requirement: "REQ-keylink-pattern-matchable"
    verification:
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestFixturePairSubsetAndSatisfiability/good-fixture-from-missing-is-silent"
        status: pass
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestFixturePairSubsetAndSatisfiability/good-fixture-to-fallback-matches"
        status: pass
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestFixturePairSubsetAndSatisfiability/bad-fixture-unsatisfiable-entry"
        status: pass
    human_judgment: false
  - id: D4
    description: "Committed known-good fixture yields zero offenders and known-corrupted fixture yields at least one, naming file/line/shape/fix, in one test run (D-06)"
    requirement: "REQ-keylink-pattern-matchable"
    verification:
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestFixturePairEscaping"
        status: pass
    human_judgment: false
  - id: D5
    description: "ScanPlans is deterministic across two runs; a nonexistent root errors; ModeSatisfiability skips an un-executed plan (no sibling SUMMARY.md) and picks it up once one exists (D-07, D-04)"
    requirement: "REQ-keylink-pattern-matchable"
    verification:
      - kind: unit
        ref: "internal/keylinks/keylinks_test.go#TestScanPlansDeterministic"
        status: pass
    human_judgment: false

duration: ~22min
completed: 2026-08-13
status: complete
---

# Phase 01 Plan 01: Key-Link Guard (internal/keylinks) Summary

**New stdlib-only `internal/keylinks` package with a committed fixture pair proving both silent-no-op shapes (backslash escaping and unsatisfiable-but-valid patterns) go red, plus `ScanPlans` as the single matcher both the recurring gate and the one-time v0.13.x sweep will call.**

## Performance

- **Duration:** ~22 min
- **Completed:** 2026-08-13
- **Tasks:** 3
- **Files modified:** 5 (4 created, 1 modified)

## Accomplishments
- `ParsePlanKeyLinks` — a scoped, YAML-ish frontmatter scanner that reads `pattern:` only inside a `must_haves.key_links` block, proven by both fixtures carrying a body-prose line mentioning `pattern:` that never gets picked up.
- `ValidatePattern` — the escaping check (D-02) runs first and unconditionally, before any `regexp.Compile` attempt, closing the exact Pitfall-1 trap ("compiles" ≠ "correct") that let #479's 38 offenders through. Post-compile: RE2's own compile-error rejects lookahead/negative-lookahead/lookbehind for free; one `SubexpNames()` sweep rejects a named group in either Go's or JavaScript's syntax (D-08).
- `CheckSatisfiable` — mirrors `verify.cjs`'s from-file-first, to-file-fallback resolution exactly; a missing `from` file (an un-executed plan's promise) is silent, never an offender.
- `SuggestCharClassForm` — mechanically derives D-02's escape-free fix, including the drop-the-escape case for a backslash-quote pair (the real shape at `01-03-PLAN.md:51`, `koanf:\"client\"`).
- `ScanPlans` — the one shared entry point, sorted for D-07 determinism, skipping un-executed plans under `ModeSatisfiability` via the sibling-`SUMMARY.md` rule.
- Committed `testdata/good_key_links.md` / `bad_key_links.md` fixture pair covering all five declared `Shape` values (four exercised: escaping, named-group, compile-error, unsatisfiable; `ShapeUnsupportedSyntax` reserved, see Decisions).

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "a corrupted pattern in a real plan file goes red" — one path only (tracer)** - `003c2b21` (feat)
2. **Task 2: Reject the JavaScript-only regex grammar and the unsatisfiable pattern** - `18ce6e14` (feat)
3. **Task 3: ScanPlans — the single entry point both the recurring gate and the one-time sweep call** - `35c52dff` (feat)

**Plan metadata:** committed separately by the orchestrator after wave merge (worktree mode — this executor does not write STATE.md/ROADMAP.md).

## Files Created/Modified
- `internal/keylinks/keylinks.go` - KeyLink/Shape/Offender/Mode types; ParsePlanKeyLinks, ValidatePattern, SuggestCharClassForm, CheckSatisfiable, ScanPlans, OffenderLine
- `internal/keylinks/keylinks_test.go` - TestFixturePairEscaping, TestFixturePairSubsetAndSatisfiability (11 subtests), TestScanPlansDeterministic (3 subtests)
- `internal/keylinks/testdata/good_key_links.md` - known-good fixture: escape-free self-referential match, to-file-fallback entry, from-path-does-not-exist entry, plus a scoping-proof prose line
- `internal/keylinks/testdata/bad_key_links.md` - known-corrupted fixture: one entry per offender shape (escaping, named-group, compile-error, unsatisfiable), plus a scoping-proof prose line
- `.licenserc.yaml` - added `internal/keylinks/testdata/*.md` to paths-ignore (Rule 3 auto-fix; see Deviations)

## Decisions Made
- Backreference patterns land as `ShapeEscaping` (not `ShapeCompileError`) because they necessarily contain a backslash, which D-02's unconditional first-run check must catch — see frontmatter `key-decisions` for the full reasoning against Task 2's narrative `<behavior>` bullet vs. the authoritative `must_haves.truths`.
- `ShapeUnsupportedSyntax` is declared but not classified into by any production code path — RE2's compile-error messages aren't uniform enough to substring-match safely without duplicating stdlib logic. See frontmatter `key-decisions`.
- `.licenserc.yaml` gained a targeted `paths-ignore` entry for the two committed fixtures (Rule 3 blocking-issue auto-fix).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `task license:check` failed on the two committed testdata fixtures**
- **Found during:** Task 1 (end of task, pre-commit verification sweep)
- **Issue:** `.licenserc.yaml`'s `internal/**` path is doublestar-recursive over every extension including `.md`, and had no existing carve-out for `testdata/`. `task license:check` flagged `internal/keylinks/testdata/{good,bad}_key_links.md` as missing an SPDX header — but the plan explicitly forbids adding one, since these fixtures deliberately mimic a real plan's `---`-on-line-1 frontmatter contract and a leading comment would break that.
- **Fix:** Added `internal/keylinks/testdata/*.md` to `.licenserc.yaml`'s `paths-ignore`, with a comment citing the same rationale as the existing `.planning/**` carve-out.
- **Files modified:** `.licenserc.yaml`
- **Verification:** `task license:check` — `Totally checked 1374 files, valid: 295, invalid: 0, ignored: 1079, fixed: 0`
- **Committed in:** `003c2b21` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to satisfy the plan's own explicit verification requirement ("`task license:check` passes with no `.planning/**` or `testdata/*.md` file carrying an SPDX header"). No scope creep — a one-line config addition, no code behavior change.

## Fail-First Observations (verbatim, both directions)

**Task 1 — TestFixturePairEscaping, escaping shape:**

RED (offending entry temporarily deleted from `bad_key_links.md`):
```
=== RUN   TestFixturePairEscaping
    keylinks_test.go:46: bad_key_links.md: expected at least one offender, got none
--- FAIL: TestFixturePairEscaping (0.00s)
FAIL
```

GREEN (entry restored):
```
=== RUN   TestFixturePairEscaping
--- PASS: TestFixturePairEscaping (0.00s)
PASS
ok  	github.com/seanb4t/engram/internal/keylinks	0.049s
```

**Task 2 — TestFixturePairSubsetAndSatisfiability/bad-fixture-unsatisfiable-entry, unsatisfiable shape:**

RED (bad fixture's unsatisfiable entry's pattern changed to `ParsePlanKeyLinks[(]`, a symbol that DOES exist in its `from` file — the entry is expected to still be an offender, and locating it by its now-changed marker text correctly fails):
```
=== RUN   TestFixturePairSubsetAndSatisfiability/bad-fixture-unsatisfiable-entry
    keylinks_test.go:229: bad_key_links.md: expected the unsatisfiable-shape entry
--- FAIL: TestFixturePairSubsetAndSatisfiability (0.00s)
    --- FAIL: TestFixturePairSubsetAndSatisfiability/bad-fixture-unsatisfiable-entry (0.00s)
FAIL
```

GREEN (pattern reverted to `BADFIXTURE_UNSATISFIABLE_SYMBOL_XYZ`):
```
--- PASS: TestFixturePairSubsetAndSatisfiability/bad-fixture-unsatisfiable-entry (0.00s)
```

## Issues Encountered
None beyond the license-scope deviation documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/keylinks.ScanPlans` is ready for plan 01-02 to wire into a repo-wide recurring gate test (`ModeEscapingOnly` over all of `.planning/`, `ModeSatisfiability` over `.planning/phases/**` per D-04) and for plan 01-03's one-time v0.13.x Phase 1-2 reassessment sweep — both call the identical function, so they cannot drift into disagreeing matchers.
- `go list -deps ./internal/keylinks | grep -c 'github.com/seanb4t/engram'` returns `1` and `git diff --stat go.mod go.sum` is empty — the stdlib-only leaf constraint (D-05) holds with zero new dependencies.
- No blockers. `SuggestCharClassForm`'s mechanical rewrite (collapsing a doubled backslash, then wrapping a metacharacter in a character class, or dropping a backslash-quote escape) is ready to drive plan 01-02's 38-pattern repo-wide rewrite (D-09/D-10) without hand-deriving each fix.

---
*Phase: 01-gate-ci-integrity*
*Completed: 2026-08-13*
