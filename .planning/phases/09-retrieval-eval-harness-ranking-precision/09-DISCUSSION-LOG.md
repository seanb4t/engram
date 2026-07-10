# Phase 9: Retrieval Eval Harness & Ranking Precision - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-09
**Phase:** 9-retrieval-eval-harness-ranking-precision
**Areas discussed:** Score exposure (always-on vs opt-in), Ranking-fix appetite & guardrails

---

## Gray Area Selection (multi-select)

| Option | Description | Selected |
|--------|-------------|----------|
| Eval harness form & dataset home | Env-gated Go test vs runner; dataset location; live vs hermetic | |
| Score exposure: always-on vs opt-in | Score already ships always-on but undocumented | ✓ |
| Ranking-fix appetite & guardrails | Migration / model-dependency appetite; eval-driven | ✓ |
| CI regression gating | Required gate vs non-required job vs local-only | |

**User's choice:** Score exposure + Ranking-fix appetite.
**Notes:** Eval-harness form and CI gating deferred to researcher/planner discretion (with leanings recorded in CONTEXT.md).

---

## Score Exposure

| Option | Description | Selected |
|--------|-------------|----------|
| Keep always-on + document | Accept shipped always-on; document in schema + memory-contract docs; eval asserts score separation | ✓ |
| Add opt-in flag (default off) | Retrofit include_score flag; matches ROADMAP wording but regresses current behavior | |
| Always-on, but gate on full=true | Score only on full fetches; compact view score-free | |

**User's choice:** Keep always-on + document.
**Notes:** Baseline verified against code — score is already plumbed store→recallView→Connect, always-on via `omitempty`, undocumented in the tool schema. Making it opt-in would be a behavior regression. This supersedes the ROADMAP success-criterion "opt-in" wording (recorded for #261 traceability).

---

## Ranking-Fix Appetite — Hybrid / Migration

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, migration is in scope | Phase 9 may add Qdrant sparse-vector schema + reindex/backfill if the eval favors hybrid | ✓ |
| Eval-decides, keep light first | Try light levers; scope any hybrid migration as its own follow-up phase | |
| Light only, hybrid always deferred | Cap Phase 9 at eval + score docs + light levers; hybrid out of scope regardless | |

**User's choice:** Yes, migration is in scope.
**Notes:** Qdrant is currently dense-only; hybrid needs a sparse vector on the collection + reindex. `engram reindex` is precedent. Still gated on eval evidence — nothing added speculatively.

---

## Ranking-Fix Appetite — Rerank Dependency

| Option | Description | Selected |
|--------|-------------|----------|
| In-process heuristic only | Lexical overlap / MMR / score-gap; no new dependency; cross-encoder deferred | |
| Cross-encoder reranker allowed | Phase 9 may add a cross-encoder model (extra gateway call, opt-in ENGRAM_ config) if heuristics + hybrid still miss | ✓ |
| Eval-decides, no guardrail | Leave both open with no upfront cap | |

**User's choice:** Cross-encoder reranker allowed.
**Notes:** Broad appetite — light levers first, then hybrid and/or cross-encoder rerank as the eval numbers demand. Reranker stays opt-in / off the default hot path unless the eval justifies default-on.

---

## Wrap-Up

| Option | Description | Selected |
|--------|-------------|----------|
| Ready for context | Write CONTEXT.md now; eval-harness form + CI gating flow through as Claude's Discretion | ✓ |
| Explore more gray areas | Surface additional areas (dataset labeling, regression thresholds, #261 fixture) first | |

**User's choice:** Ready for context.

---

## Claude's Discretion

- **Eval harness form & dataset home** — lean: env-gated Go-test pattern (mirror `eval:summary`), #261 miss as a permanent regression fixture, `recall@k`/MRR.
- **CI regression gating** — constraint: protect-main's 8 exact-named checks; a required eval must be hermetic or become a non-required/local target; never add a skipped required workflow.

## Deferred Ideas

None — discussion stayed within phase scope. Adjacent recall-quality work is already routed to Phases 10–12 (#305 / #320 / #317).
