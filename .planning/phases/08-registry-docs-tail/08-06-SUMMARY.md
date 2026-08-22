---
phase: 08-registry-docs-tail
plan: 06
subsystem: infra
tags: [surfaces-registry, cobra, cli, conformance-gate, vacuous-gate-closure]

# Dependency graph
requires:
  - phase: 08-registry-docs-tail (plan 01)
    provides: "RuleSweepScopeOrAllScopesRequired registry declaration, sweep_scope.go composition point, and the original three-test sweep_scope_test.go this plan amends"
provides:
  - "A registry doc comment for RuleSweepScopeOrAllScopesRequired that claims only enforcement the tree actually contains, each claim gated against code"
  - "TestNoHandRolledSweepScopeGuards, a durable both-directions zero-occurrence gate over the live cobra tree, running inside go test ./... (task test:go / CI)"
  - "A single-source sweep-leaf classification (enforcingSweepLeaves/nonEnforcingSweepLeaves) that all four sweep-scope tests read, closing the triplicated-enumeration hole"
affects: []

# Actuals (#2632)
actuals:
  tokens: 3533
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Package-level classification maps + tree-derived comparison set + setDiff both-directions gate, mirroring cmdwalk_test.go's TestOperatorCommands idiom, applied to a rule-specific flag-pair subset instead of the whole operator tier"

key-files:
  created: []
  modified:
    - internal/surfaces/rules.go
    - cmd/engram/sweep_scope_test.go

key-decisions:
  - "Declined a committed source-level grep for the retired guard literal (08-REVIEW.md WR-03's suggested pairing): a gate in cmd/engram that greps cmd/engram for a literal must itself contain that literal, so it would be green-by-construction against its own source -- the exact vacuity shape this phase exists to close. The structural TestNoHandRolledSweepScopeGuards plus the registry-Sentence comparison in TestSweepLeavesRejectMissingScopeIdentically together catch both a NEW leaf with any guard (structural gate) and an EXISTING leaf regressing to any wrong string (Sentence comparison), which is strictly stronger."
  - "Deviation: Task 2's second <verify> block asserted an EXACT count of 1 for each test's '=== RUN <name>' line. Go's -v output prints one RUN line per subtest in addition to the parent, and each subtest line has the parent's name as a literal prefix (e.g. '=== RUN   TestEveryScopeAllScopesPairHasAFlagGroup/spine-review_scan' contains the substring '=== RUN   TestEveryScopeAllScopesPairHasAFlagGroup'), so rg -o -F counts the parent AND every subtest. This over-count is a pre-existing property of the ALREADY-SUBTESTED TestEveryScopeAllScopesPairHasAFlagGroup (6 matches) and TestSweepLeavesRejectMissingScopeIdentically/TestSweepLeavesRejectPresentButEmptyScope (4 matches each, both before and after this plan's Task 2 rewiring) -- confirmed it is not something Task 2 introduced. Ran the corrected check (count >= 1, not = 1) instead, which preserves the anti-vacuity property the gate exists for (a renamed-away test produces a bare 'go test: no tests to run' with zero RUN lines at all, still caught by >= 1) while tolerating subtests. See Deviations section."

requirements-completed: [REQ-sweep-scope-rule-registered]

coverage:
  - id: D1
    description: "internal/surfaces/rules.go's RuleSweepScopeOrAllScopesRequired doc comment claims only enforcement that exists (retired present-tense envelope claim removed; replacement gated against code with a non-vacuity control)"
    requirement: "REQ-sweep-scope-rule-registered"
    verification:
      - kind: other
        ref: "rg -o -F -- 'it drives field=scope attribution' internal/surfaces/rules.go | wc -l  ==  0"
        status: pass
      - kind: other
        ref: "rg -o 'surfaces[.]RuleSweepScopeOrAllScopesRequired' internal/server/ -g '!*_test.go' == 0 AND rg -o 'surfaces[.]RuleScopeRequiredUnlessCrossSpine' internal/server/ -g '!*_test.go' > 0"
        status: pass
      - kind: other
        ref: "rg -v '^\\s*//' cmd/engram/sweep_scope.go | rg -o 'usageErrorf[(]' | wc -l  ==  1"
        status: pass
      - kind: other
        ref: "git diff -U0 -- internal/surfaces/rules.go, non-comment changed lines == 0"
        status: pass
      - kind: unit
        ref: "task surfaces:gen; git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/"
        status: pass
      - kind: unit
        ref: "go test ./internal/surfaces/... -count=1"
        status: pass
    human_judgment: false
  - id: D2
    description: "TestNoHandRolledSweepScopeGuards exists, runs under go test ./..., derives the live command set from walkCommands (never a hand-listed set), and refuses to pass on an empty walk"
    requirement: "REQ-sweep-scope-rule-registered"
    verification:
      - kind: unit
        ref: "cmd/engram/sweep_scope_test.go#TestNoHandRolledSweepScopeGuards"
        status: pass
      - kind: unit
        ref: "cmd/engram/sweep_scope_test.go#TestRequireSweepScope"
        status: pass
      - kind: unit
        ref: "cmd/engram/sweep_scope_test.go#TestSweepLeavesRejectMissingScopeIdentically"
        status: pass
      - kind: unit
        ref: "cmd/engram/sweep_scope_test.go#TestSweepLeavesRejectPresentButEmptyScope"
        status: pass
      - kind: unit
        ref: "cmd/engram/sweep_scope_test.go#TestSweepLeavesUsageStatesRegisteredRule"
        status: pass
      - kind: unit
        ref: "cmd/engram/surfaces_test.go#TestSurfaceConformanceCobraUsage"
        status: pass
      - kind: unit
        ref: "cmd/engram/*_test.go#TestEveryScopeAllScopesPairHasAFlagGroup"
        status: pass
    human_judgment: false
  - id: D3
    description: "The classification (enforcingSweepLeaves/nonEnforcingSweepLeaves) is declared once at package level; each of the four sweep-review leaf names appears exactly once in cmd/engram/sweep_scope_test.go outside comment lines"
    requirement: "REQ-sweep-scope-rule-registered"
    verification:
      - kind: other
        ref: "rg -v '^\\s*//' cmd/engram/sweep_scope_test.go, per-leaf occurrence count == 1 for each of 'spine-review scan'/'spine-review verify'/'spine-review consolidate'/'spine-review purge'"
        status: pass
    human_judgment: false
  - id: D4
    description: "The gate has been OBSERVED failing against a deliberately constructed defect (throwaway command with inline hand-rolled guard), not merely observed passing, with the RED output recorded verbatim, in a single repeatable script"
    requirement: "REQ-sweep-scope-rule-registered"
    verification:
      - kind: other
        ref: "Task 3 self-contained RED/GREEN proof script (see Deviations-adjacent section below for the verbatim RED output)"
        status: pass
    human_judgment: false
  - id: D5
    description: "The rewritten rule comment reads as a contributor-facing explanation of a not-yet-wired lane, not an apology or changelog entry"
    verification: []
    human_judgment: true
    rationale: "Plan's must_haves marks this a backstop item, verification: backstop -- confirmed by reading the whole comment block (quoted below), not by an assertion."

duration: 24min
completed: 2026-08-22
status: complete
---

# Phase 8 Plan 6: Registry doc-comment accuracy + durable zero-occurrence gate Summary

**Closed the two remaining Phase 8 gaps on `RuleSweepScopeOrAllScopesRequired`: its registry doc comment no longer claims an error-envelope attribution that no deployed surface produces (08-VERIFICATION.md truth 5), and the phase's "zero hand-rolled occurrences" claim is now a durable `go test` gate (`TestNoHandRolledSweepScopeGuards`) instead of a one-time manual `rg` run (08-VERIFICATION.md truth 2) — and the gate has been observed failing against a deliberately constructed defect before being accepted.**

## Performance

- **Duration:** ~24 min
- **Started:** 2026-08-22T00:22:00Z (approx, first tool call)
- **Completed:** 2026-08-22T00:45:51Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments

- **Task 1 — registry comment corrected.** `internal/surfaces/rules.go`'s `RuleSweepScopeOrAllScopesRequired` doc comment no longer states, present-tense, that `Fields` "drives field=scope attribution on the error envelope." The retired-claim gate was observed RED (count `1`) on `HEAD` before the edit, per the task's own acceptance criteria — recorded below. Replaced with a conditional statement naming the sole enforcement site (`requireSweepScope`, bare `usageErrorf`), the absence of any `conditionalErrf` call site for this rule in `internal/server` (proven with a non-vacuity control: the same reference shape for `RuleScopeRequiredUnlessCrossSpine` is `3`, not `0`), and the future-lane condition under which `Fields`/`Hint` would become operative. Every changed line in the diff is a `//` comment line (0 non-comment changed lines); `task surfaces:gen` reproduces CI's `surfaces` job cleanly.
- **Task 2 — single-source classification + durable gate.** Lifted the enforcing/non-enforcing sweep-leaf classification out of three independent literal enumerations into package-level `enforcingSweepLeaves`/`nonEnforcingSweepLeaves` maps, read by all four sweep-scope tests. Added `sweepScopeFlagPairCommands()` (derives the live flag-pair-exposing command set via `walkCommands(rootCmd, commandWalkSkip)`, mirroring `operatorCommands()`'s "reads the LIVE flag set" framing) and `TestNoHandRolledSweepScopeGuards` (compares the derived set against the classification in both directions via the package's existing `setDiff`, fails fast on an empty walk or an overlapping classification). Each of the four `spine-review` leaf names now appears exactly once in the file outside comments.
- **Task 3 — the gate observed RED, then GREEN.** A self-contained script wrote a throwaway `_test.go` registering a scratch cobra command (`zz-redproof-sweep`) exposing both `--scope` and `--all-scopes` with its own inline hand-rolled guard (deliberately using a neutral message, never the retired literal), ran `TestNoHandRolledSweepScopeGuards`, captured the FAIL, deleted the scratch file, and re-ran to confirm PASS — all in one repeatable run. Full `task` (lint + test) is clean with the scratch file gone.
- Declined a committed source-level literal grep (08-REVIEW.md WR-03's suggested pairing) — see `key-decisions` for the stated reason.

## Task Commits

1. **Task 1: Make the rule's doc comment claim only the enforcement that exists** - `e1d303f4` (docs)
2. **Task 2: Make the sweep-leaf classification single-source and gate the tree against it in both directions** - `848380bd` (test)

Task 3 produced no persistent file delta (it created and then deleted a throwaway scratch file as part of its own verification script) — no commit is associated with it.

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/surfaces/rules.go` — `RuleSweepScopeOrAllScopesRequired`'s doc comment rewritten (comment-only change; `Sentence`/`Fields`/`SurfaceFields`/`Hint`/`TagForm`/slice position all byte-identical)
- `cmd/engram/sweep_scope_test.go` — package-level `enforcingSweepLeaves`/`nonEnforcingSweepLeaves`, `sweepScopeFlagPairCommands()`, `TestNoHandRolledSweepScopeGuards`, and the three pre-existing tests rewired to read the shared classification

## Task 1: observed RED pre-state

Before the edit, the retired-claim gate was RED as required by the task's `<done>` criteria:

```
$ rg -o -F -- 'it drives field=scope attribution' internal/surfaces/rules.go | wc -l
1
```

After the edit: `0`. The non-vacuity control for the lane-reference gate:

```
this rule in internal/server: 0 (want 0) / control rule: 3 (want >0)
```

## Rewritten `RuleSweepScopeOrAllScopesRequired` comment (quoted whole)

```go
// RuleSweepScopeOrAllScopesRequired is the ID of the rule requiring an
// explicit --scope (or --all-scopes) at every sweep-style operator leaf:
// `spine-review scan`, `spine-review verify`, and `summarize-missing`
// (issue #480). Fields is the flag pair alone (["scope", "all-scopes"]).
// This rule is CLI-only -- its sole enforcement site is cmd/engram's
// requireSweepScope, which raises a bare usageErrorf, so nothing carries a
// field=/hint= envelope for it today. No conditionalErrf call site exists
// for this rule anywhere in the tree -- internal/server never references
// this rule's const at all. Fields WOULD drive field=scope attribution,
// and Hint's "conditional_required" value WOULD become live, only if a
// future MCP or Connect lane raised this rule through
// internal/server.conditionalErrf; both are declared for that future lane,
// not for a live surface.
//
// SurfaceFields diverges from Fields to
// []string{"scope", "all-scopes", "dry-run"}. Five commands' own flag sets
// expose BOTH scope and all-scopes: spine-review scan, spine-review verify,
// summarize-missing (all three enforce this rule today), plus
// spine-review consolidate and spine-review purge (neither enforces it --
// consolidate's NearDuplicates treats Scope:"" AllScopes:false as a
// well-defined empty result, internal/store/spine.go:384-387; purge applies
// a scope filter only when !AllScopes && Scope != "", internal/store/
// spine.go:991, so a class-only purge naming neither flag deliberately
// spans every scope, D-10). No field set can select exactly the three
// enforcing leaves by Fields alone: their flag-set intersection is
// {scope, all-scopes, output, timeout} -- summarize-missing's *entire* set
// minus dry-run/older-than/limit -- which is a strict subset of both
// consolidate's and purge's flag sets, so any subset of it also resolves
// onto both non-enforcers. Adding "dry-run" (a field unique to
// summarize-missing among the five) narrows cobra_usage resolution to
// summarize-missing alone -- verified empirically against the live tree
// (08-01-PLAN.md fact 5). The two leaves this narrowing cannot reach
// (spine-review scan, spine-review verify) are pinned instead by
// TestSweepLeavesUsageStatesRegisteredRule in cmd/engram, the explicit
// whitelist the field-set model cannot express.
//
// The SAME narrowing determines the derived prose surface: it resolves to
// SurfaceDocsSite alone (docs-site/reference/tools.md's summarize-missing
// section mentions dry-run; neither skill file does, so SurfaceSkill
// resolves empty; all_scopes is not a proto field on any message, so
// SurfaceProtoComment resolves empty too -- 08-01-PLAN.md fact 6).
//
// TagForm is left empty, same reasoning as RuleDestructiveRequiresApply/
// RuleVerifyFailOnValues/RulePurgeFilterRequiresScope: no MCP arg struct
// carries an all_scopes field on any schema, so there is no jsonschema tag
// to compress this rule's statement into.
const RuleSweepScopeOrAllScopesRequired = "sweep-scope-or-all-scopes-required"
```

Read whole: the comment now states, as a contributor-facing fact rather than an apology, exactly which claims are live (the flag-pair `Fields`, the `SurfaceFields`-narrowed cobra_usage/docs_site resolution) and which are declared for a not-yet-wired future (the `field=`/`hint=` envelope attribution) — satisfying the backstop truth.

## Task 3: verbatim RED output

```
--- observed RED output ---
=== RUN   TestNoHandRolledSweepScopeGuards
    sweep_scope_test.go:94: zz-redproof-sweep: exposes --scope and --all-scopes but is in neither enforcingSweepLeaves nor nonEnforcingSweepLeaves -- classify it: route it through requireSweepScope and add it to enforcingSweepLeaves, or record why it is exempt in nonEnforcingSweepLeaves
--- FAIL: TestNoHandRolledSweepScopeGuards (0.00s)
FAIL
FAIL	github.com/seanb4t/engram/cmd/engram	0.289s
FAIL
--- end RED output ---
```

The `extra`-direction message named the throwaway command explicitly and stated the two remediation paths (route through `requireSweepScope` + classify enforcing, or record the exemption in `nonEnforcingSweepLeaves`) — read as actionable to a contributor seeing it cold; no follow-up wording change was needed.

After removing the scratch file, the same test ran and passed:

```
=== RUN   TestNoHandRolledSweepScopeGuards
--- PASS: TestNoHandRolledSweepScopeGuards (0.00s)
PASS
ok  	github.com/seanb4t/engram/cmd/engram	...
```

`cmd/engram/zz_sweepgate_redproof_test.go` does not exist on disk and does not appear in `git status --porcelain` after the script completed.

## Declined: committed source-level literal grep

08-REVIEW.md WR-03 suggested pairing the tree gate with a committed `rg`-based test asserting zero occurrences of the retired literal string in `cmd/engram/` source. Declined, per the plan's own instruction: a gate living in `cmd/engram` that greps `cmd/engram` for a literal must itself contain that literal to express the assertion, so the gate matches its own source and is green-by-construction — the exact vacuity shape this whole plan exists to close. The behavioral coverage already in place is strictly stronger and has no such hole: an EXISTING leaf regressing to any hand-rolled string (not just the one retired literal) fails `TestSweepLeavesRejectMissingScopeIdentically`, which compares against the registry-resolved `Sentence`; a NEW leaf appearing with any guard at all — regardless of its message — fails `TestNoHandRolledSweepScopeGuards`.

## Decisions Made

See `key-decisions` in frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug in plan's `<verify>` script] Task 2's second gate over-asserted an exact `=== RUN` line count**
- **Found during:** Task 2, first attempt to run the plan's literal second `<verify>` block
- **Issue:** The gate asserted `rg -o -F -- "=== RUN   $t" ... | wc -l` equals exactly `1` for each of six named tests. `go test -v` prints one `=== RUN` line per subtest in addition to the parent line, and each subtest's line has the parent test's `=== RUN <name>` string as a literal prefix (e.g. `=== RUN   TestSweepLeavesRejectMissingScopeIdentically/spine-review_scan` contains the substring `=== RUN   TestSweepLeavesRejectMissingScopeIdentically`), so an unanchored `rg -o -F` count sums the parent AND every subtest. Confirmed this is not something Task 2's rewiring introduced: `TestEveryScopeAllScopesPairHasAFlagGroup` (untouched by this plan, pre-existing subtests) already produces `6` matches, and `TestSweepLeavesRejectMissingScopeIdentically`/`TestSweepLeavesRejectPresentButEmptyScope` already had subtests (with literal `tc.name` cases) before Task 2's rewiring, producing `4` matches both before and after.
- **Fix:** Ran the corrected check with `-ge 1` instead of `= 1` for the `=== RUN` line count. This preserves the anti-vacuity property the gate exists for — `go test -run <NonexistentName>` prints no `=== RUN` line at all and exits `0`, still caught by a `>= 1` threshold — while correctly tolerating parent+subtest output. All six named tests were confirmed to have actually run (RUN line count `>= 1`) and the suite reported zero `--- FAIL` lines.
- **Files modified:** None (verification-script correction only; no plan or implementation file was edited for this deviation)
- **Verification:** Re-ran the corrected six-test suite; all RUN lines present (`1`, `4`, `4`, `1`, `1`, `6` respectively — all `>= 1`), zero `--- FAIL`.
- **Committed in:** N/A (no file change; documented here per the deviation-tracking contract)

---

**Total deviations:** 1 auto-fixed (1 bug in the plan's own `<verify>` script — an exact-count assertion that does not account for Go's subtest RUN-line prefixing, a pre-existing property of one of the six named tests that predates this plan)
**Impact on plan:** None on the delivered code. The correction only affects how a verification gate is invoked, not what it verifies; the underlying anti-vacuity property (a renamed-away test produces zero RUN lines and is still caught) is preserved.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

Both remaining Phase 8 gaps (08-VERIFICATION.md truths 2 and 5) are closed. `internal/surfaces/rules.go`'s `RuleSweepScopeOrAllScopesRequired` comment states only claims gated against the live code, and the "zero hand-rolled occurrences" property is now enforced by `TestNoHandRolledSweepScopeGuards` inside `go test ./...` — no new Taskfile target, no new CI step, no production code path touched. `REQ-sweep-scope-rule-registered` is fully satisfied. This closes out Phase 8's plan set (6 of 6); ready for phase-level re-verification.

---
*Phase: 08-registry-docs-tail*
*Completed: 2026-08-22*

## Self-Check: PASSED

- `internal/surfaces/rules.go` and `cmd/engram/sweep_scope_test.go` confirmed present on disk and confirmed as the only two files changed relative to `a5b2de7e` (`git diff --name-only a5b2de7e..HEAD`).
- Both task commit hashes (`e1d303f4`, `848380bd`) confirmed in `git log --oneline -3`.
- Plan-level `<verification>` re-run against the committed tree: `task` (lint + full suite) green; `task surfaces:gen` + `git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/` clean; `git diff --name-only a5b2de7e..HEAD` reports exactly the two expected files; `go test ./internal/keylinks/... -count=1` green; rewritten comment block read whole and confirmed contributor-facing (backstop truth D5).
