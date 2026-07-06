<!--
SPDX-License-Identifier: Apache-2.0
-->
# Design: `short_id` handle for engram memories

- **Bead:** `engram-c0yl`
- **Date:** 2026-07-06
- **Status:** Ready (design-reviewer READY, round 3 — 6/6 by-id call sites verified exhaustive)

## Problem

Memory records are identified by a UUIDv4 that is also the Qdrant point id.
The recall surfaces (`list_memory` / `search_memory`, and the session-start
recall-digest hook) show that id so callers can follow up with `get_memory`.
A 36-char UUID has no natural short form, so agents hand-truncate it to a
prefix (e.g. `96f342c3`) — and a prefix is **not a valid key**:
`get_memory("96f342c3")` fails at the Qdrant boundary with
`InvalidArgument: Unable to parse UUID`. The follow-up path the recall tools
advertise is therefore easy to break.

The fix is to give each record a short, valid, LLM-friendly handle that recall
output can carry and that every by-id tool accepts.

## Goals

- Every memory carries a short handle that round-trips through `get_memory`,
  `update_memory`, `delete_memory`, and `set_visibility`.
- Recall output (`list_memory` / `search_memory` / the digest hook) surfaces a
  handle that is **valid to paste back** — nothing to truncate.
- Handle is short and LLM-friendly: fixed length, case-insensitive, no
  ambiguous glyphs.
- Purely additive: the UUIDv4 stays the primary identity and the Qdrant point
  id; existing UUID-based lookups are unchanged.

## Non-goals

- No change to primary identity (no ULID/Sqids/numeric point ids). Grounded
  hard constraint: `qdrant.NewID` → `NewIDUUID` and every read via
  `p.Id.GetUuid()` (store.go:412, :1016, and the read path) means the point id
  **must be a UUID string**. A non-UUID id is server-rejected.
- No ordering/sortability semantics (explicitly not wanted; `created_at` already
  has a datetime index + `order_by` from PR #253).
- No reversibility / meaningful-integer encoding.

## Decision: encoding

`short_id` is a **10-character Crockford base32** token (~50 bits) over the
alphabet `0123456789ABCDEFGHJKMNPQRSTVWXYZ` (excludes `I L O U`), stored and
displayed **lowercase**.

Rationale (evaluated ULID and Sqids; both rejected for the *handle* role):

- **ULID** is dominated: 26 chars (barely shorter than a UUID — not a short
  handle); its only distinctive feature is a time-sortable prefix that is
  explicitly unwanted; de-ordered, its random suffix is just a base32 token.
- **Sqids** is a reversible encoder of *integers*; using it here would require
  minting a random integer per record and storing the encoding, at which point
  its reversibility is wasted. It reduces to "a base-N encoding of a random
  number" plus a dependency and integer plumbing.
- **Crockford base32** gives the same length (~8–10 chars) with
  case-insensitivity and an unambiguous alphabet (no `0/O`, `1/I/L`), which
  minimizes the two ways an LLM corrupts a short token (case normalization,
  glyph swaps). A fixed short length also removes the temptation to truncate —
  the original failure mode.

**Uniqueness:** global, enforced at write by check-and-retry (filter
`short_id == candidate`; regenerate on hit). 50 bits makes collisions
negligible at engram scale even before the check; the check is a cheap safety
belt (one small Qdrant round-trip, dwarfed by the embed call the write already
performs). A residual multi-match (astronomically unlikely) is caught by the
resolver as an explicit error, never a silent pick.

**Input normalization on lookup:** trim surrounding whitespace, then
Crockford-canonicalize to the stored form — map `i/I/l/L`→`1`, `o/O`→`0`, and
case-fold to **lowercase** (matching storage) — before the exact filter, so a
handle typed in any case or with confusable glyphs still resolves.

## Architecture

### 1. Data model — `internal/store/store.go`

- `Memory` (store.go:79): add `ShortID string \`json:"short_id,omitempty"\``.
- `payload()` (store.go:254): add `p["short_id"] = m.ShortID`.
- `fromPayload()` (store.go:302): read `short_id` back into `m.ShortID`.
- `ensureIndexes()` (store.go:224): add
  `{"short_id", qdrant.FieldType_FieldTypeKeyword, nil}` to the index list. The
  existing `AlreadyExists`-tolerant loop makes this idempotent — **no schema
  migration** (same mechanism PR #253 used for owner/scope/created_at).

### 2. Generation — write path (`internal/server/tools.go`, currently `uuid.NewString()` at :483)

Alongside the UUID, mint a unique `short_id`. **Contract:** generation draws a
10-char Crockford base32 token (50 random bits, lowercase), is collision-checked
against a global `short_id == candidate` filter (an exact-filter `Count`,
mirroring the existing `CountOwnerless` precedent at store.go:1362) and retried
on any hit, and sets `m.ShortID` before `Upsert`.

The generator lives in `internal/store` (or a small `internal/shortid` helper)
so the write path, `schedule_memory`, `store_discovery`, and the backfill
command share **one** implementation and **one** uniqueness discipline (see §6).
Discovery records (category `discovery`) are the same `Memory` type and get a
`short_id` too.

### 3. Resolution — one store method, called by every by-id handler

Add one store method: `ResolvePointID(ctx, idOrShort) (uuid string, err error)`:

1. `s := strings.TrimSpace(idOrShort)` — **trim first, before the UUID check.**
   `google/uuid`'s `Parse` is length-strict and rejects surrounding whitespace,
   so a padded *valid* UUID would otherwise fall through to the short_id branch
   and 404. Trimming precedes both branches.
2. `uuid.Parse(s)` succeeds → return `s` (fast path).
3. else Crockford-canonicalize `s` (map `i/I/l/L`→`1`, `o/O`→`0`, lowercase) and
   exact-filter `short_id == canonical`:
   - exactly 1 match → return its UUID
   - 0 matches → `ErrNotFound`
   - >1 match → `ErrAmbiguousShortID` (invariant violation; never expected)

Resolution is **owner-agnostic** — it maps a handle to a point id and applies no
authz. Each by-id **handler** calls `ResolvePointID` first, then passes the
resolved UUID into the existing (UUID-only) store methods, whose ownership gates
are unchanged. Keeping resolution in the handler layer (not inside each store
mutator) is deliberate: it means no store method needs a signature change to
return the resolved id. The trade-off this makes explicit: resolution is **not**
inherited by calling a resolved store method — the store methods stay UUID-only —
so **every independent call site must invoke `ResolvePointID` itself**. Those
call sites span both the MCP tool layer and the Connect read API:

| Call site | Layer | Downstream store gate |
|-----------|-------|-----------------------|
| `get_memory` | MCP tool | `GetReadable` (store.go:1035) |
| `update_memory` | MCP tool | `FetchForUpdate` |
| `delete_memory` | MCP tool | `Delete` |
| `set_visibility` | MCP tool | `SetVisibility` (store.go:1211) |
| `store_discovery` (replace branch) | MCP tool | `OwnedOrAbsent` (store.go:1103) |
| `GetMemory` | Connect RPC | `GetReadable` (connectapi.go:173) |

The Connect `EngramService` is **read-only** (`ListScopes`, `ListMemories`,
`SearchMemories`, `GetMemory`, `SearchDiscoveries`), so `GetMemory` is its only
by-id *input* surface — the list/search RPCs emit `short_id` in results (§5) but
take no id argument. This reconciles §5's "Connect `GetMemory` accepts either
form" with the enumeration here.

The `store_discovery` replace branch (tools.go:552–570) is the subtle one: it
must resolve `a.ID` to the canonical UUID **before**
both the `OwnedOrAbsent` check and the `id := <resolved>` / `m.ID = id`
assignment, so the subsequent `Upsert` overwrites the *existing* point instead
of minting a new point keyed on the raw short_id string (which would reproduce
the exact "Unable to parse UUID" failure this design eliminates). Because the
handler resolves first, `OwnedOrAbsent` keeps its `error`-only signature.

### 4. Authz invariant (ADR engram-xa6)

Resolution must preserve *"return 404 not-found for unauthorized id-addressed
operations."* A `short_id` that resolves to another owner's **private** record
must surface as **not-found (404)**, identical to guessing that record's UUID —
never 403, never a distinguishable "exists but forbidden." A `short_id` for
another owner's **shared** record is readable, matching UUID behavior. Because
each handler resolves owner-agnostically and then calls the **unchanged**
ownership gate (`GetReadable` / `OwnedOrAbsent` / etc.), a short id leaks nothing
in its *response* that a UUID guess would not.

One structural caveat, called out as out of scope so it is not mistaken for an
oversight: resolution adds a *timing* asymmetry that raw-UUID addressing lacks.
A nonexistent short_id costs one Qdrant round trip (filter → 0 rows); a short_id
resolving to another owner's private record costs two (filter → `Get`/gate).
Response *content* is identical (both 404), preserving engram-xa6; only latency
differs. Constant-time id resolution is not a goal for this store.

### 5. Recall output & wire — the part that fixes the digest bug

- `recallView` (`internal/server/summary.go:40`, built by `toRecallView` at
  :88): add `ShortID`, so `list_memory` and `search_memory` emit a valid short
  handle in every result.
- `store_memory` / `schedule_memory` / `store_discovery` results: return
  `short_id` alongside `id` so the caller has the handle immediately.
- Connect proto `Memory` (`proto/engram/v1`, fields `id=1`..`score=17`): add
  `string short_id = 18;` (additive); regenerate the committed `gen/` tree.
  Connect `GetMemory` accepts either form via the same resolver.

### 6. Backfill — `engram backfill-short-ids` (new operator command)

Mirrors `migrate-remap-owner`: scroll all points; for any lacking `short_id`,
generate a unique one and write it with **`SetPayload` only** (no re-embed —
single-key payload write, exactly the pattern `SetVisibility` already uses at
store.go:1211). Vectors and the rest of the payload stay verbatim; in
particular, `SetPayload` touching only `short_id` **preserves the
absent-owner-key invariant** — pre-isolation records must not gain an `owner`
key. Dry-run first, like the existing migrate/reindex commands.

**Uniqueness:** backfill uses the **same global `short_id == candidate`
check-and-retry as the write path (§2)**, *plus* an in-run assigned-set so ids
minted earlier in the same run — not yet count-visible — are also avoided. The
global check is what keeps a *resumed* or a *concurrent* backfill (racing live
`store_memory` traffic) from reusing an id committed outside the current run;
the in-run set alone would not.

### 7. Documentation & skill updates

Additive edits everywhere the id is described:

- **MCP tool descriptions** (`internal/server/tools.go`): `get_memory`,
  `update_memory`, `delete_memory`, `set_visibility` — note the id arg accepts
  a full UUID **or** a short_id; `store`/`schedule`/`store_discovery` mention
  the returned `short_id`.
- **Skills** (`skill/engram/`): `hooks/session-start-memory-recall` (surface the
  short_id in the digest guidance and state the full id/short_id copy rule — no
  truncated prefixes); `skills/curating-memory` (fetch-by-id accepts short_id);
  `skills/discovering` (replace-by-id accepts short_id); `skills/migrating-from-beads`
  and `skills/promoting-memory` (id-reporting mentions short_id where relevant).
- **Docs-site** (`docs-site/src/content/docs/`): `reference/tools.md`
  (id-argument rows → "UUID or short_id"; add `short_id` to returns);
  `reference/memory-record.md` (add a `short_id` field row); `guides/upgrade.md`
  (note the new field + the `backfill-short-ids` migration).

## Boundary / edge test matrix

**Resolution / dispatch**

1. Valid full UUID that exists → fast path returns the record.
2. Well-formed UUID that does not exist → 404 not-found (engram-xa6).
3. Valid short_id, owned → returns the record.
4. Valid short_id for another owner's **private** record → **404 not-found**
   (no existence leak) — engram-xa6.
5. Valid short_id for another owner's **shared** record → readable.
6. Nonexistent short_id → 404 not-found.
7. Empty string → `InvalidArgument`.
8. Whitespace-padded id/short_id → trimmed **before** the UUID check; a padded
   *valid* UUID resolves via the fast path (not a short_id miss), a padded
   short_id via the short_id branch.
9. Case-insensitivity: short_id stored lowercase, looked up UPPERCASE → resolves.
10. Crockford glyph aliases: lookup with `O`/`I`/`L` where canonical has
    `0`/`1` → normalized, resolves.
11. Neither UUID nor known short_id (incl. an 8-char UUID prefix like the
    original `96f342c3`) → treated as short_id miss → 404 (a clean error, not
    the raw Qdrant "Unable to parse UUID").
12. Forced duplicate short_id → `ErrAmbiguousShortID`, never a silent pick.

**Generation / uniqueness**

13. Generated short_id uses only the Crockford alphabet, length 10, lowercase.
14. Collision on first candidate (stubbed store) → retry yields a different id.
15. Two sequential stores never collide; document residual TOCTOU risk +
    ambiguity safety net.

**Persistence / mapping**

16. Round-trip: store → `fromPayload` carries `short_id`.
17. `update_memory` (content change) preserves the existing `short_id` (stable;
    never regenerated or cleared).
18. `short_id` present in `recallView` from both `list_memory` and
    `search_memory`.
19. `short_id` round-trips on the Connect `Memory` proto.

**Backfill**

20. Record missing `short_id` → gets one via `SetPayload`.
21. Record already carrying `short_id` → skipped (idempotent).
22. Vectors unchanged after backfill (no re-embed) — asserted.
23. Absent-owner-key invariant preserved — backfill never adds an `owner` key.
24. Batch assigns globally-unique short_ids (global check-and-retry + in-run
    set); a second/resumed run reuses no id committed by the first.
25. Paging: >1000 points backfilled via scroll.
26. Dry-run writes nothing.

**Index**

27. `ensureIndexes` idempotent (`AlreadyExists`-tolerant) — second call is a
    no-op.

**By-short_id operations**

28. `update_memory` / `delete_memory` / `set_visibility` addressed by short_id →
    resolve, then operate on the correct point (each handler resolves before its
    store gate).
29. `store_discovery` replace addressed by short_id → overwrites the **existing**
    point (point count unchanged); never mints a new point keyed on the raw
    short_id string.
30. Padded *valid* UUID → resolves via the fast path (guards the
    trim-before-`Parse` ordering from finding C2).
31. Connect `GetMemory` RPC by short_id → resolves, parity with MCP
    `get_memory` (guards the sixth, Connect-layer call site).

## Rollout

1. Ship the additive field + index + generation + resolution + recall/wire
   surfacing + tool/skill/docs updates in one change (all backward-compatible;
   old records simply have an empty `short_id` until backfilled).
2. Run `engram backfill-short-ids` (dry-run, then apply) against prod.
3. The session-start recall-digest hook update lands with the skill changes so
   the digest starts surfacing valid short handles.

## Resolved decisions

These were open during brainstorming; now locked:

- **Length: 10 chars / 50 bits.** Short enough to be LLM-friendly, wide enough
  that collisions are negligible at engram scale even before the write-time
  uniqueness check.
- **`short_id` is stable — never rotated.** `update_memory` (content change or
  otherwise) preserves it; it is generated once at store time and only ever
  (re)assigned by the backfill command when absent.
- **Uniqueness scope: global.** One `short_id` index, resolver returns 0/1,
  no owner term in the resolution filter (authz stays in the downstream
  ownership gate).
<!-- adr-capture: sha256=0897414850b0dcc0; session=523506b4; ts=2026-07-06T18:20:21Z; adrs=engram-zzq0,engram-02ta -->
