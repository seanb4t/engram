<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Design: Server-Side Windowed + Cursor Recall

- **Bead:** engram-nx2t
- **Date:** 2026-06-29
- **Status:** Design (pending design-reviewer)

## Problem

An agent wanting "memories from June 27" or "everything before yesterday" has no
direct way to ask for it. `list_memory` exposes only `scope` + `limit` (+ `tags`,
`full`); there is no creation-time window and no way to page deterministically.
The current workaround is to **over-fetch recent-first and filter client-side on
`created_at`** — which only works while the desired records fall inside the most
recent `limit`, and silently misses anything past the `scanCap=1000` ceiling.

Underneath that surface gap is a structural one: `owner`, `scope`, and
`created_at` are **not indexed in Qdrant**, so every `List`/`ListScheduled`/
`ListScopes` scrolls up to `scanCap=1000` matching points and filters + sorts
**in memory**. This has two consequences:

1. The **authz/owner filter does not scale** — it is a linear scan bounded by an
   arbitrary cap, not an index lookup.
2. **`scanCap` silently truncates.** The `approximate` flag is the system
   admitting it cannot tell you the real total once a scope exceeds 1000
   readable records.

This work fixes both: a true server-side recall path (indexed filter +
server-side `created_at` range + `order_by` + exact `Count`) and a date window +
cursor paging surfaced on every recall tool.

## Constraints from the existing system

- **`created_at` is stored as an RFC3339 string** in the Qdrant payload
  (`internal/store/store.go` `toPayload`/`fromPayload`). Today that string is
  *not* range-filterable, which is why `SummarizeMissing` and `prune-expired`
  filter `created_at` **in code** after scrolling. Qdrant *can* range-filter and
  order a datetime field **once it carries a `Datetime` payload index** — the
  string values already stored are RFC3339 and parse natively. No re-stamping of
  payloads is required; only index creation.
- **Recall is pull-based and authz-gated.** Every windowing/paging change must
  compose with — never bypass — the owner/visibility/active-window gate
  (`listFilter` + `activeWindowConditions`). The date window is an *additional*
  `must` condition, applied alongside the authz envelope.
- **ADR engram-lkm (Accepted 2026-06-11)** added `offset` + `categories` +
  `visibility` + `total` + `approximate` to `ListMemories`, chose offset over
  cursor ("cursor deferred"), and justified `scanCap` as "sufficient for the
  single-operator use case." This design **supersedes the deferred-cursor
  decision** and **retires `scanCap`/`approximate` for `List`** — see ADRs below.
- **The MCP `list_memory` tool returns a bare array** (`shapeRecall` → `[]any`).
  Returning a cursor requires reshaping the tool output (see §6).
- **go-client v1.18.2 has every primitive needed** (verified): `CreateFieldIndex`
  + `FieldType_FieldTypeDatetime`/`Keyword`; `FieldCondition.DatetimeRange`
  (gt/gte/lt/lte); `ScrollPoints.OrderBy` + `OrderBy.StartFrom`
  (`NewStartFromDatetime`/`NewStartFromTimestamp`); `Count` (already used 5× in
  `store.go`).

## Goals / Non-Goals

**Goals**

- Date-windowed recall (`created_after`/`created_before`) on **all** recall tools:
  `list_memory`, `search_memory`, `list_scheduled`.
- Deterministic cursor paging on `list_memory` (MCP default), offset retained for
  the operator-console UI.
- Qdrant payload indexes on `owner`, `scope`, `created_at` — turning recall into a
  true server-side query and deleting `scanCap`/`approximate` for `List`.
- No data migration: indexes backfill over existing points on next boot.

**Non-Goals (YAGNI / deferred)**

- Relative-duration window inputs ("last 7d") — absolute RFC3339 only for v1.
- `ListScopes` aggregation rework (Facet API) — it benefits from the indexes for
  free; cross-scope GROUP-BY counting is a separate follow-up.
- Cursor paging on `search_memory` (stays vector top-k) and `list_scheduled`
  (stays limit-only).
- Free-text server filters.

## Architecture

Three layers, bottom-up — each only pays off because the one above queries it:

```
Qdrant payload indexes   →   Store recall rebuild        →   Surfaces
owner / scope / created_at   List/Search/ListScheduled       Connect + MCP tools
(keyword/keyword/datetime)   indexed filter + range +        date window +
                             order_by + Count(exact)         cursor / offset
```

### 1. Index foundation (`ensureCollection`)

After the collection is ensured, idempotently create three payload indexes on
**every boot** (so existing deployments self-heal):

| Field        | Index type | Params          | Why                                              |
|--------------|------------|-----------------|--------------------------------------------------|
| `owner`      | keyword    | `is_tenant=true`| Tenant optimization co-locates a caller's points → fast authz filtering |
| `scope`      | keyword    | —               | Scope is a `must` on every recall path           |
| `created_at` | datetime   | —               | Enables `DatetimeRange` filter + `order_by`      |

- Creation is idempotent: a Qdrant "index already exists" response is treated as
  success; any other error fails collection setup loudly (recall correctness
  depends on the indexes existing).
- Pre-isolation records (missing `owner` key) are simply absent from the `owner`
  index — consistent with their existing "invisible until backfilled" status.
- The indexes apply to the single shared collection; the discovery scope
  (`discovery:repo:*`) lives in the same collection and benefits identically.

### 2. Store recall rebuild (Approach A — full server-side)

`store.List` is rebuilt from *scroll-to-cap → in-memory sort → slice* to a true
server-side query:

- **Filter:** existing `listFilter` (scope/owner/visibility/tags/categories +
  `activeWindowConditions`) **plus** a `created_at` `DatetimeRange` when a window
  is supplied.
- **Order:** `order_by created_at desc` (server-side, via `ScrollPoints.OrderBy`).
- **Total:** `Count(filter)` → **exact**. The `scanCap` scroll ceiling and the
  `approximate` return are **removed from `List`**.
- **Paging:** one `ListOptions`, two mutually-exclusive modes:
  - **Offset** (UI page-jump): Qdrant has **no numeric `OFFSET`** —
    `ScrollPoints.Offset` is a point-ID continuation cursor, not a skip count. So
    offset mode issues an `order_by created_at desc` scroll for **`offset + limit`**
    records and returns the trailing `limit` (discarding the `offset`-length head).
    `total` comes from `Count(filter)` (exact). Cost is `O(offset + limit)` per
    page — fine for the operator console's modest page depth; **deep paging should
    use cursor mode**, which is `O(limit)`. This retires the `scanCap` *filter*
    ceiling (the filter is now indexed/server-side); the only remaining cost is the
    user-chosen page depth, not a fixed 1000-row scan.
  - **Cursor** (MCP, default): boundary id-set resume (see §3), `O(limit)` per page.

`ListOptions` gains: `CreatedAfter time.Time`, `CreatedBefore time.Time` (zero =
unbounded), `Cursor *Cursor` (nil = offset mode). `Offset` and `Cursor` are
mutually exclusive — supplying both is an invalid-arg error at the store boundary.

**`Search`** gains only the `created_at` `DatetimeRange` in its existing filter; it
remains a vector top-k query (no scroll, no cursor, no `Count`).

**`ListScheduled`** gains the `created_at` range and `order_by created_at desc`,
and **retires its own `scanCap`** (`store.go:710`) in favor of a server-side
`order_by` scroll bounded directly by `limit` — so it is bounded by `limit`, not
unbounded, and not by a fixed 1000 ceiling. It keeps limit-only paging (no cursor)
— management of a small windowed set.

**`ListScopes`** is **not** rebuilt: it is a cross-scope aggregation with no Qdrant
GROUP-BY equivalent. It benefits from the `owner` index for a faster readable-set
scroll but keeps its scroll-aggregate + `more` flag. Facet-API rework is a noted
follow-up, out of scope here.

### 3. Cursor design + the tie-break

Cursor paging orders by `created_at desc` (the existing datetime field — **no new
field, no migration**) and makes page boundaries **order-independent** by carrying
the set of ids already emitted at the boundary timestamp. It deliberately does
**not** depend on Qdrant returning a stable order within an equal-`created_at`
group — a property Qdrant does not contractually guarantee.

- **Token:** opaque
  `base64url(JSON{"c": "<rfc3339 created_at>", "seen": ["<uuid>", …]})`. `c` is the
  oldest `created_at` emitted so far; `seen` is every id already returned **at
  exactly `c`**. Opaqueness keeps the encoding free to change later.
- **Resume:** `order_by created_at desc, start_from = NewStartFromDatetime(c)`
  (Qdrant `start_from` is inclusive, so the `c` group is re-scanned from the top).
  **Critical fetch-size rule:** Qdrant's scroll returns *at most* `limit` records
  per call, so the resume MUST request **`limit + len(seen)`** records — otherwise,
  when a timestamp group is larger than `limit`, the re-scan returns only ids
  already in `seen`, the page yields zero new records, and traversal terminates
  early (this is precisely the `limit=1` failure the pin below guards against).
  After fetching `limit + len(seen)`, **drop every record whose id ∈ `seen`** and
  take the next `limit`. As the page emits further records at `c`, append their ids
  to `seen`; the moment an older timestamp is emitted, `c` advances and `seen`
  resets to just that new boundary's ids.
- **Cost note:** for a single timestamp group of cardinality `G` paged at `limit`,
  total work is `O(G²/limit)` because `seen` (and thus the per-page fetch) grows as
  the group is traversed. This is bounded and only bites pathological same-instant
  bulk imports; the nanosecond-precision mitigation below keeps `G` at 1 for new
  writes, so in normal operation `seen` stays empty and the resume fetches exactly
  `limit`.
- **Why it is order-independent:** dedup is by explicit id *membership*, not by
  position. Even if Qdrant returns the equal-`c` group in a different order on the
  resume query, the already-seen ids are dropped and the remainder taken — no
  record is duplicated or skipped. Identical timestamps (the June-27 migration
  imported several that may share one) are handled by construction.
- **Bound + its pin:** the costs are token size and resume fetch-size (above),
  both growing with `seen` while a single timestamp group is traversed and reset
  on advance. A test inserts N records sharing one timestamp plus M with distinct
  timestamps, pages the whole set at **`limit=1`** to exhaustion, and asserts the
  union equals the full set with **zero duplicates and zero skips**, regardless of
  intra-group order. (At `limit=1` this only passes if the resume honors the
  `limit + len(seen)` fetch rule — it is the direct guard for finding-1.)
- **Future ties (optional, additive):** new writes may stamp `created_at` at
  nanosecond precision (still RFC3339, still the same datetime index, no
  migration), so going-forward collisions become astronomically rare and `seen`
  stays near-empty; legacy second-precision records remain correct via the id-set.

### 4. Date-window semantics

- **`created_after`** → inclusive (`gte`); **`created_before`** → exclusive
  (`lt`). Half-open `[after, before)`. Contiguous day-windows tile without
  overlap; "prior to yesterday" = `created_before` yesterday-midnight.
- Either bound may be omitted (unbounded on that side).
- **Named distinctly from `not_before`/`not_after`** (the validity-window /
  recall-gate axis) on purpose: this filters *creation time*, a different axis.
  Naming collision here would be a correctness trap for callers.
- Input is absolute RFC3339; a malformed value is a loud invalid-arg error, never
  a silent empty result.

### 5. Wire (Connect) changes — all additive

- `ListMemoriesRequest`: `+ string created_after`, `+ string created_before`,
  `+ string page_token`. `offset`, `categories`, `visibility`, `limit` unchanged.
- `ListMemoriesResponse`: `+ string next_page_token`. `total` stays (now always
  exact); `approximate` is retained for wire compatibility but is **always
  `false`** for `List` and marked deprecated.
- `SearchMemoriesRequest`: `+ string created_after`, `+ string created_before`.
- All fields are additive → `buf breaking` stays green; the generated `gen/` tree
  is regenerated and committed.

### 6. MCP tool surface changes

- **`list_memory`** args: `+ created_after`, `+ created_before` (RFC3339 strings),
  `+ cursor` (opaque). Output reshapes from a **bare array** to
  **`{ "memories": [...], "next_cursor": "<token|empty>" }`**. This is the one
  non-additive surface change; it is documented in the memory contract. The MCP
  `cursor`/`next_cursor` and the wire `page_token`/`next_page_token` (§5) carry the
  **identical** `base64url(JSON{c, seen})` token — **one encoder/decoder** shared
  by both surfaces, not two codecs.
- **`search_memory`** / **`list_scheduled`** args: `+ created_after`,
  `+ created_before`. Output unchanged.
- An empty/absent `next_cursor` signals the last page.

### 7. Docs + skill updates (in scope)

- **`CLAUDE.md` memory contract** + **`docs-site` `reference/tools.md`**: document
  the window params, the cursor, and the new `list_memory` output shape.
- **engram skill** (`skill/engram/…`): the curating-memory / recall guidance and
  the session-start recall hook reference `list_memory`'s shape; update the
  recall examples to the wrapped `{memories, next_cursor}` form and mention the
  window params where an agent would window recall.
- **Code hygiene:** the comment at `internal/store/summarize.go:93` ("created_at
  …not a Qdrant-rangeable number") becomes false once the datetime index exists.
  Correct it — `created_at` *is* server-side rangeable post-index; `SummarizeMissing`
  may keep its in-code age filter (out of scope to rework), but the comment must
  no longer assert the field is unrangeable.

## Error handling

- Malformed RFC3339 `created_after`/`created_before` → invalid-arg (Connect
  `InvalidArgument`; MCP tool error), not a silent empty page.
- Malformed/undecodable `cursor`/`page_token` → invalid-arg, not silent reset to
  page 1.
- `Offset` + `Cursor` both supplied → invalid-arg at the store boundary.
- `created_after >= created_before` (empty window) → valid; returns an empty page
  with exact `total=0` (not an error — a legitimate query).
- Qdrant index-creation errors other than "already exists" → fail collection
  setup loudly; the server does not start with recall in a half-indexed state.

## Testing

- **Index idempotency:** `ensureCollection` run twice creates indexes once, no
  error on the second pass.
- **Migration realism (integration, testcontainers Qdrant):** records written as
  RFC3339 strings *before* the index exists become range-filterable and orderable
  *after* index creation — proves no re-stamping is needed.
- **Half-open boundaries:** a record at exactly `created_after` is included; one at
  exactly `created_before` is excluded.
- **Cursor traversal:** N records sharing one timestamp + M with distinct
  timestamps, paged at `limit=1` to exhaustion → the union equals the full set,
  with no duplicates and no skips (the tie-break pin).
- **Exact total:** `Count` under an `owner` + date-range filter equals the true
  matched count past the old `scanCap` (insert > 1000 to prove the ceiling is
  gone).
- **Isolation composition:** the window composes with the owner/visibility/active
  gate — caller B never sees caller A's windowed records; `shared` read grant
  still requires a non-empty owner; the window narrows, authz still gates.
- **Search composition:** `search_memory` with `created_after`/`created_before`
  composes with the tags hard pre-filter and the authz envelope.

## ADRs to capture (via `/capture-adrs`)

1. **Payload indexes as the recall foundation** — `owner`/`scope`/`created_at`
   indexed; `List` queries server-side; `scanCap`/`approximate` retired for
   `List`. Refines engram-lkm's `scanCap` rationale.
2. **Cursor pagination + creation-time window** — cursor default for the MCP
   recall path, offset retained for the operator-console UI; half-open
   `[created_after, created_before)` semantics; `created_at`-ordered cursor with a
   **boundary id-set** for order-independent dedup (no new field, no migration,
   no dependence on Qdrant intra-group order stability). Supersedes engram-lkm's
   "cursor deferred".

## Rollout

Pure index-add: no data migration, no operator action, no feature flag. On
deploy, `ensureCollection` creates the indexes (backfilling existing points);
the new request/tool fields are additive and default to today's behavior when
unset. A client that ignores `next_cursor` and sends no window behaves exactly as
before — except `list_memory`'s output is now the wrapped object (the one
documented break).
<!-- adr-capture: sha256=e1ef80f3ec73e55c; session=cli; ts=2026-06-30T00:50:38Z; adrs=engram-ef28,engram-1frj -->
