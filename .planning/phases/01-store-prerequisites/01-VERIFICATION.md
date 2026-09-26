---
phase: 01-store-prerequisites
verified: 2026-09-26T01:20:00Z
status: human_needed
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
covered_digest: "v1:sha256:8378b8f9b1dfa440aa45f9ac450faa5bd6384573dc62f923397f8cefa0d04e03"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 15/18
  gaps_closed:
    - "STORE-01 concurrency (backstop): ArchiveAs/RestoreAs concurrent calls on one id serialize under the shared lock, leaving exactly one terminal state — now proven by TestArchiveAsRestoreAsConcurrentSerialize (32 concurrent calls, -race, real Qdrant)"
    - "STORE-03 concurrency (backstop): ListTags is an exact point-in-time snapshot; concurrent writes never expose an unreadable record — now proven by TestListTagsUnderConcurrentWrites (-race, real Qdrant, two concurrent writer goroutines)"
    - "STORE-02 concurrency (backstop): a RelatedMemories candidate disappearing between the vector Query and the payload fetch is dropped silently, never surfaced as an error — now proven deterministically by TestRelatedMemoriesCandidateVanishesBeforeFetch via the new relatedBeforeFetchHook test seam"
  gaps_remaining:
    - "01-03's judgment-tier prohibition (\"MUST NOT fold edge types into one comparable score or re-rank across types\") is strengthened by a new passing behavioral test (TestRelatedMemoriesAdmitsByTypeNotScore) but the plan declares this prohibition verification: judgment, not test. Per the judgment-tier prohibition policy, a judgment-tier item is never closed to an authoritative pass by test evidence alone — it always routes to an explicit human sign-off (interactive) or a flagged non-authoritative LLM-judge verdict (autonomous). This verifier's judgment: the new test is strong, directly-applicable evidence (confirms RelatedEdge carries only per-type fields and assembleRelated's admission order strictly follows the citations/tags/vector call-site order, matching relatedEdgeRank) — but this remains a flagged item requiring human sign-off, not a resolved gap."
  regressions: []
behavior_unverified_items: []
human_verification:
  - test: "01-03's judgment-tier prohibition — no blended cross-type score"
    expected: "Per-type evidence fields only; admission and sort order strictly by relatedEdgeRank (fixed type order), never a blended score."
    why_human: "The plan's own frontmatter marks this prohibition verification: judgment (not test-backed). A new behavioral test (TestRelatedMemoriesAdmitsByTypeNotScore, commit 14c7aa5e) now demonstrates this directly and passes live against real Qdrant with -race, and this verifier independently re-ran it and confirmed the admission-order code path (assembleRelated is called with citations, tags, vector in that literal order, matching relatedEdgeRank's canonical order) — but per the judgment-tier prohibition policy, a judgment-tier item is never auto-resolved by test evidence; it requires an explicit human sign-off rather than resting on an LLM-judge verdict (mine or the executor's) alone."
---

# Phase 01: Store Prerequisites Verification Report

**Phase Goal:** `internal/store` gains authz-gated Archive/Restore, RelatedMemories, and ListTags as pure store methods, so the authz shape and test-infrastructure gaps are settled before any RPC or UI is built on top of them.
**Verified:** 2026-09-26T01:20:00Z
**Status:** human_needed
**Re-verification:** Yes — after gap closure (commit `14c7aa5e`)

## Re-Verification Summary

The prior verification (2026-09-26T04:45:00Z) found all four roadmap Success Criteria fully VERIFIED but routed the phase to `human_needed` on four items: three plan-tagged `verification: backstop` concurrency truths (STORE-01, STORE-02, STORE-03) with no behavioral test, plus one `verification: judgment` prohibition (no blended cross-type score) with only a structural code read as evidence. The user declined manual testing of these four items.

Commit `14c7aa5e` (`test(store): pin phase 01 concurrency and type-order guarantees`) adds `internal/store/concurrent_gates_test.go` (4 new tests) plus a new test-only seam, `relatedBeforeFetchHook` (`internal/store/relatedmemories.go:139`, nil in production, mirrors the existing `updateAfterReadHook` pattern), to make the RelatedMemories vanish-between-query-and-fetch race deterministic.

This verifier independently re-ran all four new tests, and the full `internal/store` package suite, **from its own process** (not trusting the orchestrator's or SUMMARY's PASS claims):

```
ENGRAM_REQUIRE_QDRANT=1 go test -race ./internal/store/ \
  -run 'TestArchiveAsRestoreAsConcurrentSerialize|TestListTagsUnderConcurrentWrites|TestRelatedMemoriesCandidateVanishesBeforeFetch|TestRelatedMemoriesAdmitsByTypeNotScore' -v
--- PASS: TestArchiveAsRestoreAsConcurrentSerialize (0.78s)
--- PASS: TestListTagsUnderConcurrentWrites (0.63s)
--- PASS: TestRelatedMemoriesCandidateVanishesBeforeFetch (0.63s)
--- PASS: TestRelatedMemoriesAdmitsByTypeNotScore (0.69s)
ok  	github.com/seanb4t/engram/internal/store	4.939s

ENGRAM_REQUIRE_QDRANT=1 go test -race ./internal/store/...
ok  	github.com/seanb4t/engram/internal/store	192.587s
ok  	github.com/seanb4t/engram/internal/store/storetest	10.043s
```

**Verdict on each of the four closed/attempted items:**

1. **STORE-01 backstop (ArchiveAs/RestoreAs serialize)** — CLOSED, ✓ VERIFIED. `TestArchiveAsRestoreAsConcurrentSerialize` fires 32 concurrent `ArchiveAs`/`RestoreAs` calls (alternating) on one id through `wg.Go`, released simultaneously via a `start` channel, then asserts every call returns `Changed` or `Already` with no error, the final `archived_at` state matches the parity of `Changed` outcomes (proving serialization — a torn/lost-update interleaving would produce a parity mismatch), and the rest of the payload (`Content`/`Owner`/`Visibility`/`Tags`) is untorn. This directly exercises the declared invariant using the real gated wrappers and the real per-id lock, not a structural argument.

2. **STORE-03 backstop (ListTags point-in-time snapshot)** — CLOSED, ✓ VERIFIED. `TestListTagsUnderConcurrentWrites` runs two real writer goroutines (one continuously upserting another owner's private records carrying the same "hot" tag plus a private-only "b-only" tag; one continuously toggling the caller's own record via real `ArchiveAs`/`RestoreAs`) concurrently with 25 `ListTags` calls, asserting the "hot" count only ever reads 2 or 3 (never inflated by the other owner's private growing set, and the private-only tag never surfaces) and no unexpected tag ever appears. This is a genuine concurrent-write test against the caller's read + recall-gate filter, not a static filter-composition argument.

3. **STORE-02 backstop (RelatedMemories candidate vanishes)** — CLOSED, ✓ VERIFIED. `TestRelatedMemoriesCandidateVanishesBeforeFetch` uses the new `relatedBeforeFetchHook` seam to deterministically fire real `Delete` and `SetVisibility` calls against two of three vector-neighbour candidates in the exact window between the sub-query and `fetchPayloadsByID`'s payload fetch, then asserts the two vanished candidates are silently absent (no error) from the result, the surviving candidate is unaffected, and `Truncated` is `false`. Read `internal/store/relatedmemories.go:611-685` (`assembleRelated`) to confirm the hook is a genuine pre-fetch seam (called once, right before the real `fetchPayloadsByID` call) rather than a shortcut that bypasses the production code path — confirmed: the hook only observes/mutates via real store calls (`s.Delete`, `s.SetVisibility`), and the fetch itself is the unmodified production call.

4. **01-03 judgment-tier prohibition (no blended cross-type score)** — NOT CLOSED, remains flagged for human sign-off (see `human_verification` below). `TestRelatedMemoriesAdmitsByTypeNotScore` is a strong new behavioral test — it seeds a citation-only candidate and a near-identical vector neighbour, and with `k=1` (only the vector candidate would get a vector edge) asserts the citation-only candidate is admitted *and ordered first*, each entry carrying only its own type's evidence field. Reading the call site (`internal/store/relatedmemories.go:754`: `s.assembleRelated(ctx, f, anchor.ID, chain, citations, tags, vector)`) confirms admission order is enforced by the code's own argument order (citations before vector), matching `relatedEdgeRank`'s canonical order — this is not incidental/fixture-only ordering, it is the production call site itself. However, the plan's frontmatter fixes this prohibition's `verification` field to `judgment`, and per the judgment-tier prohibition policy a judgment-tier item is never auto-resolved to an authoritative pass by test evidence, regardless of how strong that evidence is — it always requires either an explicit human sign-off (interactive verify) or a flagged, non-authoritative LLM-judge verdict (autonomous verify). This verifier's own code read is exactly such a non-authoritative judgment and cannot itself close the item. It remains a `human_verification` entry with strengthened evidence attached.

Because one human-verification item remains (the judgment-tier prohibition), the phase status stays `human_needed`, not `passed`, per Step 9 Rule 2 of the verification decision tree (a passing status requires an empty human-verification section) and the explicit prohibition-routing rule ("a flagged prohibition... must never be silently absorbed into a passed verdict").

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

Unchanged from the initial verification — all four roadmap Success Criteria (SC1-SC4) were already ✓ VERIFIED with live test evidence and are unaffected by this gap-closure commit. See the initial verification's evidence (retained below for continuity).

| # | Truth (Roadmap SC) | Status | Evidence |
|---|------|--------|----------|
| SC1 | A test proves a caller can archive/restore only records they own — a shared record they can read is rejected exactly as Delete/Update/Supersede reject it — enforced in `internal/store`, the phase's first test | ✓ VERIFIED | `internal/store/archive_authz_test.go:TestArchiveAsOwnerGate` + 5 sibling tests pass live against Qdrant (re-confirmed in the full-package run above) |
| SC2 | The CLI's subject-less `spine-review archive`/`restore` path keeps working unchanged | ✓ VERIFIED | `cmd/engram/spine_review_archive.go` byte-identical to `16f44eda`; unaffected by `14c7aa5e` |
| SC3 | `RelatedMemories(subj, id)` returns supersession, shared-tag, shared-citation, and vector-neighbour edges with the caller's read predicate composed into the Qdrant filter, a bounded edge count, and a documented multi-edge rule | ✓ VERIFIED | `internal/store/relatedmemories.go`; all 16 pre-existing `TestRelatedMemories*` tests plus the 2 new ones pass live in the full-package run above |
| SC4 | `ListTags(subj, scope)` returns facet counts over a new `tags` payload index under the caller's read filter, and the recall-gate test allowlist recognizes a filtered `Facet` call | ✓ VERIFIED | `internal/store/listtags.go`; unaffected by `14c7aa5e`, re-confirmed in the full-package run above |

### Observable Truths (Plan-Level Decisions, Representative Sample)

D-01 through D-16 and all non-concurrency truths are unchanged from the initial verification (✓ VERIFIED, see prior report for evidence; re-confirmed live via the full-package run above). The three backstop concurrency truths, previously routed to human verification, are now resolved with direct behavioral evidence:

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| STORE-01 concurrency (backstop) | ArchiveAs/RestoreAs concurrent calls on one id serialize under the shared lock, leaving exactly one terminal state | ✓ VERIFIED | `TestArchiveAsRestoreAsConcurrentSerialize` (32 concurrent calls, `-race`, real Qdrant) — independently re-run, PASS |
| STORE-03 concurrency (backstop) | ListTags is an exact point-in-time snapshot; concurrent writes never expose an unreadable record | ✓ VERIFIED | `TestListTagsUnderConcurrentWrites` (`-race`, real Qdrant, two concurrent writer goroutines) — independently re-run, PASS |
| STORE-02 concurrency (backstop) | A RelatedMemories candidate disappearing between the vector Query and the payload fetch is dropped silently, never surfaced as an error | ✓ VERIFIED | `TestRelatedMemoriesCandidateVanishesBeforeFetch` via `relatedBeforeFetchHook` — independently re-run, PASS |
| 01-03 prohibition (judgment-tier): no blended cross-type score | RelatedEdge/assembleRelated never fold vector score, tag weight, citation count, and supersession depth into one comparable ranking number | ⚠️ flagged (judgment-tier) | `TestRelatedMemoriesAdmitsByTypeNotScore` passes live and this verifier's code read confirms the admission-order mechanism, but the plan declares `verification: judgment` — routes to human sign-off per policy, not auto-resolved by test evidence |

**Score:** 18/18 truths verified (0 present-and-wired-but-behaviorally-unproven backstop truths remain — all three were closed by direct behavioral tests independently re-run by this verifier). One judgment-tier prohibition remains flagged for human sign-off, per policy, and is not counted in the truths score (prohibitions are tracked separately from truths).

### Required Artifacts

Unchanged from initial verification, plus the new test file:

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/concurrent_gates_test.go` | Concurrency + type-order behavioral tests | ✓ VERIFIED | 4 tests present, all pass live (`-race`, real Qdrant); `relatedBeforeFetchHook` seam correctly nil-guarded in production (`internal/store/relatedmemories.go:139,629-630`) |

All artifacts listed in the initial verification (`internal/authz/authz.go`, `internal/store/spine.go`, `internal/store/archive_authz_test.go`, `internal/store/listtags.go`, `internal/store/store.go`, `internal/store/listtags_test.go`, `internal/store/relatedmemories.go`, `internal/store/relatedmemories_test.go`, `internal/store/schemaversion_recallgate_test.go`) are unaffected by this commit and remain ✓ VERIFIED.

### Key Link Verification

Unchanged — all 16 declared key_links across the four plans remain ✓ WIRED (unaffected by the new test-only file, which adds no new production key links; `relatedBeforeFetchHook` is a nil-by-default test seam, not a production wiring path).

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| ArchiveAs/RestoreAs concurrent serialization | `ENGRAM_REQUIRE_QDRANT=1 go test -race ./internal/store/ -run TestArchiveAsRestoreAsConcurrentSerialize -v` | PASS (0.78s) | ✓ PASS |
| ListTags under concurrent writes | `ENGRAM_REQUIRE_QDRANT=1 go test -race ./internal/store/ -run TestListTagsUnderConcurrentWrites -v` | PASS (0.63s) | ✓ PASS |
| RelatedMemories candidate vanishes before fetch | `ENGRAM_REQUIRE_QDRANT=1 go test -race ./internal/store/ -run TestRelatedMemoriesCandidateVanishesBeforeFetch -v` | PASS (0.63s) | ✓ PASS |
| RelatedMemories admits by type, not blended score | `ENGRAM_REQUIRE_QDRANT=1 go test -race ./internal/store/ -run TestRelatedMemoriesAdmitsByTypeNotScore -v` | PASS (0.69s) | ✓ PASS |
| Full internal/store package regression | `ENGRAM_REQUIRE_QDRANT=1 go test -race ./internal/store/...` (run once) | `ok internal/store 192.587s`, `ok internal/store/storetest 10.043s` | ✓ PASS |

All five checks were run independently by this verifier, from a fresh process, against real Qdrant (testcontainers) with the race detector enabled — not inferred from SUMMARY.md or the orchestrator's narration.

### Requirements Coverage

Unchanged from initial verification — STORE-01, STORE-02, STORE-03 remain ✓ SATISFIED; no new requirements introduced by this gap-closure commit.

### Anti-Patterns Found

None. `internal/store/concurrent_gates_test.go` scanned for `TODO|FIXME|XXX|TBD|HACK|PLACEHOLDER` (case-insensitive): zero matches. The new `relatedBeforeFetchHook` var is correctly documented as test-only and nil-guarded in production code (`internal/store/relatedmemories.go:629`: `if relatedBeforeFetchHook != nil {`).

### Human Verification Required

1. **01-03's judgment-tier prohibition — no blended cross-type score**
   **Test:** Confirm `RelatedEdge`/`assembleRelated` never fold vector score, tag weight, citation count, and supersession depth into one comparable ranking number.
   **Expected:** Per-type evidence fields only; admission and sort order strictly by `relatedEdgeRank` (fixed type order), never a blended score.
   **Why human:** The plan's own frontmatter marks this prohibition `verification: judgment` (not test-backed). A new behavioral test (`TestRelatedMemoriesAdmitsByTypeNotScore`, commit `14c7aa5e`) now demonstrates this directly and passes live against real Qdrant with `-race`, and this verifier independently re-ran it and confirmed the admission-order code path (`assembleRelated` is called with `citations, tags, vector` in that literal order at `internal/store/relatedmemories.go:754`, matching `relatedEdgeRank`'s canonical order) — but per the judgment-tier prohibition policy, a judgment-tier item is never auto-resolved by test evidence; it requires an explicit human sign-off rather than resting on an LLM-judge verdict (mine or the executor's) alone.

### Gaps Summary

No blocking gaps. Three of the four items that previously routed this phase to `human_needed` are now closed with direct, independently-reproduced behavioral test evidence (`TestArchiveAsRestoreAsConcurrentSerialize`, `TestListTagsUnderConcurrentWrites`, `TestRelatedMemoriesCandidateVanishesBeforeFetch`) — all pass live against real Qdrant with the race detector enabled, and the full `internal/store` package suite passes cleanly alongside them (no regressions introduced). All four roadmap Success Criteria remain fully verified.

The remaining item — 01-03's judgment-tier "no blended cross-type score" prohibition — is now backed by a strong, independently-verified behavioral test (`TestRelatedMemoriesAdmitsByTypeNotScore`) plus direct code confirmation of the admission-order mechanism, but per this project's judgment-tier prohibition policy this class of item is never auto-resolved to a `passed` status by test evidence alone; it requires an explicit human sign-off. This keeps the phase at `human_needed` rather than `passed`, with exactly one flagged item remaining (down from four).

---

*Verified: 2026-09-26T01:20:00Z*
*Verifier: Claude (gsd-verifier)*
