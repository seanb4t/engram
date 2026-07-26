---
phase: 26-structured-citations-category-filter-chat-base-url
plan: 05
subsystem: api
tags: [citations, provenance, qdrant-payload, connect-rpc, idempotency, authz]

# Dependency graph
requires:
  - phase: 26-03
    provides: SearchMemoriesRequest.categories = 8 shipped and threaded (unrelated field, same connectapi.go region this plan also edits)
  - phase: 26-04
    provides: internal/openaiurl chat base URL wiring (unrelated files; wave-4 sibling)
provides:
  - "store.Memory.Citations optional on ANY category (payload() gate split, D-01) — required-1 for discoveries unchanged"
  - "storeArgs.Citations single declaration, inherited by store_memory/schedule_memory/supersede_memory via Go field embedding (D-04)"
  - "toMemory maps a.Citations into store.Memory.Citations — closes the research-flagged silent-drop gap"
  - "validateCitations(cites, minCount) shared validator extracted from validateStoreDiscovery's inline loop (D-05)"
  - "shapeProtoMemories clears Citations/Kind on Connect's compact (full=false) ListMemories/SearchMemories response (D-07)"
affects: [curating-memory-skill, docs-site-tool-pages, future-citation-consumers]

# Tech tracking
tech-stack:
  added: []
  patterns: ["independent optional-payload-key gates (kind vs. citations) replacing one shared category conditional", "shared validator parameterized by a minCount floor instead of two near-duplicate validation loops"]

key-files:
  created: []
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/server/tools.go
    - internal/server/tools_test.go
    - internal/server/connectapi.go
    - internal/server/connectapi_test.go

key-decisions:
  - "D-01 gate split: payload()'s single `category == \"discovery\"` block became two independent conditionals — kind stays discovery-exclusive, citations write whenever len(m.Citations) > 0 for ANY category"
  - "D-02 confirmed by construction: citations write EXCLUSIVELY inside payload(); no targeted SetPayload anywhere references the citations key, so Update/UpdatePayload/Supersede/IncrementAccess preserve citations for free — proven by six sub-tests, zero production-code changes in Task 2"
  - "D-04 field-embedding precedent (IdempotencyKey) reused verbatim for Citations: one declaration on storeArgs, inherited by scheduleArgs and supersedeArgs"
  - "SUPERSEDED by 26-REVIEW CR-01 (fixed in c222c783). This plan assumed citations sat OUTSIDE the idempotency content fingerprint, so a keyed replay with different citations would replay rather than conflict. That assumption was wrong: citations are client-authored (D-04 put them on the shared storeArgs), so Phase 24's D-07 fingerprint must cover them and D-10 requires a reject. Excluding them meant a keyed retry with corrected citations returned success while silently discarding the correction. contentFingerprint now includes citations (length-prefixed per field, deliberately NOT sorted — order is caller-authored); a keyed replay with different citations returns store.ErrIdempotencyConflict. The subtest that encoded the old assumption was corrected in the same commit."
  - "validateStoreDiscovery's ref-required-non-empty check (already shipped pre-phase, contrary to the plan's stated pre-state) was preserved verbatim inside the extracted validateCitations helper — no behavior change beyond the extraction itself"
  - "D-07 Connect fix scoped to shapeProtoMemories's non-full branch only: pb.Citations = nil and pb.Kind = \"\" alongside the existing Content/Summary handling; GetMemory is never shaped, so it always returns citations by construction"

patterns-established:
  - "Optional payload key gated on its own non-empty check, matching the summary_model/short_id idiom, rather than nested inside a category conditional"
  - "citationFixture()/assertCitationsEqual() test helpers (tools_test.go) reused across Task 1's round-trip test and Task 2's six-sub-test regression suite"

requirements-completed: [REQ-memory-citations]

coverage:
  - id: D1
    description: "A memory-category record stored with citations returns those citations verbatim from get_memory, with kind/ref/locator/pin/excerpt intact and in submitted order (GAP 1)"
    requirement: "REQ-memory-citations"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestPayloadCitations"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreMemoryCitationsRoundTrip"
        status: pass
    human_judgment: false
  - id: D2
    description: "A citation-free record writes NO citations payload key at all (not an empty list) — byte-identical to pre-phase payload shape; kind stays discovery-exclusive"
    requirement: "REQ-memory-citations"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestPayloadCitations"
        status: pass
    human_judgment: false
  - id: D3
    description: "validateCitations enforces kind membership, non-empty ref, the 50-citation cap, and the 16 KiB excerpt cap identically at minCount 0 (memory) and minCount 1 (discovery, unchanged)"
    requirement: "REQ-memory-citations"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestCitationsValidation"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestValidateStoreDiscovery"
        status: pass
    human_judgment: false
  - id: D4
    description: "Citations survive every existing write path with zero preservation code added: content-changing update, shared-only payload-only update, supersession back-stamp, keyed idempotent replay, duplicate-citation persistence/ordering, and repeated access-count bumps (D-02)"
    requirement: "REQ-memory-citations"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestUpdateMemoryPreservesCitations"
        status: pass
    human_judgment: false
  - id: D5
    description: "Connect's compact (full=false) ListMemories/SearchMemories clear citations and kind; full=true and the never-shaped GetMemory return them intact (GAP 2 / D-07)"
    requirement: "REQ-memory-citations"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestConnectCompactViewOmitsCitations"
        status: pass
    human_judgment: false
  - id: D6
    description: "MCP search_memory/list_memory compact results carry no citations key while full results do — companion guard against a future recallView field reintroducing the Connect-side leak"
    requirement: "REQ-memory-citations"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestSearchListMemoryCompactViewOmitsCitations"
        status: pass
    human_judgment: false
  - id: D7
    description: "Citations add no authorization surface: a shared citation-carrying record readable by a second owner is still not writable by them (not-found-shaped error)"
    requirement: "REQ-memory-citations"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestCitationsDoNotGrantWriteAccess"
        status: pass
    human_judgment: false
  - id: D8
    description: "Citations are never auto-populated — content that deliberately looks citation-rich (file path, URL, commit SHA) produces zero citations unless the caller explicitly supplies them"
    requirement: "REQ-memory-citations"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestCitationsNotAutoPopulated"
        status: pass
    human_judgment: false

duration: 6min
completed: 2026-07-25
status: complete
---

# Phase 26 Plan 05: Structured Citations on Curated Memories Summary

**Curated `memory`-category records can now carry optional, structured `citations` (file/commit/url/repo provenance anchors) through one relaxed `payload()` write gate, a single `storeArgs.Citations` field inherited by all three write tools via Go field embedding, and a Connect-side compact-view fix that closes an information-disclosure gap MCP's hand-written recall view never had.**

## Performance

- **Duration:** 6 min (3 task commits, 20:30:08 → 20:35:56 PT)
- **Tasks:** 3/3 completed
- **Files modified:** 6 (2 production: `internal/store/store.go`, `internal/server/connectapi.go`; 4 test)

## Accomplishments

- Split `payload()`'s single `category == "discovery"` block into two independent conditionals (D-01): `kind` stays discovery-exclusive, `citations` writes whenever `len(m.Citations) > 0` for ANY category. A citation-free record's payload is byte-identical to before this change.
- Closed the phase's single highest-risk gap (GAP 1): `storeArgs.toMemory` now maps `a.Citations` into `store.Memory.Citations`. Before this, adding the field to `storeArgs` alone would have let `store_memory` report success while silently discarding citations.
- Extracted `validateCitations(cites, minCount)` from `validateStoreDiscovery`'s inline loop (D-05) — `store_discovery` keeps `minCount=1` unchanged, the three memory write handlers call it with `minCount=0`.
- Proved D-02 (citations write exclusively inside `payload()`) with a six-sub-test regression suite covering every existing write path — zero preservation code added to `Update`, `UpdatePayload`, `Supersede`, or `IncrementAccess`.
- Closed GAP 2: Connect's `shapeProtoMemories` now clears `Citations`/`Kind` in its non-full branch, matching MCP's `recallView` (which already omitted them for free as a hand-written allow-list). Without this fix, ordinary compact Connect responses could carry up to 50 citations × 16 KiB excerpts per record.
- Pinned SC1's never-auto-populated invariant and SC4's no-new-authz-surface invariant as falsifiable tests, not just design intent.

## Task Commits

1. **Task 1: Carry one citation from the MCP argument to the Qdrant payload and back (D-01..D-06)** — `d72e1654` (feat)
2. **Task 2: Prove citations survive every existing write path (D-02)** — `0c11f39b` (test)
3. **Task 3: Keep citations out of the compact recall view on both transports (D-07) and off the authz surface (D-16)** — `938beb0b` (fix)

_All three tasks were `tdd="true"`/behavior-first; no separate RED/GREEN split was applicable since each task's tests and implementation landed in one atomic commit per the plan's task-commit granularity._

## Files Created/Modified

- `internal/store/store.go` — `payload()` gate split into independent `kind` and `citations` conditionals; `Memory.Kind`/`Memory.Citations` doc comments corrected (citations no longer described as discovery-only)
- `internal/store/store_test.go` — `TestPayloadCitations` (live Upsert/Get round trip, no-citations-key assertion, kind-stays-discovery-exclusive assertion)
- `internal/server/tools.go` — `storeArgs.Citations` field; `validateCitations` extraction; `toMemory` citations mapping; `validateCitations` call added to `storeMemory`/`scheduleMemory`/`supersedeMemory`
- `internal/server/tools_test.go` — `TestStoreMemoryCitationsRoundTrip`, `TestCitationsValidation`, `TestUpdateMemoryPreservesCitations` (6 sub-tests), `TestSearchListMemoryCompactViewOmitsCitations`, `TestCitationsNotAutoPopulated`, plus shared `citationFixture()`/`assertCitationsEqual()` helpers
- `internal/server/connectapi.go` — `shapeProtoMemories`'s non-full branch clears `pb.Citations`/`pb.Kind`
- `internal/server/connectapi_test.go` — `TestConnectCompactViewOmitsCitations` (5 sub-cases), `TestCitationsDoNotGrantWriteAccess`

## Decisions Made

- **D-01/D-02/D-04/D-05/D-07** all confirmed and implemented exactly as specified in PLAN.md — see `key-decisions` above for the specific implementation choices within each.
- One factual correction versus the plan's stated pre-state: `validateStoreDiscovery`'s empty-`ref` rejection was **already shipped** (commit `dc1d29a0`, PR #28, pre-dating this phase) rather than being a new tightening introduced by the validator extraction. The extraction preserves this check verbatim; no discovery-path behavior changed.

## Deviations from Plan

None — plan executed exactly as written. The single test-authoring bug encountered (a `defer cleanupErr(t, ..., s.Delete(...))` in the first draft of `TestPayloadCitations`, which eagerly evaluated `s.Delete` as a deferred-call argument and deleted the record before the subsequent `Get`) was caught and fixed during the same task's development, before any commit — not a deviation from the plan, a normal test-development iteration.

## Issues Encountered

None blocking. One noteworthy non-issue: the harness's `date +%Y` sandbox restriction referenced in the harness note did not apply here (no web search performed this plan).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `store.Memory.Citations` and the `citationArg` wire type are stable and exercised on both transports; a future phase adding citations to `update_memory` or `store_rule` can build directly on `validateCitations`/`toMemory`'s pattern.
- Agent-facing guidance (curating-memory skill, docs-site tool pages) for the new `citations` field on `store_memory`/`schedule_memory`/`supersede_memory` is NOT yet written — flagged in `26-PATTERNS.md`'s "new capability ships with skill/docs guidance" pattern but out of this plan's file scope (`files_modified` did not include skill/docs files). Recommend a follow-up doc pass before the milestone ships.
- No blockers for `26-06` (the phase's final plan, not part of this wave).

---
*Phase: 26-structured-citations-category-filter-chat-base-url*
*Plan: 05*
*Completed: 2026-07-25*

## Self-Check: PASSED

All 6 modified source/test files and the SUMMARY.md itself confirmed present on disk; all 3 task commit hashes (`d72e1654`, `0c11f39b`, `938beb0b`) confirmed present in `git log`.
