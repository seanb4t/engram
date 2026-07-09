---
gsd_state_version: 1.0
milestone: v0.9.x — Recall Quality
milestone_name: "- [ ] **Phase 9: Retrieval Eval Harness & Ranking Precision** - Labeled retrieval eval"
current_phase: 9
current_phase_name: not yet planned
status: planning
stopped_at: Phase 9 context gathered
last_updated: "2026-07-09T17:43:01.996Z"
last_activity: 2026-07-09
last_activity_desc: "opened milestone v0.9.x via `/gsd-new-milestone`: routed 6 requirements to Phases 9–12 (REQUIREMENTS.md), added phase details (ROADMAP.md), updated PROJECT.md Active section. Theme = Recall Quality, focused scope (consolidation + write-lane deferred to v0.10.x)."
progress:
  total_phases: 12
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-09)

**Core value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.
**Current focus:** v0.9.x — Recall Quality (opened 2026-07-09). Four phases (9–12): retrieval eval + ranking precision (#261), embedder query/document asymmetry (#305), async-on-write summaries (#320), per-memory usage signals (#317). No phase planned yet — next step `/gsd-plan-phase 9`.

## Current Position

Phase: 9 of 12 — **Retrieval Eval Harness & Ranking Precision** (not yet planned)
Plan: None. Next: `/gsd-plan-phase 9`.
Status: Milestone v0.9.x opened; requirements + roadmap written. v0.8.x baseline (Phases 1–8) complete and shipped.
Last activity: 2026-07-09 — opened milestone v0.9.x via `/gsd-new-milestone`: routed 6 requirements to Phases 9–12 (REQUIREMENTS.md), added phase details (ROADMAP.md), updated PROJECT.md Active section. Theme = Recall Quality, focused scope (consolidation + write-lane deferred to v0.10.x).

Progress: [░░░░░░░░░░] 0% (v0.9.x — 0/4 phases planned)

## Performance Metrics

**Velocity:**

- Total plans completed: 0 (v0.9.x just opened)
- Average duration: n/a
- Total execution time: n/a

**v0.9.x Phases:**

| Phase | Requirements | Status |
|-------|--------------|--------|
| 9. Retrieval Eval & Ranking Precision | 0/3 | Planned |
| 10. Asymmetric Query/Document Embeddings | 0/1 | Planned |
| 11. Async-on-Write Summaries | 0/1 | Planned |
| 12. Per-Memory Usage Signals | 0/1 | Planned |

**Shipped (v0.8.x baseline):** Phases 1–8 complete (24 requirements). See ROADMAP.md Progress table.

## Accumulated Context

### Decisions

56 ADR-locked decisions (25 core + 31 companion refinements) logged in PROJECT.md `<decisions>` block (all LOCKED, source `docs/adr/engram-*.md`). Foundational for v0.9.x:

- [Phase 2]: Summary-by-default recall (DEC-ambu) — Phase 11 async summaries build on the `FillSummary` seam.
- [Phase 4]: Asymmetric embedder text-prefix knobs (DEC-zyhq) — Phase 10 extends these with native API params + doc-side prefix.
- [Phase 6]: OTLP-only non-blocking telemetry (DEC-dwi, DEC-uxh) — Phases 9 (eval) and 12 (usage signals) reuse the OTLP → ClickStack seam.

### Pending Todos

- `/gsd-plan-phase 9` — plan the retrieval eval harness + ranking precision phase.

### Blockers/Concerns

- **Phase 10 baseline ambiguity**: `REQ-asymmetric-embedder-params` (Phase 4, shipped) describes query/doc param maps + doc-side instruction, but GitHub #305 reports those classes (cloud native API params; E5/nomic doc-side prefix) are NOT covered by the shipped text-prefix knob. Phase-10 planning must verify the exact shipped baseline against `internal/embed/` before scoping.

## Deferred Items

Carried to v0.10.x. NOT part of v0.9.x scope.

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| Auth | Connect **write-lane** RPCs (StoreMemory/StoreDiscovery) + CSRF hardening (#322) | Not started | 2026-07-09 |
| Auth | Session **refresh-token rotation** / re-seal on access-token expiry (#323) | Not started | 2026-07-09 |
| Cleanup | short_id polish (cluster C: 10 issues), embed refactors (#302/#304), summarize CronJob (#269), proto fidelity (#307), PR #62 cleanups (#306) | Not started | 2026-07-09 |

## Session Continuity

Last session: 2026-07-09T17:43:01.990Z
Stopped at: Phase 9 context gathered
Resume file: .planning/phases/09-retrieval-eval-harness-ranking-precision/09-CONTEXT.md
