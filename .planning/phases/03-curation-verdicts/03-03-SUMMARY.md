---
phase: 03-curation-verdicts
plan: 03
subsystem: cli
tags: [sweep-scope, consolidate, surfaces, docs, curating-spine-skill, tdd]

# Dependency graph
requires:
  - phase: 03-curation-verdicts
    provides: "plan 03-01's spine-review consolidate command and its --scope/--all-scopes flag pair, which this plan enforces the shared sweep-scope rule against"
provides:
  - "spine-review consolidate reclassified into enforcingSweepLeaves: requireSweepScope is the first statement of its RunE, before output-format validation, min-score parsing, and store/decider construction"
  - "the registered sweep-scope Sentence published on consolidate's --all-scopes Usage, help.golden and catalog.golden (now 4 occurrences each)"
  - "internal/surfaces.RuleSweepScopeOrAllScopesRequired's doc comment and SurfaceFields reasoning re-derived for four enforcing leaves, purge as the sole exemption"
  - "cli.md, upgrade.md (#19 entry + table row), and the curating-spine skill (canonical + vendored) all state the scope-or-all-scopes requirement"
affects: ["03-06 (text-lane rendering of consolidate's verdict object touches the same file, spine_review_consolidate.go)"]

# Actuals (#2632)
actuals:
  tokens: 4546
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Guard-as-first-RunE-statement reuse: requireSweepScope(spineConsolidateScope, spineConsolidateAllScopes) as literally the first line of RunE, mirroring scan/verify/summarize-missing exactly (no new logic, pure reuse)"
    - "Store-construction-never-called proof pattern: substitute a package-level var constructor with a function that calls t.Error if invoked, restored via t.Cleanup — proves a guard precedes expensive/side-effecting construction without mocking the guard itself"

key-files:
  created: []
  modified:
    - cmd/engram/spine_review_consolidate.go
    - cmd/engram/sweep_scope.go
    - cmd/engram/sweep_scope_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - internal/surfaces/rules.go
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/guides/upgrade.md
    - skill/engram/skills/curating-spine/SKILL.md
    - internal/skills/data/curating-spine/SKILL.md

key-decisions:
  - "Re-derived SurfaceFields flag-set-intersection reasoning against the live four-enforcer tree rather than assuming it unchanged: the intersection of {scan, verify, consolidate, summarize-missing}'s flag sets is still {scope, all-scopes, output, timeout} (consolidate's extra flags — top-k, min-score — don't shrink the intersection below what summarize-missing already bounded it to), so it remains a strict subset of purge's flag set and dry-run still narrows cobra-usage resolution to summarize-missing alone among the five sweep-scope-pair commands."
  - "Re-pointed purge's spine.go citation from the stale line 991 to its current line 1047 (the `!opts.AllScopes && opts.Scope != \"\"` filter build), found via rg -n rather than trusting the old number, per the plan's explicit instruction."
  - "Task 3's skill-invocation edit was scoped to exactly line 70's body sentence (3 added / 2 removed lines per git show --numstat), leaving the frontmatter's own `consolidate --output json` mention and every consent/judgment step untouched, per the plan's prohibition."

requirements-completed: [CUR-04]

coverage:
  - id: D1
    description: "spine-review consolidate with neither --scope nor --all-scopes exits 2 (exitUsage) with the registered sweep-scope Sentence, verified end to end via a live `go run` invocation and via TestSweepLeavesRejectMissingScopeIdentically/spine-review_consolidate"
    requirement: CUR-04
    verification:
      - kind: integration
        ref: "cmd/engram/sweep_scope_test.go#TestSweepLeavesRejectMissingScopeIdentically/spine-review_consolidate"
        status: pass
      - kind: integration
        ref: "cmd/engram/sweep_scope_test.go#TestSweepLeavesRejectPresentButEmptyScope/spine-review_consolidate"
        status: pass
    human_judgment: false
  - id: D2
    description: "The scope guard is the FIRST statement of consolidate's RunE — with neither flag, output-format validation, min-score parsing, and store/decider construction never run; consolidate --output yaml with neither flag still returns the scope rule error, never the output-format error"
    requirement: CUR-04
    verification:
      - kind: unit
        ref: "cmd/engram/sweep_scope_test.go#TestSpineReviewConsolidateScopeGuardPrecedesEverything"
        status: pass
    human_judgment: false
  - id: D3
    description: "consolidate's --all-scopes Usage publishes the registered Sentence; help.golden and catalog.golden each carry it 4 times (scan, verify, summarize-missing, consolidate)"
    requirement: CUR-04
    verification:
      - kind: unit
        ref: "cmd/engram/sweep_scope_test.go#TestSweepLeavesUsageStatesRegisteredRule"
        status: pass
      - kind: integration
        ref: "cmd/engram/golden_test.go#TestHelpGolden"
        status: pass
      - kind: integration
        ref: "cmd/engram/golden_test.go#TestCatalogGolden"
        status: pass
    human_judgment: false
  - id: D4
    description: "spine-review consolidate is classified in enforcingSweepLeaves; spine-review purge is the sole remaining nonEnforcingSweepLeaves entry, and every table-driven sweep-scope test passes; TestNoHandRolledSweepScopeGuards confirms no unclassified live command exposes the flag pair"
    requirement: CUR-04
    verification:
      - kind: unit
        ref: "cmd/engram/sweep_scope_test.go#TestNoHandRolledSweepScopeGuards"
        status: pass
    human_judgment: false
  - id: D5
    description: "RuleSweepScopeOrAllScopesRequired's doc comment describes four enforcing leaves and purge as the sole exemption, with the SurfaceFields flag-set-intersection reasoning re-verified against the post-change tree; internal/surfaces and cmd/engram tests pass unchanged"
    requirement: CUR-04
    verification:
      - kind: unit
        ref: "go test ./internal/surfaces/... ./cmd/engram/ -count=1"
        status: pass
    human_judgment: false
  - id: D6
    description: "docs-site no longer states consolidate-with-neither-flag yields a well-defined empty result: cli.md states the scope-or-all-scopes rule and exit-2 rejection, reference/tools.md's sweep-leaf sentence names consolidate, and upgrade.md gains a numbered #19 entry plus a matching Do-you-need-to-act row"
    requirement: CUR-04
    verification:
      - kind: other
        ref: "rg negative/positive greps over cli.md, tools.md, upgrade.md (see plan Task 3 <verify>)"
        status: pass
    human_judgment: false
  - id: D7
    description: "The curating-spine skill's candidate-pair invocation now passes --scope <scope> or --all-scopes (one required); nothing else in the skill changed; the vendored copy under internal/skills/data matches the canonical file"
    requirement: CUR-04
    verification:
      - kind: unit
        ref: "internal/skills/embed_test.go#TestSkillsEmbedMatchesVendored"
        status: pass
      - kind: other
        ref: "git show --numstat HEAD -- skill/engram/skills/curating-spine/SKILL.md (3 added / 2 removed lines)"
        status: pass
    human_judgment: false

# Metrics
duration: 10 min
completed: 2026-09-24
status: complete
commits: 3
plan_head_before: fd386b2ea7df51afb10bb77839143b01e3881400
---

# Phase 3 Plan 3: Consolidate Scope-Guard Enforcement Summary

**`spine-review consolidate` now rejects a missing `--scope`/`--all-scopes` with the same registered sweep-scope rule (exit 2) as scan/verify/summarize-missing, instead of silently reporting a zero-candidate result, and every surface that described the old behavior — the rule registry comment, the reference guide, the CLI guide, the upgrade guide, and the curating-spine skill — was brought current in the same plan.**

## Performance

- **Duration:** 10 min
- **Started:** 2026-09-24T01:44:00Z
- **Completed:** 2026-09-24T01:54:25Z
- **Tasks:** 3
- **Files modified:** 11

## Accomplishments

- `spine-review consolidate` reclassified into `enforcingSweepLeaves`; `requireSweepScope(spineConsolidateScope, spineConsolidateAllScopes)` is now the literal first statement of its RunE, before `operatorOutputFormat`, `parseMinScore`, and `spineConsolidateStoreFromEnv` — proven by a new test substituting the store constructor with one that fails the test if called.
- `--all-scopes`' Usage now ends with the registered Sentence, mirroring scan/verify/summarize-missing's exact concatenation shape; `help.golden`/`catalog.golden` regenerated via `task surfaces:gen`, carrying the Sentence 4 times each (diff confined to consolidate's `--all-scopes` usage line).
- `RuleSweepScopeOrAllScopesRequired`'s registry doc comment rewritten for four enforcers (scan, verify, consolidate since #508, summarize-missing) and purge as the sole exemption, with the `SurfaceFields` flag-set-intersection claim and the `dry-run`-narrows-to-summarize-missing-alone claim both re-derived against the live post-change tree rather than assumed unchanged; the stale purge citation (`spine.go:991`) re-pointed to its current line (`spine.go:1047`).
- `reference/tools.md`'s sweep-leaf sentence, `cli.md`'s consolidate section, `upgrade.md`'s new `### 19.` entry and table row, and the curating-spine skill's candidate-pair invocation (canonical + vendored via `task skills:vendor`) all now state the scope-or-all-scopes requirement.

## Task Commits

Each task was committed atomically:

1. **Task 1: consolidate rejects a missing scope with the registered rule, end to end** - `61fff194` (fix, tracer/tdd)
2. **Task 2: The rule's registration comment and the reference guide's sweep-leaf list** - `32fe5fd3` (docs)
3. **Task 3: CLI guide, upgrade entry, and the curating-spine invocation line** - `019ed35e` (docs)

**Plan metadata commit:** recorded separately after this SUMMARY is committed.

_Note: Task 1 is a `tdd="true"` tracer — the RED evidence (test-classification move causing 3 sweep-scope tests to fail) was captured before the guard and Usage string landed, then the guard, Usage change, and new precedence-proving test were committed together as one `fix` commit per the plan's own instruction ("commit as fix(consolidate): ...")._

## Files Created/Modified

- `cmd/engram/spine_review_consolidate.go` - `requireSweepScope` call as RunE's first statement; `--all-scopes` Usage now states the requirement and ends with the registered Sentence
- `cmd/engram/sweep_scope.go` - `requireSweepScope`'s doc comment now lists four enforcing leaves
- `cmd/engram/sweep_scope_test.go` - `spine-review consolidate` moved from `nonEnforcingSweepLeaves` to `enforcingSweepLeaves`; new `TestSpineReviewConsolidateScopeGuardPrecedesEverything`
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` - regenerated; consolidate's `--all-scopes` usage only
- `internal/surfaces/rules.go` - `RuleSweepScopeOrAllScopesRequired`'s doc comment rewritten for four enforcers, purge's citation re-pointed
- `docs-site/src/content/docs/reference/tools.md` - sweep-leaf sentence now names `spine-review consolidate`
- `docs-site/src/content/docs/guides/cli.md` - consolidate section states the scope-or-all-scopes rule and exit-2 rejection
- `docs-site/src/content/docs/guides/upgrade.md` - new `### 19.` entry, matching table row
- `skill/engram/skills/curating-spine/SKILL.md`, `internal/skills/data/curating-spine/SKILL.md` - invocation line now passes `--scope`/`--all-scopes`

## Decisions Made

- Re-derived (not assumed) the `SurfaceFields` flag-set-intersection reasoning for the four-enforcer tree: still `{scope, all-scopes, output, timeout}`, still a strict subset of purge's flag set, `dry-run` still narrows to summarize-missing alone.
- Re-pointed the purge exemption's `spine.go` line citation from the stale `991` to the current `1047`, found via `rg -n` rather than trusting the old number, per the plan's explicit instruction.
- Task 3's skill edit scoped to exactly the invocation sentence (3 added / 2 removed lines), leaving the frontmatter's own mention and every consent/judgment step untouched.

## Deviations from Plan

None - plan executed exactly as written.

## TDD Gate Compliance

**Task 1 (tracer, `tdd="true"`):** RED confirmed before implementation — moving `"spine-review consolidate": true` into `enforcingSweepLeaves` alone (no production code change yet) caused `TestSweepLeavesRejectMissingScopeIdentically/spine-review_consolidate`, `TestSweepLeavesRejectPresentButEmptyScope/spine-review_consolidate`, and `TestSweepLeavesUsageStatesRegisteredRule` to fail:

```
sweep_scope_test.go:153: spine-review consolidate: exitCodeFromError(err) = 5, want 2 (exitUsage)
sweep_scope_test.go:156: spine-review consolidate: err.Error() = "EnsureCollection: CollectionExists() failed: ...", want registered Sentence "a sweep requires an explicit --scope or --all-scopes: name one scope, or opt into every scope"
--- FAIL: TestSweepLeavesRejectMissingScopeIdentically (0.03s)
    --- FAIL: TestSweepLeavesRejectMissingScopeIdentically/spine-review_consolidate (0.03s)
sweep_scope_test.go:227: spine-review consolidate: --all-scopes Usage does not contain the registered Sentence ...
--- FAIL: TestSweepLeavesRejectPresentButEmptyScope (0.02s)
--- FAIL: TestSweepLeavesUsageStatesRegisteredRule (0.00s)
```

The pre-guard consolidate attempted a live Qdrant dial (exit 5, connection-refused error) instead of rejecting at `exitUsage` — proving the guard genuinely was not yet wired. After landing the `requireSweepScope` call as RunE's first statement, the Usage string change, and the new `TestSpineReviewConsolidateScopeGuardPrecedesEverything`, the full targeted set went GREEN (7/7 PASS, including the `spine-review_consolidate` subtest), followed by golden regeneration and the full `cmd/engram` suite + `golangci-lint` clean.

**Tracer feedback gate:** re-ran Task 1's full `<verify>` (both automated blocks) after commit, before starting Task 2 — both passed (7/7 PASS test set; both goldens carrying the Sentence exactly 4 times). Per `workflow.human_verify_mode` defaulting to `end-of-phase` and the tracer's `<verify>` carrying only `<automated>` blocks (no `<human-check>`), expansion continued without a checkpoint.

## Issues Encountered

A stale per-plan commit ledger (`.git/gsd-plan-head-before-03-03`) was found on disk, dated Sep 19 from an earlier milestone's use of the same phase/plan numbering — its recorded hash did not match this session's actual pre-plan `HEAD` (`fd386b2e`, confirmed via `git log`/`git rev-parse 61fff194^`). Corrected the ledger to the verified base before computing `commits`/`plan_head_before` below, rather than trusting the stale value.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- CUR-04 satisfied; `spine-review consolidate` now has parity with every other sweep-style operator leaf's scope enforcement.
- Plan 03-06 (text-lane rendering of consolidate's verdict object) touches the same file (`spine_review_consolidate.go`) next — no conflict expected, this plan's changes are confined to the RunE guard and the `--all-scopes` Usage string, both ahead of where 03-06's rendering work will land.
- No blockers. `go build ./...`, `go test ./cmd/engram/ ./internal/surfaces/... ./internal/skills/ -count=1`, `golangci-lint run ./cmd/engram/...`, `task lint`, and `task license:check` are all clean.

---
*Phase: 03-curation-verdicts*
*Completed: 2026-09-24*

## Self-Check: PASSED

All 11 modified files verified present on disk; all 3 task commit hashes (`61fff194`, `32fe5fd3`, `019ed35e`) verified present in `git log --oneline --all`.
