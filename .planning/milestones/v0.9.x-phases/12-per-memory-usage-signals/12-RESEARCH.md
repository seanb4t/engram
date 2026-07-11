# Phase 12: Per-Memory Usage Signals - Research

**Researched:** 2026-07-10
**Domain:** Go + Qdrant payload mutation, OTel span attributes, bounded async worker pool, connect/protobuf codegen
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Increment **only at the tool-handler boundary** — the MCP `getMemory` handler
  (`tools.go` ~908, after a *successful* `GetReadable`) and the MCP `updateMemory` handler
  (~843), plus the Connect `GetMemory` handler (`connectapi.go` ~174). **Never** inside
  `store.Get` / `GetReadable` / `getWritable` — those are reused internally (ownership gates,
  `FetchForUpdate`, `setVisibility`'s category read) and counting there would inflate the
  signal with non-user fetches. A denied/`ErrNotFound` get does **not** increment.
  `update_memory`'s internal `FetchForUpdate` is not a counted "get" — the update is a single
  counted event.
- **D-02:** **Never** increment on `search_memory` / `list_memory` / `list_scheduled`
  result-set membership (REQ hard invariant — noisy, write-amplifying, racy). Result-set ids
  ride OTLP spans instead (D-06).
- **D-03:** Add a single payload field **`access_count`** (uint64, monotonic total of
  strong-signal touches = get-by-id + update) plus **`last_accessed_at`** (RFC3339, recency
  for stale-vs-hot curation). New `Memory` struct fields wired into `payload()` /
  `fromPayload()`. Legacy records: missing key reads as `0` / zero time — **no backfill
  command needed**. The get-vs-update *distinction* lives in the OTLP span operation
  attribute (D-06), **not** in extra payload fields — the hybrid split is: payload = coarse
  MCP-visible curation number; spans → ClickStack = fine-grained analytics.
- **D-04:** **Update path is synchronous and free** — `update_memory` already performs a full
  `Upsert` (re-embed + full payload write), so bump `access_count` / `last_accessed_at` on
  `cur` *before* `Upsert` (zero extra write; no new race beyond the update's own
  `FetchForUpdate`→`Upsert` RMW). **Get path is best-effort async** — a fire-and-forget
  side-write off the synchronous read path, so `get_memory` latency and success are **never**
  coupled to the counter write (the read succeeds even if the bump is dropped or the write
  fails).
- **D-05:** **RMW race → accept lost updates.** Qdrant has no atomic increment, so the counter
  write is read-value-then-`SetPayload(value+1)`, last-writer-wins. Concurrent bumps on the
  same record may lose an increment — **acceptable**: the payload counter is a soft curation
  signal, and precise counts live in OTLP→ClickStack (D-06). **No** per-record mutex /
  singleflight / optimistic-retry (over-engineering for a soft signal; wouldn't help across
  replicas anyway).
- **D-06:** Attach returned record ids + count as attributes on the **existing recall spans**
  (`search_memory`, `list_memory`, `get_memory`): a bounded `engram.recall.ids` string slice
  (capped to avoid unbounded attribute cardinality) + `engram.recall.count`. Rides existing
  OTLP (no-op when OTLP is unconfigured). This is the analytics half of the hybrid design and
  the **only** place search/list result-set membership is recorded — it never touches the
  payload counter.
- **D-07:** Expose `access_count` + `last_accessed_at` as **read-only fields** on existing
  outputs — `get_memory` (full view), `list_memory` items, and the Connect `GetMemory` /
  `ListMemories` responses (console). **No new dedicated curation MCP tool this phase** — keep
  the surface minimal; a "least-used / stale" query is a future tool or a ClickStack
  dashboard. Reading and displaying the stored value in a search/list result is **not**
  "membership counting" — the D-02 invariant is only about *incrementing*, never about
  *reading*.
- **D-08:** Usage counters **MUST NOT** affect ranking or the recall gate in any way — not a
  rerank signal, tiebreaker, filter, or gate input. `store.SearchReranked` / `rerank.go` must
  not read `access_count`. Usage-weighted recall is explicitly out of scope (REQUIREMENTS
  Out-of-Scope; Phase 9 constraint, `09-CONTEXT.md`). Add a **negative-space test** asserting
  the reranker's output is invariant under `access_count`.
- **D-09:** Gate the payload `access_count` write behind a koanf field **`usage.signals` /
  `ENGRAM_USAGE_SIGNALS`** (string bool, parsed at point-of-use, mirroring `ui.enabled` /
  `summarize.on_write`; registered in `registry.go`, parseability-checked in `validate.go`).
  **Default `true` (on)** — unlike `summarize.on_write` (which egresses content to an
  external model and defaults off), usage signals are local, non-egressing curation metadata,
  so on-by-default is the right product default. The OTLP-span recall-id analytics (D-06)
  always ride existing telemetry (already gated by OTLP config), independent of this flag.
  Operators wanting zero read-path writes set it false.
- **D-10:** The get-path async incrementer is **lightweight best-effort**: bounded,
  non-blocking, **drop-on-full → OTLP counter**, **no retry** (a lost bump is fine; and
  because there is **no content egress**, none of Phase 11's privacy/repudiation audit —
  `LogSummaryEgress`, egress stamps — applies here). It **MUST reuse the Phase 11
  shutdown-safety kernel** if it uses a channel/pool: the RWMutex `closed` guard on the
  channel + `inFlight` WaitGroup reserve-before-send (CR-01) to prevent
  send-on-closed-channel on shutdown, and a best-effort drain in the existing 15s shutdown
  window (drop remainder). Whether it literally reuses `summaryqueue.go` or is a smaller new
  primitive is planner discretion.

### Claude's Discretion

- Exact async primitive shape (reuse `summaryqueue.go` worker-pool vs. a smaller bounded
  goroutine + semaphore) — either is fine per D-10's constraints.
- OTLP metric/instrument names and the `engram.recall.ids` slice cap value.
- Whether `last_accessed_at` is stamped on both get and update (recommended) or update-only.
- Store-method surface for the get-path write (e.g. a new `IncrementAccess(ctx, id)`
  mirroring `SetVisibility`'s `SetPayload` pattern).

### Deferred Ideas (OUT OF SCOPE)

- **Usage-weighted recall / ranking by `access_count`** — explicit future deliberate decision;
  hard out-of-scope here (REQUIREMENTS Out-of-Scope; D-08).
- **Dedicated curation MCP tool** (e.g. `list_by_usage`, stale-record report) — future; this
  phase exposes the value on existing read outputs only (D-07).
- **Separate `get_count` vs `update_count` payload fields** — kept in OTLP span attributes for
  now; promote to payload only if a concrete curation need appears.
- **Backfill command for legacy `access_count`** — unnecessary; missing key reads as 0.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-usage-signals | Strong per-record usage signals to inform curation — increment counters only on `get_memory` fetch-by-id and `update_memory` (never search/list result-set membership); hybrid storage (recall ids on OTLP spans → ClickStack for analytics; a payload `access_count` on get/update for MCP-visible curation tools); server-set operational metadata that never silently affects ranking. | Payload round-trip plan (§Payload), handler-boundary wiring (§Counting Boundary), OTLP span plan (§Recall-Span Analytics), ranking-isolation test plan (§Don't Hand-Roll / Validation Architecture), config gate (§Config). |
</phase_requirements>

## Summary

This is a design-first phase: CONTEXT.md's D-01..D-10 already lock the architecture. The
implementation is a small, well-scoped set of changes across four files plus a proto/codegen
ripple, all following patterns already established in this codebase (Phase 11's async
worker-pool kernel, `SetVisibility`'s partial-payload write, the existing OTel span
attribute idiom). The two riskiest facts a planner must know going in:

1. **`recallView` (the compact `list_memory`/`search_memory` shape) is a hand-written
   whitelist struct, not the raw `Memory`** — `internal/server/summary.go:40` (`recallView`)
   and `:89` (`toRecallView`). Adding fields to `store.Memory` alone does **not** surface them
   on the default (non-`full`) recall shape; D-07's "`list_memory` items" requirement needs an
   explicit edit to `recallView` + `toRecallView` in addition to the `Memory` struct edit.
   `get_memory` is unaffected by this — it always returns the full `store.Memory` directly
   (`tools.go:1036-1037`), so it gets the new fields for free once `Memory`/`payload`/
   `fromPayload` are updated.
2. **Exposing `access_count`/`last_accessed_at` on the Connect API requires a proto field
   addition + `task proto:gen` regen** — `proto/engram/v1/engram.proto`'s `Memory` message
   currently uses field numbers 1–18; two new fields (`19`, `20`) are additive and NOT a
   breaking change under `buf breaking`, but CI's `buf` job regenerates `gen/` and fails the
   build (`git diff --exit-code -- gen/`) if the committed `gen/` tree is stale. This is
   mechanical but easy to forget as a task.

Everything else — the payload round-trip, the `SetVisibility`-style partial write, the
free update-path bump, the async get-path incrementer, the OTel span attributes, and the
config gate — has a direct, already-shipped precedent in this codebase to copy.

**Primary recommendation:** Follow the existing patterns literally: extend `Memory`/
`payload`/`fromPayload` (D-03), add `IncrementAccess` mirroring `SetVisibility`'s
`SetPayload` primitive, bump `cur.AccessCount`/`cur.LastAccessedAt` before `Upsert` in the
`updateMemory` handler (free, D-04), fire a best-effort async increment from `getMemory`
(reusing the `summaryQueue` shutdown-safety kernel or a smaller variant per D-10), attach
`engram.recall.ids`/`engram.recall.count` in the existing `store.Search`/`List`/`Get` span
defers (D-06), add the proto fields + regen (D-07), gate behind `ENGRAM_USAGE_SIGNALS`
(D-09, default true), and add a negative-space `RerankHits` test (D-08).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `access_count`/`last_accessed_at` persistence | Database / Storage (Qdrant payload) | API/Backend (Go store package) | The counter is stored data; the store package owns the read-modify-write primitive (`IncrementAccess`), mirroring `SetVisibility`. |
| Counting decision (when to increment) | API / Backend (MCP + Connect handlers) | — | D-01 is explicit: counting lives in `deps` handler methods (`tools.go`, `connectapi.go`), never inside reused store gates (`Get`/`GetReadable`/`getWritable`) — those are called by internal callers that must not inflate the signal. |
| Async get-path increment execution | API / Backend (in-process worker pool) | — | A bounded goroutine pool inside the same process as the MCP/Connect handlers; no external queue/service. |
| Recall-id analytics (OTLP spans) | API / Backend (store-layer span emission) | CDN/External (ClickStack, out of this repo) | Spans are emitted at the store layer (`store.Search`/`List`/`Get`); ClickStack ingestion/query is an external, already-provisioned (Phase 6) OTLP consumer — no new code in this repo talks to ClickStack directly. |
| MCP-visible curation surface (`access_count` in tool output) | API / Backend (MCP tool handlers, JSON serialization) | — | `get_memory` gets it for free via the `Memory` struct's JSON tags; `list_memory`/`search_memory` need an explicit `recallView` field addition (see Summary finding 1). |
| Connect-visible curation surface | API / Backend (Connect handlers) → wire | Frontend Server (operator console reads it) | Requires a proto field bump + codegen; the SvelteKit console (out of this phase's scope per D-07 "no new tool") can read it once the field exists on the wire, but no console UI change is required by this phase. |
| Config gate (`ENGRAM_USAGE_SIGNALS`) | API / Backend (koanf config loader) | — | Parsed at point-of-use in `buildDepsFromEnv`/handler construction, mirroring `ui.enabled`/`summarize.on_write` — never a native bool at `Load()` time. |
| Ranking isolation (must NOT read `access_count`) | API / Backend (`store.SearchReranked`/`rerank.go`) | — | `rerank.go`'s `RerankHits` is a pure function over `Memory`; the invariant is enforced by never adding an `AccessCount` read to its tie-break chain, verified by a negative-space unit test. |

## Standard Stack

This phase adds no new external dependencies — it is entirely additive Go code reusing
already-vendored libraries.

### Core (already in go.mod — no new dependency)
| Library | Version (verified in go.mod/go.sum) | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/qdrant/go-client` | v1.18.3 `[VERIFIED: go.mod]` | Qdrant gRPC client — `SetPayload`, `NewValueMap`, `GetIntegerValue` | Already the store's sole Qdrant driver; no alternative considered. |
| `go.opentelemetry.io/otel` / `otel/attribute` | already vendored `[VERIFIED: go doc against vendored module]` | Span attributes for `engram.recall.ids`/`engram.recall.count` | `attribute.StringSlice(k string, v []string) KeyValue` confirmed present via `go doc go.opentelemetry.io/otel/attribute StringSlice` against the module in `$GOPATH/pkg/mod`. |
| `github.com/cenkalti/backoff/v5` | already direct (Phase 11) `[VERIFIED: go.mod]` | Only needed if the async incrementer reuses `summaryqueue.go`'s retry machinery — **not needed** per D-10 ("no retry"); a smaller primitive can skip this import entirely. |
| `go tool buf` (buf CLI via Go tool directive) | pinned in go.mod `tool` block `[VERIFIED: Taskfile.yaml `proto:gen`/`proto:lint` targets]` | Regenerates `gen/go` + `gen/ts` from `proto/engram/v1/engram.proto` after the `Memory` message gains fields | Existing codegen pipeline; CI's `buf` job re-runs `buf generate` and diffs `gen/` for drift. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Read-modify-write `SetPayload` counter (D-05) | Qdrant server-side atomic increment | Qdrant has no atomic payload-increment primitive as of v1.18 (confirmed by the RMW pattern already used nowhere else in this codebase and by the CONTEXT.md's explicit acceptance of lost updates) — not available, not a real alternative. |
| Reusing `summaryqueue.go` wholesale for the async incrementer | A smaller bounded-channel + semaphore primitive (no backoff/v5 import, no retry state machine) | `summaryqueue.go`'s retry/backoff machinery (`backoff.Retry`, `MaxTries`, `maxElapsed`) is explicitly NOT needed per D-10 ("no retry") — reusing it wholesale pulls in unused complexity; a smaller primitive that reuses only the `mu`/`closed`/`inFlight` shutdown-safety kernel is a tighter fit. Planner discretion per CONTEXT.md. |

**Installation:** none — no `go get` required; all libraries are already vendored.

**Version verification:** `go.mod` pins `github.com/qdrant/go-client v1.18.3`; confirmed present in the module cache at `/Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.18.3` and its `value_map.go` was read directly to confirm integer-type handling (see Common Pitfalls #1).

## Package Legitimacy Audit

**Not applicable — no new external packages are introduced by this phase.** Every library
used (`go-client`, `otel`, `backoff/v5` if reused, `buf`) is already a direct dependency in
`go.mod`, verified present via `go.mod`/`go.sum` inspection. No `npm view` / registry check
needed since this is a Go-only phase with zero new imports.

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│ MCP tool call: get_memory(id)                                       │
│   tools.go:908 deps.getMemory                                       │
│     ├─ ResolvePointID (owner-agnostic)                              │
│     ├─ store.GetReadable (ownership gate) ──► success ──┐           │
│     │                                                     │          │
│     │   [D-01: count HERE, after success, not inside     │          │
│     │    GetReadable/Get — those are reused internally]   │          │
│     │                                                     ▼          │
│     │                                     usageQueue.tryEnqueue(id) │
│     │                                     (bounded, non-blocking,   │
│     │                                      D-10 fire-and-forget)    │
│     │                                                     │          │
│     └─ return m (Memory) to caller ◄───────────────────────┘        │
│         (get_memory latency/success NEVER coupled to the bump)      │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼ (async worker)
                   store.IncrementAccess(ctx, id)
                       Get(id) → SetPayload(access_count+1, last_accessed_at)
                       [D-05: RMW, lost-update-tolerant, no retry, no mutex]
                       [drop-on-full → OTLP counter, never blocks/errors caller]

┌─────────────────────────────────────────────────────────────────────┐
│ MCP tool call: update_memory(id, content, ...)                      │
│   tools.go:843 deps.updateMemory                                    │
│     ├─ ResolvePointID + FetchForUpdate (ownership gate, ONE Get)     │
│     ├─ [D-04: bump cur.AccessCount++, cur.LastAccessedAt = now()    │
│     │    on the ALREADY-FETCHED cur, before Upsert — zero extra     │
│     │    write, piggybacks on the update's own re-embed+Upsert]     │
│     └─ store.Update(ctx, cur, ...) → Upsert (full payload write)    │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ Connect RPC: GetMemory / SearchMemories / ListMemories               │
│   connectapi.go:174 GetMemory  ──► same D-01 counting as MCP getMemory│
│   connectapi.go:144 SearchMemories ──► NO counting (D-02); ids ride  │
│   connectapi.go:95  ListMemories   ──► the recall span instead (D-06)│
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ OTLP recall-span analytics (D-06) — store layer, NOT the payload     │
│   store.Search / store.List / store.Get (existing tracer.Start spans)│
│     defer: span.SetAttributes(                                       │
│       attribute.StringSlice("engram.recall.ids", boundedIDs),        │
│       attribute.Int("engram.recall.count", len(out)))                │
│   → OTLP exporter (Phase 6, already wired, no-op if unconfigured)    │
│   → ClickStack (external; out of this repo's code)                   │
└─────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

No new files/directories are strictly required — this phase edits existing files. If the
async incrementer is a new primitive (vs. reusing `summaryqueue.go`), a new file follows the
existing flat-package convention:

```
internal/store/
├── store.go              # Memory struct, payload(), fromPayload(), new IncrementAccess()
├── rerank.go              # UNCHANGED except a new negative-space test in rerank_test.go
internal/server/
├── tools.go                # deps.getMemory / deps.updateMemory — increment call sites
├── connectapi.go            # Connect GetMemory — increment call site
├── usagequeue.go            # NEW (if not reusing summaryqueue.go) — bounded async incrementer
├── summaryqueue.go           # Phase 11 precedent — shutdown-safety kernel to mirror/reuse
internal/config/
├── registry.go               # + {Key: "usage.signals", Env: "ENGRAM_USAGE_SIGNALS", Default: "true"}
├── validate.go                 # + ParseBool check, unconditional (mirrors OnWrite)
internal/telemetry/
├── metrics.go                    # + usage-signal drop/enqueue counters (SummaryQueueMetrics template)
proto/engram/v1/
├── engram.proto                    # + access_count (uint64, field 19), last_accessed_at (Timestamp, field 20)
gen/go/, gen/ts/                     # regenerated via `task proto:gen`
```

### Pattern 1: Partial-payload write via `SetPayload` (the `IncrementAccess` primitive)

**What:** A point-ID-selector `SetPayload` call that mutates only the touched keys, preserving
the vector — no re-embed, no full `Upsert`.

**When to use:** Any get-path (read-triggered) mutation that must not cost an embed call —
exactly `SetVisibility`'s use case, and exactly what D-04's async get-path bump needs.

**Example (existing precedent — `SetVisibility`, `store.go:1334-1360`):**
```go
// Source: internal/store/store.go:1354-1358 (existing SetVisibility)
_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
    CollectionName: s.collection, Wait: qdrant.PtrOf(true),
    Payload:        qdrant.NewValueMap(map[string]any{"visibility": vis}),
    PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
})
```
`IncrementAccess` follows the same shape: `Get(id)` to read the current `access_count`
(RMW — D-05 accepts lost updates), then `SetPayload{"access_count": cur+1, "last_accessed_at":
now.Format(time.RFC3339)}`. Unlike `SetVisibility`, `IncrementAccess` should NOT re-run the
ownership gate (`getWritable`) — the caller (the async worker, fired from an already-gated
`getMemory` handler call) has already established the caller may read/act on this id; a
second internal `Get` is just to fetch the current counter value, not to re-authorize.

### Pattern 2: Free update-path bump (mutate `cur` before `Upsert`)

**What:** `Update` already re-embeds and writes the full payload via `Upsert`
(`store.go:1289-1324`); bumping counter fields on the already-in-memory `cur` before that call
costs nothing extra.

**Example:**
```go
// Source: internal/store/store.go:1301 (existing Update, illustrating the insertion point)
cur.Content = content
// [NEW] D-04: piggyback the update-path bump here, before Upsert:
cur.AccessCount++
cur.LastAccessedAt = s.now() // or time.Now() if no clock injected into this call chain
if shared != nil {
    // ... existing logic unchanged
}
```
Note `Update` does not currently receive a clock; `s.now()` (the `Store.now` field, already
injectable via `WithClock`, `store.go:178-183`) is the natural time source — reuse it rather
than adding a new parameter.

### Pattern 3: Bounded async worker pool with shutdown-safety kernel (D-10)

**What:** A channel-backed pool that (a) never blocks the caller (`select` with `default`
drop), (b) never panics on send-to-closed-channel during shutdown (RWMutex `closed` guard +
pre-send `RLock`), (c) provides a deterministic `Wait()` drain seam for tests via a
reservation-based `inFlight` WaitGroup.

**When to use:** Exactly the get-path counter bump — see `internal/server/summaryqueue.go` in
full for the shipped, code-reviewed (CR-01) version of this pattern. The reusable kernel is:
`ch chan T`, `mu sync.RWMutex` + `closed bool`, `wg sync.WaitGroup` (worker lifecycle),
`inFlight sync.WaitGroup` (drain seam), `tryEnqueue` (RLock-guarded reserve-before-send),
`Shutdown(ctx)` (Lock, flip closed, close(ch), wait bounded by ctx), `Wait()` (test-only drain
seam).

**What NOT to copy from `summaryqueue.go` (per D-10 "no retry"):** the `backoff.Retry` call,
`summaryQueueMaxTries`/`summaryQueueMaxInterval` constants, `maxElapsed` derivation, and
`Retried`/`Failed` counters tied to retry semantics. The usage-signal worker's `process`
function should be a single attempt: call `IncrementAccess` once, on error just log +
increment a `Dropped`/`Failed` OTLP counter, no retry loop.

### Pattern 4: OTel span attribute for a bounded-cardinality string slice (D-06)

**What:** Attach the list of returned record ids (capped) plus a count to an existing span,
using `attribute.StringSlice`.

**Verified API (via `go doc` against the vendored module):**
```go
// go.opentelemetry.io/otel/attribute
func StringSlice(k string, v []string) KeyValue
```

**Example insertion point (existing `store.Search` span defer, `store.go:552-560`):**
```go
// Source: internal/store/store.go:552-560 (existing Search, illustrating the insertion point)
defer func() {
    telemetry.RecordStoreOp(ctx, "Search", start, err)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    } else {
        span.SetAttributes(attribute.Int("engram.result_count", len(out)))
        // [NEW] D-06: bounded recall-id analytics, e.g. cap at 50:
        ids := recallIDs(out, recallIDCap)
        span.SetAttributes(
            attribute.StringSlice("engram.recall.ids", ids),
            attribute.Int("engram.recall.count", len(out)),
        )
    }
}()
```
`store.List` (`store.go:753-830`, three return paths — cursor mode, offset mode, empty
short-circuit) and `store.Get` (`store.go:1074-1097`, single record, no existing
`result_count` attribute today) both need the equivalent addition. `store.Get`'s span
currently has no result-count/ids attribute at all (single record, id is already known to the
caller) — CONTEXT.md's D-06 explicitly lists `get_memory` among the spans that carry recall
ids, so add `engram.recall.ids = [id]` / `engram.recall.count = 1` there too for
data-completeness in ClickStack even though it looks redundant with the tool arg.

### Anti-Patterns to Avoid

- **Counting inside `store.Get`/`GetReadable`/`getWritable`:** These are called by
  `OwnedOrAbsent`, `FetchForUpdate`, `SetVisibility`'s ownership check, and other internal
  paths that are NOT a user-facing "get" event. Counting there inflates the signal — this is
  explicitly forbidden by D-01. Count only in the `deps` handler methods after the gate
  succeeds.
- **Adding `AccessCount` as a rerank tiebreaker "since it's already on the struct":** D-08 is a
  hard invariant. `rerank.go`'s `RerankHits` must never read the field. Guard with the
  negative-space test (see Validation Architecture).
- **Making the get-path bump synchronous / error-coupled to `get_memory`:** D-04 requires the
  bump be fire-and-forget; a synchronous `IncrementAccess` call in `getMemory` that returns an
  error on Qdrant unavailability would silently break `get_memory` during a Qdrant blip —
  exactly what D-04 forbids.
- **Reusing `backoff.Retry`/`maxElapsed` machinery for the get-path worker:** D-10 explicitly
  wants no-retry, drop-on-full. Retrying a soft counter bump adds latency/complexity for zero
  correctness benefit (D-05 already accepts lost updates).
- **Forgetting `recallView`:** editing only `store.Memory` and assuming `list_memory`/
  `search_memory` will "just pick it up" — they won't; `toRecallView` is an explicit
  allow-list (see Summary finding 1).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Atomic counter increment against Qdrant | A custom distributed lock / per-record mutex / optimistic-retry-until-success loop | Plain `Get` → `SetPayload(value+1)`, RMW, accept lost updates (D-05) | Qdrant has no atomic increment primitive; a lock only protects this one process (Qdrant itself has no cross-client CAS on payload fields), so it wouldn't even close the race across replicas/instances — pure complexity for no correctness gain. This is a deliberate, already-decided tradeoff (D-05), not an open problem. |
| Bounded async worker pool with safe shutdown | A new goroutine-per-call + manual `sync.WaitGroup` + ad hoc "close channel and hope" | The Phase 11 kernel (`mu`/`closed`/`inFlight`/`tryEnqueue`/`Shutdown`) — either reused wholesale from `summaryqueue.go` or extracted into a smaller shared primitive | The CR-01 send-on-closed-channel bug was already found and fixed once in this exact codebase (Phase 11); re-deriving the same fix independently risks reintroducing it. D-10 explicitly mandates reuse of this kernel. |
| Recall-id analytics storage | A new Qdrant collection, a new SQL table, or a new payload field per query | Existing OTLP span attributes → ClickStack (already provisioned in Phase 6) | The store already has an OTLP pipeline with zero additional infrastructure; adding storage for this is explicitly the anti-goal of the hybrid design (D-03/D-06). |
| Config bool parsing | A custom env-var-to-bool parser | `strconv.ParseBool` at point-of-use, exactly like `buildSummaryQueue`'s `onWrite, err := strconv.ParseBool(cfg.Summarize.OnWrite)` | Established, tested idiom already used for `ui.enabled`/`summarize.on_write`; deviating invites subtle parsing bugs (e.g. accepting `"1"`/`"yes"` inconsistently). |

**Key insight:** Every "hard" problem this phase touches (atomic counters against Qdrant,
safe async shutdown, config gating) has already been solved once in this codebase for a
structurally identical problem (visibility toggling, async summary fill, `ui.enabled`). The
implementation risk here is almost entirely about *finding and copying* those precedents
correctly, not about designing anything new.

## Common Pitfalls

### Pitfall 1: Assuming Qdrant coerces integers to doubles (JSON-number ambiguity)

**What goes wrong:** A developer worries `access_count` (uint64) might round-trip through
Qdrant's JSON-like payload as a float, losing precision or requiring float-based comparisons.

**Why it happens:** Qdrant payloads are conceptually "JSON", and JSON has no native integer
type — so it's a reasonable a priori concern.

**How to avoid:** `[VERIFIED: go-client v1.18.3 source]` — `NewValueMap`'s doc table
(`value_map.go:32-44`) and `NewValue`'s `switch` (`value_map.go:68-113`) show `uint64` (and
`int`/`int32`/`int64`/`uint`/`uint32`) are converted via `NewValueInt(int64(v))`, stored as
`Value_IntegerValue{IntegerValue int64}` — a dedicated protobuf oneof variant, NOT coerced to
`DoubleValue`. `fromPayload`'s existing `not_before`/`not_after` fields already read via
`v.GetIntegerValue()` (`store.go:373,377`) confirming this round-trips losslessly today.
`access_count` should follow the identical pattern: write as `uint64` (auto-converted to
`IntegerValue`), read via `v.GetIntegerValue()` (returns `int64`; cast to `uint64` — safe for
any realistic access count, `int64` overflows only past ~9.2×10^18).

**Warning signs:** A test asserting `access_count == 3.0` (float comparison) or manual JSON
inspection showing `"access_count": 3` without a decimal point (this is actually the CORRECT,
expected observation — Qdrant's HTTP API and internal storage do distinguish int/float; the
Go client's `IntegerValue`/`DoubleValue` split maps directly to it).

### Pitfall 2: `recallView`'s allow-list silently drops new fields

**What goes wrong:** A developer adds `AccessCount`/`LastAccessedAt` to `store.Memory` and
`payload()`/`fromPayload()`, tests `get_memory` (works, since it returns raw `Memory`), and
assumes `list_memory`/`search_memory` (which call `shapeRecall` → `toRecallView` when
`full=false`, the DEFAULT) also carry the new fields — they silently don't, because
`toRecallView` (`internal/server/summary.go:89-96`) constructs a `recallView` struct
(`summary.go:40-53`) that explicitly lists which `Memory` fields survive the compact shape.

**Why it happens:** `store.Memory`'s JSON tags make it *feel* like every field is
automatically serialized everywhere; that's true for `get_memory` (raw `Memory` return) and
`full=true` recall, but false for the default compact shape.

**How to avoid:** Explicitly add `AccessCount uint64` and `LastAccessedAt time.Time` fields
(with appropriate `json:"..."` tags, likely `omitempty` for `LastAccessedAt` when zero) to
`recallView` AND populate them in `toRecallView`. D-07 says "expose... on... `list_memory`
items" — this is the concrete edit that satisfies that clause.

**Warning signs:** A UAT/manual test of `list_memory` (default, no `full=true`) shows no
`access_count` field in the JSON output while `get_memory` on the same record shows it.

### Pitfall 3: Proto field addition without regen fails CI's drift check, not `buf breaking`

**What goes wrong:** A developer edits `proto/engram/v1/engram.proto` (adds fields 19/20 to
`Memory`), edits `memoryToProto` in `connectapi.go` to populate them, but forgets to run
`task proto:gen` (or runs it but doesn't commit the resulting `gen/` diff).

**Why it happens:** The code compiles fine against the STALE `gen/go` package until the new
proto-only fields are referenced in Go code that expects them to exist on the generated
struct — a `memoryToProto` reference to a not-yet-generated `engramv1.Memory.AccessCount`
field is a compile error, which is a fast, obvious failure. The dangerous case is the reverse
direction: forgetting to update `gen/ts/` (the frontend TypeScript types) — the Go build
succeeds, but CI's dedicated `buf` job (`.github/workflows/ci.yaml:107-123`) runs `go tool buf
generate` fresh and does `git diff --exit-code -- gen/`, failing the PR if the committed
`gen/` tree doesn't match a fresh regen. Additive field numbers (19, 20 after the existing
1-18) are NOT flagged by `buf breaking --against main` (adding new fields is non-breaking) —
so `buf breaking` will pass; only the separate drift-check step catches a missed
`task proto:gen`.

**How to avoid:** After editing `engram.proto`, always run `task proto:gen` locally and
`git add gen/` before committing. Confirm with `git status -- gen/` showing no unstaged
changes.

**Warning signs:** CI's `buf` job (not `buf breaking`, the "generated-code drift" step) fails
with "gen/ is stale; run 'task proto:gen'".

### Pitfall 4: `instrumentTools` (the generic MCP middleware) is the WRONG seam for recall-id span attributes

**What goes wrong:** A developer sees `internal/server/instrument.go`'s `instrumentTools`
middleware (wraps EVERY `tools/call`, sets `engram.tool`/`engram.owner` span attributes) and
assumes this is where to add `engram.recall.ids`, since it's the one place that sees every
tool call generically.

**Why it happens:** It genuinely is the seam for tool-agnostic span attributes (tool name,
owner, outcome) — but it operates on `mcp.Result` (a generic `*mcp.CallToolResult` with a text
blob + arbitrary structured content), not on typed `[]store.Memory`. Extracting record ids
from that generic result would require type-asserting/reflecting into tool-specific output
shapes inside a supposedly tool-agnostic middleware — fragile and duplicative across the MCP
and Connect code paths (which don't share `instrumentTools` at all; Connect uses
`otelconnect.NewInterceptor()`, `connectapi.go:233-245`).

**How to avoid:** Attach `engram.recall.ids`/`engram.recall.count` at the STORE layer instead
— `store.Search`/`store.List`/`store.Get`'s own `tracer.Start("store.Search"/...)` spans
(`store.go:544-576`, `753-830`, `1074-1097`), which already have the typed `[]Memory`/`Memory`
result in scope in their `defer` blocks and already set `engram.result_count` there. This is
ALSO the single seam shared by both MCP and Connect call paths (both eventually call
`store.Search`/`store.List`/`store.Get`/`store.SearchReranked`), so one edit covers both
surfaces — consistent with D-06's design intent and the existing `engram.result_count`
precedent.

**Warning signs:** Recall-id attributes appear on MCP tool-call spans (`tool/search_memory`)
but not on Connect `SearchMemories` spans, or vice versa — a sign the attribute was attached
at a transport-specific layer instead of the shared store layer.

## Runtime State Inventory

Not applicable — this is a greenfield additive-field phase (Skipping per instructions: no
rename/refactor/migration is involved). New payload keys (`access_count`,
`last_accessed_at`) are additive; legacy records simply lack the key, which
`fromPayload`-style `if v, ok := p["access_count"]; ok { ... }` guards already handle as a
zero value with no backfill needed (D-03 explicit).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Qdrant (via testcontainers in tests; live instance in prod) | `store_test.go` integration tests (`testStore(t)`, skips if `testQdrantAddr == ""`) | Not probed in this research session (no live Qdrant/docker check run) `[ASSUMED: pattern only, not re-verified this session]` | — | Tests already self-skip when unavailable — no phase-blocking risk; CI provisions Qdrant via testcontainers per existing `store_test.go` setup. |
| `go tool buf` | `task proto:gen`/`proto:lint`, CI `buf` job | `[VERIFIED: Taskfile.yaml + go.mod tool directive]` present in this repo's toolchain | pinned via `go.mod` `tool` block | None needed — already the repo's standing codegen pipeline. |
| OTLP collector (ClickStack) | D-06 span attributes to be USEFUL (not required for the code to compile/run — spans no-op when unconfigured) | Assumed provisioned per Phase 6 (`REQ-observability-telemetry`, shipped) `[ASSUMED: not re-verified this session — Phase 6 is marked Complete in REQUIREMENTS.md]` | — | Spans are a no-op (existing OTLP idiom, DEC-dwi) when `ENGRAM_OTEL_*` is unset — this phase's code has zero runtime dependency on ClickStack actually running. |

**Missing dependencies with no fallback:** none identified.

**Missing dependencies with fallback:** OTLP/ClickStack absence — already handled by the
existing no-op OTLP idiom; not a phase-blocking gap.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + `tracetest.SpanRecorder` for span assertions `[VERIFIED: internal/store/instrument_test.go]` |
| Config file | none — `go test ./...` via `task test` (Taskfile.yaml; not independently re-verified this session but consistent with every other Go phase in this repo) |
| Quick run command | `go test ./internal/store/... ./internal/server/... -run TestUsage -v` |
| Full suite command | `task` (lint + test, per CLAUDE.md) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-usage-signals | `IncrementAccess` bumps `access_count`/`last_accessed_at` via `SetPayload`, RMW-tolerant | integration (needs Qdrant) | `go test ./internal/store/... -run TestIncrementAccess -v` | ❌ Wave 0 — new file, likely `internal/store/usage_test.go` |
| REQ-usage-signals | `update_memory` bump is free (piggybacks `Upsert`, no extra store call) | unit/integration | `go test ./internal/store/... -run TestUpdateBumpsAccessCount -v` | ❌ Wave 0 — extend `store_test.go` or new `usage_test.go` |
| REQ-usage-signals | Get-by-id increments; search/list do NOT (D-02 negative-space) | integration | `go test ./internal/server/... -run TestUsageSignal -v` | ❌ Wave 0 — new file, likely `internal/server/usagequeue_test.go` (mirrors `summaryqueue_test.go` structure, if present) |
| REQ-usage-signals | Reranker output is invariant under `access_count` (D-08 hard invariant) | unit, pure | `go test ./internal/store/... -run TestRerankHitsIgnoresAccessCount -v` | ❌ Wave 0 — extend `rerank_test.go` |
| REQ-usage-signals | `store.Search`/`List`/`Get` spans carry `engram.recall.ids`/`engram.recall.count` (D-06) | integration (needs Qdrant + `tracetest.SpanRecorder`) | `go test ./internal/store/... -run TestStore.*EmitsRecallSpan -v` | ❌ Wave 0 — extend `instrument_test.go` |
| REQ-usage-signals | `ENGRAM_USAGE_SIGNALS` gates the payload write (D-09 config gate; parseability + AND-gate) | unit | `go test ./internal/config/... -run TestUsageSignals -v` and `go test ./internal/server/... -run TestBuildUsageQueue -v` | ❌ Wave 0 — extend `validate_test.go` (or equivalent) + `tools_test.go` |
| REQ-usage-signals | Async worker shutdown-safety: no send-on-closed-channel, bounded drain | integration/unit, mirrors Phase 11's `summaryqueue_test.go` | `go test ./internal/server/... -run TestUsageQueueShutdown -v` | ❌ Wave 0 — new file |

### Sampling Rate

- **Per task commit:** `go test ./internal/store/... ./internal/server/... ./internal/config/... -run TestUsage` (or the equivalent targeted `-run` per touched package)
- **Per wave merge:** `task test` (full suite)
- **Phase gate:** Full suite green (`task`, lint+test) before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/store/usage_test.go` (or extend `store_test.go`) — covers `IncrementAccess` RMW behavior, `payload()`/`fromPayload()` round-trip for the two new fields, legacy-record zero-value default (D-03)
- [ ] `internal/store/rerank_test.go` extension — negative-space `RerankHits` invariance test (D-08) — construct two `Memory` records identical except `AccessCount`, assert unchanged rank order
- [ ] `internal/store/instrument_test.go` extension — `engram.recall.ids`/`engram.recall.count` span attribute presence on `store.Search`/`store.List`/`store.Get` (D-06), following the exact `withSpanRecorder`/`spanByName` pattern already in this file
- [ ] `internal/server/usagequeue_test.go` (new, if a new primitive is built) — mirrors the shutdown-safety test coverage `summaryqueue.go` presumably has (search for `summaryqueue_test.go` when planning to confirm exact test names to mirror)
- [ ] `internal/server/tools_test.go` extension — `getMemory` triggers async increment (observable via a fake/injected `usageQueue.fill` closure, mirroring `summaryQueue`'s test-injected `fill func`), `updateMemory` bump lands in the same `Upsert` call (no extra store call — assert via a call-counting fake store or an integration-level `access_count` check pre/post update)
- [ ] `internal/config/validate_test.go` extension (file name assumed — confirm exact existing test file for `Config.Validate()`) — `ENGRAM_USAGE_SIGNALS` parseability check
- [ ] Framework install: none — stdlib `testing` + already-vendored `tracetest` package, no new install needed

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Phase adds no new auth surface; counting happens strictly after the existing `GetReadable`/`FetchForUpdate` ownership gates succeed (D-01) — no new auth decision point. |
| V3 Session Management | No | Unchanged. |
| V4 Access Control | No (reaffirms existing) | The increment path deliberately does NOT re-run an independent authz check inside `IncrementAccess` — it trusts that the caller (async worker fired from an already-`GetReadable`-gated `getMemory` call) already established access. This is safe because `IncrementAccess` only reads/writes `access_count`/`last_accessed_at` on an id the caller already proved readable — it never expands what the caller can read/write. No new access-control surface is introduced. |
| V5 Input Validation | No new surface | `access_count`/`last_accessed_at` are server-set only — confirmed no `updateArgs`/`idArgs`/`storeArgs` struct exposes a client-writable field for either (`tools.go:431-441` inspected directly) — a malicious client cannot inject an arbitrary `access_count` value via any tool argument. |
| V6 Cryptography | No | Not applicable — no new cryptographic operation. |

### Known Threat Patterns for {Go + Qdrant MCP server}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unbounded goroutine spawn on high get_memory volume (resource exhaustion) | Denial of Service | D-10's bounded channel + drop-on-full (mirroring `summaryQueue`'s `tryEnqueue` `select`/`default`) — already the mitigation baked into the locked design; the planner must ensure the new queue's capacity is a config-tunable or a sane hardcoded bound, not unbounded. |
| Information disclosure via `last_accessed_at` cross-actor leakage | Information Disclosure | Not a new risk — `access_count`/`last_accessed_at` are read only through the EXISTING per-actor isolation gates (`GetReadable`, `List`'s owner/scope filter); no new read path bypasses `ownerOrSharedCondition`/`ownerScopeFilter`. Confirm in code review that neither field is added to any filter/index that could leak via a side channel (e.g. a `created_at`-style range-filter payload index — D-03/research confirms no index is planned for `access_count`). |
| Send-on-closed-channel panic during shutdown (availability) | Denial of Service (crash) | The exact CR-01 bug already found+fixed in Phase 11 — D-10 explicitly mandates reusing that kernel (RWMutex `closed` guard + reserve-before-send). A planner who builds a NEW async primitive without copying this guard reintroduces a known, previously-shipped bug class in this codebase. |
| Qdrant RMW lost-update exploited to suppress a usage signal (e.g. hide "hot" status of a record an attacker wants ignored) | Tampering (low severity) | Explicitly accepted by D-05 as a soft-signal tradeoff — not a security control, a curation-accuracy tradeoff. No mitigation needed; the counter is advisory, never authoritative for any access-control or ranking decision (D-08 enforces the latter). |

## Code Examples

### Payload round-trip addition (illustrative; exact field names per D-03)

```go
// Source: internal/store/store.go — payload() (existing pattern at ~271-320)
p["access_count"] = m.AccessCount // uint64 → auto-converted to Value_IntegerValue
if !m.LastAccessedAt.IsZero() {
    p["last_accessed_at"] = m.LastAccessedAt.Format(time.RFC3339)
}
```

```go
// Source: internal/store/store.go — fromPayload() (existing pattern at ~322-416)
if v, ok := p["access_count"]; ok {
    m.AccessCount = uint64(v.GetIntegerValue())
}
if v, ok := p["last_accessed_at"]; ok {
    if t, err := time.Parse(time.RFC3339, v.GetStringValue()); err == nil {
        m.LastAccessedAt = t
    }
}
```

### `attribute.StringSlice` verified signature

```go
// go.opentelemetry.io/otel/attribute (verified via `go doc` against the vendored module)
func StringSlice(k string, v []string) KeyValue
```

## State of the Art

Not applicable in the traditional sense (no external library/framework version drift to
track) — this phase's "state of the art" is entirely intra-repo precedent-following. The one
relevant fact: `github.com/qdrant/go-client` has moved from v1.14.0 to v1.18.3 in this
module's history (both present in the local module cache), and the currently-pinned v1.18.3
is what was verified for the integer/double payload-type behavior — no newer version needs
research for this phase.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A live Qdrant/testcontainers setup is available in CI for the new integration tests (not independently re-verified this session; inferred from existing `testStore(t)`/`testQdrantAddr` skip-pattern) | Environment Availability | Low — tests already self-skip via the existing `testQdrantAddr == ""` guard if unavailable locally; CI provisioning is unchanged by this phase. |
| A2 | ClickStack (OTLP consumer) is actually running/reachable in the target deployment (Phase 6 marked Complete in REQUIREMENTS.md, not independently re-verified this session) | Environment Availability | Low — this phase's code has zero runtime dependency on ClickStack; spans no-op safely if OTLP is unconfigured (existing DEC-dwi idiom). |
| A3 | `task test` / `task` invoke `go test ./...` with no non-obvious flags that would change how targeted `-run` filtering works | Validation Architecture | Low — standard Go testing convention; if `task test` uses `-race` or coverage flags, targeted `-run` commands remain valid, just slower. |
| A4 | No existing `internal/config/validate_test.go` or `internal/server/summaryqueue_test.go` file names were directly confirmed this session (referenced by pattern, not `ls`-verified) | Validation Architecture (Wave 0 Gaps) | Low — a `find`/`ls` at plan time will confirm exact file names before task-writing; does not change the substance of what needs testing. |

## Open Questions

1. **Does Qdrant store an integer payload losslessly, or coerce to double?**
   - What we know: `[VERIFIED: go-client v1.18.3 source, value_map.go]` — `uint64`/`int64`
     etc. convert to a dedicated `Value_IntegerValue` protobuf oneof variant, distinct from
     `Value_DoubleValue`; the existing `not_before`/`not_after` fields already round-trip via
     `GetIntegerValue()` in production code today.
   - What's unclear: nothing — this is resolved. Not an open question for the planner.
   - Recommendation: use `uint64` for `AccessCount`, write via `payload()`'s `map[string]any`
     (auto-converts), read via `v.GetIntegerValue()` cast to `uint64`.

2. **Does exposing `access_count` on the Connect `Memory` proto require a proto field
   addition + `task proto:gen` regen?**
   - What we know: `[VERIFIED: proto/engram/v1/engram.proto`, `gen/go/engram/v1/*.go]` — the
     `Memory` message currently ends at field 18; Connect responses are built exclusively via
     `memoryToProto` from `store.Memory`, with no dynamic/reflection-based field passthrough.
     Adding fields 19/20 is additive (non-breaking per `buf breaking`), but CI's separate
     drift-check step (`buf` job, `.github/workflows/ci.yaml:120-123`) fails the build if
     `gen/` isn't regenerated and committed.
   - What's unclear: nothing — resolved. The planner should schedule an explicit
     "proto field + `task proto:gen` + commit `gen/`" task.
   - Recommendation: one dedicated task for the proto+codegen ripple, sequenced before the
     `memoryToProto`/`shapeProtoMemories` edits that reference the new generated fields.

3. **Is a payload index needed for `access_count`?**
   - What we know: `[VERIFIED: store.go ensureIndexes, ~240-269]` — only `owner`, `scope`,
     `created_at`, `short_id` are indexed; none are filter/sort keys this phase would use for
     `access_count` (no filter, no order-by, no facet use planned per D-07/D-08).
   - What's unclear: nothing — resolved. No index needed.
   - Recommendation: do not add an index; this also avoids inadvertently making
     `access_count` filterable/sortable in a way that could tempt a future "usage-weighted
     recall" implementation to bypass the D-08 boundary via a raw Qdrant filter rather than
     the reranker.

4. **Confirm the async get-path incrementer's failure mode under a Qdrant outage is a
   silent drop, never a `get_memory` error.**
   - What we know: `[VERIFIED: tools.go:908-922 deps.getMemory]` — the handler currently
     returns immediately after `GetReadable` succeeds; D-04 requires the counter bump be
     fire-and-forget (async), so as long as the increment call is dispatched via
     `usageQueue.tryEnqueue(id)` (non-blocking, mirroring `summaryQueue.tryEnqueue`) AFTER
     the `return m, err` path is otherwise unaffected — i.e. `tryEnqueue` must be called but
     its outcome must never be checked/propagated by `getMemory` — this is structurally
     guaranteed by copying the `summaryQueue.tryEnqueue` calling convention exactly (the
     Phase 11 write-path call sites already demonstrate the "call and ignore" pattern; grep
     `tryEnqueue(` call sites in `tools.go` for the exact precedent before writing this
     phase's equivalent call).
   - What's unclear: nothing structurally — resolved by precedent. The only planner
     responsibility is to NOT accidentally check/return an error from the enqueue call.
   - Recommendation: mirror the exact `d.summaryQueue.tryEnqueue(id)` call-and-ignore
     precedent at the `getMemory`/Connect `GetMemory` call sites.

## Environment Availability

*(See dedicated section above — included per template; not duplicated here.)*

## Sources

### Primary (HIGH confidence)
- `internal/store/store.go` (this repo, read directly) — `Memory` struct (~86-140), `payload`
  (~271-320), `fromPayload` (~322-416), `WithClock`/`New` (~178-192), `Get`/`GetReadable`/
  `getWritable`/`FetchForUpdate`/`Update`/`SetVisibility` (~1074-1360), `Search`/`List`
  (~544-830), `ensureIndexes` (~240-269).
- `internal/server/tools.go` (this repo, read directly) — `deps` struct (~34-53),
  `buildDepsFromEnv`/`buildSummaryQueue` (~159-224), `updateMemory`/`getMemory`
  (~843-922), `idArgs`/`updateArgs` (~431-441), `Register` (~993-1096).
- `internal/server/connectapi.go` (this repo, read directly) — `memoryToProto`
  (~32-44), `ListMemories`/`SearchMemories`/`GetMemory` (~95-201), `mountConnect`
  (~229-249).
- `internal/server/summaryqueue.go` (this repo, read in full) — the D-10 shutdown-safety
  kernel to reuse/mirror.
- `internal/server/instrument.go` (this repo, read in full) — confirms `instrumentTools` is
  the wrong seam for recall-id attributes (Pitfall 4).
- `internal/store/rerank.go` (this repo, read in full) — confirms `RerankHits` is pure and
  does not read `AccessCount` today (baseline for the D-08 negative-space test).
- `internal/store/instrument_test.go` (this repo, read in full) — the exact
  `tracetest.SpanRecorder`/`withSpanRecorder`/`spanByName` pattern to extend for D-06 tests.
- `internal/server/summary.go` (this repo, read directly) — `recallView` struct
  (~40-53), `toRecallView` (~89-96), `shapeRecall` (~76-86) — the Pitfall 2 finding.
- `internal/config/registry.go` / `internal/config/validate.go` (this repo, read directly) —
  the `field` registry shape and `ENGRAM_SUMMARY_ON_WRITE` parseability-check precedent for
  D-09.
- `internal/telemetry/metrics.go` (this repo, read directly, ~111-185) — `SummaryQueueMetrics`
  template for new usage-signal OTLP counters.
- `proto/engram/v1/engram.proto` (this repo, read directly, ~11-32) — `Memory` message
  current field numbers (1-18), confirming 19/20 are the next available.
- `gen/go/engram/v1/*.go` (this repo, read directly) — confirms the generated Go struct
  mirrors the proto exactly, no hand-editing possible (codegen is the only path).
- `.github/workflows/ci.yaml` (this repo, read directly, ~107-123) — the `buf` CI job's
  lint/breaking/drift-check three-step structure.
- `Taskfile.yaml` (this repo, read directly, ~136-143) — `proto:lint`/`proto:gen` targets.
- `/Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.18.3/qdrant/value_map.go` (module
  cache, read directly) — `NewValueMap`/`NewValue`/`TryValueMap` type-conversion table,
  resolving Open Question 1.
- `go doc go.opentelemetry.io/otel/attribute StringSlice` (run directly against the vendored
  module) — confirms `attribute.StringSlice(k string, v []string) KeyValue` exists.
- GitHub issue #317 (`gh issue view 317`, fetched directly) — confirms CONTEXT.md's D-01..D-10
  faithfully reflect the original design discussion.
- `.planning/phases/09-retrieval-eval-harness-ranking-precision/09-CONTEXT.md` (this repo,
  read directly, ~32-35) — the exact "usage signals must never affect ranking" cross-phase
  constraint wording D-08 derives from.

### Secondary (MEDIUM confidence)
None — every claim in this document was either read directly from repo source, verified via
a tool invocation (`go doc`, `gh issue view`), or explicitly logged in the Assumptions Log
above.

### Tertiary (LOW confidence)
None beyond what is logged in the Assumptions Log (A1-A4), all of which are pattern-inferred
rather than independently re-verified this session, and all flagged low-risk if wrong.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; every library/version claim verified directly
  against `go.mod`/module cache source.
- Architecture: HIGH — every integration point (payload round-trip, handler boundary, span
  seam, proto/codegen ripple, config gate, async kernel) was located and read directly in this
  repo, not inferred from CONTEXT.md's summary alone.
- Pitfalls: HIGH — all four pitfalls are grounded in direct code reading (the `recallView`
  allow-list gap and the `instrumentTools` wrong-seam finding were NOT called out explicitly
  in CONTEXT.md's `<code_context>` and are genuinely new findings from this research pass).

**Research date:** 2026-07-10
**Valid until:** 30 days (stable, intra-repo-only findings; no external API/library drift risk
for this phase's scope) — re-verify if `qdrant/go-client` or the proto/buf toolchain is
upgraded before this phase is planned/executed.
