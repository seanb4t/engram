---
phase: 01-store-prerequisites
verified: 2026-09-28T17:33:35Z
status: passed
score: 18/18 must-haves verified
covered_files:
  - ".planning/REQUIREMENTS.md"
  - ".planning/phases/01-store-prerequisites/01-01-PLAN.md"
  - ".planning/phases/01-store-prerequisites/01-01-SUMMARY.md"
  - ".planning/phases/01-store-prerequisites/01-02-PLAN.md"
  - ".planning/phases/01-store-prerequisites/01-02-SUMMARY.md"
  - ".planning/phases/01-store-prerequisites/01-03-PLAN.md"
  - ".planning/phases/01-store-prerequisites/01-03-SUMMARY.md"
  - ".planning/phases/01-store-prerequisites/01-04-PLAN.md"
  - ".planning/phases/01-store-prerequisites/01-04-SUMMARY.md"
  - "cmd/engram/spine_review_archive.go"
  - "internal/authz/authz.go"
  - "internal/authz/policy_corpus_test.go"
  - "internal/authz/schema.json"
  - "internal/store/archive_authz_test.go"
  - "internal/store/concurrent_gates_test.go"
  - "internal/store/listtags.go"
  - "internal/store/listtags_test.go"
  - "internal/store/relatedmemories.go"
  - "internal/store/relatedmemories_test.go"
  - "internal/store/schemaversion_recallgate_test.go"
  - "internal/store/spine.go"
  - "internal/store/store.go"
covered_digest: "v2:sha256:0cb8291b68ce10c12197f3825dba10f89e3d62fc3946266defb612ed8e5b985d"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 18/18
  gaps_closed: []
  gaps_remaining:
    - "01-03's judgment-tier prohibition (\"MUST NOT fold edge types into one comparable score or re-rank across types\") remains unresolved. Unchanged by later-phase edits: Phase 3's addition of the `full bool` parameter to Store.RelatedMemories/assembleRelated (commit 1fa1e745) inserted a new positional argument but did not touch admission order — the call site is still `s.assembleRelated(ctx, f, anchor.ID, full, chain, citations, tags, vector)`, citations-before-tags-before-vector, matching relatedEdgeRank's canonical order. TestRelatedMemoriesAdmitsByTypeNotScore still passes. Per the judgment-tier prohibition policy this remains a flagged item requiring explicit human sign-off, never auto-resolved by test evidence."
  regressions: []
---

# Phase 01: Store Prerequisites Verification Report (Re-Verification — Staleness Check)

**Phase Goal:** `internal/store` gains authz-gated Archive/Restore, RelatedMemories, and ListTags as pure store methods, so the authz shape and test-infrastructure gaps are settled before any RPC or UI is built on top of them.
**Verified:** 2026-09-28T17:33:35Z
**Status:** human_needed
**Re-verification:** Yes — the prior `01-VERIFICATION.md` (2026-09-26T01:20:00Z, commit `14c7aa5e`) went stale because later milestone phases (Phase 3's `03-05` plan, commits `6b9a745a` and `1fa1e745`) edited files this phase's must-haves cover: `internal/store/relatedmemories.go`, `internal/store/relatedmemories_test.go`, `internal/store/concurrent_gates_test.go`, and `internal/store/schemaversion_recallgate_test.go`. This report re-verifies the phase goal against current HEAD (`94c9552a`) from scratch, independently re-running every test this verifier's own process could reach.

## Why the prior report was stale

Commit `1fa1e745` ("feat(server): add RelatedMemories Connect RPC with oneof edge evidence and full opt-in") is Phase 3 work, but it changed a Phase 1 production signature: `Store.RelatedMemories(ctx, id, subj, k)` gained a fifth parameter, `full bool` (compact-by-default, full-opt-in payload projection), and `Store.assembleRelated` gained the same parameter plus a `relatedShape(m, full)` helper replacing bare `summaryShape(m)` calls. This is exactly the class of change this task was dispatched to check for regression. `internal/store/store.go` was also touched (56/−19) but only for `Store.ListScheduled`/`collectOrderedPages` cursor support — code outside this phase's STORE-01/02/03 scope, not touched by this review beyond confirming it doesn't intersect Archive/Restore/ListTags/RelatedMemories.

`internal/store/store.go`'s Archive/Restore paths, `internal/store/spine.go`, `internal/store/listtags.go`, `internal/store/archive_authz_test.go`, `internal/store/listtags_test.go`, `internal/authz/authz.go`, `internal/authz/policy_corpus_test.go`, `internal/authz/schema.json`, and `cmd/engram/spine_review_archive.go` are byte-for-byte unchanged since `14c7aa5e` (confirmed via `git diff --stat 14c7aa5e HEAD -- <file>` for each, all empty).

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth (Roadmap SC) | Status | Evidence |
|---|------|--------|----------|
| SC1 | A test proves a caller can archive/restore only records they own — a shared record they can read is rejected exactly as Delete/Update/Supersede reject it — enforced in `internal/store`, the phase's first test | ✓ VERIFIED | `internal/store/archive_authz_test.go` unchanged since `14c7aa5e`; re-ran live against real Qdrant: `TestArchiveAsOwnerGate`, `TestRestoreAsOwnerGate`, `TestArchiveAsRestoreAsOwnedRule`, `TestArchiveAsFailsClosed`, `TestArchiveAsAnonymousBucket`, `TestArchiveAsIdempotent`, `TestArchiveAsRestoreAsConcurrentSerialize` — all 7 PASS |
| SC2 | The CLI's subject-less `spine-review archive`/`restore` path keeps working unchanged | ✓ VERIFIED | `git diff --quiet 16f44eda HEAD -- cmd/engram/spine_review_archive.go` exits 0 (byte-identical); 11 pre-existing `TestArchive*/TestRestore*` store tests PASS; 10 `cmd/engram` archive/spine-review tests PASS |
| SC3 | `RelatedMemories(subj, id)` returns supersession, shared-tag, shared-citation, and vector-neighbour edges with the caller's read predicate composed into the Qdrant filter, a bounded edge count, and a documented multi-edge rule | ✓ VERIFIED | All 18 `TestRelatedMemories*` tests PASS live against real Qdrant on current HEAD, including `TestRelatedMemoriesEntryShape` (summary-view default unaffected by Phase 3's new `full` parameter) and `TestRelatedMemoriesMultiEdgeEntry`/`TestRelatedMemoriesAllEdgesDeterministic` |
| SC4 | `ListTags(subj, scope)` returns facet counts over a new `tags` payload index under the caller's read filter, and the recall-gate test allowlist recognizes a filtered `Facet` call | ✓ VERIFIED | `internal/store/listtags.go`/`listtags_test.go` unchanged since `14c7aa5e`; all 7 `TestListTags*` tests PASS live; `TestRecallEmissionSetIsCompleteAndClassified` and `TestSchemaVersionNeverGatesRecall` (incl. `ListTags/anonymous`, `ListTags/owner`, `RelatedMemories/anonymous`, `RelatedMemories/owner` subtests) PASS |

**Score:** 4/4 roadmap Success Criteria verified, no regression from later-phase edits.

### Observable Truths (Plan-Level, Concurrency Backstops and the Flagged Prohibition)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| STORE-01 concurrency (backstop) | ArchiveAs/RestoreAs concurrent calls on one id serialize under the shared lock | ✓ VERIFIED | `TestArchiveAsRestoreAsConcurrentSerialize` re-run with `-race`, real Qdrant — PASS |
| STORE-03 concurrency (backstop) | ListTags is an exact point-in-time snapshot under concurrent writes | ✓ VERIFIED | `TestListTagsUnderConcurrentWrites` re-run with `-race`, real Qdrant — PASS |
| STORE-02 concurrency (backstop) | A RelatedMemories candidate vanishing between vector Query and payload fetch is dropped silently | ✓ VERIFIED | `TestRelatedMemoriesCandidateVanishesBeforeFetch` re-run with `-race`, real Qdrant — PASS; unaffected by the `full`-parameter addition (the vanish window is inside `assembleRelated`'s fetch call, which still uses the same `relatedBeforeFetchHook` seam) |
| 01-03 prohibition (judgment-tier): no blended cross-type score | RelatedEdge/assembleRelated never fold vector score, tag weight, citation count, and supersession depth into one comparable ranking number | ⚠️ flagged (judgment-tier), unchanged | `TestRelatedMemoriesAdmitsByTypeNotScore` re-run — PASS; code read of current `assembleRelated` call site (`internal/store/relatedmemories.go:754`, now `s.assembleRelated(ctx, f, anchor.ID, full, chain, citations, tags, vector)`) confirms citations/tags/vector order is unchanged by the `full` insertion. Per policy this remains a `human_verification` item, not auto-resolved. |

**Score:** 18/18 truths verified (4 roadmap SCs + 14 plan-level truths sampled/re-confirmed via the test evidence above and the full prior report's D-01..D-16 coverage, none of which touch the files changed by Phase 3). One judgment-tier prohibition remains flagged for human sign-off, tracked separately from the truths score per policy.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/authz/authz.go` | `ActionArchive` action constant | ✓ VERIFIED | Unchanged since `14c7aa5e`; `Action = "archive"` present once |
| `internal/store/spine.go` | `ArchiveAs`/`RestoreAs` gated wrappers | ✓ VERIFIED | Unchanged since `14c7aa5e` |
| `internal/store/archive_authz_test.go` | Authz-in-store archive/restore tests | ✓ VERIFIED | Unchanged; 7/7 tests PASS live |
| `internal/authz/policy_corpus_test.go` | `ActionArchive` in the per-action corpus | ✓ VERIFIED | Unchanged; `TestPolicyCorpus_*` (6 tests) PASS |
| `internal/authz/schema.json` | `"archive"` schema entry | ✓ VERIFIED | Unchanged |
| `internal/store/listtags.go` | `TagCount`, `ListTags`, `facetTags`, `recallVisibleFilter` | ✓ VERIFIED | Unchanged since `14c7aa5e` |
| `internal/store/listtags_test.go` | ListTags integration tests | ✓ VERIFIED | Unchanged; 7/7 tests PASS live |
| `internal/store/relatedmemories.go` | `RelatedMemories`, edge helpers | ✓ VERIFIED | **Changed by Phase 3** (`full bool` param added to `RelatedMemories`/`assembleRelated`/`relatedSupersessionChain`; new `relatedShape` helper); re-verified substantive and correctly wired — default (`full=false`) path preserves the original summary-view contract |
| `internal/store/relatedmemories_test.go` | RelatedMemories integration tests | ✓ VERIFIED | **Changed by Phase 3** (some tests updated for the new signature); 18/18 tests PASS live |
| `internal/store/concurrent_gates_test.go` | Concurrency + type-order behavioral tests | ✓ VERIFIED | **Changed by Phase 3** (call-site update for the new signature, +6/−? lines); all 3 backstop tests re-confirmed PASS with `-race` |
| `internal/store/schemaversion_recallgate_test.go` | Recall gate vocabulary for Facet/ListTags/RelatedMemories | ✓ VERIFIED | **Changed by Phase 3** (minor call-site update); `TestRecallEmissionSetIsCompleteAndClassified` and `TestSchemaVersionNeverGatesRecall` both PASS with all seeded entry points including `ListTags` and `RelatedMemories` |
| `cmd/engram/spine_review_archive.go` | Byte-identical to `16f44eda` | ✓ VERIFIED | `git diff --quiet` exits 0 |

### Key Link Verification

All 16 declared `key_links` across the four plans (01-01..01-04) were checked. Two links in 01-03-PLAN.md and 01-04-PLAN.md were **updated in-plan** by a dedicated maintenance commit, `22aec07f` ("fix(keylinks): update stale key_links patterns after relatedmemories.go's full-knob signature change") — landed by Phase 3's own plan 03-05 specifically because its `full`-parameter change made the original literal patterns unsatisfiable. This is filling in a value (a pattern string) in an already-declared key_link, not inventing new structure, and the commit message documents the from/to/via semantics are unchanged. Verified both updated patterns match current code:

- `s.fetchPayloadsByID(ctx, f, view, ` — 1 match in `internal/store/relatedmemories.go`
- `s.assembleRelated(ctx, f, anchor.ID, full, chain, citations, tags, vector)` — 1 match in `internal/store/relatedmemories.go`

The remaining 14 key_links (archive/restore gating, `recallVisibleFilter`/`facetTags` composition, CLI wiring, recall-gate classification) are unaffected by the Phase 3 diff and were re-confirmed via `internal/keylinks`' own automated gate:

```
go test ./internal/keylinks/ -count=1 -v
--- PASS: TestActiveMilestoneKeyLinksSatisfiable
--- PASS: TestReassessV013Phase12
--- PASS: TestReassessmentTableIsComplete
... (12/12 tests PASS)
ok  	github.com/seanb4t/engram/internal/keylinks	0.210s
```

`TestActiveMilestoneKeyLinksSatisfiable` scans every PLAN.md's declared `key_links` patterns across the active milestone and asserts each is currently satisfiable against the codebase — its PASS is independent, automated confirmation that no key_link (Phase 1's or any other active-milestone phase's) has gone stale as of HEAD.

### Behavioral Spot-Checks / Test Execution

All commands run independently by this verifier, from its own process, against real Qdrant (testcontainers), never inferred from SUMMARY.md or prior verification narration.

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build compiles | `go build ./...` | clean, no output | ✓ PASS |
| gofmt clean | `gofmt -l internal/store internal/authz cmd/engram` | empty | ✓ PASS |
| Archive/Restore gate + concurrency | `go test ./internal/store/ -run '^Test(ArchiveAs\|RestoreAs)' -v` | 7/7 PASS | ✓ PASS |
| Policy corpus | `go test ./internal/authz/ -run '^TestPolicyCorpus' -v` | 6/6 PASS | ✓ PASS |
| ListTags contract | `go test ./internal/store/ -run '^TestListTags' -v` | 7/7 PASS | ✓ PASS |
| ListTags concurrency (`-race`) | `go test ./internal/store/ -run '^TestListTagsUnderConcurrentWrites$' -race -v` | PASS | ✓ PASS |
| RelatedMemories full contract | `go test ./internal/store/ -run '^TestRelatedMemories' -v` | 18/18 PASS | ✓ PASS |
| Recall gate (Facet/ListTags/RelatedMemories vocabulary) | `go test ./internal/store/ -run '^(TestRecallEmissionSetIsCompleteAndClassified\|TestSchemaVersionNeverGatesRecall\|TestFilterWalkerSeesEveryPosition)$' -v` | 3/3 top-level PASS (29 subtests) | ✓ PASS |
| SC2 pre-existing Archive/Restore regression | `go test ./internal/store/ -run '^Test(Archive\|Restore)(Idempotent\|NoOpWhenNeverArchived\|UnknownID\|SurvivesWholePayloadUpdate\|SurvivesConcurrentUpdate\|RecallGate)' -v` | 11/11 PASS | ✓ PASS |
| SC2 CLI regression | `go test ./cmd/engram/ -run '^Test(Archive\|SpineReviewArchive\|SpineReviewRestore)' -v` | 10/10 PASS | ✓ PASS |
| Backstop concurrency tests with `-race` | `go test ./internal/store/ -race -run '^(TestArchiveAsRestoreAsConcurrentSerialize\|TestRelatedMemoriesCandidateVanishesBeforeFetch\|TestRelatedMemoriesAdmitsByTypeNotScore)$' -v` | 3/3 PASS | ✓ PASS |
| Key-links gate | `go test ./internal/keylinks/ -count=1 -v` | 12/12 PASS | ✓ PASS |
| License check | `task license:check` | 2685 checked, 0 invalid | ✓ PASS |
| Lint | `golangci-lint run ./internal/store/... ./internal/authz/...` | 0 issues | ✓ PASS |

No full-package `go test ./internal/store/...` run was repeated in this re-verification — the targeted runs above cover every must-have truth and every file this phase's covered set includes, per the host-load constraint (other phase verifiers running concurrently); the prior report already recorded one full-package PASS at `14c7aa5e`, and nothing in that run's exercised surface has regressed since (files unchanged, or changed files' relevant tests re-run and green above).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| STORE-01 | 01-01 | Authz-gated Archive/Restore through the owner-write gate | ✓ SATISFIED | SC1/SC2 evidence above |
| STORE-02 | 01-03, 01-04 | `RelatedMemories(subj, id)` typed edges, read-filtered, bounded | ✓ SATISFIED | SC3 evidence above |
| STORE-03 | 01-02 | `ListTags(subj, scope)` facet counts, read-filtered, recall-gate widened | ✓ SATISFIED | SC4 evidence above |

REQUIREMENTS.md maps all three (STORE-01, STORE-02, STORE-03) to Phase 1 as "Mapped" — no orphaned requirements for this phase.

### Anti-Patterns Found

None. Scanned all Phase-1-covered files (including the three changed by Phase 3: `relatedmemories.go`, `relatedmemories_test.go`, `concurrent_gates_test.go`, `schemaversion_recallgate_test.go`) for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` (case-insensitive): zero matches.

### Human Verification Required

1. **01-03's judgment-tier prohibition — no blended cross-type score**
   **Test:** Confirm `RelatedEdge`/`assembleRelated` never fold vector score, tag weight, citation count, and supersession depth into one comparable ranking number.
   **Expected:** Per-type evidence fields only; admission and sort order strictly by `relatedEdgeRank` (fixed type order), never a blended score.
   **Why human:** The plan's own frontmatter marks this prohibition `verification: judgment` (not test-backed). `TestRelatedMemoriesAdmitsByTypeNotScore` still demonstrates this directly and passes live against real Qdrant on current HEAD, and this verifier confirmed the admission-order code path is unchanged by Phase 3's `full`-parameter insertion (`internal/store/relatedmemories.go:754`: `s.assembleRelated(ctx, f, anchor.ID, full, chain, citations, tags, vector)` — citations, tags, vector still in that literal order). Per the judgment-tier prohibition policy, a judgment-tier item is never auto-resolved by test evidence; it requires an explicit human sign-off rather than resting on an LLM-judge verdict (this verifier's or any predecessor's) alone. Carried forward unresolved from the prior verification — no new information changes its disposition.

### Gaps Summary

No gaps, no regressions. This re-verification confirms:

- All 4 roadmap Success Criteria remain fully verified against current HEAD.
- 18/18 must-have truths remain verified; none were weakened, broken, or made unsatisfiable by the later-phase edits to `internal/store/relatedmemories.go`, `relatedmemories_test.go`, `concurrent_gates_test.go`, or `schemaversion_recallgate_test.go`.
- Phase 3's `full bool` parameter addition to `Store.RelatedMemories`/`assembleRelated`/`relatedSupersessionChain` is additive and backward-compatible: the default (`full=false`) path is byte-for-byte equivalent to the pre-change summary-view behavior (`relatedShape(m, false) == summaryShape(m)`), confirmed by `TestRelatedMemoriesEntryShape` passing unmodified in intent.
- The two Phase-1 `key_links` patterns whose literal call-shape changed were updated in-plan by the same commit that changed the code (`22aec07f`), and both now match current code exactly; the automated `internal/keylinks` gate independently confirms every active-milestone key_link (including all 16 of this phase's) is satisfiable at HEAD.
- The single previously-flagged item — the judgment-tier "no blended cross-type score" prohibition — is unaffected by the later-phase changes and remains a `human_verification` item per policy, unchanged from the prior report. This keeps the phase at `human_needed` rather than `passed`.

---

*Verified: 2026-09-28T17:33:35Z*
*Verifier: Claude (gsd-verifier)*

## Human Sign-Off Carried Forward (orchestrator, 2026-09-28)

The single `human_verification` item — 01-03's judgment-tier prohibition (no blended cross-type
score) — was already signed off explicitly by Sean on 2026-09-26 (`01-UAT.md` test 4: "pass —
explicit human sign-off (Sean, 2026-09-26)"). This re-verification found the relevant code path
unchanged apart from Phase 3's additive `full` parameter (admission order citations → tags →
vector, matching `relatedEdgeRank`, and `TestRelatedMemoriesAdmitsByTypeNotScore` still passes),
so that sign-off still applies and the phase status is `passed`. No new human item was raised.
