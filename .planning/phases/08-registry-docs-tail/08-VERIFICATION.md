---
phase: 08-registry-docs-tail
verified: 2026-08-22T01:15:00Z
status: passed
score: 12/12 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 8/12
  gaps_closed:
    - "Zero hand-rolled occurrences of the removed sweep-scope guard literal, protected by a durable gate (truth 2) — TestNoHandRolledSweepScopeGuards added in 08-06, runs inside `go test ./...`"
    - "internal/surfaces/rules.go's doc comment for RuleSweepScopeOrAllScopesRequired accurately describes its own enforcement (truth 5) — retired present-tense envelope claim removed in 08-06, replaced with a comment gated against code"
    - "CLAUDE.md's cmd/engram/ Layout row marks every deprecated command consistently (truth 11) — backfill-short-ids annotated in 08-05"
    - "CLAUDE.md's Archived-state paragraph names all four soft-hide recall surfaces (truth 12) — search_discovery/list_scheduled added in 08-05"
  gaps_remaining: []
  regressions: []
deferred: []
coincidental_reliance_items: []
---

# Phase 8: Registry & docs tail Verification Report

**Phase Goal:** The shared scope-or-all-scopes guard is a registered, conformance-gated rule instead of a hand-rolled check, and the docs plus CLAUDE.md describe what this milestone actually shipped instead of what it superseded.
**Verified:** 2026-08-22
**Status:** passed
**Re-verification:** Yes — after gap closure (plans 08-05, 08-06)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `RuleSweepScopeOrAllScopesRequired` is declared once in `internal/surfaces/rules.go` and composed by all three sweep leaves (`spine-review scan`, `spine-review verify`, `summarize-missing`) in both their `RunE` rejection and `--all-scopes` Usage string | ✓ VERIFIED | `go test ./cmd/engram -run 'TestRequireSweepScope\|TestSweepLeavesRejectMissingScopeIdentically\|TestSweepLeavesRejectPresentButEmptyScope\|TestSweepLeavesUsageStatesRegisteredRule\|TestSurfaceConformanceCobraUsage\|TestEveryScopeAllScopesPairHasAFlagGroup' -count=1 -v` — re-run independently this pass, all PASS (no regression from 08-06's rewiring onto shared `enforcingSweepLeaves`/`nonEnforcingSweepLeaves` classification) |
| 2 | Zero hand-rolled occurrences of the removed guard literal remain under `cmd/engram/`, protected by a gate that asserts zero rather than a one-time count | ✓ VERIFIED | `TestNoHandRolledSweepScopeGuards` (added 08-06) exists in `cmd/engram/sweep_scope_test.go`, runs under plain `go test ./...` (`task test:go`'s exact command, no build tags), and is genuinely non-vacuous — I independently constructed a scratch cobra command exposing `--scope`+`--all-scopes` with its own inline guard, confirmed the gate FAILS and names the command (`zz-verifier-redproof: exposes --scope and --all-scopes but is in neither enforcingSweepLeaves nor nonEnforcingSweepLeaves`), then removed it and confirmed PASS. Corroborated independently by 08-REVIEW.md's own mutation test (removing `summarize-missing` from the classification, and adding a stale key, both in the other direction) |
| 3 | The rule's `SurfaceFields` correctly narrows cobra-Usage resolution to the three enforcing leaves and provably excludes `spine-review consolidate`/`spine-review purge`, which expose the same flags without enforcing | ✓ VERIFIED | `TestSweepLeavesUsageStatesRegisteredRule` positive+negative halves pass; `TestSurfaceConformanceCobraUsage` passes (re-run, no regression). (Quality note, unchanged: this proof lives only at the CLI-package level — 08-REVIEW.md WR-04 — a warning, not a functional gap.) |
| 4 | The anchored region in `docs-site/reference/tools.md` is byte-identical to the registered `Sentence`, and the goldens/generator reproduce cleanly | ✓ VERIFIED | `rg -o -F 'engram:rule:start sweep-scope-or-all-scopes-required' docs-site/src/content/docs/reference/tools.md` → 1; `task surfaces:gen` followed by `git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/` clean (re-run this pass) |
| 5 | `internal/surfaces/rules.go`'s doc comment for the new rule accurately describes its own enforcement (no false present-tense operational claim) | ✓ VERIFIED | Retired claim (`it drives field=scope attribution`) count is `0` (was `1`). Replacement comment (read whole) states the rule is CLI-only, names `requireSweepScope`/bare `usageErrorf` as the sole enforcement site, and states `Fields`/`Hint` are inert today. Independently confirmed: `rg -o 'surfaces\.RuleSweepScopeOrAllScopesRequired' internal/server/ -g '!*_test.go'` → 0; control rule `RuleScopeRequiredUnlessCrossSpine` → 3 (non-vacuity proof the zero isn't a typo'd rule name); `cmd/engram/sweep_scope.go` raises exactly 1 `usageErrorf`. Corroborated by 08-REVIEW.md WR-02 |
| 6 | `reference/memory-record.md`'s field-reference table covers all 8 wire-visible keys the store's `Memory` struct carries, proven by set difference | ✓ VERIFIED | Unchanged by gap-closure plans (file not touched by 08-05/08-06); regression spot-check reconfirms no stale-claim regression |
| 7 | The false "Connect lane lacks record-state fields" paragraph and its tracker citation are removed from `reference/memory-record.md` | ✓ VERIFIED | `rg -o -F 'does not carry \`superseded_by\`'` and `rg -o 'github.com/seanb4t/engram/issues'` both `0` (re-run) |
| 8 | `reference/tools.md`'s `get_memory` state list is corrected: off-by-one `expired` wording fixed, canonical order, `schema_version` added | ✓ VERIFIED | `rg -o -F '\`not_after\` in the past'` → `0` (re-run) |
| 9 | `docs-site/guides/migrate.md` exists as a standalone evergreen guide, scoped strictly to the schema-version mechanism | ✓ VERIFIED | File exists, 313 lines, unchanged by gap-closure plans |
| 10 | CLAUDE.md's migrations Conventions bullet states the shipped mechanism and scope boundary | ✓ VERIFIED | `rg -o 'database migrations, viper, cocogitto' CLAUDE.md` → `0` (re-run; untouched by 08-05's edits, which were scoped to the Layout row and Archived-state paragraph only) |
| 11 | CLAUDE.md's `cmd/engram/` Layout row names every command the live catalog exposes, tier-split, with deprecated aliases marked consistently | ✓ VERIFIED | `backfill-short-ids` now reads `` `backfill-short-ids` (deprecated, use `migrate`) `` alongside the pre-existing `migrate-remap-owner (alias: \`migrate-set-owner\`, deprecated)`. Confirmed via direct file read; `backfillShortIDsCmd.Deprecated = "use: engram migrate"` at `cmd/engram/backfill.go:58` is the code fact this annotation now correctly reflects. Corroborated by 08-REVIEW.md WR-09 |
| 12 | CLAUDE.md's Memory contract section names `schema_version` and the archived state, in agreement with `reference/memory-record.md`'s wording | ✓ VERIFIED | Archived-state paragraph now reads `` `search_memory`/`list_memory`/`search_discovery`/`list_scheduled` `` — all four surfaces, matching the adjacent Supersession paragraph's set exactly (confirmed via direct read of both paragraphs) and matching the store's live gate-site count: `rg -v '^\s*//' internal/store/store.go \| rg -o 'qdrant\.NewIsEmpty("archived_at")' \| wc -l` → `4` (independently re-derived, comment-stripped). Corroborated by 08-REVIEW.md WR-07 |

**Score:** 12/12 truths verified. All 4 gaps from the prior verification pass (truths 2, 5, 11, 12) are closed and independently re-derived from source, not merely re-read from the SUMMARYs. No regressions found in the 8 previously-passing truths.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/surfaces/rules.go` | Declares `RuleSweepScopeOrAllScopesRequired`, accurate doc comment | ✓ VERIFIED | Const + struct literal unchanged (0 non-comment diff lines per 08-06's own gate, re-confirmed by reading the file); doc comment rewritten, claims match code |
| `cmd/engram/sweep_scope.go` | Single composition point | ✓ VERIFIED | Untouched by gap closure; 1 `usageErrorf` call site confirmed |
| `cmd/engram/sweep_scope_test.go` | Package-level classification + zero-occurrence gate | ✓ VERIFIED | `enforcingSweepLeaves`/`nonEnforcingSweepLeaves` declared once; `TestNoHandRolledSweepScopeGuards` present, passes, independently reproduced RED against a constructed defect |
| `internal/surfacesgen/main.go` | `ruleTargets` entry | ✓ VERIFIED | Unchanged; `task surfaces:gen` clean |
| `docs-site/.../memory-record.md` | Complete field contract | ✓ VERIFIED | Unchanged by gap closure, regression-checked |
| `docs-site/.../tools.md` | `schema_version` in `get_memory`, corrected state list | ✓ VERIFIED | Unchanged, regression-checked |
| `docs-site/.../guides/migrate.md` | Evergreen operator guide | ✓ VERIFIED | Unchanged, 313 lines, exists |
| `docs-site/.../guides/upgrade.md` | Corrected release note | ✓ VERIFIED | Unchanged |
| `CLAUDE.md` | Routing doc brought current | ✓ VERIFIED | Both confirmed gaps (Layout row deprecation marking, Archived-state surface list) repaired; `migrate-set-owner` wording and line 97 left byte-identical to `HEAD` as required |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `cmd/engram/summarize.go` | `cmd/engram/sweep_scope.go` | `requireSweepScope(` call | ✓ WIRED | Unchanged, re-confirmed |
| `cmd/engram/sweep_scope.go` | `internal/surfaces/rules.go` | `surfaces.RuleByID(...)` | ✓ WIRED | Unchanged |
| `internal/surfacesgen/main.go` | `internal/surfaces/rules.go` | `ruleTargets` | ✓ WIRED | Unchanged |
| `docs-site/reference/tools.md` | `docs-site/reference/memory-record.md` | link | ✓ WIRED | Unchanged |
| `docs-site/guides/upgrade.md` | `docs-site/guides/migrate.md` | link | ✓ WIRED | Unchanged |
| `CLAUDE.md` | `docs-site/guides/migrate.md` | reference | ✓ WIRED | Unchanged |
| `cmd/engram/sweep_scope_test.go` | `internal/surfaces/rules.go` | `surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)` | ✓ WIRED | New in 08-06; confirmed present, tests resolve the rule through the registry rather than an inlined literal |
| `cmd/engram/sweep_scope_test.go` | `cmd/engram/cmdwalk.go` | `walkCommands(rootCmd, commandWalkSkip)` | ✓ WIRED | New in 08-06; `sweepScopeFlagPairCommands()` confirmed to use the shared live-tree walker, not a hand-listed set |
| `CLAUDE.md` | `cmd/engram/backfill.go` | Layout row names `backfill-short-ids`, whose `.Deprecated` field is live | ✓ WIRED | New in 08-05; confirmed `backfillShortIDsCmd.Deprecated` at `backfill.go:58` matches the row's annotation |
| `CLAUDE.md` | `docs-site/.../memory-record.md` | shared four-surface recall-gate vocabulary | ✓ WIRED | New in 08-05; confirmed both pages now state the identical surface set |

### Data-Flow Trace (Level 4)

Not applicable — documentation/registry-conformance work with no rendered dynamic data path.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `TestNoHandRolledSweepScopeGuards` exists and passes on current tree | `go test ./cmd/engram -run TestNoHandRolledSweepScopeGuards -count=1 -v` | `--- PASS: TestNoHandRolledSweepScopeGuards` | ✓ PASS |
| Gate is non-vacuous: fails against a deliberately constructed unclassified leaf | Independently authored scratch `_test.go` registering `zz-verifier-redproof` (both `--scope`+`--all-scopes`, own inline guard); ran gate, deleted scratch, re-ran | FAIL (named the command, actionable message) → removed → PASS; `git status --porcelain` clean afterward | ✓ PASS |
| Full sweep-leaf test suite (regression) | `go test ./cmd/engram -run 'TestRequireSweepScope\|TestSweepLeavesRejectMissingScopeIdentically\|TestSweepLeavesRejectPresentButEmptyScope\|TestSweepLeavesUsageStatesRegisteredRule\|TestSurfaceConformanceCobraUsage\|TestEveryScopeAllScopesPairHasAFlagGroup' -count=1 -v` | All PASS, no `--- FAIL` | ✓ PASS |
| Package regression: `internal/surfaces`, `internal/keylinks` | `go test ./internal/surfaces/... ./internal/keylinks/... -count=1` | `ok` both packages | ✓ PASS |
| `task surfaces:gen` reproduces cleanly | `task surfaces:gen; git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/` | clean | ✓ PASS |
| `task lint` clean | `task lint` | `All checks passed!` | ✓ PASS |
| Working tree clean (no stray phase changes) | `git status --porcelain` (scoped to phase files) | only unrelated phase-06 review iteration files untracked, no phase-08 file dirty | ✓ PASS |

**Note on Go toolchain output:** the `--- PASS: <TestName>` / `--- FAIL: <TestName>` line was used as the anti-vacuity evidence throughout, rather than `=== RUN`. It only appears when the specific named test actually executed, confirmed via the `-run <NonexistentName>` control (which prints `testing: warning: no tests to run` / `[no tests to run]` and no `--- PASS`/`--- FAIL` line for any name), so the anti-vacuity property the plans' gates were designed to prove is fully preserved. CORRECTION (orchestrator, post-verification): an earlier draft of this note claimed go1.26.7 does not emit `=== RUN` lines at all. That is false. go1.26.7 emits them normally — all six for a 5-subtest test, verified by redirecting the identical pipeline to a file and reading it back, and by `wc -l` reporting 14 lines where only 7 were visible. The lines are stripped in the rendering of command stdout back to the agent, not by the toolchain. Do not infer a tool's behavior from what appears in captured output; redirect to a file and read the file. See engram memory `t4aq8704ss`.

### Probe Execution

Not applicable — no `scripts/*/tests/probe-*.sh` files for this phase.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| REQ-sweep-scope-rule-registered | 08-01 (+ 08-06 gap closure) | Sweep guard registered as a `surfaces.ConditionalRule`, conformance-gated | ✓ SATISFIED | All sub-truths (1-5) verified; the durable zero-occurrence gate (truth 2) and the accurate doc comment (truth 5) — both previously failing — are now closed and independently re-derived |
| REQ-docs-record-state | 08-02 + 08-03 | `memory-record.md`/`tools.md` document full record state; migration guide exists | ✓ SATISFIED | All sub-truths (6-9) verified, unchanged by this wave, regression-checked |
| REQ-claude-md-migrations-convention | 08-04 (+ 08-05 gap closure) | CLAUDE.md migrations convention revised | ✓ SATISFIED | All sub-truths (10-12) verified; the Layout row deprecation marking (truth 11) and Archived-state surface list (truth 12) — both previously failing — are now closed and independently re-derived |

No orphaned requirements — `.planning/REQUIREMENTS.md`'s Phase 8 mapping lists exactly these three, all marked `[x]` Complete, and all three appear in a plan's `requirements:` frontmatter (08-01/08-06 and 08-04/08-05 both correctly declare `REQ-sweep-scope-rule-registered` and `REQ-claude-md-migrations-convention` respectively on their gap-closure plans).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers found in phase-touched files (`CLAUDE.md`, `internal/surfaces/rules.go`, `cmd/engram/sweep_scope_test.go`, `cmd/engram/sweep_scope.go`) | — | — | — | The one `TODO` hit in `CLAUDE.md` is prose ("Do not use markdown TODO lists for durable tracking"), not a debt marker |
| Remaining open findings from 08-REVIEW.md (WR-01, WR-04, WR-05, WR-06, WR-10, IN-01 through IN-08) | various | Polish/documentation-completeness items, unchanged since the prior review pass | ℹ️ Info/⚠️ Warning (non-blocking) | None target a must-have truth for this phase or a roadmap success criterion; all are either pre-existing (present before gap closure, already excluded from the 4 prior FAILED truths) or explicitly downgraded-to-info wording notes (WR-08/IN-08, the `migrate-set-owner` "alias" phrasing — correctly classified as info-only per the known-false-positive guidance; the underlying deprecated-supersession fact is real and pinned by `TestMigrateSetOwnerEquivalentToRemapOwnerMissing`) |

### Human Verification Required

None. All 12 truths were resolved by direct, independently-reproduced code/doc inspection and test execution (including an original mutation test I constructed myself for the durable gate) — no item was left uncertain.

### Gaps Summary

No gaps remain. All four truths that failed in the prior verification pass are now closed:

1. **Truth 2 (durable zero-occurrence gate)** — `TestNoHandRolledSweepScopeGuards` was added in 08-06, runs inside plain `go test ./...` (the exact command `task test:go` and CI already execute, no new Taskfile target or CI step), and is genuinely non-vacuous. I did not trust the plan's own RED-proof transcript — I independently authored a second, differently-named scratch defect and reproduced the RED→GREEN cycle myself.
2. **Truth 5 (rules.go doc comment accuracy)** — the retired present-tense error-envelope claim is gone (0 occurrences, confirmed), and the replacement's factual claims are independently gated: 0 references to the rule's const in non-test `internal/server/`, vs. 3 for a control rule that genuinely has an envelope-bearing lane; exactly 1 `usageErrorf` in the sole enforcement site.
3. **Truth 11 (CLAUDE.md Layout row deprecation marking)** — `backfill-short-ids` now carries `(deprecated, use \`migrate\`)`, matching the code fact (`backfillShortIDsCmd.Deprecated` at `backfill.go:58`) and the annotation density already used for its sibling.
4. **Truth 12 (CLAUDE.md Archived-state recall-gate scope)** — the paragraph now names all four soft-hide surfaces, matching both the adjacent Supersession paragraph (byte-for-byte set) and the store's live gate-site count (independently re-derived as 4, comment-stripped).

No regressions were introduced in the 8 previously-passing truths, and `task` (lint) plus the full relevant Go test surface (`cmd/engram`, `internal/surfaces`, `internal/keylinks`) are clean. The working tree carries no uncommitted phase-08 changes. Remaining review findings (WR-01/04/05/06/10, IN-01 through IN-08) are pre-existing polish items that do not map to any must-have truth or roadmap success criterion for this phase — they are appropriate backlog material, not phase blockers.

---

*Verified: 2026-08-22*
*Verifier: Claude (gsd-verifier)*
