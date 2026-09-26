---
phase: 01-store-prerequisites
verified: 2026-09-26T04:45:00Z
status: human_needed
score: 15/18 must-haves verified
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
  - "internal/authz/authz.go"
  - "internal/authz/policy_corpus_test.go"
  - "internal/authz/schema.json"
  - "internal/store/archive_authz_test.go"
  - "internal/store/listtags.go"
  - "internal/store/listtags_test.go"
  - "internal/store/relatedmemories.go"
  - "internal/store/relatedmemories_test.go"
  - "internal/store/schemaversion_recallgate_test.go"
  - "internal/store/spine.go"
  - "internal/store/store.go"
covered_digest: "v1:sha256:76e04ac1230f77f5060e95105ae18ef0538a94548c058c04e414e47e6eeb3348"
behavior_unverified: 0
overrides_applied: 0
behavior_unverified_items:
  - truth: "Concurrency (STORE-01, 01-01 plan): a concurrent gated ArchiveAs and RestoreAs on one id serialize under s.locker.Lock and leave the record in exactly one of the two states"
    test: "Run ArchiveAs and RestoreAs concurrently on the same id (e.g. via the updateAfterReadHook barrier seam TestArchiveSurvivesConcurrentUpdate already uses, but driving the gated wrappers instead of the subject-less Archive/Restore) and assert the record ends up in exactly one terminal state with no partial write."
    expected: "Exactly one of ArchiveOutcomeChanged/ArchiveOutcomeAlready wins per direction; archived_at is either fully present or fully absent, never torn."
    why_human: "No test drives ArchiveAs/RestoreAs through a deterministic concurrent interleaving; the existing TestArchiveSurvivesConcurrentUpdate/TestRestoreSurvivesConcurrentUpdate barrier tests exercise the subject-less Archive/Restore only. The gated wrappers reuse the identical lock and shared core, so the property likely holds, but that is a structural argument, not a behavioral test, and the plan explicitly tags this truth verification: backstop."
  - truth: "Concurrency (STORE-03, 01-02 plan): ListTags counts are an exact point-in-time facet snapshot and concurrent writes can shift counts between calls but never expose an unreadable record"
    test: "Run ListTags concurrently with a write that adds/removes a tag on a record outside the caller's read scope, and assert the count never reflects that record."
    expected: "Facet count changes only reflect records the caller's read + recall-gate filter admits, at any point-in-time snapshot."
    why_human: "No concurrent-write test exists for ListTags; the filter-composition argument is verified statically (recallVisibleFilter travels on every Facet request), but the plan explicitly tags this truth verification: backstop and no test exercises the interleaving."
  - truth: "Concurrency (STORE-02, 01-03 plan): a RelatedMemories candidate deleted or made unreadable between the vector Query and the payload fetch is dropped silently by fetchPayloadsByID's re-applied filter, never surfaced as an error or a distinguishable truncation"
    test: "Delete or revoke read access to a vector-neighbour candidate between the Query and the fetch (e.g. via a test seam mirroring updateAfterReadHook) and assert RelatedMemories returns without that candidate and without error."
    expected: "The candidate is absent from Related; Truncated is unaffected by this specific drop; no error surfaces."
    why_human: "No test drives this specific race window; the code path (assembleRelated skips ids missing from the fetched map) is structurally sound but unexercised by a timing-controlled test, and the plan tags this truth verification: backstop."
human_verification:
  - test: "Concurrent ArchiveAs/RestoreAs on one id"
    expected: "Record ends in exactly one of the two states, never torn"
    why_human: "No behavioral test exercises this; backstop-tagged truth, see behavior_unverified_items"
  - test: "Concurrent write during ListTags facet read"
    expected: "Facet count never exposes an unreadable record"
    why_human: "No behavioral test exercises this; backstop-tagged truth, see behavior_unverified_items"
  - test: "Candidate disappearing between RelatedMemories' vector Query and payload fetch"
    expected: "Candidate silently dropped, no error, no distinguishable truncation signal"
    why_human: "No behavioral test exercises this; backstop-tagged truth, see behavior_unverified_items"
  - test: "01-03's judgment-tier prohibition: edge types are never folded into one comparable score or re-ranked across types"
    expected: "RelatedEdge keeps per-type evidence fields only; assembleRelated admits by fixed relatedEdgeRank order, never a blended score"
    why_human: "Prohibition is verification: judgment (not test-backed) per the plan's own frontmatter — this verifier's code read confirms the structural claim (RelatedEdge has no blended-score field, and admission order is by relatedEdgeRank), but per the judgment-tier prohibition policy this needs an explicit human sign-off rather than an LLM-judge verdict standing alone as authoritative."
---

# Phase 01: Store Prerequisites Verification Report

**Phase Goal:** `internal/store` gains authz-gated Archive/Restore, RelatedMemories, and ListTags as pure store methods, so the authz shape and test-infrastructure gaps are settled before any RPC or UI is built on top of them.
**Verified:** 2026-09-26T04:45:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth (Roadmap SC) | Status | Evidence |
|---|------|--------|----------|
| SC1 | A test proves a caller can archive/restore only records they own — a shared record they can read is rejected exactly as Delete/Update/Supersede reject it — enforced in `internal/store`, the phase's first test | ✓ VERIFIED | `git log --reverse` shows `a314b3a1 test(store): add failing authz-in-store archive gate test (D-05)` as the first Phase 1 test commit ("fails to compile: Store.ArchiveAs does not exist yet"); `internal/store/archive_authz_test.go:TestArchiveAsOwnerGate` (and five sibling tests) run GREEN live against Qdrant (`ENGRAM_REQUIRE_QDRANT=1 go test`), asserting owner success, shared-reader `ErrNotFound` + unmutated record, and anonymous rejection |
| SC2 | The CLI's subject-less `spine-review archive`/`restore` path keeps working unchanged | ✓ VERIFIED | `cmd/engram/spine_review_archive.go` is byte-identical to commit `16f44eda` (`diff` confirmed empty); `Store.Archive`/`Store.Restore` signatures unchanged in `internal/store/spine.go`; all pre-existing `TestArchive*`/`TestRestore*` store tests and the `cmd/engram` archive/restore tests pass live |
| SC3 | `RelatedMemories(subj, id)` returns supersession, shared-tag, shared-citation, and vector-neighbour edges with the caller's read predicate composed into the Qdrant filter (never post-filtered), a bounded edge count, and a documented multi-edge rule | ✓ VERIFIED | `internal/store/relatedmemories.go`: all four edge helpers (`relatedSupersessionChain`, `relatedCitationEdges`, `relatedTagEdges`, `relatedVectorEdges`) route through `recallVisibleFilter`/`edgeFilter`, and `assembleRelated` fetches candidate payloads via `fetchPayloadsByID(ctx, f, ...)` under that same filter — never a post-filter; `relatedTotalCeiling`=64 bounds the merged result with a compile-time assertion (`const _ = uint(relatedTotalCeiling - relatedSupersessionCap - relatedCitationCap - relatedTagCap - 1)`) that per-type caps stay below it; D-06's one-entry-per-candidate rule is documented in `RelatedMemory`'s doc comment and enforced by `assembleRelated`'s dedup-by-id logic. All 16 `TestRelatedMemories*` tests pass live against Qdrant |
| SC4 | `ListTags(subj, scope)` returns facet counts over a new `tags` payload index under the caller's read filter, and the recall-gate test allowlist recognizes a filtered `Facet` call | ✓ VERIFIED | `internal/store/store.go:676` adds `{"tags", qdrant.FieldType_FieldTypeKeyword, nil}` to `ensureIndexes`; `internal/store/listtags.go:ListTags`/`facetTags` compose `recallVisibleFilter` into the `Facet` request; `internal/store/schemaversion_recallgate_test.go` lists `Facet` in `recognizedFilterCarryingRequestMethods`, has a `*qdrant.FacetCounts` arm in the capture interceptor, and seeds `Store.ListTags` as a recall entry point. `TestEnsureIndexesCreatesTagsIndex`, `TestListTags*` (6 tests), `TestRecallEmissionSetIsCompleteAndClassified`, and `TestSchemaVersionNeverGatesRecall` all pass live |

### Observable Truths (Plan-Level Decisions, Representative Sample)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| D-01/D-02 | One Cedar action `authz.ActionArchive` gates both `ArchiveAs` and `RestoreAs`; both lock first via `s.locker.Lock`, then `getWritable(..., authz.ActionArchive)` inside the lock, then share `archiveResolved`/`restoreResolved` with the subject-less verbs; no `.cedar` policy files changed | ✓ VERIFIED | `internal/authz/authz.go:32` (`ActionArchive Action = "archive"`); `internal/store/spine.go:837-861` (`ArchiveAs`) and `:948-972` (`RestoreAs`) show lock-then-getWritable-then-shared-core; `git diff` on `.cedar` files shows no changes (confirmed by 01-REVIEW.md and by policy corpus test passing unmodified) |
| D-03/D-04 | Owner can archive/restore a `rule`-category record; a readable-but-not-owned record returns identical `ErrNotFound`/`Outcome: NotFound` to a nonexistent id | ✓ VERIFIED | `TestArchiveAsRestoreAsOwnedRule` and `TestArchiveAsFailsClosed` (byte-for-byte error-text equality per 01-REVIEW.md) pass live |
| D-05 | `TestArchiveAsOwnerGate` is the phase's first test, committed RED | ✓ VERIFIED | See SC1 evidence above |
| D-06 | One `RelatedMemory` entry per candidate, `Edges` list every reaching type in canonical order (supersession, citation, tag, vector), never returned twice | ✓ VERIFIED | `relatedEdgeRank` fixes canonical order; `assembleRelated`'s `index`/`seen` maps enforce one-entry-per-id; `TestRelatedMemoriesMultiEdgeEntry` passes live |
| D-07 | Shared-tag edges are rarity-weighted (`ln(n/df)`), a tag carried by >half the visible set is ubiquitous and contributes no weight/edge, top-8 kept | ✓ VERIFIED | `relatedTagEdges` (relatedmemories.go:217-339) implements exactly this; `TestRelatedMemoriesTagEdgeRarity` explicitly asserts a ubiquitous "common" tag never appears in `SharedTags` and computes exact expected weights |
| D-08 | Shared citation = same `kind`+`ref` via a nested filter, independent of locator/pin/excerpt | ✓ VERIFIED | `relatedCitationEdges` uses `qdrant.NewNestedFilter("citations", ...)` matching kind+ref only; `TestRelatedMemoriesCitationEdge` passes live |
| D-09 | Supersession chain walks forward (`superseded_by`) and backward (`supersedes`, breadth-first) under a depth cap, silently stopping a branch on an unreadable/nonexistent member | ✓ VERIFIED | `relatedSupersessionChain` (relatedmemories.go:472-524); `TestRelatedMemoriesSupersessionChain`/`TestRelatedMemoriesSupersessionCaps` pass live |
| D-10/D-11 | Every gated sub-query and the payload fetch carry the caller's cross-spine read + recall-gate filter; supersession-chain members appear even when soft-hidden, but vector/tag/citation edges exclude archived/superseded/expired/scheduled records | ✓ VERIFIED | `recallVisibleFilter` composed via `edgeFilter` into every `Count`/`Facet`/`Scroll`/`Query`; `GetReadable` (not recall-gated) used only for the chain walk; `TestRelatedMemoriesGatedEdgesFollowRecallGate`/`TestRelatedMemoriesTagAndCitationEdgesFollowGates`/`TestRelatedMemoriesReadScope` pass live |
| D-12 | Per-edge-type caps (vector 8/k, supersession 16, tag 8, citation 8) stay below `relatedTotalCeiling` (64) enforced at compile time; `Truncated` set only when a vector candidate is dropped | ✓ VERIFIED | Compile-time assertion at relatedmemories.go:360; `TestRelatedMemoriesBounds` pins the exact-8/k=3/k=1000-truncated-at-64 boundaries live |
| D-13/D-14/D-15/D-16 | `ListTags` counts recall-visible-only records under the caller's (possibly cross-spine) read filter, exact counts, descending order, `more` flag never a silent cut, backed by a new `tags` keyword index | ✓ VERIFIED | See SC4 evidence; `TestListTagsRecallVisibleOnly`, `TestListTagsScope`, `TestListTagsMatchesListTotals`, `TestListTagsLimitAndMore` all pass live |
| STORE-01 concurrency (backstop) | ArchiveAs/RestoreAs concurrent calls on one id serialize under the shared lock, leaving exactly one terminal state | ⚠️ insufficient_spec (behavior_unverified) | No test drives ArchiveAs/RestoreAs through a controlled concurrent interleaving (only the subject-less Archive/Restore have barrier-controlled concurrency tests: `TestArchiveSurvivesConcurrentUpdate`/`TestRestoreSurvivesConcurrentUpdate`). Plan tags this `verification: backstop` — routed to human verification, not counted as verified |
| STORE-03 concurrency (backstop) | ListTags is an exact point-in-time snapshot; concurrent writes never expose an unreadable record | ⚠️ insufficient_spec (behavior_unverified) | No concurrent-write test exists for ListTags. Plan tags this `verification: backstop` — routed to human verification, not counted as verified |
| STORE-02 concurrency (backstop) | A RelatedMemories candidate disappearing between the vector Query and the payload fetch is dropped silently, never surfaced as an error | ⚠️ insufficient_spec (behavior_unverified) | No timing-controlled test exercises this race window in `relatedmemories_test.go`. Plan tags this `verification: backstop` — routed to human verification, not counted as verified |

**Score:** 15/18 sampled must-haves verified (3 present-and-wired-but-behaviorally-unproven backstop truths routed to human verification). Every roadmap Success Criterion (SC1-SC4) is fully VERIFIED. The full plan-level must_haves lists (46 truths across the four PLAN.md files) were cross-checked via `gsd-tools query verify.artifacts`/`verify.key-links` (all pass) and targeted code reading; the table above is a representative sample covering every decision (D-01 through D-16) plus the three explicitly `backstop`-tagged concurrency truths, which is where all gaps in coverage live.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/authz/authz.go` | `ActionArchive` constant | ✓ VERIFIED | `ActionArchive Action = "archive"` present |
| `internal/authz/schema.json` | reference Cedar schema entry | ✓ VERIFIED | `"archive": {` present |
| `internal/authz/policy_corpus_test.go` | `ActionArchive` in corpus | ✓ VERIFIED | present in `allActions` and shared-read-deny list |
| `internal/store/spine.go` | `ArchiveAs`/`RestoreAs` + shared cores | ✓ VERIFIED | present, matches signature |
| `internal/store/archive_authz_test.go` | authz-in-store tests | ✓ VERIFIED | `TestArchiveAsOwnerGate` + 5 more, all pass live |
| `internal/store/listtags.go` | `TagCount`, `ListTags`, `facetTags` | ✓ VERIFIED | present, matches signature |
| `internal/store/store.go` | `tags` keyword payload index | ✓ VERIFIED | present in `ensureIndexes` |
| `internal/store/listtags_test.go` | ListTags integration tests | ✓ VERIFIED | 6 tests, all pass live |
| `internal/store/relatedmemories.go` | `RelatedMemories` + full edge set | ✓ VERIFIED | present, matches signature, all 4 edge types |
| `internal/store/relatedmemories_test.go` | RelatedMemories tests | ✓ VERIFIED | 16 tests, all pass live |
| `internal/store/schemaversion_recallgate_test.go` | widened recall gate (Facet, ListTags, RelatedMemories, tag/citation transmitters) | ✓ VERIFIED | all classifications present; gate tests pass live |

Programmatic confirmation via `gsd-tools query verify.artifacts` on all four PLAN.md files: all_passed=true for every plan (5/5, 4/4, 3/3, 3/3 artifacts).

### Key Link Verification

All key_links declared across the four plans (16 total) verified via `gsd-tools query verify.key-links`: every link's pattern found in source, `verified: true` for all. Representative links:

| From | To | Via | Status |
|------|-----|-----|--------|
| `internal/store/spine.go` | `internal/store/store.go` | `ArchiveAs`/`RestoreAs` gate through `getWritable(..., authz.ActionArchive)` | ✓ WIRED |
| `internal/store/listtags.go` | `internal/store/listtags.go` | `ListTags` counts through `facetTags` | ✓ WIRED |
| `internal/store/relatedmemories.go` | `internal/store/searchfetch.go` | candidates fetched under the sub-queries' own filter via `fetchPayloadsByID` | ✓ WIRED |
| `internal/store/relatedmemories.go` | `internal/store/listtags.go` | rarity weights read the same `facetTags` `ListTags` uses | ✓ WIRED |
| `cmd/engram/spine_review_archive.go` | `internal/store/spine.go` | CLI still calls subject-less `Archive` by function value | ✓ WIRED |

### Behavioral Spot-Checks / Test Execution

Ran the phase's named tests live against a real Qdrant (testcontainers, `ENGRAM_REQUIRE_QDRANT=1`) rather than trusting SUMMARY/REVIEW claims:

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Archive/Restore authz gate | `go test ./internal/store/ -run 'TestArchiveAsOwnerGate\|TestRestoreAsOwnerGate\|TestArchiveAsRestoreAsOwnedRule\|TestArchiveAsFailsClosed\|TestArchiveAsAnonymousBucket\|TestArchiveAsIdempotent' -v` | all PASS | ✓ PASS |
| Pre-existing Archive/Restore + CLI unchanged | `go test ./internal/store/ -run 'TestArchive\|TestRestore'` and `go test ./cmd/engram/... -run 'Archive\|Restore'` | 18 + 10 tests, all PASS | ✓ PASS |
| ListTags facet counts | `go test ./internal/store/ -run 'TestListTags\|TestEnsureIndexesCreatesTagsIndex' -v` | 7 tests PASS | ✓ PASS |
| RelatedMemories (all 4 edge types) | `go test ./internal/store/ -run 'TestRelatedMemories' -v` | 16 tests PASS | ✓ PASS |
| Recall gate widened for Facet/ListTags/RelatedMemories/tag+citation transmitters | `go test ./internal/store/ -run 'TestRecallEmissionSetIsCompleteAndClassified\|TestSchemaVersionNeverGatesRecall' -v` | PASS | ✓ PASS |
| Cedar policy corpus (ActionArchive) | `go test ./internal/authz/... -run 'TestPolicyCorpus' -v` | 6 tests PASS | ✓ PASS |
| Byte-identity of CLI archive path | `git show 16f44eda:cmd/engram/spine_review_archive.go` diffed against HEAD | empty diff | ✓ PASS |
| Build/format | `go build ./...`, repo `gofmt -l .` | clean | ✓ PASS |
| No stray write RPCs in RelatedMemories | `rg 'SetPayload\|Upsert\|DeletePayload' internal/store/relatedmemories.go` | no matches | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| STORE-01 | 01-01 | Authz-gated Archive/Restore, owner-write-gate parity, CLI unchanged | ✓ SATISFIED | `requirements-completed: [STORE-01]` in 01-01-SUMMARY.md; code + live tests confirm |
| STORE-02 | 01-03, 01-04 | RelatedMemories with 4 edge types, composed read filter, bounded, documented multi-edge rule | ✓ SATISFIED | `requirements-completed: [STORE-02]` recorded on 01-04-SUMMARY.md (01-03 declared `[]` due to the shared-ID gate noted in its own SUMMARY — both plans jointly implement STORE-02); code + live tests confirm |
| STORE-03 | 01-02 | ListTags facet counts, new `tags` index, recall-gate allowlist widened | ✓ SATISFIED | `requirements-completed: [STORE-03]` in 01-02-SUMMARY.md; code + live tests confirm |

REQUIREMENTS.md rows for STORE-01/02/03 still show table status `Mapped` rather than a completed marker — per the task's known tooling limitation, `requirements.mark-complete` refuses to flip a `Mapped` row, so executors recorded completion in each SUMMARY's `requirements-completed` field instead. This is not treated as a gap.

No orphaned requirements: `grep -E "Phase 1" .planning/REQUIREMENTS.md` maps only STORE-01/02/03 to this phase, and all three are declared across the four plans' `requirements` frontmatter.

### Anti-Patterns Found

None. Scanned all files in `covered_files` plus every artifact path from the four plans for `TODO|FIXME|XXX|TBD|HACK|PLACEHOLDER|placeholder|not yet implemented|not available|coming soon` (case-insensitive): zero matches in any phase-touched source or test file.

### Human Verification Required

1. **Concurrent ArchiveAs/RestoreAs on one id**
   **Test:** Drive `ArchiveAs` and `RestoreAs` on the same id through a controlled concurrent interleaving (mirroring `TestArchiveSurvivesConcurrentUpdate`'s barrier seam, but against the gated wrappers).
   **Expected:** The record ends in exactly one terminal state (archived or restored), never a torn write.
   **Why human:** No test exercises this specific interleaving for the new gated wrappers; the plan itself tags this truth `verification: backstop`, and presence/wiring of the shared lock is not, by the verification rules, sufficient evidence on its own.

2. **Concurrent write during a ListTags facet read**
   **Test:** Issue `ListTags` concurrently with a write that adds/removes a tag on a record outside the caller's read scope.
   **Expected:** The returned facet counts never reflect a record the caller cannot read, at any point-in-time snapshot.
   **Why human:** No concurrent-write test exists for `ListTags`; plan tags this truth `verification: backstop`.

3. **RelatedMemories candidate disappearing mid-call**
   **Test:** Delete or revoke read access to a vector-neighbour candidate between the vector `Query` and the payload fetch.
   **Expected:** The candidate is silently dropped from `Related`, no error surfaces, and `Truncated` is unaffected by this specific drop.
   **Why human:** No timing-controlled test exercises this race window; plan tags this truth `verification: backstop`.

4. **01-03's judgment-tier prohibition — no blended cross-type score**
   **Test:** Confirm `RelatedEdge`/`assembleRelated` never fold vector score, tag weight, citation count, and supersession depth into one comparable ranking number.
   **Expected:** Per-type evidence fields only; admission and sort order strictly by `relatedEdgeRank` (fixed type order), never a blended score.
   **Why human:** This verifier's code read confirms the structural claim, but the plan's own frontmatter marks this prohibition `verification: judgment` (not test-backed) — per the judgment-tier prohibition policy, that calls for an explicit human sign-off rather than resting on an LLM-judge verdict alone.

### Gaps Summary

No gaps that block the phase goal. All four roadmap Success Criteria are fully verified with live test evidence (Qdrant via testcontainers), not merely SUMMARY/REVIEW claims. The one open thread is three concurrency-shaped truths that each PLAN explicitly marked `verification: backstop` (meaning: the planner recognized these as hard to pin with a deterministic test and flagged them for exactly this kind of review) plus one judgment-tier prohibition. All three backstop truths share the same underlying structural argument — the new gated/read methods reuse the exact lock, core, and filter-composition code paths that already have either a dedicated concurrency test (Archive/Restore) or a static AST/interceptor conformance check (the recall gate) — but none has a dedicated timing-controlled test proving the specific new-method invariant. This routes the phase to `human_needed` rather than `passed`, per the verification rules' treatment of `backstop`-tagged truths (presence + wiring never counts as sufficient evidence on its own). None of these are grounds to withhold phase completion; they are recommended follow-up hardening, most naturally as a small addendum to `internal/store/archive_authz_test.go`/`listtags_test.go`/`relatedmemories_test.go` reusing the existing `updateAfterReadHook`-style barrier seam.

---

*Verified: 2026-09-26T04:45:00Z*
*Verifier: Claude (gsd-verifier)*
