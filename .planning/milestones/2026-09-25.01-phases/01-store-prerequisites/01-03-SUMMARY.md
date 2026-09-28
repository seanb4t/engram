---
phase: 01-store-prerequisites
plan: 03
subsystem: database
tags: [qdrant, recall-gate, vector-search, supersession, store]

# Dependency graph
requires:
  - phase: 01-store-prerequisites
    provides: "01-02's recallVisibleFilter, facetTags, and fetchPayloadsByID/backfillNoSummaryContent shared primitives, reused unchanged by RelatedMemories' gated vector edge and two-phase fetch"
provides:
  - "Store.RelatedMemories(ctx, id, subj, k) — the full four-edge result contract (RelatedResult, RelatedMemory, RelatedEdge, RelatedEdgeType, SupersessionDirection, WeightedTag, CitationRef), the read-filtered query-by-id vector edge, and the bidirectional supersession chain walk"
  - "relatedSupersessionChain — superseded_by forward to the live head, supersedes backward breadth-first through predecessors, bounded to 8 hops/direction and 16 members total"
  - "assembleRelated — the one-entry-per-candidate merge (D-06) shared by every edge type this milestone, admission-ordered by type never by a blended score"
  - "the recall gate widened to an eighth caller-facing entry point: RelatedMemories seeded, Store.relatedVectorEdges classified in recallTransmitters, two new invocation rows"
affects:
  - "Plan 01-04 — adds the tag (D-07) and citation (D-08) edges into this same RelatedMemories/assembleRelated contract; STORE-02 is only complete once that plan lands"
  - "Phase 3 — the RelatedMemories RPC/tool (RPC-04) wraps this method directly; nothing above the store needs to filter its output"
  - "Phase 5 — the local graph view reads RelatedMemories for its neighbourhood rendering"

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 14143
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Two-phase gated candidate merge: relatedVectorEdges (and, from plan 01-04, the tag/citation sub-queries) return unresolved relatedCandidate{id, edge} hits under edgeFilter(recallVisibleFilter); assembleRelated resolves their payloads in ONE fetchPayloadsByID batch under the SAME filter value, then merges by type order (relatedEdgeRank) into RelatedMemory entries — the enforcement point is the shared filter value, never a post-filter re-check."
    - "Supersession chain as pre-resolved RelatedMemory, not relatedCandidate: each hop is a GetReadable (full record, not recall-gated) so a soft-hidden member still surfaces (D-11); the chain is fed into assembleRelated as the seed set candidates merge against, not as a gated sub-query — an unreadable member ends its own branch without a second fetch phase."
    - "edgeFilter mirrors searchfetch.go's includeIDs discipline: returns a NEW filter, never mutates the caller's f, and adds an anchor-exclusion MustNot rather than folding it into the caller's Must."

key-files:
  created:
    - internal/store/relatedmemories.go
    - internal/store/relatedmemories_test.go
  modified:
    - internal/store/schemaversion_recallgate_test.go

key-decisions:
  - "D-06, D-09, D-10, D-11 (chain side), D-12 implemented exactly as CONTEXT.md locked them (see the frontmatter list below). D-07/D-08 are plan 01-04's."
  - "Task 1 (tracer) landed the FULL result contract (all four edge types, including the tag/citation evidence fields plan 01-04 fills) plus the vector edge and the recall-gate widening in ONE commit — the plan's explicit reason: relatedVectorEdges' new Query emission enters the gate's scanned vocabulary the moment it exists, so the suite must never be red between commits."
  - "Task 2's RED phase added seven tests; only the two chain-specific tests (TestRelatedMemoriesSupersessionChain, TestRelatedMemoriesSupersessionCaps) failed — the other five (GatedEdgesFollowRecallGate, ReadScope, Bounds, Deterministic, EntryShape) pin behavior Task 1 already implemented (D-10, D-11's vector-gated half, D-12's bounds) and passed immediately. This matches the plan's own action text verbatim ('the chain tests fail because RelatedMemories passes no chain yet') — recorded rather than treated as an unexpected-GREEN violation."
  - "relatedSupersessionChain resolves each hop via GetReadable (full Memory, not recall-gated) rather than folding the walk into a gated sub-query, so a soft-hidden chain member (archived/superseded/expired/scheduled) still appears per D-11, while an unreadable member's own pointers are never followed (no structure leak)."

requirements-completed: []
# STORE-02 is declared by BOTH this plan and 01-04 (shared-ID gate, #2388);
# `gsd_run query requirements.ready-ids` returned 0/1 ready — 01-04 has not
# produced a SUMMARY yet, so REQUIREMENTS.md is intentionally left untouched.
# D-06, D-09, D-10, D-11 (chain side), D-12 are complete as of this plan;
# D-07/D-08 (tag/citation edges) remain for 01-04, which will mark STORE-02
# complete once both plans have summaries.

coverage:
  - id: D1
    description: "Tracer: RelatedMemories resolves the anchor, finds vector neighbours via ONE read-filtered query-by-id Query, and fetches candidate payloads through the same filter (D-10, SC3 core)"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesVectorEdge"
        status: pass
    human_judgment: false
  - id: D2
    description: "Anchor resolution: unreadable/nonexistent/nil-Subject anchors are GetReadable's indistinguishable ErrNotFound; a soft-hidden (archived) but readable anchor still returns its neighbourhood"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesAnchorAccess"
        status: pass
    human_judgment: false
  - id: D3
    description: "Recall gate widened: RelatedMemories seeded as an eighth caller-facing entry point, Store.relatedVectorEdges classified in recallTransmitters, schema_version proven absent from its transmitted filters"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/schemaversion_recallgate_test.go#TestRecallEmissionSetIsCompleteAndClassified"
        status: pass
      - kind: unit
        ref: "internal/store/schemaversion_recallgate_test.go#TestSchemaVersionNeverGatesRecall"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-09/D-11: the supersession chain walks both directions, soft-hidden members appear, a candidate reached by two edge types is one entry (D-06), and a chain broken by an unreadable member leaks no structure"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesSupersessionChain"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-09 bounds: the forward walk stops at 8 hops, the backward walk admits at most 16 members (the lowest-sorted ids at the cap boundary)"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesSupersessionCaps"
        status: pass
    human_judgment: false
  - id: D6
    description: "D-11 gated-edge half: archived, superseded, expired, and not-yet-active scheduled candidates are excluded from the vector edge — only a live candidate surfaces"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesGatedEdgesFollowRecallGate"
        status: pass
    human_judgment: false
  - id: D7
    description: "D-10 cross-spine reach: a same-owner neighbour in a different scope surfaces with its own Scope; a shared record from another owner surfaces; a private record from another owner does not; anonymous callers see only the ownerless bucket"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesReadScope"
        status: pass
    human_judgment: false
  - id: D8
    description: "D-12 bounds: k=0 defaults to 8 vector edges, caller k caps the vector edge, k above MaxRecallLimit is rejected naming \"k\", and the total ceiling (64) truncates with Truncated=true"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesBounds"
        status: pass
    human_judgment: false
  - id: D9
    description: "D-12 precision/idempotency: equal-score vector neighbours tiebreak on id ascending; two consecutive calls over unchanged data are deep-equal"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesDeterministic"
        status: pass
    human_judgment: false
  - id: D10
    description: "Entry shape: chain and vector entries carry Citations nil and, when a Summary is present, Content empty; a no-summary vector neighbour keeps its backfilled Content; the anchor itself carries no Citations"
    requirement: STORE-02
    verification:
      - kind: unit
        ref: "internal/store/relatedmemories_test.go#TestRelatedMemoriesEntryShape"
        status: pass
    human_judgment: false
  - id: D11
    description: "Concurrency: a candidate deleted or made unreadable between the vector Query and the payload fetch is dropped by fetchPayloadsByID's re-applied filter — never surfaced as an error or a distinguishable truncation"
    requirement: STORE-02
    verification: []
    human_judgment: true
    rationale: "Flagged in the plan's own must_haves as verification: backstop (spec-less fallback, flagged-unverified) — the shared-filter-value structural argument is sound (the same mechanism TestRelatedMemoriesReadScope and TestRelatedMemoriesVectorEdge exercise for a single snapshot), but no dedicated interleaving test forces a concurrent delete mid-fetch the way TestArchiveSurvivesConcurrentUpdate does for plan 01-01's writes. Flagging for the phase verifier rather than silently claiming full proof."

duration: 34min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 3: RelatedMemories Vector and Supersession Edges Summary

**`Store.RelatedMemories` returns an anchor's read-filtered vector neighbourhood plus its bidirectional supersession chain through one shared merge contract (assembleRelated), with the recall gate proven to walk its new query-by-id Query and payload fetch.**

## Performance

- **Duration:** 34 min
- **Started:** 2026-09-26T03:05:00Z (approximate)
- **Completed:** 2026-09-26T03:38:49Z
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- `internal/store/relatedmemories.go` — the full four-edge result contract (`RelatedEdgeType`, `SupersessionDirection`, `WeightedTag`, `CitationRef`, `RelatedEdge`, `RelatedMemory`, `RelatedResult`), `edgeFilter` (mirrors `searchfetch.go`'s `includeIDs` discipline), `relatedVectorEdges` (one query-by-id `Query` under `edgeFilter(recallVisibleFilter, anchorID)`), `assembleRelated` (the two-phase fetch and one-entry-per-candidate merge, D-06), `relatedSupersessionChain` (D-09's bidirectional walk), and `Store.RelatedMemories` itself.
- `internal/store/relatedmemories_test.go` — 9 tests: the tracer (`TestRelatedMemoriesVectorEdge`) and anchor-access rules from Task 1, plus Task 2's supersession chain, caps, gated-edge exclusion, cross-spine read scope, bounds/truncation, determinism, and entry-shape tests.
- `internal/store/schemaversion_recallgate_test.go` widened in the same commit as the vector edge (Task 1): `RelatedMemories` is now the eighth `recallEntryPointSeeds` member, `Store.relatedVectorEdges` is classified in `recallTransmitters`, two new invocation rows (`RelatedMemories/anonymous`, `RelatedMemories/owner`) exercise the live capture, and `seedRecallGateRelatedFixtures` seeds the anchor/neighbour pairs those rows need.
- The supersession chain (Task 2) follows `superseded_by` forward and `supersedes` backward breadth-first, bounded to 8 hops per direction and 16 members total; soft-hidden members (archived, superseded, expired, scheduled) still appear per D-11, while an unreadable member ends its branch without leaking further chain structure.
- The vector and chain candidate lists share ONE merge function (`assembleRelated`): a candidate reached by both a supersession edge and a vector edge (the live head of a chain that is also vector-similar to the anchor) comes back as exactly one entry with both edges, in the canonical type order (supersession, citation, tag, vector).

## Task Commits

Each task was committed atomically (Task 2 as RED → GREEN, per TDD):

1. **Task 1: RelatedMemories end to end — tracer** - `7828fac9` (feat) — contract, vector edge, and the recall-gate widening, in one commit
2. **Task 2: Supersession walk — RED** - `fd9cbd6d` (test) — seven tests added; the two chain-specific tests fail (no chain wired yet), five already pass (pinning Task 1's contract)
3. **Task 2: Supersession walk — GREEN** - `a6e4dac2` (feat) — `relatedSupersessionChain` implemented and wired into `RelatedMemories`
4. **Deviation fix** - `7e190baf` (fix) — `seedRecallGateRelatedFixtures` parameter order (revive `context-as-argument`)

**Plan metadata:** (this commit, pending) — SUMMARY + STATE/ROADMAP

_No REFACTOR commit was needed for Task 2 — the GREEN implementation needed no follow-up cleanup._

## Files Created/Modified

- `internal/store/relatedmemories.go` - the full RelatedMemories contract, vector edge, supersession chain, and merge
- `internal/store/relatedmemories_test.go` - 9 tests across both tasks
- `internal/store/schemaversion_recallgate_test.go` - recall gate widened for `RelatedMemories` and `Store.relatedVectorEdges`

## Decisions Made

- D-06, D-09, D-10, D-11 (chain side), D-12 implemented exactly as CONTEXT.md locked them (see frontmatter `key-decisions`).
- Task 1 landed the complete result contract — including the tag/citation evidence fields (`SharedTags`, `TagWeight`, `SharedCitations`) that plan 01-04 will populate — plus the vector edge and the recall-gate widening in one tracer commit, per the plan's explicit reasoning that the new `Query` emission enters the gate's scanned vocabulary the instant it exists.
- Five of Task 2's seven new tests passed immediately in the "RED" run because they pin vector-path behavior Task 1 already implemented (D-10, D-11's vector-gated half, D-12's bounds) rather than the new chain code; only the two chain-specific tests were genuinely RED. This is the plan's own stated expectation, not a TDD violation.
- `TestRelatedMemoriesAnchorAccess`'s archived-anchor assertion needed its own isolated collection: `RelatedMemories` reads cross-spine (D-10) by design, so sharing a collection with an earlier private-anchor fixture in the same test would have made that private record a spurious vector neighbour of the archived anchor purely by owner match — not a product bug, a test-isolation requirement the cross-spine design itself implies.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Lint] Reordered `seedRecallGateRelatedFixtures`'s parameters to `ctx`-first**
- **Found during:** Task 2's plan-level `golangci-lint run ./internal/store/...` verification
- **Issue:** The recall-gate seeding helper added in Task 1 was written `(t *testing.T, ctx context.Context, s *Store)`, tripping revive's `context-as-argument` check. Every other `(ctx, t, s)`-shaped helper already in this package (`migrate_test.go`, `migrate_status_test.go`, `revert_test.go`, etc.) puts `ctx` first.
- **Fix:** Reordered to `(ctx context.Context, t *testing.T, s *Store)` and updated its one call site.
- **Files modified:** `internal/store/schemaversion_recallgate_test.go`
- **Verification:** `golangci-lint run ./internal/store/...` reports 0 issues; `go test ./internal/store/ -count=1` still green (104.5s).
- **Commit:** `7e190baf`

---

**Total deviations:** 1 auto-fixed (1 lint-only, no behavior change).
**Impact on plan:** Cosmetic; required to keep `task lint` clean. No scope creep, no behavior change to any store method.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `Store.RelatedMemories`, `assembleRelated`, and `edgeFilter` are ready for plan 01-04 to add the tag (D-07) and citation (D-08) edges into the same result contract and merge function — no new merge mechanism needed, only new `relatedCandidate` producers.
- `Store.relatedSupersessionChain` and its result shape are stable; plan 01-04 does not touch it.
- STORE-02 stays open in REQUIREMENTS.md until plan 01-04 also produces a SUMMARY — `gsd_run query requirements.ready-ids` confirmed 0/1 ready (the shared-ID gate, #2388) at this plan's close, so REQUIREMENTS.md was intentionally left unedited rather than hand-marked.
- Full `internal/store` package test suite passes with both plans' tests together (`go test ./internal/store/ -count=1`, 104.5s); `go test ./internal/keylinks/ -count=1` confirms this plan's code still matches the plan frontmatter's `key_links` patterns.

---
*Phase: 01-store-prerequisites*
*Completed: 2026-09-26*

## Self-Check: PASSED

All created/modified files verified present on disk (`internal/store/relatedmemories.go`, `internal/store/relatedmemories_test.go`, `internal/store/schemaversion_recallgate_test.go`, this SUMMARY.md); all four task/deviation commit hashes (`7828fac9`, `fd9cbd6d`, `a6e4dac2`, `7e190baf`) verified present in `git log`.
