---
gsd_state_version: 1.0
milestone: v0.9.x — Recall Quality
milestone_name: "- [x] **Phase 9: Retrieval Eval Harness & Ranking Precision** - Labeled retrieval eval"
current_phase: 9
current_phase_name: Retrieval Eval Harness & Ranking Precision
status: verifying
stopped_at: "Phase 9 Plan 03 complete (accept-d06): D-06 reranker shipped on MCP+Connect; live eval clears #261 rank bar (recall@8=1.00, MRR=1.000). Phase 9 all 3 plans done — ready for verification."
last_updated: "2026-07-10T00:38:30.321Z"
last_activity: 2026-07-09
last_activity_desc: Phase 9 execution started
progress:
  total_phases: 12
  completed_phases: 1
  total_plans: 3
  completed_plans: 3
  percent: 8
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-09)

**Core value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.
**Current focus:** Phase 9 — Retrieval Eval Harness & Ranking Precision

## Current Position

Phase: 9 (Retrieval Eval Harness & Ranking Precision) — COMPLETE (ready for verification)
Plan: 3 of 3 (all complete)
Status: Phase complete — ready for verification
Last activity: 2026-07-09 — Phase 9 Plan 03 complete (accept-d06)

Progress: [██▌░░░░░░░] 25% (v0.9.x — 1/4 phases executed: Phase 9 complete)

## Performance Metrics

**Velocity:**

- Total plans completed: 0 (v0.9.x just opened)
- Average duration: n/a
- Total execution time: n/a

**v0.9.x Phases:**

| Phase | Requirements | Status |
|-------|--------------|--------|
| 9. Retrieval Eval & Ranking Precision | 3/3 | Complete |
| 10. Asymmetric Query/Document Embeddings | 0/1 | Planned |
| 11. Async-on-Write Summaries | 0/1 | Planned |
| 12. Per-Memory Usage Signals | 0/1 | Planned |

**Shipped (v0.8.x baseline):** Phases 1–8 complete (24 requirements). See ROADMAP.md Progress table.
| Phase 09-retrieval-eval-harness-ranking-precision P01 | 20min | 2 tasks | 4 files |
| Phase 09-retrieval-eval-harness-ranking-precision P02 | 15min | 2 tasks | 3 files |
| Phase 09-retrieval-eval-harness-ranking-precision P03 | 35min | 3 tasks | 8 files |

## Accumulated Context

### Decisions

56 ADR-locked decisions (25 core + 31 companion refinements) logged in PROJECT.md `<decisions>` block (all LOCKED, source `docs/adr/engram-*.md`). Foundational for v0.9.x:

- [Phase 2]: Summary-by-default recall (DEC-ambu) — Phase 11 async summaries build on the `FillSummary` seam.
- [Phase 4]: Asymmetric embedder text-prefix knobs (DEC-zyhq) — Phase 10 extends these with native API params + doc-side prefix.
- [Phase 6]: OTLP-only non-blocking telemetry (DEC-dwi, DEC-uxh) — Phases 9 (eval) and 12 (usage signals) reuse the OTLP → ClickStack seam.
- [Phase 9]: Reused server.StoreAndEmbedderFromEnvNoEnsure() for the *embed.Client only (full prod parity); built the eval Store directly from testQdrantAddr to avoid ambient ENGRAM_QDRANT_ADDR leakage (round-2 finding 1)
- [Phase 9]: seedRecord uses a fixture-local key (not the Qdrant point ID) because Qdrant point IDs must be UUIDs; a key->UUID map resolves rank lookups
- [Phase 9]: D-04 supersession: ROADMAP Phase-9 success-criterion 2 'similarity score (opt-in)' wording is superseded by the shipped always-on search_memory score behavior (accepted as correct/better DX).
- [Phase 9]: search_memory score docs (09-02) stay order-agnostic: score FIELD only, not result ORDER — the post-rerank order caveat for reference/tools.md:95 is deliberately deferred to 09-03.
- [Phase 9]: D-06 reranker: pure lexical-overlap term-set-intersection boost (stdlib only), tie-broken by raw Score then ID; shared via Store.SearchReranked (candidateK=min(max(k*4,32),100), k<=0 rejected) called by MCP deps.searchMemory, Connect engramAPI.SearchMemories, and the retrieval eval

### Pending Todos

- `/gsd-plan-phase 9` — plan the retrieval eval harness + ranking precision phase.

### Blockers/Concerns

- **Phase 10 baseline ambiguity**: `REQ-asymmetric-embedder-params` (Phase 4, shipped) describes query/doc param maps + doc-side instruction, but GitHub #305 reports those classes (cloud native API params; E5/nomic doc-side prefix) are NOT covered by the shipped text-prefix knob. Phase-10 planning must verify the exact shipped baseline against `internal/embed/` before scoping.
- ~~Phase 9 Plan 03 Task 3 checkpoint~~ RESOLVED: **accept-d06**. Live eval (gemini-embedding-2 @3072 via OpenRouter) clears the #261 rank bar — query-a/b both rank 1/8, recall@8=1.00, MRR=1.000. D-07/D-08 evaluated and not needed. Follow-up: run the qwen3-embedding-8b @4096 prod-parity eval (with PR#262 query instruction) once OpenRouter recovers.

## Deferred Items

Carried to v0.10.x. NOT part of v0.9.x scope.

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| Auth | Connect **write-lane** RPCs (StoreMemory/StoreDiscovery) + CSRF hardening (#322) | Not started | 2026-07-09 |
| Auth | Session **refresh-token rotation** / re-seal on access-token expiry (#323) | Not started | 2026-07-09 |
| Cleanup | short_id polish (cluster C: 10 issues), embed refactors (#302/#304), summarize CronJob (#269), proto fidelity (#307), PR #62 cleanups (#306) | Not started | 2026-07-09 |

## Session Continuity

Last session: 2026-07-10T00:38:30.316Z
Stopped at: Phase 9 Plan 03 complete (accept-d06): D-06 reranker shipped on MCP+Connect; live eval clears #261 rank bar (recall@8=1.00, MRR=1.000). Phase 9 all 3 plans done — ready for verification.
Resume file: None
