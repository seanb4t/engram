---
phase: 08-registry-docs-tail
verified: 2026-08-21T23:40:00Z
status: gaps_found
score: 8/12 must-haves verified
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "CLAUDE.md's cmd/engram/ Layout row is internally consistent about deprecation: it marks migrate-set-owner as deprecated but omits that backfill-short-ids is also deprecated in the live cobra tree"
    status: failed
    reason: "backfillShortIDsCmd.Deprecated = \"use: engram migrate\" (cmd/engram/backfill.go:58) is real, live deprecation, and both guides/migrate.md and guides/cli.md describe backfill-short-ids as a thin delegating alias onto engram migrate. The Layout row (CLAUDE.md line 15) explicitly annotates the OTHER deprecated verb (migrate-set-owner) with a 'deprecated' marker but lists backfill-short-ids as an ordinary, current operator command. Given the row's own stated purpose is routing an agent to the right verb, and it already demonstrates the annotation pattern for one deprecated command, the omission reads as a positive (and false) assertion that backfill-short-ids is current."
    artifacts:
      - path: "CLAUDE.md"
        issue: "Layout table cmd/engram/ row (line 15) marks migrate-set-owner deprecated but not backfill-short-ids"
    missing:
      - "Mark backfill-short-ids as deprecated (delegates to engram migrate) in the Layout row, matching the treatment already given to migrate-set-owner"
  - truth: "internal/surfaces/rules.go's doc comment for RuleSweepScopeOrAllScopesRequired accurately describes its own enforcement (no false present-tense operational claim)"
    status: failed
    reason: "The comment at rules.go:167-168 states, in the present tense, that Fields ('the flag pair alone') 'drives field=scope attribution on the error envelope.' No conditionalErrf call site exists for RuleSweepScopeOrAllScopesRequired anywhere in the tree -- internal/server/tools.go and connectapi.go reference five other rules via conditionalErrf, none is this one. The sole enforcement site is requireSweepScope (cmd/engram/sweep_scope.go), which returns a bare usageErrorf(\"%s\", ...) carrying no field=/hint= envelope. TagForm is deliberately left empty for the same reason (no MCP/Connect lane), and Hint: \"conditional_required\" is equally inert. The claim is inherited boilerplate from the general Fields field-doc comment on the ConditionalRule type, restated in this rule's own comment as if operationally true for it, when it is not. rules.go is the single declared source of truth every other surface composes from; a false present-tense claim there will be believed by the next contributor wiring a lane for a sweep verb. Independently confirmed by 08-REVIEW.md WR-02."
    artifacts:
      - path: "internal/surfaces/rules.go"
        issue: "RuleSweepScopeOrAllScopesRequired's doc comment (lines 167-168) asserts the rule drives field=scope error-envelope attribution via conditionalErrf, but no such call site exists for this rule -- it is CLI-only and enforced via bare usageErrorf"
    missing:
      - "Reword the comment to state the conditional explicitly, e.g.: this rule is CLI-only, the sole enforcement site (requireSweepScope) raises a bare usageErrorf with no field=/hint= envelope today, and Fields/Hint are declared for a future conditionalErrf lane rather than a live surface"
  - truth: "CLAUDE.md's Memory contract Archived-state paragraph agrees with reference/memory-record.md's statement of the same recall-gate fact, per plan 08-04's own acceptance criterion requiring the two pages to use the same vocabulary with no place where CLAUDE.md's compression diverges"
    status: failed
    reason: "CLAUDE.md's new Archived-state paragraph states: 'an archived record drops out of `search_memory`/`list_memory` but stays reachable by id via `get_memory`' -- naming only 2 recall surfaces. qdrant.NewIsEmpty(\"archived_at\") is applied at FOUR recall-gate sites in internal/store/store.go: Search (:1129), SearchDiscovery (:1228), List (:1399), and the scheduled-records listing (:1635) -- confirmed by the file's own comment, 'IsEmpty(\"archived_at\")) appears at four sites in this file.' docs-site/src/content/docs/reference/memory-record.md (written by the sibling plan 08-02 in this same phase) correctly states all four: 'soft-hidden from recall (`search_memory`, `list_memory`, `search_discovery`, `list_scheduled`)'. CLAUDE.md's own adjacent Supersession paragraph (pre-existing, two paragraphs above) also correctly names all four for the identical superseded_by gate. The Archived-state paragraph is new content this phase (08-04 Task 2) wrote and was instructed to cross-check against reference/memory-record.md's wording; the SUMMARY reports full agreement, but the two-vs-four-surface gap is a real, confirmed factual understatement introduced by this phase."
    artifacts:
      - path: "CLAUDE.md"
        issue: "Archived-state paragraph in the Memory contract section names only search_memory/list_memory as recall surfaces archived_at hides from, omitting search_discovery and list_scheduled"
    missing:
      - "State all four soft-hide surfaces (search_memory, list_memory, search_discovery, list_scheduled) for archived_at, matching reference/memory-record.md and the file's own Supersession paragraph"
  - truth: "Zero hand-rolled occurrences of the removed sweep-scope guard literal remain under cmd/engram/, protected by a gate that asserts zero rather than a one-time count, so a fourth sweep leaf reintroducing the literal fails loudly (plan 08-01's own must_have truth #2 wording)"
    status: failed
    reason: "The literal is confirmed absent today (rg -o -F -- '--scope <scope> or --all-scopes is required' cmd/engram/ reports 0). But no automated regression mechanism enforces this going forward: there is no Go test (cmd/engram/sweep_scope_test.go's four tests are a hand-maintained enforcing/non-enforcing whitelist, not a walk-the-tree zero-occurrence check), no Taskfile target, and no CI job that runs this rg command or an equivalent. It was verified exactly once, manually, as a plan-execution acceptance-criteria step, and the result is recorded in the SUMMARY -- but nothing in `task` or CI reruns it. A fourth sweep-style leaf added later with its own inline hand-rolled guard would not be caught by TestSurfaceConformanceCobraUsage either, because the SurfaceFields narrowing to {scope, all-scopes, dry-run} resolves a leaf without --dry-run as not-applicable. This is the exact vacuous-gate shape this repo's verification notes call out as its #1 recurring defect, and it was independently confirmed by the phase's own code review (08-REVIEW.md WR-03)."
    artifacts:
      - path: "cmd/engram/sweep_scope_test.go"
        issue: "TestSweepLeavesUsageStatesRegisteredRule whitelists three enforcing and two non-enforcing commands; no test asserts zero commands outside both sets expose --scope+--all-scopes with a non-registry rejection"
    missing:
      - "A Go test (or equivalent CI step) that walks the live command tree and asserts zero commands exposing both --scope and --all-scopes reject with anything other than the registered Sentence, per 08-REVIEW.md WR-03's suggested TestNoHandRolledSweepScopeGuards"
deferred: []
coincidental_reliance_items: []
---

# Phase 8: Registry & docs tail Verification Report

**Phase Goal:** The shared scope-or-all-scopes guard becomes a registered conditional rule (#480); docs and CLAUDE.md brought current with what this milestone actually ships
**Verified:** 2026-08-21
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `RuleSweepScopeOrAllScopesRequired` is declared once in `internal/surfaces/rules.go` and composed by all three sweep leaves (`spine-review scan`, `spine-review verify`, `summarize-missing`) in both their `RunE` rejection and `--all-scopes` Usage string | ✓ VERIFIED | `go test ./cmd/engram -run 'TestRequireSweepScope\|TestSweepLeavesRejectMissingScopeIdentically\|TestSweepLeavesRejectPresentButEmptyScope\|TestSweepLeavesUsageStatesRegisteredRule\|TestSurfaceConformanceCobraUsage\|TestEveryScopeAllScopesPairHasAFlagGroup' -count=1 -v` — all named tests RUN and PASS (matched, not vacuous: named-subtest output confirmed per leaf) |
| 2 | Zero hand-rolled occurrences of the removed guard literal remain under `cmd/engram/`, protected by a gate that asserts zero rather than a one-time count | ✗ FAILED | Literal count is `0` today, but no automated test/CI step re-runs the check — see gaps. Corroborated by 08-REVIEW.md WR-03. |
| 3 | The rule's `SurfaceFields` correctly narrows cobra-Usage resolution to the three enforcing leaves and provably excludes `spine-review consolidate`/`spine-review purge`, which expose the same flags without enforcing | ✓ VERIFIED | `TestSweepLeavesUsageStatesRegisteredRule` positive+negative halves pass; `TestSurfaceConformanceCobraUsage` passes. (Quality note: this proof lives only at the CLI-package level, not as a package-level `internal/surfaces` unit test — 08-REVIEW.md WR-04 — a warning, not a functional gap.) |
| 4 | The anchored region in `docs-site/reference/tools.md` is byte-identical to the registered `Sentence`, and the goldens/generator reproduce cleanly | ✓ VERIFIED | `test "$(rg -o -F 'engram:rule:start sweep-scope-or-all-scopes-required' docs-site/src/content/docs/reference/tools.md \| wc -l)" = "1"`; SUMMARY records `task surfaces:gen` fixed-point and clean `git diff --exit-code` over the CI path set |
| 5 | `internal/surfaces/rules.go`'s doc comment for the new rule accurately describes its own enforcement (no false present-tense operational claim) | ✗ FAILED | Comment at rules.go:167-168 states `Fields ... drives field=scope attribution on the error envelope` in the present tense. No `conditionalErrf` call site exists for `RuleSweepScopeOrAllScopesRequired` anywhere in the tree (`rg -n conditionalErrf` shows only 5 call sites in `internal/server`, none for this rule); the sole enforcement site (`sweep_scope.go`) uses bare `usageErrorf`, which carries no `field=`/`hint=` envelope. The claim is inherited boilerplate from the general `Fields` doc comment, inoperative for this specific rule as deployed. Independently confirmed by 08-REVIEW.md WR-02. |
| 6 | `reference/memory-record.md`'s field-reference table covers all 8 wire-visible keys the store's `Memory` struct carries, proven by set difference | ✓ VERIFIED | `comm -23` derived-set-difference command (section-bounded to `## Field reference`) run directly against the current tree: empty output |
| 7 | The false "Connect lane lacks record-state fields" paragraph and its tracker citation are removed from `reference/memory-record.md` | ✓ VERIFIED | `rg -o -F 'does not carry \`superseded_by\`'` and `rg -o 'github.com/seanb4t/engram/issues'` both report `0` against the current file |
| 8 | `reference/tools.md`'s `get_memory` state list is corrected: off-by-one `expired` wording fixed, canonical order (`archived, superseded, expired, scheduled`), `schema_version` added | ✓ VERIFIED | `rg -o -F '\`not_after\` in the past'` reports `0`; order-extraction command reproduces `archived superseded expired scheduled `; `schema_version` present in the `get_memory` section |
| 9 | `docs-site/guides/migrate.md` exists as a standalone evergreen guide, scoped strictly to the schema-version mechanism, with every json key of the three CLI report structs (plus Connect's `pending`) covered and all internal links resolving | ✓ VERIFIED | File exists, 313 lines; derived json-key set difference against `migrateOutputDoc`/`migrateStatusReportDoc`/`revertOutputDoc` is empty; scope-boundary grep (`migrate-remap-owner`, `summarize-missing`) reports `0` matches on the page |
| 10 | CLAUDE.md's migrations Conventions bullet states the shipped mechanism, both automation-contract halves, and the deliberate scope boundary, replacing the false "database migrations" exclusion | ✓ VERIFIED | `rg -o 'database migrations, viper, cocogitto' CLAUDE.md` reports `0`; boundary and automation sentences read and confirmed present and accurate against `internal/server/tools.go`'s `warnPendingMigrations` and `.planning/REQUIREMENTS.md`'s Out of Scope table |
| 11 | CLAUDE.md's `cmd/engram/` Layout row names every command the live catalog exposes, tier-split, with deprecated aliases marked consistently | ✗ FAILED | Row-scoped inventory gate is empty (all 23 catalog commands present) and the `deprecated` marker is present — but the marker is applied inconsistently: `migrate-set-owner` is flagged deprecated while the equally-deprecated `backfill-short-ids` (`backfill.go:58`) is not. See gaps. Corroborated by 08-REVIEW.md WR-09. |
| 12 | CLAUDE.md's Memory contract section names `schema_version` and the archived state, in agreement with `reference/memory-record.md`'s wording, with no place where CLAUDE.md's compression diverges (plan 08-04's own acceptance criterion) | ✗ FAILED | `schema_version`, `archived_at`, `spine-review archive`, and all four state words are present in-section (mechanical gate passes) — but the Archived-state paragraph understates the recall-gate scope (2 of 4 surfaces named) where `reference/memory-record.md` states all 4 correctly. See gaps. Corroborated by 08-REVIEW.md WR-07. |

**Score:** 8/12 truths verified (12 truths derived from the phase's roadmap success criteria and the four plans' must_haves; 3 of the 4 failures cluster under REQ-claude-md-migrations-convention and REQ-sweep-scope-rule-registered)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/surfaces/rules.go` | Declares `RuleSweepScopeOrAllScopesRequired` | ✓ VERIFIED | Const + struct literal present, appended to tail of `rules`; doc comment contains one inaccurate present-tense claim (see truth 5) |
| `cmd/engram/sweep_scope.go` | Single composition point (`sweepScopeRule`, `requireSweepScope`) | ✓ VERIFIED | Exists, 2-parameter signature as planned, single `RuleByID` lookup site |
| `cmd/engram/sweep_scope_test.go` | Three-leaf agreement + empty-scope + Usage whitelist tests | ✓ VERIFIED (partial coverage gap) | All four named tests exist and pass; does not include a zero-occurrence/unknown-leaf gate (see gaps) |
| `internal/surfacesgen/main.go` | `ruleTargets` entry for the new rule | ✓ VERIFIED | Entry present, keyed by the const, targeting `tools.md` only |
| `docs-site/src/content/docs/reference/memory-record.md` | Complete field contract, window boundary, schema-version narrowing | ✓ VERIFIED | All 8 rows present; boundary sentences correct against `activeWindowConditions`; Connect-lane paragraph corrected |
| `docs-site/src/content/docs/reference/tools.md` | `schema_version` in `get_memory`, corrected state list | ✓ VERIFIED | Confirmed by direct grep/extraction against current file |
| `docs-site/src/content/docs/guides/migrate.md` | Evergreen operator guide (NEW) | ✓ VERIFIED | Exists, substantive (313 lines), scoped correctly, json-key gate empty |
| `docs-site/src/content/docs/guides/upgrade.md` | Corrected schema-version release note | ✓ VERIFIED | Stale-claim greps report `0`; links to `/guides/migrate/` |
| `CLAUDE.md` | Routing doc brought current | ⚠️ PARTIAL | Migrations bullet and tier-split largely accurate; Layout row and Memory contract both carry a confirmed factual/consistency gap (see truths 11–12) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `cmd/engram/summarize.go` | `cmd/engram/sweep_scope.go` | `requireSweepScope(` call | ✓ WIRED | `rg -o 'requireSweepScope\('` reports `1` in `summarize.go`; same pattern confirmed in `spine_review_scan.go`/`spine_review_verify.go` |
| `cmd/engram/sweep_scope.go` | `internal/surfaces/rules.go` | `surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)` | ✓ WIRED | One lookup site in `sweep_scope.go`, shared by helper and Usage compositions |
| `internal/surfacesgen/main.go` | `internal/surfaces/rules.go` | `ruleTargets` keyed by const | ✓ WIRED | Confirmed present |
| `docs-site/reference/tools.md` | `docs-site/reference/memory-record.md` | `/reference/memory-record/` link | ✓ WIRED | Link present per 08-02 SUMMARY; internal-link loop reported clean |
| `docs-site/guides/upgrade.md` | `docs-site/guides/migrate.md` | `/guides/migrate/` link | ✓ WIRED | `rg -o '[/]guides[/]migrate[/]'` ≥ 1 on `upgrade.md`; target file exists |
| `CLAUDE.md` | `docs-site/guides/migrate.md` | `guides/migrate` reference | ✓ WIRED | `rg -o 'guides[/]migrate' CLAUDE.md` reports `2`; target exists |

### Data-Flow Trace (Level 4)

Not applicable — this phase is documentation/registry-conformance work with no rendered dynamic data path to trace. The relevant "data flow" is registry value → composed surface, verified above via direct grep against the composed values (not a static/hardcoded fallback).

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All three sweep leaves reject identically when neither `--scope` nor `--all-scopes` is given | `go test ./cmd/engram -run TestSweepLeavesRejectMissingScopeIdentically -v` | PASS/PASS/PASS (3 subtests) | ✓ PASS |
| Present-but-empty `--scope` treated as absent at all three leaves | `go test ./cmd/engram -run TestSweepLeavesRejectPresentButEmptyScope -v` | PASS/PASS/PASS (3 subtests) | ✓ PASS |
| `--all-scopes` Usage carries the registered Sentence on enforcing leaves only | `go test ./cmd/engram -run TestSweepLeavesUsageStatesRegisteredRule -v` | PASS | ✓ PASS |
| Field-reference table completeness (derived set difference) | `comm -23 <(...) <(...)` | empty output | ✓ PASS |
| Migration guide json-key completeness (derived set difference) | `comm -23 <(...) <(...)` | empty output | ✓ PASS |
| CLAUDE.md `cmd/engram/` row-scoped inventory gate | shell pipeline per plan `<verify>` | empty miss list, `deprecated` present | ✓ PASS |

### Probe Execution

Not applicable — no `scripts/*/tests/probe-*.sh` files exist for this phase; no probe declarations in the plans or SUMMARYs.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| REQ-sweep-scope-rule-registered | 08-01 | Sweep guard registered as a `surfaces.ConditionalRule` | ⚠️ SATISFIED WITH GAPS | Core mechanism real and tested (truths 1, 3, 4); zero-occurrence enforcement is a one-time check, not a durable gate (truth 2); registry doc comment carries an inaccurate operational claim (truth 5) |
| REQ-docs-record-state | 08-02 + 08-03 | `memory-record.md`/`tools.md` document full record state; migration guide exists | ✓ SATISFIED | All sub-truths (6–9) verified; requirement was correctly marked complete only after both contributing plans (08-02 and 08-03) had landed — confirmed via commit ordering (`2e1fd9f6` [08-03] precedes `73da4c8a` [08-02 completion, which flips the REQUIREMENTS.md checkbox]) |
| REQ-claude-md-migrations-convention | 08-04 | CLAUDE.md migrations convention revised | ⚠️ SATISFIED WITH GAPS | Migrations bullet accurate (truth 10); Layout row and Memory contract section both carry confirmed factual/consistency defects (truths 11–12) |

No orphaned requirements — `.planning/REQUIREMENTS.md`'s Phase 8 mapping lists exactly these three, and all three appear in a plan's `requirements:` frontmatter.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cmd/engram/spine_review_scan.go`, `spine_review_verify.go`, `summarize.go` | `--all-scopes` flag `Usage` string | Constraint stated twice (registry Sentence + hand-typed "required if --scope is omitted" prefix) | ℹ️ Info | Not a functional defect (Sentence appears verbatim, tests enforce byte equality) but weakens the "compose once, never re-type" framing the plan's must-have asserts. Corroborated by 08-REVIEW.md WR-01. Does not change any truth's status above. |
| `CLAUDE.md` (Layout row) | line 15 | "alias" used loosely for `migrate-set-owner`, which is a separate command with an incompatible flag set (not a cobra alias or thin delegate) | ℹ️ Info | The deprecated-supersession relationship is real and tested (`TestMigrateSetOwnerEquivalentToRemapOwnerMissing`), but the word "alias" risks an agent copying `--from`/`--to` flags onto the wrong command. Softer than truths 11/12 since the underlying fact (deprecated, superseded) is accurate; flagged per 08-REVIEW.md WR-08 for awareness, not counted as a FAILED truth. |
| No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers found | — | — | — | Scanned all 16 phase-touched files; zero debt markers. |

### Human Verification Required

None. All findings above were resolved by direct code/doc inspection and are stated as confirmed FAILED or VERIFIED — no item was left uncertain.

### Gaps Summary

Two of Phase 8's requirements were substantially and verifiably achieved: REQ-docs-record-state (memory-record.md, tools.md, and the new migrate.md guide are all accurate, complete by derived-set-difference gates, and internally consistent) is fully satisfied with no reservations. The other two requirements — REQ-sweep-scope-rule-registered and REQ-claude-md-migrations-convention — have their core mechanisms genuinely working (the registry rule is declared once and composed correctly at all three enforcement sites; CLAUDE.md's migrations bullet correctly describes the shipped mechanism) but each carries confirmed, independently-verified factual or completeness defects that this phase's own goal ("brought current") and its own plans' acceptance criteria (explicit cross-page agreement checks, explicit "asserts zero, never a count" framing) commit to and do not deliver:

1. **CLAUDE.md's Layout row inconsistently marks deprecation** — `backfill-short-ids` is live-deprecated in code but not flagged, while the sibling `migrate-set-owner` is. This is the exact failure mode the row's "deprecated" marker exists to prevent, applied to only one of two qualifying commands.
2. **CLAUDE.md's Archived-state paragraph understates the recall-gate scope** — states 2 of the 4 actual soft-hide surfaces for `archived_at`, contradicting both the store's own code (4 gate sites) and the sibling reference page this same phase wrote with the correct count.
3. **The zero-hand-rolled-occurrence claim has no durable gate** — true today, but enforced by a one-time manual check rather than a test or CI step, so a future regression would not be caught. This is the repo's own named recurring failure pattern (vacuous gate), and it was independently found by this phase's own code review.

All three were corroborated by 08-REVIEW.md (WR-02/WR-03/WR-07/WR-09), which mutation-tested its findings rather than merely asserting them, and by this verification's own independent re-derivation from source (`internal/store/store.go`'s four `archived_at` gate sites, `backfill.go:58`'s `.Deprecated` field, and the absence of any `conditionalErrf`/zero-occurrence Go test). None of these defects blocks the core deliverable (the registered rule works; the guide and reference pages are accurate); they are documentation-accuracy and gate-durability gaps in artifacts whose entire purpose is to be a trustworthy, current source of truth.

---

*Verified: 2026-08-21*
*Verifier: Claude (gsd-verifier)*
