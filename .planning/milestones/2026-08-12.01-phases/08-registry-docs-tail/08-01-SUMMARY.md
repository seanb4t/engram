---
phase: 08-registry-docs-tail
plan: 01
subsystem: infra
tags: [surfaces-registry, cobra, cli, conformance-gate, codegen]

# Dependency graph
requires:
  - phase: 03-spine-review-and-cli-hardening
    provides: "RulePurgeFilterRequiresScope precedent (registry rule + requirePurgeFilterScope composition template) this plan mirrors"
provides:
  - "RuleSweepScopeOrAllScopesRequired, a declared surfaces.ConditionalRule enforced and published at all three sweep leaves"
  - "requireSweepScope/sweepScopeRule, the single shared composition point cmd/engram/sweep_scope.go carries"
  - "SurfaceFields-divergence precedent for narrowing cobra_usage resolution when Fields alone cannot isolate enforcing commands from exposing-only ones"
affects: [08-02, 08-03, 08-04]

# Actuals (#2632)
actuals:
  tokens: 6714
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SurfaceFields override + explicit whitelist test, for a rule whose enforcing commands cannot be isolated from exposing-only commands by Fields alone"

key-files:
  created:
    - cmd/engram/sweep_scope.go
    - cmd/engram/sweep_scope_test.go
  modified:
    - internal/surfaces/rules.go
    - internal/surfaces/normalize_test.go
    - internal/surfacesgen/main.go
    - cmd/engram/summarize.go
    - cmd/engram/summarize_test.go
    - cmd/engram/spine_review_scan.go
    - cmd/engram/spine_review_verify.go
    - docs-site/src/content/docs/reference/tools.md
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden

key-decisions:
  - "SurfaceFields = Fields + \"dry-run\" narrows cobra_usage resolution to summarize-missing alone, since no field subset of {scope, all-scopes} can isolate the three enforcing leaves from spine-review consolidate/purge, which expose the same pair without enforcing the rule."
  - "The two leaves SurfaceFields cannot reach (spine-review scan, spine-review verify) are pinned by a dedicated whitelist test (TestSweepLeavesUsageStatesRegisteredRule) instead, with a negative half proving consolidate/purge never carry the sentence."
  - "Anchored on docs-site/reference/tools.md alone (not cli.md, not either SKILL.md) — the derived SurfaceDocsSite/SurfaceSkill/SurfaceProtoComment applicability set, confirmed empirically, not by inference from issue #480's prediction."

requirements-completed: [REQ-sweep-scope-rule-registered]

coverage:
  - id: D1
    description: "RuleSweepScopeOrAllScopesRequired is declared once in internal/surfaces/rules.go and composed by all three sweep leaves in both their rejection path and their --all-scopes help text"
    requirement: "REQ-sweep-scope-rule-registered"
    verification:
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
        ref: "internal/surfaces/conformance_test.go#TestSurfaceConformanceProseFiles"
        status: pass
    human_judgment: false
  - id: D2
    description: "Zero hand-rolled occurrences of the removed guard literal remain under cmd/engram/"
    requirement: "REQ-sweep-scope-rule-registered"
    verification:
      - kind: other
        ref: "rg -o -F -- '--scope <scope> or --all-scopes is required' cmd/engram/ | wc -l"
        status: pass
    human_judgment: false
  - id: D3
    description: "task surfaces:gen regeneration is a fixed point and the goldens moved only in the composed --all-scopes Usage text"
    verification:
      - kind: other
        ref: "task surfaces:gen (twice) + shasum comparison of git diff over proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/"
        status: pass
    human_judgment: false
  - id: D4
    description: "The prose surrounding the new anchor in docs-site/reference/tools.md reads as a genuine sentence, not an orphaned fragment"
    verification: []
    human_judgment: true
    rationale: "Plan's must_haves marks this a backstop item — verified by reading the rendered page, not by an assertion. Recorded here for a human to confirm."

duration: 50min
completed: 2026-08-21
status: complete
---

# Phase 8 Plan 1: Register the sweep scope-or-all-scopes rule Summary

**Closed issue #480: the shared `--scope`-or-`--all-scopes` sweep guard is now one declared `surfaces.ConditionalRule` (`RuleSweepScopeOrAllScopesRequired`), composed at all three sweep leaves' rejection path and `--all-scopes` help text, with a `SurfaceFields` divergence that narrows cobra-Usage resolution to `summarize-missing` alone and an explicit whitelist test pinning the two leaves that narrowing cannot reach.**

## Performance

- **Duration:** ~50 min (estimated; start time not independently stamped)
- **Completed:** 2026-08-21T22:08:07Z
- **Tasks:** 3
- **Files modified:** 12 (2 created, 10 modified)

## Accomplishments

- Declared `RuleSweepScopeOrAllScopesRequired` in `internal/surfaces/rules.go`, with `SurfaceFields` diverging from `Fields` to `{"scope", "all-scopes", "dry-run"}` — the one field-set shape that isolates the three enforcing leaves (`spine-review scan`, `spine-review verify`, `summarize-missing`) from the two commands that expose the same flag pair without enforcing it (`spine-review consolidate`, `spine-review purge`).
- Added `cmd/engram/sweep_scope.go`: `sweepScopeRule()` (single registry lookup, panic-on-missing) and `requireSweepScope(scope, allScopes)` (two parameters — no unused `command` param), the one composition point all three leaves and their Usage strings share.
- Converted all three sweep leaves (`summarize.go`, `spine_review_scan.go`, `spine_review_verify.go`) onto the shared helper, in both the `RunE` rejection and the `--all-scopes` flag `Usage` composition. Re-ran the occurrence scan before finalizing Task 3: zero remaining hand-rolled leaves found (the plan's own "three sites, re-verify" caveat).
- Anchored the canonical sentence on the ONE surface `ApplicableSurfaces` derives it to: `docs-site/reference/tools.md`'s `summarize-missing` section (`SurfaceDocsSite`). `SurfaceSkill` and `SurfaceProtoComment` do not resolve — neither SKILL.md mentions `dry-run`, and `all_scopes` is not a proto field on any message. Confirmed empirically (`TestSurfaceConformanceProseFiles`, `TestSurfaceConformanceCobraUsage`), not inferred from issue #480's original three-anchor prediction.
- `cmd/engram/sweep_scope_test.go`: `TestRequireSweepScope` (direct table test, no command invocation), `TestSweepLeavesRejectMissingScopeIdentically`, `TestSweepLeavesRejectPresentButEmptyScope` (message compared against `surfaces.RuleByID`, never inlined), and `TestSweepLeavesUsageStatesRegisteredRule` (the whitelist the field-set model cannot express — positive half for the three enforcers, negative half proving `consolidate`/`purge` never carry the sentence).
- Filed [seanb4t/engram#508](https://github.com/seanb4t/engram/issues/508) for the residual `spine-review consolidate` silently-empty-result gap, deliberately out of this phase's scope (08-CONTEXT.md excludes runtime-behavior changes).
- `task` (lint + full Go/Python suite) is green on the committed tree; `task surfaces:gen` reproduces CI's `surfaces` job cleanly (`git diff --exit-code` over the same path set CI checks).

## Applicability set (recorded as the plan requires)

- `cobra_usage` → `summarize-missing` only (proven by removing its Usage composition and observing exactly one `command=summarize-missing` failure line — never `consolidate`/`purge`/`scan`/`verify`).
- `docs_site` → `tools.md` (one anchored file satisfies `checkProseSurface`'s "found ≥ 1 in the surface's target set" requirement; `cli.md` is a `SurfaceDocsSite` target file but carries no anchor for this rule).
- `skill` → does not resolve (neither `curating-memory/SKILL.md` nor `discovering/SKILL.md` mentions `dry-run`).
- `proto_comment` → does not resolve (`all_scopes` is not a proto field on any message).

## Task Commits

1. **Task 1: Pin the one genuinely uncovered rejection before anything is refactored** - `7ad2bf4c` (test)
2. **Task 2: End-to-end "the sweep constraint is declared once" (tracer)** - `7db3ed49` (feat)
3. **Task 3: Convert the remaining two leaves, pin all three against the registry, close the gate at zero** - `1cc52903` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `cmd/engram/sweep_scope.go` - `sweepScopeRule()`/`requireSweepScope()`, the shared composition point
- `cmd/engram/sweep_scope_test.go` - four tests pinning the three-leaf agreement, the empty-scope edge, and the Usage whitelist
- `internal/surfaces/rules.go` - `RuleSweepScopeOrAllScopesRequired` const + doc comment + struct literal, appended to the tail of `rules`
- `internal/surfaces/normalize_test.go` - added `cobraSummarizeFields` fixture (see Deviations)
- `internal/surfacesgen/main.go` - `ruleTargets` entry for the new rule (tools.md alone)
- `cmd/engram/summarize.go` - guard converted, `--all-scopes` Usage composed
- `cmd/engram/summarize_test.go` - `TestSummarizeMissingRequiresScopeOrAllScopes` characterization pin
- `cmd/engram/spine_review_scan.go` - guard converted, `--all-scopes` Usage composed
- `cmd/engram/spine_review_verify.go` - guard converted, `--all-scopes` Usage composed
- `docs-site/src/content/docs/reference/tools.md` - hand-rolled bold statement replaced with the generated anchor
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` - regenerated (`task surfaces:gen`); diff confined to the composed `--all-scopes` Usage text on both goldens

## Decisions Made

See `key-decisions` in frontmatter. All three were pre-decided by the plan's own decision record (D-01/D-02 and the `SurfaceFields` divergence rationale); execution confirmed each one empirically rather than introducing a new decision.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `internal/surfaces/normalize_test.go`'s synthetic cobra fixture did not cover the new rule's `SurfaceFields`**
- **Found during:** Task 2, immediately after adding the rule literal
- **Issue:** `TestEveryRuleResolvesToNonEmptySurfaceSet` iterates every declared rule against a hand-maintained synthetic `exposedForTest()` fixture (not the live tree). None of that fixture's existing field lists (`cobraSearchListFields`, `cobraDestructiveFields`, `cobraVerifyFields`, `cobraPurgeFields`) include `"dry-run"` — the field this rule's `SurfaceFields` override adds — so the new rule resolved to an empty applicable-surface set against the fixture and the test failed.
- **Fix:** Added a `cobraSummarizeFields` fixture list mirroring `summarize-missing`'s own cobra flag set, following the file's existing per-command-list convention, and unioned it into `exposedForTest()`'s `SurfaceCobraUsage` entry.
- **Files modified:** `internal/surfaces/normalize_test.go`
- **Verification:** `go test ./internal/surfaces -run TestEveryRuleResolvesToNonEmptySurfaceSet -count=1 -v` passes; full `go test ./internal/surfaces/...` green.
- **Committed in:** `7db3ed49` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix to a test fixture the new rule's own `SurfaceFields` override made stale)
**Impact on plan:** Necessary for `internal/surfaces`'s existing sanity-check test to keep passing after adding a rule with a `SurfaceFields` override. No scope creep — the fixture file was already a listed sibling of the package under change, not a new surface.

## Issues Encountered

**Task 1's RED-defeat check did not distinguish the guard's absence from a downstream construction error, in this environment.** Task 1's acceptance criteria ask for confirming the pinned test goes RED when `summarize.go`'s guard is temporarily defeated (returns nil unconditionally). Observed: it stayed GREEN. Root cause — with the guard removed, `RunE` falls through to `server.StoreAndSummarizerFromEnv()`, which fails with an unrelated "ENGRAM_SUMMARY_MODEL is empty" construction error; `classifyOperatorErrConstruction`'s catch-all wraps ANY unclassified construction error in `usageErrorf`, which coincidentally also yields `exitUsage`. Verified this is not new to this task: the identical property holds for the pre-existing `TestSpineReviewScanRequiresScopeOrAllScopes` (same shape, same catch-all path) when its sibling guard is defeated the same way. Out of scope to fix — it is a pre-existing characteristic of the whole test family's exit-code-only assertion style, not something Task 1 introduced. Recorded here rather than silently treating the unmet acceptance criterion as satisfied.

Task 2/3's own RED-defeat checks (registry substring collision, prose corruption, Usage-composition removal on both `spine_review_verify.go` and `spine_review_consolidate.go`, and the guard-defeat checks in `spine_review_scan.go`) all produced the expected distinguishing signal — the `summarize.go` case above is the one exception, and only because `summarize.go`'s downstream path (unlike `spine_review_scan.go`'s, which reaches a real dial-refused error under this environment's Qdrant-address defaults) hits a config-validation error path that happens to classify identically.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for 08-02. `RuleSweepScopeOrAllScopesRequired` and the `SurfaceFields`-divergence pattern are available as precedent for any later plan in this phase that needs to narrow cobra_usage resolution the same way. Issue #480 is closable; issue #508 (the `spine-review consolidate` residual) is filed and open, tracked outside this milestone's scope per 08-CONTEXT.md.

---
*Phase: 08-registry-docs-tail*
*Completed: 2026-08-21*

## Self-Check: PASSED

- All `key-files.created`/`key-files.modified` confirmed present on disk (`[ -f ]`).
- All three task commit hashes (`7ad2bf4c`, `7db3ed49`, `1cc52903`) confirmed in `git log`.
- Plan-level `<verification>` re-run against the committed tree: `go test ./cmd/engram/... ./internal/surfaces/...` green, zero-occurrence gate reports `0`, `task surfaces:gen` reproduces clean, `task` (lint + full suite) green, `go test ./internal/keylinks/...` green, `git diff --exit-code go.mod go.sum` exits 0, internal links on the edited page resolve.
