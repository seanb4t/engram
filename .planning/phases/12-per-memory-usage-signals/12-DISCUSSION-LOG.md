# Phase 12: Per-Memory Usage Signals - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-10
**Phase:** 12-per-memory-usage-signals
**Mode:** `--auto` (autonomous single-pass — recommended option auto-selected per gray area)
**Areas discussed:** Counting boundary, Payload fields & storage, Increment path (sync/async), RMW race handling, OTLP span analytics, MCP curation surface, Ranking isolation, Config gating, Async engine

---

## Counting boundary — what increments the counter

| Option | Description | Selected |
|--------|-------------|----------|
| Tool-handler boundary only | Increment in `getMemory`/`updateMemory` handlers + Connect `GetMemory`; never inside reused `store.Get`/`GetReadable` | ✓ |
| Inside `store.Get`/`GetReadable` | Simpler single site, but inflated by internal ownership-gate fetches | |

**Auto-selected:** Tool-handler boundary only (D-01/D-02).
**Notes:** `store.Get`/`GetReadable` are reused by ownership gates, `FetchForUpdate`, and `setVisibility`'s category read — counting there would over-count. Denied/not-found gets do not increment. Never count search/list membership (REQ invariant).

---

## Payload fields & storage

| Option | Description | Selected |
|--------|-------------|----------|
| Single `access_count` + `last_accessed_at` | One coarse counter + recency; get/update split lives in OTLP spans | ✓ |
| Separate `get_count` + `update_count` | Distinguish reads from edits in the payload | |

**Auto-selected:** Single `access_count` + `last_accessed_at` (D-03).
**Notes:** Hybrid-storage split — payload = coarse MCP curation number; OTLP→ClickStack = fine analytics. Legacy records read missing key as 0 (no backfill).

---

## Increment path — synchronous vs async

| Option | Description | Selected |
|--------|-------------|----------|
| Update piggybacks on Upsert; get is async | Update bump is free (already writing payload); get uses best-effort off-path side-write | ✓ |
| Both synchronous RMW | Adds a write + latency to every read; couples read success to a write | |

**Auto-selected:** Update piggyback + async get (D-04/D-05).
**Notes:** `update_memory` already does a full `Upsert`, so the bump costs nothing. Only `get_memory` (a pure read today) needs a side-write, made best-effort so read latency/success is never coupled. RMW race → accept lost updates, last-writer-wins; precise counts live in ClickStack.

---

## OTLP span analytics

| Option | Description | Selected |
|--------|-------------|----------|
| Capped result-id attributes on recall spans | `engram.recall.ids` (bounded) + `engram.recall.count` on search/list/get spans | ✓ |
| One span per returned id | Precise but unbounded span cardinality | |

**Auto-selected:** Capped result-id attributes on existing recall spans (D-06).
**Notes:** Zero storage change; rides existing OTLP (no-op when unconfigured). The only place search/list membership is recorded — never touches the payload counter.

---

## MCP curation surface

| Option | Description | Selected |
|--------|-------------|----------|
| Read-only fields on existing outputs | Expose `access_count`/`last_accessed_at` on get_memory/list_memory + Connect; no new tool | ✓ |
| New dedicated curation tool | e.g. `list_by_usage` / stale-report | |

**Auto-selected:** Read-only fields on existing outputs (D-07).
**Notes:** Minimal surface. Displaying the stored value in search/list results is not "membership counting" — the invariant is only about incrementing.

---

## Ranking isolation

| Option | Description | Selected |
|--------|-------------|----------|
| Hard invariant: usage never touches ranking | Reranker must not read `access_count`; negative-space test | ✓ |
| Allow soft tiebreaker | — rejected: out of scope, violates Phase 9 constraint | |

**Auto-selected:** Hard invariant (D-08) — locked by REQ Out-of-Scope + Phase 9 constraint.

---

## Config gating

| Option | Description | Selected |
|--------|-------------|----------|
| `ENGRAM_USAGE_SIGNALS` koanf bool, default ON | Gate payload write; OTLP span piece always rides telemetry | ✓ |
| Always-on, no flag | No operator escape hatch for read-path writes | |
| Opt-in default OFF (mirror summary-on-write) | Too conservative — nobody gets curation metadata by default | |

**Auto-selected:** `ENGRAM_USAGE_SIGNALS` default ON (D-09).
**Notes:** Unlike `summarize.on_write` (egresses content → defaults off), usage signals are local/non-egressing → on-by-default is the right product default. **Default flagged for user review.**

---

## Async engine

| Option | Description | Selected |
|--------|-------------|----------|
| Lightweight best-effort, reuse Phase 11 shutdown kernel | Bounded, drop-on-full → OTLP counter, no retry; reuse RWMutex closed-guard | ✓ |
| Full summaryqueue machinery with backoff retry | Overkill for a soft, non-egressing counter | |

**Auto-selected:** Lightweight best-effort reusing the Phase 11 shutdown-safety kernel (D-10).
**Notes:** No content egress → no privacy/repudiation audit. Reuse-vs-new primitive is planner discretion, but the CR-01 send-on-closed-channel guard is mandatory.

---

## Claude's Discretion

- Exact async primitive (reuse `summaryqueue.go` vs. smaller bounded goroutine+semaphore).
- OTLP instrument names; `engram.recall.ids` slice cap value.
- `last_accessed_at` on both get+update (recommended) vs. update-only.
- Store-method surface for the get-path write (`IncrementAccess` mirroring `SetVisibility`).

## Deferred Ideas

- Usage-weighted recall / ranking by `access_count` — explicit future decision (out of scope).
- Dedicated curation MCP tool (`list_by_usage`, stale-report).
- Separate `get_count`/`update_count` payload fields (kept in spans for now).
- Backfill command for legacy `access_count` (unnecessary — missing reads as 0).
