---
phase: 06-typed-operator-renderer
plan: 06
subsystem: cli
tags: [cobra, encoding-json, operator-renderer, spine-review, cli-output]

requires:
  - phase: 06-typed-operator-renderer
    provides: "06-01: renderOperatorView/viewFields/assertViewIdentity mechanism this plan converts three more reports onto"
provides:
  - "spine-review scan converted: spineScanSummary trimmed to a headline-only producer (R1); spineScanReportDoc gains a Scope field (json:\"scope\", R2 gap closure) and spineScanDoc's signature changes to (res, scope)"
  - "spine-review consolidate converted: consolidateSummary trimmed to headline-only, with a conditional no-min_score-filter clause preserving that nuance without a second rendering rule; consolidateReportDoc/consolidatePairDoc untouched, json byte-identical"
  - "spine-review verify converted: verifySummary trimmed to headline-only (scan instant plus four tier counts); verifyReportDoc/verifyEntryDoc untouched, json byte-identical"
  - "cmd/engram/operator_view_scan_test.go: spineViewFixtures (3 commandKeys x 2 docs each) and TestSpineViewIdentity, running plan 06-01's shared identity gate over all three converted reports plus a consolidate-specific omitempty field-count assertion"
affects: [06-07-PLAN.md]

actuals:
  tokens: 8000
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "R1 headline trim applied to three more xxxSummary functions -- each now returns fmt.Sprintf(...) directly (no strings.Builder) since there is exactly one line to build"
    - "R2 gap closure: spineScanReportDoc.Scope is this plan's one additive json key, placed first in the struct so the rendered field table leads with the scan target exactly as the headline always has"
    - "Conditional headline clause (consolidateSummary's no-min_score-filter sentence) as the sanctioned way to preserve human-facing nuance a deleted line used to carry, without reintroducing a second rendering rule for the field the json lane's omitempty already encodes"

key-files:
  created:
    - cmd/engram/operator_view_scan_test.go
  modified:
    - cmd/engram/spine_review_scan.go
    - cmd/engram/spine_review_test.go
    - cmd/engram/spine_review_consolidate.go
    - cmd/engram/spine_review_consolidate_test.go
    - cmd/engram/spine_review_verify.go
    - cmd/engram/spine_review_verify_test.go
    - cmd/engram/operator_output_test.go

key-decisions:
  - "spineScanReportDoc.Scope placed FIRST in the struct (before Total) so the rendered field table leads with the scan target exactly as spineScanSummary's headline always has -- changes marshaled key order but adds no key removal and no tag change"
  - "No all_scopes key added: spineReviewScanCmd already requires --scope xor --all-scopes, so an empty Scope value is an unambiguous all-scopes encoding without a second boolean"
  - "consolidateSummary's no-min_score-filter clause is appended to the SAME single headline line (not a second line) when minScore is nil; when minScore is non-nil the headline states nothing about it at all -- the threshold lives solely in the min_score json key, per consolidateReportDoc's pointer-plus-omitempty design"
  - "R2 gap check for consolidate and verify concluded no key is added: every fact each pre-conversion sentence stated is already a document key (or the length of one) -- scan's Scope is the only key this plan adds, consistent with 06-CONTEXT.md naming it as the sole such gap"

patterns-established:
  - "Deviation pattern for R1-trimmed summaries: when a helper test elsewhere in the package asserts a text-sentence fact that R1 just deleted from the sentence, narrow that test's declared-fact list to what the trimmed headline still states -- do not restore the fact to the sentence, and do not delete the still-valid facts along with it"

requirements-completed: [REQ-operator-renderer-typed]

coverage:
  - id: D1
    description: "spine-review scan's text/json gap is closed: spineScanReportDoc gains a scope key, spineScanDoc's signature changes to (res, scope), and the call site passes the flag value through"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_test.go#TestSpineScanDocEmptyResultMarshalsEmptyArray"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_test.go#TestSpineScanSummaryFormat"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_scan_test.go#TestSpineViewIdentity/spine-review_scan"
        status: pass
    human_judgment: false
  - id: D2
    description: "spine-review scan, consolidate and verify each render as headline-plus-complete-table: every per-row/per-signal loop in their xxxSummary functions is deleted, and the trimmed headline still passes the shared identity gate for every fixture variant (named/empty scope, filtered/unfiltered min_score, populated/zero tiers)"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/operator_view_scan_test.go#TestSpineViewIdentity"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestConsolidateSummaryMinScoreRendering"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifySummaryFormat"
        status: pass
    human_judgment: false
  - id: D3
    description: "consolidateReportDoc.MinScore's omitempty symmetry (absent key = no filter, present key = filtered at that value) holds at the rendered level, not just the json struct level -- proven by a field-line-count comparison, never a label-text comparison"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestConsolidateMinScoreOmitemptySymmetry"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_scan_test.go#TestSpineViewIdentity/spine-review_consolidate/min_score_omitempty_field_count"
        status: pass
    human_judgment: false
  - id: D4
    description: "consolidate's and verify's json documents are byte-unchanged: no field, tag, or value changed in consolidateReportDoc, consolidatePairDoc, verifyReportDoc, or verifyEntryDoc"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestConsolidateDocEmptyCandidatesMarshalsEmptyArray"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyDocEmptyResultMarshalsEmptyArrays"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-08-17
status: complete
---

# Phase 6 Plan 6: Spine-Review Scan/Consolidate/Verify Conversion Summary

**Three remaining spine-review reports (scan, consolidate, verify) converted to the one-serialization-plus-a-view mechanism, closing scan's long-standing text/json scope gap by adding one additive `scope` key.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-08-17T14:15:00Z (approx.)
- **Completed:** 2026-08-17T14:50:00Z
- **Tasks:** 3
- **Files modified:** 8 (1 created, 7 modified)

## Accomplishments

- `spine-review scan`'s text/json gap closed: `spineScanReportDoc` gains a `Scope` field (`json:"scope"`, placed first), `spineScanDoc`'s signature changes to `(res store.SpineScanResult, scope string)`, and the call site threads `spineScanScope` through. No `all_scopes` key added — the existing `--scope`/`--all-scopes` mutual-exclusivity already makes an empty scope value unambiguous.
- `spineScanSummary` trimmed to a single headline line (target, scan instant, total, owners) per R1; the three health-signal lines and the `(scope, category)` breakdown loop are deleted — they render only through `renderOperatorView`'s field table now.
- `consolidateSummary` trimmed to a single headline line; the per-pair loop is deleted. When `minScore` is nil, a clause is appended to the same line stating no filter was applied (preserving that nuance without reintroducing a second rendering rule); when non-nil, the headline says nothing about the threshold — it lives solely in the `min_score` json key.
- `verifySummary` trimmed to a single headline line (scan instant plus the four tier counts); all three entry loops (moved/broken/unverifiable) are deleted.
- R2's gap-closure check ran for all three commands: scan's `scope` is the only new key this plan adds; consolidate's and verify's pre-conversion sentences stated no fact that isn't already a document key (or the length of one).
- `cmd/engram/operator_view_scan_test.go` created: `spineViewFixtures` (3 commandKeys × 2 documents each, covering named/empty scope, filtered/unfiltered `min_score`, and populated/zero-valued verify tiers) plus `TestSpineViewIdentity`, running plan 06-01's shared `assertViewIdentity` gate, plus one group-specific field-count assertion proving the consolidate `min_score` omitempty symmetry at the rendered level.

## Task Commits

Each task was committed atomically:

1. **Task 1: Convert `spine-review scan` and give the scan target a document key** — `b0670813` (feat)
2. **Task 2: Convert `spine-review consolidate` and `spine-review verify`** — `0c2dd049` (feat)
3. **Task 3: Put scan, consolidate and verify under the shared identity gate** — `19b67358` (test)

**Plan metadata:** committed alongside this SUMMARY.

## Files Created/Modified

- `cmd/engram/spine_review_scan.go` — `spineScanReportDoc.Scope` added; `spineScanDoc` signature changed; `spineScanSummary` trimmed to headline-only; `strings` import dropped (no longer used)
- `cmd/engram/spine_review_test.go` — `TestSpineScanDocEmptyResultMarshalsEmptyArray`'s call updated for the new signature; `TestSpineScanSummaryFormat` rewritten per R4 (single-line headline assertions plus a rendered-view field-count assertion, replacing the deleted per-signal substring checks)
- `cmd/engram/spine_review_consolidate.go` — `consolidateSummary` trimmed to headline-only with the conditional no-filter clause; `consolidateReportDoc`'s doc comment extended; `strings` import dropped
- `cmd/engram/spine_review_consolidate_test.go` — `TestConsolidateRowNamesBothScopes` and `TestConsolidateNeverLabelsPairAsDuplicateOrCluster` moved from asserting summary-text substrings to asserting the rendered view's candidate rows; `TestConsolidateSummaryMinScoreRendering` rewritten for the new headline shape; `TestConsolidateMinScoreOmitemptySymmetry` added (new)
- `cmd/engram/spine_review_verify.go` — `verifySummary` trimmed to headline-only; `verifyReportDoc`'s doc comment extended
- `cmd/engram/spine_review_verify_test.go` — `TestVerifySummaryFormat` rewritten per R4 (tier-count assertions only)
- `cmd/engram/operator_output_test.go` — `TestOperatorOutputEmpty`'s scan entry gains the second argument; `operatorParityRows()`'s scan/consolidate/verify rows have their `facts` lists narrowed and the scan row's `spineScanDoc` call updated (see Deviations below)
- `cmd/engram/operator_view_scan_test.go` — `spineViewFixtures`, `TestSpineViewIdentity` (new)

## Decisions Made

- `spineScanReportDoc.Scope` placed first in the struct so the rendered field table leads with the scan target exactly as the headline always has — this changes marshaled key order but adds no key removal and no tag change; key order is not part of any consumer contract `encoding/json` guarantees.
- No `all_scopes` key added to `spineScanReportDoc`: `spineReviewScanCmd` already requires exactly one of `--scope`/`--all-scopes`, so the empty-scope encoding is sufficient and unambiguous.
- `consolidateSummary`'s no-filter clause is appended to the same single headline line, never a second line, and never restated when `minScore` is non-nil — the threshold's presence/absence in the rendered view is carried entirely by `consolidateReportDoc.MinScore`'s pointer-plus-`omitempty` design, so restating it in the headline would be the second rendering rule this phase exists to remove.
- R2's gap check for consolidate and verify concluded no key is added by this task: every fact each pre-conversion sentence stated is already a document key (or derivable as one's length), consistent with 06-CONTEXT.md naming `spineScanReportDoc.Scope` as the sole such gap across this plan's three reports.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `operatorParityRows()`'s scan/consolidate/verify rows needed more than the plan's stated single-line fix to `operator_output_test.go`**
- **Found during:** Task 1, running `go test ./cmd/engram/...`
- **Issue:** The plan's prohibition stated this plan's ONLY edit to `cmd/engram/operator_output_test.go` would be the argument list of the `spineScanDoc(store.SpineScanResult{})` call inside `TestOperatorOutputEmpty` — one insertion, one deletion. That assumption was incomplete: `operatorParityRows()` (which feeds the still-active `TestOperatorOutputParity`, not yet retired — D-09 defers its removal to plan 06-07) also calls `spineScanDoc(spineRes)` with the old single-argument signature (a second, unlisted call site that would not compile after Task 1's signature change), and its `facts` lists for the scan, consolidate, and verify rows assert that specific values appear as substrings of the pure text sentence. R1's headline trim removed exactly the substrings those `facts` entries were checking (health-signal counts, per-pair ids, per-entry record ids), so those three rows would fail `TestOperatorOutputParity` after Tasks 1 and 2, even once compilation was fixed.
- **Fix:** Added the second argument to the scan row's `spineScanDoc` call. Narrowed each of the three rows' `facts` lists to the values that remain present in the trimmed headline (scan: total/owners; consolidate: top_k/scanned/queried; verify: the four tier counts) — never restoring the deleted facts to the sentence, and never removing a fact that still holds. Documented the reason inline as a comment on each affected row.
- **Files modified:** `cmd/engram/operator_output_test.go`
- **Verification:** `go test ./cmd/engram/...` exits 0 (includes `TestOperatorOutputParity`); `task` (lint + test, full module) exits 0.
- **Committed in:** `b0670813` (Task 1 commit)
- **Note on the plan's own verification claim:** `git diff --stat cmd/engram/operator_output_test.go` for this plan's full range shows 25 insertions / 5 deletions, not the plan's stated "exactly 1 insertion and 1 deletion." The additional edits are confined to the SAME two pre-existing functions (`operatorParityRows()`, `TestOperatorOutputEmpty`) that this file already carries — no new function, no restructuring — so the plan's stated cross-plan-parallelism rationale ("so this plan can run in parallel with 06-03, 06-04 and 06-05, none of which touch that file") still holds: those sibling plans still do not touch this file.

---

**Total deviations:** 1 auto-fixed (1 Rule 1 — a plan-assumption gap about a second call site and now-broken assertions in a file this plan's frontmatter already scoped, discovered only once the R1 headline trim was applied and the full test suite run).
**Impact on plan:** Necessary for `task` (lint + test) to pass and for `TestOperatorOutputParity` — a pre-existing, still-active gate this plan did not intend to break — to keep passing until plan 06-07 retires it per D-09. No scope creep: the fix stayed inside the two functions the plan already listed as in-scope for this file, and every other `<acceptance_criteria>` grep and struct-body diff check passed exactly as specified.

## Issues Encountered

None beyond the auto-fixed deviation documented above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All three remaining spine-review reports (`scan`, `consolidate`, `verify`) are converted to the one-serialization-plus-a-view mechanism; combined with plan 06-01 (`prune-expired`) and this phase's other wave-2 plans, every report is now converted except whatever plans 06-03/06-04/06-05 own.
- `cmd/engram/operator_view_scan_test.go` declares this plan's `spineViewFixtures` group function, ready for plan 06-07 to merge alongside every sibling group's own `<group>ViewFixtures` into the both-directions enumeration gate against `operatorCommands()`.
- Plan 06-07 also owns retiring `TestOperatorOutputParity`/`operatorParityRows()` entirely (D-09) — this plan's narrowed `facts` lists for the scan/consolidate/verify rows are a stopgap that keeps that pre-existing gate honest until then, not a permanent fixture.
- No blockers.

---
*Phase: 06-typed-operator-renderer*
*Completed: 2026-08-17*

## Self-Check: PASSED

- `cmd/engram/spine_review_scan.go` exists and modified: FOUND
- `cmd/engram/spine_review_test.go` exists and modified: FOUND
- `cmd/engram/spine_review_consolidate.go` exists and modified: FOUND
- `cmd/engram/spine_review_consolidate_test.go` exists and modified: FOUND
- `cmd/engram/spine_review_verify.go` exists and modified: FOUND
- `cmd/engram/spine_review_verify_test.go` exists and modified: FOUND
- `cmd/engram/operator_output_test.go` exists and modified: FOUND
- `cmd/engram/operator_view_scan_test.go` exists (new file): FOUND
- Commit `b0670813` (Task 1): FOUND in `git log --oneline --all`
- Commit `0c2dd049` (Task 2): FOUND in `git log --oneline --all`
- Commit `19b67358` (Task 3): FOUND in `git log --oneline --all`
- `go test ./cmd/engram/...` exits 0: PASSED
- `task` (lint + test, full module) exits 0: PASSED
- `task license:check` exits 0: PASSED
- `git diff` shows no field/tag change inside `consolidateReportDoc`, `consolidatePairDoc`, `verifyReportDoc`, or `verifyEntryDoc`: PASSED
- `spineScanReportDoc` gains exactly one key (`scope`), no existing key/tag changed: PASSED
- All plan-level `<verification>` and `<success_criteria>` commands re-run and passing (see acceptance-criteria greps above; the one documented exception is the `operator_output_test.go` diff-size claim, addressed under Deviations)
