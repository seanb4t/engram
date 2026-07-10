# Phase 12: Per-Memory Usage Signals - Context

**Gathered:** 2026-07-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Track **strong per-record usage** as operational curation metadata — a payload
`access_count` maintained on `get_memory` fetch-by-id and `update_memory`, plus
recall-result ids emitted on OTLP spans for ClickStack analytics. Usage signals
are **server-set** and exist to inform curation (surface hot/stale records); they
**never silently affect what recall returns**. This is a design-first metadata
phase — it adds a counter and its plumbing, not a ranking behavior.

**In scope:** payload `access_count` (+ `last_accessed_at`) on get-by-id/update;
best-effort maintenance off the read path; recall ids on OTLP spans; read-only
exposure of the counter on existing MCP/Connect read outputs; a config gate.

**Out of scope (hard):** usage-weighted recall / ranking by `access_count` (its
own future deliberate decision); incrementing on search/list result-set
membership; a bespoke curation tool; a `/metrics` scrape endpoint.
</domain>

<decisions>
## Implementation Decisions

### Counting boundary — what increments the counter
- **D-01:** Increment **only at the tool-handler boundary** — the MCP
  `getMemory` handler (`tools.go` ~908, after a *successful* `GetReadable`) and
  the MCP `updateMemory` handler (~843), plus the Connect `GetMemory` handler
  (`connectapi.go` ~174). **Never** inside `store.Get` / `GetReadable` /
  `getWritable` — those are reused internally (ownership gates,
  `FetchForUpdate`, `setVisibility`'s category read) and counting there would
  inflate the signal with non-user fetches. A denied/`ErrNotFound` get does
  **not** increment. `update_memory`'s internal `FetchForUpdate` is not a
  counted "get" — the update is a single counted event.
- **D-02:** **Never** increment on `search_memory` / `list_memory` /
  `list_scheduled` result-set membership (REQ hard invariant — noisy,
  write-amplifying, racy). Result-set ids ride OTLP spans instead (D-06).

### Payload fields & storage (hybrid)
- **D-03:** Add a single payload field **`access_count`** (uint64, monotonic
  total of strong-signal touches = get-by-id + update) plus **`last_accessed_at`**
  (RFC3339, recency for stale-vs-hot curation). New `Memory` struct fields wired
  into `payload()` / `fromPayload()`. Legacy records: missing key reads as `0` /
  zero time — **no backfill command needed**. The get-vs-update *distinction*
  lives in the OTLP span operation attribute (D-06), **not** in extra payload
  fields — the hybrid split is: payload = coarse MCP-visible curation number;
  spans → ClickStack = fine-grained analytics.

### Increment path — synchronous vs async
- **D-04:** **Update path is synchronous and free** — `update_memory` already
  performs a full `Upsert` (re-embed + full payload write), so bump
  `access_count` / `last_accessed_at` on `cur` *before* `Upsert` (zero extra
  write; no new race beyond the update's own `FetchForUpdate`→`Upsert` RMW).
  **Get path is best-effort async** — a fire-and-forget side-write off the
  synchronous read path, so `get_memory` latency and success are **never**
  coupled to the counter write (the read succeeds even if the bump is dropped or
  the write fails).
- **D-05:** **RMW race → accept lost updates.** Qdrant has no atomic increment,
  so the counter write is read-value-then-`SetPayload(value+1)`, last-writer-wins.
  Concurrent bumps on the same record may lose an increment — **acceptable**: the
  payload counter is a soft curation signal, and precise counts live in
  OTLP→ClickStack (D-06). **No** per-record mutex / singleflight / optimistic-retry
  (over-engineering for a soft signal; wouldn't help across replicas anyway).

### OTLP span analytics (zero storage change)
- **D-06:** Attach returned record ids + count as attributes on the **existing
  recall spans** (`search_memory`, `list_memory`, `get_memory`): a bounded
  `engram.recall.ids` string slice (capped to avoid unbounded attribute
  cardinality) + `engram.recall.count`. Rides existing OTLP (no-op when OTLP is
  unconfigured). This is the analytics half of the hybrid design and the **only**
  place search/list result-set membership is recorded — it never touches the
  payload counter.

### MCP-visible curation surface
- **D-07:** Expose `access_count` + `last_accessed_at` as **read-only fields** on
  existing outputs — `get_memory` (full view), `list_memory` items, and the
  Connect `GetMemory` / `ListMemories` responses (console). **No new dedicated
  curation MCP tool this phase** — keep the surface minimal; a "least-used /
  stale" query is a future tool or a ClickStack dashboard. Reading and displaying
  the stored value in a search/list result is **not** "membership counting" — the
  D-02 invariant is only about *incrementing*, never about *reading*.

### Ranking isolation (hard invariant)
- **D-08:** Usage counters **MUST NOT** affect ranking or the recall gate in any
  way — not a rerank signal, tiebreaker, filter, or gate input.
  `store.SearchReranked` / `rerank.go` must not read `access_count`.
  Usage-weighted recall is explicitly out of scope (REQUIREMENTS Out-of-Scope;
  Phase 9 constraint, `09-CONTEXT.md`). Add a **negative-space test** asserting
  the reranker's output is invariant under `access_count`.

### Config gating
- **D-09:** Gate the payload `access_count` write behind a koanf field
  **`usage.signals` / `ENGRAM_USAGE_SIGNALS`** (string bool, parsed at
  point-of-use, mirroring `ui.enabled` / `summarize.on_write`; registered in
  `registry.go`, parseability-checked in `validate.go`). **Default `true` (on)** —
  unlike `summarize.on_write` (which egresses content to an external model and
  defaults off), usage signals are local, non-egressing curation metadata, so
  on-by-default is the right product default. The OTLP-span recall-id analytics
  (D-06) always ride existing telemetry (already gated by OTLP config),
  independent of this flag. Operators wanting zero read-path writes set it false.

### Async engine
- **D-10:** The get-path async incrementer is **lightweight best-effort**:
  bounded, non-blocking, **drop-on-full → OTLP counter**, **no retry** (a lost
  bump is fine; and because there is **no content egress**, none of Phase 11's
  privacy/repudiation audit — `LogSummaryEgress`, egress stamps — applies here).
  It **MUST reuse the Phase 11 shutdown-safety kernel** if it uses a channel/pool:
  the RWMutex `closed` guard on the channel + `inFlight` WaitGroup
  reserve-before-send (CR-01) to prevent send-on-closed-channel on shutdown, and a
  best-effort drain in the existing 15s shutdown window (drop remainder). Whether
  it literally reuses `summaryqueue.go` or is a smaller new primitive is planner
  discretion.

### Claude's Discretion
- Exact async primitive shape (reuse `summaryqueue.go` worker-pool vs. a smaller
  bounded goroutine + semaphore) — either is fine per D-10's constraints.
- OTLP metric/instrument names and the `engram.recall.ids` slice cap value.
- Whether `last_accessed_at` is stamped on both get and update (recommended) or
  update-only.
- Store-method surface for the get-path write (e.g. a new
  `IncrementAccess(ctx, id)` mirroring `SetVisibility`'s `SetPayload` pattern).
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase requirement & design intent
- `.planning/ROADMAP.md` — Phase 12 goal + 3 Success Criteria (counting sites,
  hybrid storage, ranking-isolation invariant).
- `.planning/REQUIREMENTS.md` — `REQ-usage-signals` (line ~104) + **Out of Scope**
  row "Usage-weighted recall (ranking by `access_count`)" (line ~119) — the hard
  ranking boundary.
- GitHub issue **#317** ("Design: per-memory usage signals") — the settled design
  questions (strong-signals-only; hybrid storage; never-affect-ranking; Qdrant has
  no atomic increment → RMW).
- `docs/superpowers/specs/2026-07-06-rule-memory-kind-design.md` — origin
  brainstorm (`engram-3jo0`); progressive-disclosure rules make
  `get_memory(short_id)` a genuine "rule was consulted" event.

### Ranking-isolation constraint (Phase 9)
- `.planning/phases/09-retrieval-eval-harness-ranking-precision/09-CONTEXT.md`
  §ranking boundary (lines ~34–35) — "Usage signals must **never** affect
  ranking (Phase 12 constraint)."

### Async / shutdown-safety precedent (Phase 11)
- `.planning/phases/11-async-on-write-summaries/11-CONTEXT.md` — the
  bounded-queue / best-effort / drop-and-count design this phase mirrors (minus
  the egress audit).
- `internal/server/summaryqueue.go` — the worker-pool + RWMutex `closed`-guard +
  `inFlight` WaitGroup kernel (CR-01) to reuse for shutdown safety.
- `internal/telemetry/metrics.go` — `SummaryQueueMetrics` (lines ~118–158) is the
  OTLP counter template for drop/enqueue instruments.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/server/summaryqueue.go`** — bounded non-blocking worker pool with
  the shutdown-safe `close` guard; the get-path incrementer reuses this kernel
  (or a smaller variant of it) per D-10.
- **`store.SetVisibility` (`store.go` ~1334)** — the canonical **partial payload
  write** pattern: `SetPayload` by point-ID selector, preserves the vector, no
  re-embed. This is exactly the primitive the get-path counter write needs
  (`SetPayload{"access_count": n+1, "last_accessed_at": ...}`).
- **`internal/telemetry/metrics.go` `SummaryQueueMetrics`** — template for the
  drop/enqueue OTLP counters.
- **`internal/server/instrument.go` `instrumentTools` (~21)** — the tool-span
  middleware seam where `engram.recall.ids` / `engram.recall.count` attributes
  attach.

### Established Patterns
- **Payload round-trip** — every persisted field is added in `payload()`
  (`store.go` ~271) *and* `fromPayload()` (~322); `access_count` +
  `last_accessed_at` join the `Memory` struct (~86) and both funcs.
- **koanf field registry** — one row in `registry.go` (`{Key: "usage.signals",
  Env: "ENGRAM_USAGE_SIGNALS", Default: "true"}`), parseability check in
  `validate.go`, parsed at point-of-use (`ui.enabled` analog) — never a native
  bool at load.
- **Store method shape** — `tracer.Start` span + `telemetry.RecordStoreOp` +
  error-status defer wrap every store method; a new `IncrementAccess` follows it.
- **Handler-boundary wiring** — counting lives in `deps` handlers in `tools.go`,
  not in reused store primitives (D-01).

### Integration Points
- `internal/store/store.go` — `Memory` struct (~86), `payload`/`fromPayload`
  (~271/~322), new `IncrementAccess(ctx, id)` using `SetPayload`; `Update`/`Upsert`
  (~1289) gets the free update-path bump.
- `internal/server/tools.go` — `getMemory` (~908) and `updateMemory` (~843)
  handlers fire the increment (get → async; update → piggyback).
- `internal/server/connectapi.go` — `GetMemory` (~174) increments; recall spans on
  `SearchMemories` (~144) / `ListMemories` (~95) carry ids (D-06).
- `internal/server/instrument.go` — recall-id span attributes.
- `internal/config/registry.go` + `validate.go` — `ENGRAM_USAGE_SIGNALS`.
- `serve.go` — worker lifecycle + shutdown drain if a queue is used (mirror the
  Phase 11 `Register` → shutdown-func wiring).

</code_context>

<specifics>
## Specific Ideas

- The MCP-visible number is deliberately **coarse** (a single `access_count`); any
  fine-grained "which ids appeared in which result sets, when, for whom" question
  is answered by **ClickStack over the OTLP recall spans**, not by more payload
  fields. This is the whole point of the hybrid split.
- `get_memory(short_id)` on a **rule** is a genuine "rule was consulted" curation
  event (progressive-disclosure recall) — a motivating use case for counting
  get-by-id specifically.

</specifics>

<deferred>
## Deferred Ideas

- **Usage-weighted recall / ranking by `access_count`** — explicit future
  deliberate decision; hard out-of-scope here (REQUIREMENTS Out-of-Scope; D-08).
- **Dedicated curation MCP tool** (e.g. `list_by_usage`, stale-record report) —
  future; this phase exposes the value on existing read outputs only (D-07).
- **Separate `get_count` vs `update_count` payload fields** — kept in OTLP span
  attributes for now; promote to payload only if a concrete curation need appears.
- **Backfill command for legacy `access_count`** — unnecessary; missing key reads
  as 0.

### Reviewed Todos (not folded)
None — no pending todos matched Phase 12 scope.

</deferred>

---

*Phase: 12-per-memory-usage-signals*
*Context gathered: 2026-07-10*
