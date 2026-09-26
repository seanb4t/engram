---
phase: 01-store-prerequisites
plan: 04
subsystem: database
tags: [qdrant, recall-gate, vector-search, tags, citations, store]

# Dependency graph
requires:
  - phase: 01-store-prerequisites
    provides: "01-02's facetTags/recallVisibleFilter (rarity-weight source) and 01-03's RelatedMemories contract, edgeFilter, and assembleRelated one-entry-per-candidate merge, reused unchanged by this plan's tag and citation edges"
provides:
  - "Store.relatedTagEdges — rarity-weighted (ln(n/df)) shared-tag edges over the caller's readable, recall-visible set, with an ubiquity cut-off and an exact-Count fallback when the shared facetTags call truncates below an anchor tag"
  - "Store.relatedCitationEdges — shared-citation edges matched by kind+ref within one citation object via a nested Qdrant filter, never combining kind from one citation with ref from another"
  - "RelatedMemories' full four-edge merge complete: supersession, citation, tag, vector, one entry per candidate in canonical order, under the caller's read predicate throughout"
  - "the recall gate widened again: Store.relatedTagEdges and Store.relatedCitationEdges classified in recallTransmitters; the RelatedMemories fixtures now carry a shared tag and citation so the live capture walks every new filter"
affects:
  - "Phase 3 — the RelatedMemories RPC/tool (RPC-04) now serves the complete edge set"
  - "Phase 5 — the local graph view's neighbourhood rendering gets meaningful (rarity-weighted, not ubiquitous) tag edges and real citation edges"

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 11170
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Rarity weight as inverse document frequency: relatedTagEdges reads df from the SAME filtered facetTags call ListTags uses (never a second Facet call site), falling back to an exact Count only when the facet's own over-ask truncation hid a tag — df is never estimated, only ever exact or unavailable-then-fetched."
    - "Citation identity as ONE nested object, not two independent fields: relatedCitationEdges matches kind AND ref inside a single qdrant.NewNestedFilter over 'citations', so a record with {kind:X,ref:Y} in one citation and {kind:Z,ref:Y} in another can never falsely satisfy a search for {kind:X,ref:Y} via cross-citation field mixing."
    - "Both new edge producers slot into the SAME relatedCandidate/assembleRelated contract 01-03 established — no new merge mechanism, just two more gated lists passed to assembleRelated in canonical order (chain, citations, tags, vector)."

key-files:
  created: []
  modified:
    - internal/store/relatedmemories.go
    - internal/store/relatedmemories_test.go
    - internal/store/schemaversion_recallgate_test.go

key-decisions:
  - "D-06, D-07, D-08, D-10, D-11, D-12 implemented exactly as CONTEXT.md locked them (see the plan's must_haves). The FLAGGED ASSUMPTION (natural-log IDF with a more-than-half ubiquity cut-off, rarest-first probing, summed TagWeight) was accepted as planner discretion and implemented verbatim."
  - "Task 1 (tracer) committed as an explicit RED-then-GREEN pair even though it is type=\"tracer\" (not tdd=\"true\"): the plan's own action text names this sequencing, and the RED commit (tests only) references the not-yet-defined relatedTagFacetLimit package var, so that intermediate commit does not build in isolation — an accepted, documented departure from a build-clean-at-every-commit norm, matching the plan's explicit instruction rather than the stricter INVALID_RED gate (which only binds tdd=\"true\" tasks)."
  - "Task 2's four RED tests (TestRelatedMemoriesCitationEdge, TestRelatedMemoriesMultiEdgeEntry, TestRelatedMemoriesTagAndCitationEdgesFollowGates, TestRelatedMemoriesAllEdgesDeterministic) reference only pre-existing symbols (RelatedEdgeCitation, CitationRef existed from plan 01-03's contract), so that RED commit DOES build — it fails at runtime for the intended reason (no citation edges are ever produced), confirmed by running the suite before the GREEN commit."
  - "seedRecallGateRelatedFixtures' new tag/citation/filler shape (n=4, df=2 per subject) was chosen so the tag sits exactly at the D-07 boundary (2*df == n, kept, not ubiquitous) — the fixture doubles as a boundary regression check for the gate's own capture expectations, not just a smoke test."

requirements-completed: [STORE-02]
# STORE-02 was declared by both this plan and 01-03 (shared-ID gate, #2388).
# `gsd_run query requirements.ready-ids` reported 1/1 ready once this plan's
# own commits landed (01-03 already had its SUMMARY) — marked complete via
# `requirements.mark-complete`, never hand-edited.

coverage:
  - id: D1
    description: "Tracer (D-07): relatedTagEdges returns exact rarity-weighted shared-tag edges — n from an exact Count, df from the shared facetTags call, ubiquitous tags (2*df > n) excluded, top candidates in weight-then-id order, wired into RelatedMemories and classified in the recall gate in the same commit"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesTagEdgeRarity"
        status: pass
      - kind: unit
        ref: "internal/store/schemaversion_recallgate_test.go#TestRecallEmissionSetIsCompleteAndClassified"
        status: pass
      - kind: unit
        ref: "internal/store/schemaversion_recallgate_test.go#TestSchemaVersionNeverGatesRecall"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-07/D-12 boundary and precision: exactly relatedTagCap (8) tag edges come back, the 8 lowest ids among equal-weight candidates, ascending"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesTagEdgeCap"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-07 completeness: when the facet truncates below the anchor's own tag, its df comes from an exact fallback Count, never an estimate"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesTagWeightBeyondFacetLimit"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-08 boundary: a shared citation is the same kind AND ref within one citation object regardless of locator/pin/excerpt, the same ref under a different kind is not a match, ranked by shared-citation count, and capped at relatedCitationCap (8)"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesCitationEdge"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-06: a candidate reached by citation, tag, AND vector edges is ONE entry, Edges in canonical order (citation, tag, vector) each carrying non-empty/non-zero evidence"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesMultiEdgeEntry"
        status: pass
    human_judgment: false
  - id: D6
    description: "D-10/D-11 for the new edge types: archived, superseded, expired, scheduled, and b-private carriers of the anchor's rare tag/citation never surface at all; a b-shared carrier surfaces with both edges for an authenticated caller and is invisible to an anonymous caller; the rare tag's weight counts only live readable carriers"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesTagAndCitationEdgesFollowGates"
        status: pass
    human_judgment: false
  - id: D7
    description: "D-12 idempotency across all four edge types at once: two consecutive calls over unchanged data (supersession, citation, tag, vector all present) are deep-equal"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesAllEdgesDeterministic"
        status: pass
    human_judgment: false
  - id: D8
    description: "D-12 compile-time invariant: relatedSupersessionCap + relatedCitationCap + relatedTagCap stays below relatedTotalCeiling, enforced by a const expression that fails to compile otherwise"
    requirement: STORE-02
    verification:
      - kind: other
        ref: "go build ./... (the const _ = uint(relatedTotalCeiling - relatedSupersessionCap - relatedCitationCap - relatedTagCap - 1) expression in internal/store/relatedmemories.go)"
        status: pass
    human_judgment: false
  - id: D9
    description: "Recall gate widened for both new emitters: the live capture for RelatedMemories/anonymous and RelatedMemories/owner walks exactly Count, Facet, Query, and three Scrolls (tag probe, citation probe, payload fetch), and Store.relatedTagEdges/Store.relatedCitationEdges are classified as recall transmitters"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/schemaversion_recallgate_test.go#TestSchemaVersionNeverGatesRecall"
        status: pass
    human_judgment: false
  - id: D10
    description: "Full repo verification: internal/store, internal/authz, cmd/engram pass together; internal/keylinks confirms the plan frontmatter's key_links patterns still match source; gofmt, dprint, license:check, and golangci-lint (package and repo-wide) are all clean"
    requirement: STORE-02
    verification:
      - kind: other
        ref: "ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ ./internal/authz/ ./cmd/engram/ -count=1"
        status: pass
      - kind: other
        ref: "go test ./internal/keylinks/ -count=1"
        status: pass
    human_judgment: false

duration: 42min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 4: Rarity-Weighted Tag and Citation Edges Summary

**`Store.RelatedMemories` now returns rarity-weighted shared-tag edges (inverse document frequency over the caller's own recall-visible set) and shared-citation edges (kind+ref matched within one citation object via a nested Qdrant filter), completing STORE-02's four-edge contract.**

## Performance

- **Duration:** 42 min
- **Started:** 2026-09-25T23:48:00Z (approximate)
- **Completed:** 2026-09-26T00:30:00Z (approximate)
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- `Store.relatedTagEdges` (D-07): exact `n` via `Count`, `df` per anchor tag read through the SAME filtered `facetTags` call `ListTags` uses (an exact-Count fallback only when the facet's own truncation hides a tag), an ubiquity cut-off (`2*df > n` contributes zero weight), rarest-first probing capped at `relatedTagProbeCap`, and candidates ranked by summed `ln(n/df)` weight, capped at `relatedTagCap` (8).
- `Store.relatedCitationEdges` (D-08): distinct `(kind, ref)` pairs from the anchor's citations, each probed via `qdrant.NewNestedFilter("citations", ...)` so kind and ref must both live inside ONE citation object — never combining fields across two different citations on the same record — ranked by shared-citation count and capped at `relatedCitationCap` (8).
- `RelatedMemories` now composes all four edge types through the unchanged `assembleRelated` merge from plan 01-03: `chain, citations, tags, vector`, in canonical admission order (supersession, citation, tag, vector) — a candidate reached by more than one type is still exactly one entry (D-06).
- A compile-time invariant (`const _ = uint(relatedTotalCeiling - relatedSupersessionCap - relatedCitationCap - relatedTagCap - 1)`) guarantees the non-vector per-type caps never crowd the vector edge out of `relatedTotalCeiling` (D-12).
- The recall gate (`schemaversion_recallgate_test.go`) is widened again: `Store.relatedTagEdges` and `Store.relatedCitationEdges` are classified in `recallTransmitters`; `Store.facetTags`' justification now names both `ListTags` and `RelatedMemories`; `seedRecallGateRelatedFixtures` gained a shared tag, a shared citation, and two filler records per subject (n=4, df=2 — deliberately at the D-07 boundary); the `RelatedMemories` invocation rows' exact capture expectation is now `Count, Facet, Query, Scroll, Scroll, Scroll` (visible-set Count, the facet, the tag probe, the citation probe, the vector Query, one payload-fetch batch).

## Task Commits

Each task committed as an explicit RED-then-GREEN pair:

1. **Task 1 (tracer) — RED** - `eb18b8ac` (test) — `TestRelatedMemoriesTagEdgeRarity`, `TestRelatedMemoriesTagEdgeCap`, `TestRelatedMemoriesTagWeightBeyondFacetLimit`
2. **Task 1 (tracer) — GREEN** - `874d93d2` (feat) — `relatedTagEdges`, its wiring into `RelatedMemories`, and the recall-gate row for `Store.relatedTagEdges`
3. **Task 2 — RED** - `123e2f6e` (test) — `TestRelatedMemoriesCitationEdge`, `TestRelatedMemoriesMultiEdgeEntry`, `TestRelatedMemoriesTagAndCitationEdgesFollowGates`, `TestRelatedMemoriesAllEdgesDeterministic`
4. **Task 2 — GREEN** - `2b9d11bd` (feat) — `relatedCitationEdges`, the compile-time cap invariant, `RelatedMemories`' full merge call, and the extended recall-gate fixtures/rows

**Plan metadata:** (this commit, pending) — SUMMARY + STATE/ROADMAP/REQUIREMENTS

_No REFACTOR commit was needed for either task — neither GREEN implementation needed follow-up cleanup._

## Files Created/Modified

- `internal/store/relatedmemories.go` - `relatedTagEdges`, `relatedCitationEdges`, their consts/vars, the compile-time cap invariant, and `RelatedMemories`' full four-list merge
- `internal/store/relatedmemories_test.go` - 7 new tests across both tasks (3 tag-edge, 4 citation/multi-edge/gate/idempotency)
- `internal/store/schemaversion_recallgate_test.go` - two new recall-gate transmitter rows, an amended `facetTags` justification, extended fixtures, and re-derived `RelatedMemories` row expectations

## Decisions Made

- D-06, D-07, D-08, D-10, D-11, D-12 implemented exactly as CONTEXT.md locked them; the FLAGGED ASSUMPTION (natural-log IDF, more-than-half ubiquity cut-off, rarest-first probing) was accepted as planner discretion.
- Task 1's RED commit (tests referencing the not-yet-defined `relatedTagFacetLimit` package var) does not build in isolation — this is the plan's own explicit sequencing instruction for a `type="tracer"` task (not `tdd="true"`, so the stricter INVALID_RED gate does not bind it) and is recorded here rather than silently treated as a norm violation.
- Task 2's RED commit DOES build (its four tests reference only pre-existing contract symbols from plan 01-03) and was confirmed to fail for the intended reason — no citation edges are produced — before the GREEN commit landed.
- The recall-gate fixture's new tag/citation shape (n=4, df=2 per subject) was deliberately placed exactly at the D-07 ubiquity boundary (`2*df == n`, kept) so the gate doubles as a boundary regression check.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Corrected a stale cross-milestone commit ledger before measuring `actuals.commits`**
- **Found during:** SUMMARY authoring, computing the `actuals` block per the executor protocol's per-plan commit ledger
- **Issue:** `.git/gsd-plan-head-before-01-04` already existed on disk, left over from an EARLIER milestone that also numbered a plan `01-04` (dated 2026-09-22, well before this milestone's phase 1 started). The ledger protocol's `[ -f "$_GSD_LEDGER" ] || git rev-parse HEAD > "$_GSD_LEDGER"` check found the stale file and did not overwrite it, so the first `git rev-list --count` measurement returned 52 (the entire milestone-to-date history) instead of this plan's own 4 commits — phase/plan numbers are reused across milestones in this repo (see CLAUDE.md's CalVer milestone-label note), and the git-dir-scoped ledger filename collides across them.
- **Fix:** Overwrote the ledger with `git rev-parse 48bbe9b0` — the commit that was HEAD when this plan's execution began (per the conversation's opening `git status`, `48bbe9b0 docs(01-03): complete RelatedMemories vector and supersession edges plan`) — then re-measured: `commits: 4`.
- **Files modified:** none (git-dir-local file, not tracked)
- **Verification:** `git rev-list --count 48bbe9b0..HEAD` = 4, matching the four commits actually made by this plan.
- **Commit:** not applicable (untracked ledger file)

---

**Total deviations:** 1 auto-fixed (1 blocking — Rule 3, no code or test change).
**Impact on plan:** None on shipped behavior; corrects only this SUMMARY's self-reported `actuals.commits` metric. Worth flagging upstream: the git-dir-scoped ledger filename (`gsd-plan-head-before-{phase}-{plan}`) is not milestone-scoped, so it can silently collide when a milestone reuses phase/plan numbers from an earlier milestone in the same repository.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- STORE-02 is now fully complete: `Store.RelatedMemories` returns all four typed edges (supersession, citation, tag, vector) under the caller's read predicate, bounded, deterministic, one entry per candidate.
- Phase 3's `RelatedMemories` Connect RPC and MCP tool (RPC-04) can wrap this method directly with no further store-layer work.
- Phase 5's local graph view has real, non-degenerate tag and citation edges to render (rarity-weighted, ubiquity-excluded).
- Full `internal/store` (122.966s), `internal/authz`, and `cmd/engram` suites pass together; `internal/keylinks` confirms this plan's `key_links` patterns still match source; `gofmt -l .`, `dprint check`, `task license:check`, and `golangci-lint run ./...` are all clean.

---
*Phase: 01-store-prerequisites*
*Completed: 2026-09-26*

## Self-Check: PASSED

All modified files verified present on disk (`internal/store/relatedmemories.go`, `internal/store/relatedmemories_test.go`, `internal/store/schemaversion_recallgate_test.go`, this SUMMARY.md); all four task commit hashes (`eb18b8ac`, `874d93d2`, `123e2f6e`, `2b9d11bd`) verified present in `git log`.
