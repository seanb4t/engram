# Phase 4: List, ListScheduled & Search Bounded Reads - Research

**Researched:** 2026-09-19
**Domain:** Go + Qdrant memory server — wiring `Store.List`/`ListScheduled`/`Search`/`SearchDiscovery`
onto Phase 3's shared bounded-read primitives, plus the D-01..D-12 contract changes this requires
**Confidence:** HIGH (every code claim below was read from the working tree or the pinned
`go-client@v1.19.2` module cache this session; the one claim that could not be verified live is
flagged `[ASSUMED]` in the Assumptions Log)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-00 (user preference `1w3h5sy56m`):** choose by idiom and long-term maintenance, never by effort.
- Phase 1: `store.NewQdrantClient` is the one constructor; tests pin `storetest.RecvLimit` (4 MiB);
  `storetest.SeedOversized` provides the many-small and few-large fixtures.
- Phase 2: an over-limit response surfaces as `store.ErrResponseTooLarge` → `resource_exhausted` /
  exit 10 (its hint code is renamed by D-11 below).
- Phase 3: D-02 small RPCs + running byte total; D-03 the two primitives; D-04 project payloads by
  view (`summaryView` excludes `content` and `citations`); D-05 two-phase ids→payload NOT adopted
  for List; D-06 budgets are fixed 2 MiB seams; D-07 batch-of-1 fallback → named error, never a
  silent skip; `maxListLimit = 1000` already caps a cursor page.
- ADR `engram-1frj` (LOCKED): boundary id-set cursor for recall paging, offset-for-UI kept. This
  phase does not touch it.

**Decision B — Connect `ListMemories` limit contract:**

- **D-01:** `limit: 0` resolves to the page maximum, **1000** (the existing `maxListLimit`) — on
  Connect `ListMemories`, `engram list --limit 0`, the console, and at the store
  (`ListOptions.Limit == 0` resolves to `maxListLimit` in offset mode). The contract is stated
  numerically everywhere; "all" is removed from `store.go:1461` "limit 0 = all", `connectapi.go:274`
  "0 = all", `tools.md:195-196` "an unset limit means all". `total` stays exact. MCP `list_memory`'s
  documented `0 → 20` default is unchanged. Offset-for-UI stays.
- **D-02:** One documented maximum, 1000, for every recall count knob: `limit` (Connect
  `ListMemories`, MCP `list_memory`/`list_scheduled`) and `k` (Connect `SearchMemories`/
  `SearchDiscoveries`, MCP `search_memory`/`search_discovery`, `engram search --k`). One
  store-level constant (renamed/aliased at Claude's discretion so the name fits list AND search).
- **D-03:** `list_rules` inherits the same ceiling: "the complete rule set, up to 1000 rules per
  scope" (tool description, `rules.go:206` comment, `tools.md`, CLAUDE.md).
- **D-04:** The `0 → 1000` change is BREAKING, documented in `guides/upgrade.md` (precedent: the
  `--timeout` zero-semantics entry), naming Connect `ListMemories` and `engram list`.

**Cursorless byte-cut pages:**

- **D-05:** Offset-mode `Store.List` and `Store.ListScheduled` assemble the FULL requested count:
  iterate `scrollOrderedPage` pages (following `Next`) until `limit` records are in hand or
  `Exhausted`. Every RPC stays under `rpcByteBudget`; response bounded by count (≤1000 × the
  view's per-record ceiling). Revises Phase 3 D-06: `pageByteBudget` bounds cursor-mode responses
  ONLY — state this explicitly in `boundedread.go`'s doc comment.
- **D-06:** Cursor mode honors `pageByteBudget`: a `CutByBudget` page returns a non-empty
  `next_cursor` and is never the last page; `next_cursor` empty only when `Exhausted`.
- **D-07:** No offset ceiling. Deep offset walks the skipped prefix with a keys-only budgeted view
  (ids + `created_at`, tens of thousands per RPC), then fetches the page in the caller's view by
  keyset resume from the prefix boundary. The handoff into `scrollOrderedPage`'s `Seen`/boundary is
  Claude's discretion.

**Search `k`:**

- **D-08:** `k == 0` keeps today's per-surface defaults (Connect 20; MCP `search_memory` 8;
  `search_discovery` 8). `k > 1000` rejected (D-10).
- **D-09:** Two-phase search. `Query` requests no payload (ids + scores, one small RPC at any
  `k ≤ 1000`); payloads are then fetched through byte-budgeted `Scroll`s filtered by `has_id(batch)`
  AND the identical authz/recall filter the Query used, in the caller's view (`summaryView` default,
  `fullView` on `full=true`), re-ordered by the held scores. A record absent from the fetch drops
  out of the hits. Phase 3 D-05 stands for List; two-phase is adopted for Search ONLY because the
  order is held by the caller and the filter is re-applied (TOCTOU/`GetPoints`-order risks that
  ruled it out for List do not apply). Applies to `Store.Search` (hence `SearchReranked`) and
  `Store.SearchDiscovery`; the new fetch helper joins `recallTransmitters`.

**Over-maximum rejection:**

- **D-10:** A `limit`/`k` above 1000 is REJECTED, never clamped, with new hint `out_of_range` —
  `field=limit hint=out_of_range` / `field=k hint=out_of_range`, `classMalformed` →
  `invalid_argument` → CLI exit 2. Added to `HintCode` (`argerror.go`), `errors.md`, and hint
  conformance/attribution tests. Applies at the server boundary on every D-02 surface; CLI does not
  duplicate the check. `maxListLimit` (today a silent cursor-page cap) becomes a rejection too.
- **D-11:** Phase 2's `too_large` hint is renamed `response_too_large` (`HintTooLarge` →
  `HintResponseTooLarge`; envelope `field=response hint=response_too_large`; Connect
  `resource_exhausted`/exit 10 unchanged). Update in one change:
  `internal/server/{argerror,connecterror,responsetoolarge}.go` + tests,
  `cmd/engram/exitcode_baseline_test.go`, `docs-site/.../reference/errors.md`, `guides/cli.md`
  (exit 10 row), `guides/upgrade.md:376`, `internal/store/redevidence_harness_test.go`'s Phase 2
  mapping + the patch `02-03-errors-doc-drops-too-large.patch` (regenerate), Phase 2's VERIFICATION
  covered files (`## Re-fingerprint`). `02-CONTEXT.md` is historical, not edited.

**Phase 2 regression retargets:**

- **D-12:** Phase 2's overflow regressions relying on today's unbounded `Store.List`
  (`responsetoolarge_test.go:166`, Connect/MCP siblings, red-evidence patches) are retargeted to a
  shape that still overflows after the fix — the D-07 single-record-over-limit fixture through a
  recall entry point, or an explicitly unbudgeted raw path in a `package store` test. Record each
  retarget in the plan's SUMMARY deviations.

### Claude's Discretion

- The keys-only `readView` for the prefix walk (selector + ceiling) and the boundary handoff into
  `scrollOrderedPage`; whether offset mode and `ListScheduled` share one assembly loop.
- Where in `deps.*`/`connectapi.go` the maximum check sits relative to existing scope/window checks,
  and the exact `argErrf` detail text.
- The rename/alias of `maxListLimit`.
- Test design per site (both fixture shapes at `storetest.RecvLimit`); the
  total/next_cursor/ordering/recall-gate invariance tests; the D-06 cut-page test at a shrunken page
  budget; the new `out_of_range` rejections on every surface.
- Recall-gate reclassification: `Store.scrollOrderedPage` → `recallTransmitters`, the new search
  fetch helper and the keys-only walk added with justifications.
- Phase 3 D-04's no-summary truncation fallback under projection: keep today's visible behavior
  (fetch content only for records whose summary is empty, in a second budgeted has_id fetch) — the
  mechanics are Claude's, the visible behavior is not to change.
- Phase 4 red-evidence patches (one per RED direction; registered in `redEvidenceDirs`).

### Deferred Ideas (OUT OF SCOPE)

- Adding `next_offset`/a truncation flag to offset-mode responses (not adopted — D-05 assembles the
  full count instead).
- Moving the console and `engram list` to cursor-only paging (would need an ADR superseding
  `engram-1frj`; not adopted).
- Capping the still-uncapped payload fields (GitHub #589).
- `listByCursor` tie-safety under concurrent inserts at one `created_at` (REQ-cursor-tie-safety, v2).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-list-bounded | `Store.List` succeeds or fails named, in every mode (offset `limit:0`, deep offset, cursor), reachable via MCP/Connect/console/CLI | "Wiring `Store.List` onto `scrollOrderedPage`" pattern below; D-07 keys-only walk design; retarget of the `Limit:0` short-circuit at `store.go:1449-1461` |
| REQ-list-scheduled-bounded | `list_scheduled` with a large explicit limit stays under the receive limit | `ListScheduled` already orders by `created_at` desc (verified) — direct fit for `scrollOrderedPage` looped per D-05 |
| REQ-search-k-bounded | `search_memory`/`search_discovery` coerce `k` to the documented max and stay bounded | Two-phase search design; the `candidateK` ceiling finding (SearchReranked never fetches >100 candidates regardless of `k`) |
| REQ-list-contract-unchanged | `total`/`next_cursor`/ordering/recall gating independent of batching; a byte-cut page never reported last | `orderedPage.{Exhausted,CutByBudget}` already encodes this contract (Phase 3); `recallInvocationRows`' exact-count assertions are the regression risk this phase must not trip |
| REQ-list-limit-contract-decided | Decision B recorded | D-01 already settles it in 04-CONTEXT.md; this phase implements + documents it |
</phase_requirements>

## Summary

Phase 3 already built and proved both shared primitives this phase needs
(`(*Store).scrollOrderedPage` in `orderedpage.go`, and `boundedread.go`'s `readView`/`fullView`/
`summaryView`/`perRPCLimit`). Phase 4's job is almost entirely **wiring and threading**, not new
mechanism: point `Store.List`'s offset and cursor paths, `Store.ListScheduled`, and a new
ids→payload fetch helper for `Store.Search`/`SearchDiscovery` at these primitives, then propagate a
`Full`-equivalent view selector down from the MCP/Connect boundary (which today decides
full-vs-summary only AFTER a full-payload fetch, in `shapeRecall`/`shapeProtoMemories`) into the new
`ListOptions`/`SearchOptions` fields the store methods will need. The two most consequential,
non-obvious findings from reading the code directly (not from CONTEXT.md, which does not spell
either out mechanically) are:

1. **`coreListRequest`/`coreSearchRequest` carry no `Full` field today** — `a.Full`/`req.Msg.Full`
   is applied only at the presentation layer (`shapeRecall`, `shapeProtoMemories`), strictly after
   the store already fetched a full payload. Realizing Phase 3 D-04's per-view sizing (2 vs 61
   records/RPC at default caps) REQUIRES adding `Full` to the typed core AND to `store.ListOptions`/
   `SearchOptions`, and wiring it at **three** call sites, not two: `deps.listMemory`,
   `deps.searchMemory`/`searchDiscovery`, AND `internal/server/rules.go`'s direct
   `d.st.List(...)` call (`list_rules` bypasses the typed core entirely and would silently regress
   to always-summary content once `Store.List` defaults to `summaryView()`).
2. **`SearchReranked`'s candidate pool never exceeds 100 records regardless of the caller's `k`**
   (`candidateK(k)` clamps at 100; `search/`SearchMemories always route through `SearchReranked`).
   The reranker's `lexicalOverlap` needs `Content` for every candidate, so the fetch phase for
   `search_memory`/`SearchMemories` must ALWAYS use `fullView()` for its (≤100-record) candidate
   set — the caller's `full` flag governs only the final response shaping, never the fetch view, for
   this one surface. `SearchDiscovery` has no reranker and no candidate over-fetch, so its
   fetch view legitimately follows the caller's `full` flag directly (`summaryView()`/`fullView()`).

The recall-gate AST test (`schemaversion_recallgate_test.go`) is both the sharpest guardrail and the
sharpest landmine in this phase: its `recallInvocationRows` table hardcodes an EXACT gRPC-method
multiset per row (e.g. `Search/anonymous: expectCount 1, ["Query"]`). Because that fixture seeds
ZERO records, a correctly-implemented two-phase search that skips the fetch RPC when the Query phase
returns no ids will leave those rows' expectations untouched — but this must be a deliberate
design property of the new fetch helper (skip the Scroll entirely on an empty id batch), not an
accident discovered by a failing test.

**Primary recommendation:** wire `Store.List`'s cursor mode as a thin adapter over
`scrollOrderedPage` (its `Next`/`Exhausted`/`CutByBudget` fields already match the cursor contract
almost exactly); wire offset mode and `ListScheduled` as a loop over `scrollOrderedPage` pages per
D-05; build the two-phase search fetch as a NEW helper (not a `scrollOrderedPage` reuse — it has no
ordering/keyset concern, only has_id batching) sized from `perRPCLimit(view)` per batch; thread
`Full` through `coreListRequest`/`coreSearchRequest`/`ListOptions`/`SearchOptions` and `rules.go`'s
direct call; and reclassify `Store.scrollOrderedPage` plus the new fetch helper into
`recallTransmitters` in the SAME commit that wires them in (the gate goes RED the instant they
become reachable — `.planning/research/ARCHITECTURE.md:95`).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Ordered-page wiring (List/ListScheduled) | API/Backend (`internal/store`) | — | Store-tier Qdrant paging mechanics; no handler touches this |
| Two-phase search fetch helper | API/Backend (`internal/store`) | — | Same tier as `Store.Search`/`SearchDiscovery`, which it's called from |
| Deep-offset keys-only walk | API/Backend (`internal/store`) | — | A `readView` variant, same tier as `boundedread.go` |
| `Full` threading (coreListRequest/coreSearchRequest → ListOptions/SearchOptions) | API/Backend (`internal/server` typed core) | API/Backend (`internal/store`) | The core decides which view to request; the store executes it |
| `out_of_range` maximum check | API/Backend (`internal/server` deps.\* + Connect handlers) | API/Backend (`internal/store`, backstop) | Server boundary is the single source of truth per D-10; store keeps a backstop |
| `list_rules` Full threading | API/Backend (`internal/server/rules.go`) | — | Bypasses the typed core; must be updated alongside it or it silently regresses |
| Recall-gate reclassification | API/Backend (`internal/store`, test-only) | — | AST-derived, store-package-scoped |
| Docs/proto comment updates | CDN/Static (docs-site) + API/Backend (proto comments) | — | Static-built pages; proto comments are source, not runtime |
| Console (offset pager) | Frontend Server/Browser | — | No code change — small page sizes already fit under any per-view ceiling |

## Package Legitimacy Audit

**No external packages are installed by this phase.** Every capability used
(`github.com/qdrant/go-client/qdrant`'s `NewHasID`/`NewFilterAsCondition`/`NewWithPayload*`,
`google.golang.org/protobuf/proto`) is already vendored and already used by Phase 3's primitives
`[VERIFIED: go.mod:47 "github.com/qdrant/go-client v1.19.2"; internal/store/orderedpage.go:33-42]`.
The Package Legitimacy Gate protocol is not applicable — table omitted per "no new packages"
convention (matches Phase 3's own audit).

## Standard Stack

No new libraries. This phase composes Phase 3's primitives and existing vendored qdrant-client
symbols:

| Capability | Symbol | Already used at | Confidence |
|---|---|---|---|
| Ordered per-view paging | `(*Store).scrollOrderedPage` | `internal/store/orderedpage.go:118` (built, not yet wired) | `[VERIFIED: internal/store/orderedpage.go]` |
| Per-view byte ceiling | `fullView()`/`summaryView()`/`perRPCLimit` | `internal/store/boundedread.go:214-250` | `[VERIFIED: internal/store/boundedread.go]` |
| Id-set inclusion filter | `qdrant.NewHasID(...)` inside a `Must` (the two-phase fetch's filter) | Not yet used this way; `NewHasID` itself already used in `excludeSeen` (`MustNot`, orderedpage.go:76-85) | `[VERIFIED: /Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.19.2/qdrant/conditions.go:224-232]` |
| No-payload query | `qdrant.NewWithPayload(false)` on `QueryPoints` | Not yet used on `Query`; used today for `Scroll` at `store.go:1798` (`ResolvePointID`) | `[VERIFIED: /Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.19.2/qdrant/oneof_factory.go:425-434]` |
| Keyset resume | `qdrant.NewStartFromDatetime` | `store.go:1516,1533`; `orderedpage.go` | `[VERIFIED: internal/store/orderedpage.go:117]` |
| Exact wire-size measurement | `proto.Size(p)` | `orderedpage.go:190` | `[VERIFIED: internal/store/orderedpage.go]` |

**Installation:** none — no `go.mod` change.

**Version verification:** `go.mod:47` pins `github.com/qdrant/go-client v1.19.2`; the module cache
at `/Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.19.2` is the exact version read this
session for every proto/struct citation below.

## Architecture Patterns

### System Architecture Diagram

```
 MCP list_memory/search_memory/list_scheduled/search_discovery      Connect ListMemories/
        (tools.go closures)                                         SearchMemories/
              │                                                     SearchDiscoveries
              │  a.Full ──────────────┐                                  │
              ▼                       │                                  │  req.Msg.Full ──┐
   coreListRequest{..., Full}  ◄──────┘                        coreListRequest{..., Full} ◄─┘
   coreSearchRequest{..., Full}                                coreSearchRequest{..., Full}
              │                                                          │
              └───────────────────────────┬──────────────────────────────┘
                                           ▼
                    deps.listMemory / .listScheduled / .searchMemory / .searchDiscovery
                    (out_of_range check on limit/k HERE, before scope resolution/embed — D-10)
                                           │
                                           ▼
              store.ListOptions{..., Full} / store.SearchOptions{..., Full}
                                           │
              ┌────────────────────────────┼─────────────────────────────────┐
              ▼                            ▼                                 ▼
      Store.List (offset+cursor)   Store.ListScheduled              Store.Search/SearchDiscovery
              │                            │                                 │
      cursor mode: 1 scrollOrderedPage     loop scrollOrderedPage    Query (WithPayload(false),
      call, Next/Exhausted → cursor        pages until limit or      Limit=k) → ids+scores
      offset mode: loop scrollOrderedPage  Exhausted (D-05)                  │
      pages (D-05); deep offset walks             │                  NEW fetch helper: has_id(batch)
      prefix with keysView first (D-07)           │                  AND identical filter, batched at
              │                                   │                  perRPCLimit(view), in caller's
              ▼                                   ▼                  view (summaryView unless full=true;
      s.client.Scroll (small Limit,        s.client.Scroll           SearchReranked ALWAYS fullView —
      per-view WithPayload selector,       (per-view selector,       100-candidate cap needs Content)
      OrderBy created_at + StartFrom)      OrderBy created_at)               │
                                                                               ▼
                                                                    re-sort by held scores,
                                                                    re-attach Score (memoriesFromPoints
                                                                    precedent), drop missing ids
```

### Recommended Project Structure

No new files are strictly required; the natural placement mirrors Phase 3's own choices:

```
internal/store/
├── store.go              # List/listByCursor/ListScheduled/Search/SearchReranked/
│                         #   SearchDiscovery bodies rewired onto the primitives below
├── orderedpage.go         # scrollOrderedPage (Phase 3, unchanged) — List/ListScheduled compose it
├── boundedread.go         # fullView/summaryView/perRPCLimit (Phase 3) + a NEW keysView (D-07)
├── searchfetch.go         # (new, optional) the two-phase ids→payload fetch helper (D-09) —
│                         #   or colocated in store.go beside Search; Claude's discretion
└── schemaversion_recallgate_test.go  # recallTransmitters/otherNonRecallEmitters updated in the
                          #   SAME commit that wires scrollOrderedPage + the new fetch helper in
```

### Pattern 1: `Store.List` cursor mode as a thin `scrollOrderedPage` adapter

**What:** `listByCursor` (`store.go:1493-1579`) and `scrollOrderedPage`'s `orderedPage` type
(`orderedpage.go:48-70`) already carry near-identical contracts: both use `created_at` `StartFrom`
+ a boundary-`seen` id set, and `orderedPage.Next` is exactly `store.go`'s `listCursor{C, Seen}`
shape reused verbatim (`orderedpage.go` imports no new cursor type — it returns the SAME
`listCursor` `cursor.go` already defines). The cursor-mode rewrite is therefore mechanical: decode
the incoming `opts.Cursor` into a `listCursor` (`decodeCursor`, unchanged), call
`s.scrollOrderedPage(ctx, f, view, dir, from, limit)` ONCE, and map the result:
`page.Exhausted` → `nextCursor = ""`; otherwise `nextCursor = encodeCursor(page.Next)` REGARDLESS
of whether the page stopped by count or by `CutByBudget` — D-06 requires a `CutByBudget` page to
carry a non-empty `next_cursor`, and `orderedPage.Next` is populated whenever `len(Items) > 0`
(`orderedpage.go:214-217`), so this mapping is correct by construction, not by a new branch.
**When to use:** `Store.List`'s cursor-mode path (`store.go:1451-1453` today calls
`s.listByCursor`).
**Confirmed via source read:** `orderedpage.go:48-70`'s doc comment states the contract explicitly:
"`Next` is always the resume position... `Exhausted` and `CutByBudget` are never both true."
`[VERIFIED: internal/store/orderedpage.go:14-20 (doc comment), 48-70 (struct fields)]`.

### Pattern 2: Offset mode and `ListScheduled` as a loop over `scrollOrderedPage` (D-05)

**What:** D-05 requires the FULL requested count be assembled by iterating `scrollOrderedPage`
pages. Both `Store.List`'s offset path and `Store.ListScheduled` already order by `created_at desc`
with no `StartFrom` today (`store.go:1467-1474` for List; `store.go:1677-1682` for ListScheduled) —
confirmed by reading both bodies verbatim this session. The loop shape for both:

```go
// Illustrative — exact identifiers are Claude's discretion.
var items []Memory
from := listCursor{} // zero value: first page
for uint64(len(items)) < want {
    page, err := s.scrollOrderedPage(ctx, f, view, dir, from, want-uint64(len(items)))
    if err != nil { return nil, err }
    items = append(items, page.Items...)
    from = page.Next
    if page.Exhausted { break }
    // CutByBudget: loop again immediately — the CALLER never sees an
    // internal budget cut in offset/ListScheduled mode (D-05 revises D-06:
    // pageByteBudget bounds cursor-mode responses ONLY).
}
```

For `Store.List`'s offset mode, `want = opts.Offset + opts.Limit` when `opts.Limit > 0`, else
(today's `limit:0` "all" case) `want = total` (now capped at 1000 by D-01 — `total` itself stays
the exact `Count`, but the FETCH is bounded to `min(total, maxListLimit)` per D-01/D-10). The final
client-side `all[opts.Offset:]` slice (`store.go:1483-1486`) is unchanged.
For `ListScheduled`, `want = opts.Limit` (defaulted to 20 upstream); no offset slicing.
**When to use:** `Store.List`'s offset branch, `Store.ListScheduled`.
**Ordering confirmation:** `[VERIFIED: internal/store/store.go:1600-1684 — ListScheduled's Scroll
call reads OrderBy: &qdrant.OrderBy{Key: "created_at", Direction: qdrant.PtrOf(qdrant.Direction_Desc)}
with no StartFrom, i.e. today's ListScheduled already orders identically to List's default
direction, confirming it is a direct `scrollOrderedPage` fit with no ordering change]`.

### Pattern 3: Deep-offset keys-only walk (D-07)

**What:** A new `keysView` (or similarly named) `readView` for walking the skipped `opts.Offset`
prefix cheaply. Two implementation options, both grounded in the pinned client:

- **Option A (recommended, verified-safe):** `qdrant.NewWithPayloadInclude("created_at")` — a
  small, deterministic per-record payload (id ~36 bytes UUID + one `created_at` RFC3339 string
  ~20-25 bytes + protobuf map/point framing ≈ 100-200 bytes/record total). Reuses the EXACT same
  `fromPayload`-based boundary-tracking code path `scrollOrderedPage` already uses (parse
  `m.CreatedAt`, format to RFC3339, feed `NewStartFromDatetime`) — no new decode logic.
  `[VERIFIED: internal/store/store.go:717-757 (fromPayload parses "created_at" from the payload
  map); internal/store/orderedpage.go:188-197 (boundary tracking via m.CreatedAt)]`.
- **Option B (smaller wire size, unverified this session):** `qdrant.NewWithPayload(false)` (no
  payload at all) and read the boundary timestamp from `RetrievedPoint.OrderValue` instead — the
  response ALWAYS carries this field when the request set `OrderBy`, independent of the payload
  selector (`RetrievedPoint{Id, Payload, Vectors, ShardKey, OrderValue}` —
  `[VERIFIED: /Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.19.2/qdrant/points.pb.go:9876-9888]`).
  `OrderValue` is an `Int`/`Float` oneof only — no `Datetime` variant
  `[VERIFIED: .../points.pb.go:8866-8945]` — so ordering by the `created_at` datetime index must
  encode as one of those two numerically; the exact unit (seconds vs. nanoseconds, or whether
  Qdrant's datetime index even populates `OrderValue` at all rather than leaving it nil) was NOT
  confirmed live this session. **Do not adopt Option B without a throwaway empirical check first**
  (a one-off test asserting `OrderValue.GetInt()`/`GetFloat()` against a record with a known
  `created_at`); Option A has no such open question and is the safer default.
- Either way, `keysView`'s `maxRecordBytes` should be a small fixed constant (NOT derived from
  `RecordCaps`, since content/tags/citations are excluded entirely) — e.g. 256 bytes/record with
  generous margin, giving `perRPCLimit(256) ≈ 2MiB/256 = 8192` records/RPC at the default
  `rpcByteBudget`, matching CONTEXT.md's "tens of thousands per RPC" framing (order-of-magnitude;
  the exact constant is Claude's discretion).
**When to use:** Only `Store.List`'s offset-mode deep-offset case (`opts.Offset > 0`); the
walk emits NO items to the caller, only advances `from` to the offset boundary, then a normal
`scrollOrderedPage` call in the caller's view fetches the actual page.
**Boundary handoff:** the keys-only walk's own `orderedPage.Next` (a `listCursor`) can be fed
DIRECTLY as the `from` argument to the caller-view `scrollOrderedPage` call — both walks share the
identical `created_at`/`seen`-id keyset shape, so no translation is needed between the two views'
resume tokens (Claude's discretion item in 04-CONTEXT.md, resolved: reuse `listCursor` verbatim).

### Pattern 4: Two-phase search fetch (D-09)

**What:** `Store.Search`/`SearchReranked`/`SearchDiscovery` currently issue ONE `Query` with
`WithPayload: qdrant.NewWithPayload(true)` (`store.go:1180,1264` (indices as read this session))
and receive full `ScoredPoint{Id, Payload, Score}` records directly via `memoriesFromPoints`
(`store.go:1189-1197`). D-09 splits this into:

1. **Phase 1 (unchanged Query, changed payload selector):** `Query` with
   `WithPayload: qdrant.NewWithPayload(false)` — returns `[]*ScoredPoint{Id, Score}` only (Payload
   nil/empty) `[VERIFIED: .../points.pb.go:8947-8962 (ScoredPoint fields); .../oneof_factory.go:425-434
   (NewWithPayload(false))]`. One small RPC regardless of `k` (≤1000 ids × ~40 bytes + scores is a
   few tens of KB at most — far under any budget).
2. **Phase 2 (new fetch helper):** batch the returned ids into chunks of `perRPCLimit(view)` and
   issue one `Scroll` per chunk with `Filter: &qdrant.Filter{Must: []*qdrant.Condition{
   qdrant.NewFilterAsCondition(f), qdrant.NewHasID(chunkIDs...)}}` (an INCLUDE-shaped sibling of
   `excludeSeen`'s EXCLUDE-shaped `MustNot` — same `NewFilterAsCondition`-wrap-then-augment idiom,
   confirmed reusable: `[VERIFIED: internal/store/orderedpage.go:76-92]`), in the view selected per
   surface (see Pitfall/Pattern 5 below). No `OrderBy`/`StartFrom` needed — order doesn't matter
   here, since scores are already held from phase 1.
3. **Re-attach scores and re-sort:** build an `id → score` map from phase 1's `[]*ScoredPoint`,
   decode phase 2's fetched records via `fromPayload` (unchanged — it already takes a raw
   `map[string]*qdrant.Value` payload, not a `*ScoredPoint`: `store.go:717`), set `m.Score` from
   the map (mirroring `memoriesFromPoints`'s existing `m.Score = p.Score` assignment,
   `store.go:1193`, just from a map lookup instead of the same struct), then `sort.SliceStable` by
   the held score descending (Qdrant's own phase-1 order is already this, but re-sort explicitly
   since phase 2's `Scroll` result order is NOT the phase-1 rank order). A phase-1 id absent from
   the phase-2 result set (deleted/hidden between the two calls) is simply skipped — no error.
**Max ids per `has_id`:** at `k ≤ 1000` UUIDs (~36 bytes each) the `HasIdCondition.HasId []*PointId`
repeated field costs ~40KB serialized — trivially under any SEND-side limit; this was not a design
constraint in practice `[VERIFIED: /Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.19.2/qdrant/qdrant_common.pb.go:551-557]`.
**When to use:** `Store.Search` (and transitively `SearchReranked`) and `Store.SearchDiscovery`.

### Pattern 5: View selection differs by search surface — the `SearchReranked` 100-candidate finding

**What:** `RerankHits`/`lexicalOverlap` (`internal/store/rerank.go:47-56`) computes term overlap
against `hit.Content` — it needs FULL content for every candidate, not just the final `k` returned.
`candidateK(k)` (`rerank.go:16-25`) clamps its over-fetch at a HARD ceiling of 100 regardless of the
caller's `k` (`c := k*4; if c>100 { c = 100 }` — for any `k ≥ 25`, `candidateK(k) == 100` exactly;
`internal/store/store.go` confirms `Store.Search` is called from production code ONLY inside
`SearchReranked`, `store.go:1221`, via `grep`-verified zero other production call sites). This
means:
- `search_memory`/Connect `SearchMemories` (both route through `SearchReranked`,
  `internal/server/tools.go:1693-1719`, `internal/server/connectapi.go:350`) MUST fetch phase 2 in
  **`fullView()` unconditionally**, for up to 100 candidates — the caller's `full` flag governs
  ONLY the post-rerank response shaping (`shapeRecall`/`shapeProtoMemories`), exactly as it does
  today. There is no summary-view savings available on this surface no matter what `k` is
  requested; document this so a reviewer does not expect `summaryView()` sizing here.
- `search_discovery`/Connect `SearchDiscoveries` have NO reranker (`deps.searchDiscovery` calls
  `Store.SearchDiscovery` directly, `tools.go:1841`) and NO candidate over-fetch — phase 2's view
  should follow the caller's `full` flag directly (`summaryView()` default, `fullView()` on
  `full=true`), same as List.
**Byte-budget consequence:** worst case for `search_memory` is 100 full-view records ÷
`perRPCLimit(fullRecordCeiling)` (2 at default caps) = up to 50 phase-2 RPCs — more round trips than
List typically needs, but each RPC stays small and bounded; this is an acceptable, documented
tradeoff, not a defect.

### Pattern 6: Threading `Full` through the typed core (the gap this phase must close)

**What:** `coreListRequest`/`coreSearchRequest` (`internal/server/tools.go:1550-1610`ish) carry
NO `Full` field today; `a.Full`/`req.Msg.Full` is applied only in `shapeRecall`
(`internal/server/summary.go:82-91`) / `shapeProtoMemories` (`internal/server/connectapi.go:152-166`)
— AFTER `deps.listMemory`/`searchMemory`/`searchDiscovery` already fetched a store.Memory slice.
`[VERIFIED: internal/server/tools.go:1621-1642 (listMemory, no Full field); :2622-2637 (list_memory
MCP closure builds coreListRequest{} with no Full, then calls shapeRecall(res.Memories, a.Full, ...)
separately); internal/server/connectapi.go:295-303 (ListMemories calls a.d.listMemory(...) with no
Full field, then shapeProtoMemories(res.Memories, req.Msg.Full, ...) separately)]`. To realize
Phase 3 D-04's per-view sizing this phase MUST:
1. Add `Full bool` to `coreListRequest` and `coreSearchRequest`.
2. Wire it from BOTH lanes: MCP closures (`a.Full`) and Connect handlers (`req.Msg.Full`) for
   `list_memory`/`ListMemories` and `search_memory`/`SearchMemories`.
3. Add `Full bool` (or equivalent) to `store.ListOptions` and `store.SearchOptions`; `Store.List`/
   `ListScheduled`/`Search`/`SearchDiscovery` select `s.fullView()` vs `s.summaryView()` from it
   (subject to Pattern 5's override for `SearchReranked`).
4. **Update `internal/server/rules.go:206-208`'s DIRECT `d.st.List(ctx, sc, c.Subj,
   store.ListOptions{Limit: 0, Ascending: true, ...})` call** to pass `Full: a.Full` too —
   `listRulesArgs.Full` already exists and `d.listRules` already shapes compact-vs-full downstream
   (`toRuleView`, `rules.go:181-185`) exactly like `shapeRecall` does, but it calls `Store.List`
   OUTSIDE the typed core (`coreListRequest` is never built here), so it is invisible to steps 1-2
   above and WILL silently start returning summary-shaped (no-content) records for `full=true`
   rule reads if this call site is missed. `[VERIFIED: internal/server/rules.go:192-224]`.
5. `deps.listScheduled`/`listScheduledArgs` carry NO `Full` field at all today
   (`internal/server/tools.go:1646-1667`; the MCP closure returns `d.listScheduled`'s result
   directly with no `shapeRecall` call at `tools.go:2654-2662`) — recommend `ListScheduled` simply
   always requests `s.fullView()` (byte-budgeted via the looped `scrollOrderedPage`, but otherwise
   unchanged from today's always-full content) rather than inventing a new caller-facing
   full/summary distinction this phase does not need to add.

### Anti-Patterns to Avoid

- **Assuming `a.Full`/`req.Msg.Full` already reaches the store layer.** It does not (Pattern 6) —
  the byte-budget win from Phase 3 D-04 is unrealized until this threading lands.
- **Sizing `SearchReranked`'s fetch phase from `summaryView()`.** The reranker needs `Content` for
  every one of its (≤100) candidates; a summary-view fetch would silently degrade ranking quality
  by making `lexicalOverlap` see empty content (Pattern 5).
- **Missing `internal/server/rules.go`'s direct `Store.List` call when threading `Full`.** It is
  the one caller that bypasses the typed core entirely (Pattern 6, step 4).
- **Combining `OrderBy` with point-id `Offset`** (carried from Phase 3 RESEARCH.md — still
  applicable: `scrollOrderedPage`, the offset-mode loop, and `ListScheduled` all use `OrderBy` with
  no `Offset` field; only `scrollAllPoints`'s unordered sweeps use point-id `Offset`).
- **Re-deriving a filter to prove a claim about it, instead of reading `excludeSeen`'s
  narrow-only guarantee.** The two-phase fetch's has_id-INCLUDE filter must wrap the caller's
  existing recall filter exactly like `excludeSeen` wraps it for the exclude case — inventing a
  parallel, hand-rolled filter risks silently dropping the authz/recall conditions.
- **Editing `recallInvocationRows`' exact `expectCount`/`expectMethods` reflexively.** Because the
  recall-gate's real-Qdrant fixture collection is seeded with ZERO records
  (`TestSchemaVersionNeverGatesRecall`, `newTestStore`+`EnsureCollection`, no seed call —
  `[VERIFIED: internal/store/schemaversion_recallgate_test.go:1121-1128]`), every search row
  should stay at `expectCount:1, ["Query"]` IF (and only if) the new fetch helper correctly skips
  its `Scroll` call on an empty id batch. If a naive implementation always issues at least one
  `Scroll` (even for zero ids), these six rows (`Search`/`SearchReranked`/`SearchDiscovery` ×
  anonymous/owner) go RED and must NOT be "fixed" by bumping their expected counts — fix the
  fetch helper's empty-batch short-circuit instead.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Keyset resume across RPCs | A new pagination scheme | `scrollOrderedPage` (Phase 3) | Already proven against both oversized fixtures and both views; List/ListScheduled are its intended callers |
| Id-set inclusion/exclusion filters | A hand-rolled `Filter` builder for the two-phase fetch | `excludeSeen`'s `NewFilterAsCondition`+`NewHasID` idiom (invert `MustNot`→`Must` for INCLUDE) | Only proven mechanism in this codebase for narrowing-without-widening a caller's authz filter |
| Per-view byte ceilings | New arithmetic for search's fetch phase | `fullRecordCeiling`/`summaryRecordCeiling`/`perRPCLimit` (`boundedread.go`) | Already accounts for citations (the dominant term) — a search-specific reimplementation risks the exact "content alone" pitfall Phase 3's RESEARCH.md already flagged and fixed |
| Full/summary response shaping | New per-surface shaping logic | `shapeRecall`/`shapeProtoMemories`/`toRuleView` (unchanged) | This phase changes what VIEW is fetched, never how the response is shaped — those functions already do the right thing once fed the right `Content`/`Citations` presence |

**Key insight:** almost nothing in this phase is new mechanism — the risk is entirely in wiring
completeness (every caller of `Store.List`/`Search` that needs `Full` threaded; every recall-gate
row that needs its classification, not its count, updated).

## Runtime State Inventory

Not applicable — this is a wiring/behavior-change phase, not a rename/refactor/migration phase. No
renamed strings, no stored-data key changes. (D-11's hint-code STRING rename, `too_large` →
`response_too_large`, is a wire-contract text change, not a stored-data or OS-registered-state
rename — it has no runtime state implications beyond the source/docs edits D-11 already enumerates.)

## Common Pitfalls

### Pitfall 1: `recallInvocationRows`' exact-count assertions break silently if search always issues a fetch RPC

**What goes wrong:** `TestSchemaVersionNeverGatesRecall`'s six search-shaped rows assert
`expectCount:1, expectMethods:["Query"]`. If the new fetch helper issues a `Scroll` even when zero
ids come back from phase 1, these rows fail with a confusing "captured 2, want 1" message that
looks unrelated to schema_version at all.
**Why it happens:** the recall-gate fixture seeds zero records (verified above), so every search in
this test returns zero hits — an implementation detail (does the fetch helper short-circuit on an
empty batch?) becomes directly test-visible.
**How to avoid:** make the fetch helper's first line `if len(ids) == 0 { return nil, nil }` (or
equivalent), before any RPC.
**Warning signs:** `TestSchemaVersionNeverGatesRecall` failing on a "captured count" assertion for
a `Search`/`SearchReranked`/`SearchDiscovery` row after wiring the fetch helper in.

### Pitfall 2: `Store.List`'s `Limit:0` short-circuit changes shape, and Phase 2's regression depending on it needs retargeting (D-12)

**What goes wrong:** `store.go:1449-1461` today short-circuits `Limit:0` to `fetch = total` (an
unbounded single Scroll) — this is EXACTLY the shape `internal/server/responsetoolarge_test.go`'s
overflow regression and its Connect/MCP siblings currently exploit to prove `ErrResponseTooLarge`.
Once D-01/D-05 land, `Limit:0` resolves to `min(total, 1000)` fetched via BOUNDED
`scrollOrderedPage` pages — the old regression's premise (one huge unbounded Scroll) no longer
exists, so the test would either silently stop testing anything or (worse) start failing for the
wrong reason.
**Why it happens:** D-01 and D-12 are two sides of the same change — capping `Limit:0` at 1000
necessarily invalidates the fixture shape Phase 2's regression relied on.
**How to avoid:** explicitly retarget per D-12 — reuse the D-07 batch-of-1 fallback path (a single
legacy record whose payload alone exceeds `storetest.RecvLimit`, reached through a normal recall
entry point) as the new overflow trigger, or keep an explicitly-unbudgeted raw `package store` test
path. Record the retarget in the plan's SUMMARY deviations per D-12's own instruction.
**Warning signs:** `TestStoreListOverflowIsResponseTooLarge`,
`TestConnectListMemoriesResponseTooLarge`, `TestMCPListMemoryResponseTooLarge` (all named in
`redevidence_harness_test.go`'s Phase 2 mapping) passing for the wrong reason, or silently no
longer exercising an overflow at all.

### Pitfall 3: D-10's hint classification is deliberately NOT the "obvious" one

**What goes wrong:** `argerror.go` already declares a THIRD `argClass`, `classOutOfRange`, which
maps to `connect.CodeOutOfRange` — the semantically closer match for a hint literally named
`out_of_range`. D-10 explicitly locks `classMalformed` (→ `CodeInvalidArgument`) instead. A reader
who has not re-checked CONTEXT.md might "fix" this to `classOutOfRange` as an apparent bug.
**Why it happens:** `cmd/engram/client_common.go:449-450` groups
`CodeInvalidArgument`/`CodeFailedPrecondition`/`CodeOutOfRange` ALL under `exitUsage` (2) — so
either class produces the identical CLI exit code, making the "more correct" choice
behaviorally indistinguishable at the CLI, and the user chose `classMalformed` explicitly in
discuss-phase.
**How to avoid:** use `classMalformed` exactly as D-10 states; do not second-guess it even though
`classOutOfRange` reads better. `[VERIFIED: internal/server/argerror.go:47-63 (three argClass
values); cmd/engram/client_common.go:443-451 (exitCodeForConnectErr groups all three under
exitUsage)]`.
**Warning signs:** a plan or PR that "corrects" the class to `classOutOfRange` — this is a
regression against a locked, one-way decision (D-10 says do not re-ask), even though the CLI exit
code is unaffected either way.

### Pitfall 4: The eleven-hint-code doc gate is mechanical and will catch a naming rename miss, but only on the errors.md side

**What goes wrong:** `TestErrorsDocHintCodesMatchArgErrorConstants` (`hintcodedocs_test.go`) parses
`argerror.go`'s `HintCode` constants via `go/parser` and cross-checks them against `errors.md`'s
table AND a mechanical "eleven"→"twelve" count-word check across the WHOLE page (frontmatter
included). This means renaming `HintTooLarge`→`HintResponseTooLarge` and adding
`HintOutOfRange` is caught by this ONE test for the errors.md side, but the count-word phrase
"eleven" appears FIVE times in `errors.md` (frontmatter description line 3, body prose lines 106,
153, 155, and the heading itself line 111) — all five need to become "twelve", and the test only
asserts the HEADING's count word, not the other four prose occurrences.
**Why it happens:** the mechanical gate covers exactly what it says it covers (the heading count
word + the table's code SET) — it does not grep the whole page for every stale number word.
**How to avoid:** grep the whole `errors.md` file for "eleven" after adding the new hint and fix
every occurrence, not just the one the test checks.
**Warning signs:** `task lint`/tests green, but `errors.md`'s body prose still reads "eleven" in
three places while the heading and table correctly say "twelve".
`[VERIFIED: docs-site/src/content/docs/reference/errors.md:3,106,111,153,155 (all five "eleven"
occurrences, read verbatim this session)]`.

### Pitfall 5: The proto `limit` field has NO existing comment to edit — D-01 must ADD one, not modify one

**What goes wrong:** CONTEXT.md's canonical refs describe updating "the proto field comment" for
`ListMemoriesRequest.limit`, which reads as an edit to existing text. Reading `engram.proto:73`
directly shows `uint64 limit = 2;` carries NO trailing or preceding comment on the zero-semantics
today (`limit = 2` is bare) — every other neighboring field DOES carry an inline comment (e.g.
`offset = 3` has none either, but `categories = 4` does: `// empty = all categories`).
**Why it happens:** the `0 = "all"` contract for `limit` has always lived only in Go comments
(`store.go:1461`, `connectapi.go:274`) and docs (`tools.md`), never in the proto source itself.
**How to avoid:** ADD a new comment to `limit = 2` stating the numeric contract (e.g.
`// 0 resolves to the page maximum, 1000 (D-01)`), rather than searching for text to replace.
`[VERIFIED: proto/engram/v1/engram.proto:71-73, read verbatim]`.

### Pitfall 6: `ListScheduled`'s MCP surface has no full/summary distinction to preserve — do not invent one

**What goes wrong:** unlike `list_memory`/`search_memory`, `listScheduledArgs` carries no `Full`
field and the MCP closure never calls `shapeRecall` on its result — it returns raw `store.Memory`
values directly (`internal/server/tools.go:2654-2662`, confirmed: `mems, err :=
d.listScheduled(...)` then `map[string]any{"memories": mems}` with no shaping call in between). A
plan that tries to thread a `Full`-equivalent through `ListScheduled` "for symmetry" with List
would be inventing a caller-facing distinction that does not exist and is not requested by any
requirement.
**How to avoid:** always fetch `ListScheduled` in `fullView()` (bounded via the D-05 loop), matching
today's visible content exactly; do not add a full/summary knob here.

## Code Examples

Verified patterns from the working tree and the pinned `go-client@v1.19.2` module cache:

### `scrollOrderedPage`'s existing contract (the primitive List/ListScheduled compose)

```go
// Source: internal/store/orderedpage.go:118 (signature) and :48-70 (orderedPage fields)
func (s *Store) scrollOrderedPage(ctx context.Context, f *qdrant.Filter, view readView,
    dir qdrant.Direction, from listCursor, limit uint64) (orderedPage, error)

type orderedPage struct {
    Items       []Memory
    Next        listCursor // always the resume position; == from when nothing emitted
    Exhausted   bool       // true only when Qdrant genuinely has no more matches
    CutByBudget bool       // true only when pageByteBudget stopped the page early
    Bytes       int
}
```

### The has_id INCLUDE filter (invert of `excludeSeen`'s EXCLUDE idiom)

```go
// Source pattern: internal/store/orderedpage.go:76-92 (excludeSeen, MustNot form) —
// the two-phase fetch needs the Must (INCLUDE) sibling:
func includeIDs(f *qdrant.Filter, ids []string) *qdrant.Filter {
    pointIDs := make([]*qdrant.PointId, len(ids))
    for i, id := range ids {
        pointIDs[i] = qdrant.NewID(id)
    }
    var must []*qdrant.Condition
    if f != nil {
        must = []*qdrant.Condition{qdrant.NewFilterAsCondition(f)}
    }
    must = append(must, qdrant.NewHasID(pointIDs...))
    return &qdrant.Filter{Must: must}
}
```

### Score re-attachment precedent (`memoriesFromPoints`, adapted for the two-phase fetch)

```go
// Source: internal/store/store.go:1189-1197 (existing single-phase precedent)
func memoriesFromPoints(res []*qdrant.ScoredPoint) []Memory {
    out := make([]Memory, 0, len(res))
    for _, p := range res {
        m := fromPayload(p.Id.GetUuid(), p.Payload)
        m.Score = p.Score
        out = append(out, m)
    }
    return out
}
// Two-phase adaptation: phase 1 builds map[id]float32 from []*ScoredPoint (Payload nil);
// phase 2's fromPayload(id, fetchedPayload) + a map lookup replaces p.Score directly.
```

### `RerankHits`/`lexicalOverlap`'s content dependency (Pattern 5's grounding)

```go
// Source: internal/store/rerank.go:47-56
func lexicalOverlap(queryTerms map[string]struct{}, hit Memory) int {
    hitTerms := tokenize(hit.Content + " " + strings.Join(hit.Tags, " "))
    // ...
}
// Source: internal/store/rerank.go:16-25 — the 100-candidate hard ceiling
func candidateK(k uint64) uint64 {
    c := k * 4
    if c < 32 { c = 32 }
    if c > 100 { c = 100 }
    return c
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `Store.List`/`ListScheduled` issue one unbounded full-payload `Scroll` per call | Composed via `scrollOrderedPage`/looped pages, each sized from a per-view byte ceiling | This phase | Bounded response size; more RPCs for large pages, never more bytes than the budget |
| `Store.Search`/`SearchDiscovery` issue one full-payload `Query` | Two-phase: no-payload `Query` for ids+scores, then batched has_id `Scroll` fetches | This phase (D-09) | Same result set and ordering, bounded per-RPC size; `k` can now safely go to 1000 |
| `limit`/`k` silently uncapped (or capped only by `maxListLimit`'s cursor-only reach) | Rejected above 1000 with `out_of_range`, one documented maximum everywhere | This phase (D-02/D-10) | A caller must retry with a smaller value instead of silently getting a truncated/clamped result |
| `too_large` hint | `response_too_large` hint (same semantics, clearer name distinct from `too_long`) | This phase (D-11) | Docs/tests/patches must all move together — see Pitfall 4 |
| `full`/summary decided only at response-shaping time (fetch always full) | `full` selects the FETCH view too (except `SearchReranked`'s fixed 100-candidate full-view fetch) | This phase (Pattern 6) | Real byte savings for summary-view List/Search/SearchDiscovery; no change for search_memory's candidate pool |

**Deprecated/outdated:** the "`limit 0` = all" and "`too_large`" wordings across
`store.go`/`connectapi.go`/`tools.md`/`errors.md`/`cli.md`/`upgrade.md` — see the exact file:line
list in Pitfalls 4-5 and the Sources section below.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `RetrievedPoint.OrderValue`'s exact numeric encoding (unit, and whether it is populated at all for a datetime-indexed `OrderBy` field) was not empirically confirmed this session — Pattern 3's "Option B" | Architecture Patterns, Pattern 3 | If adopted without a live check, the keys-only walk could silently misdecode the resume boundary; RESEARCH.md recommends Option A (`NewWithPayloadInclude("created_at")`) instead, which carries no such risk |
| A2 | The exact byte-size constant chosen for `keysView`'s ceiling (proposed ~256 bytes/record) is an estimate, not measured against a live Qdrant response for this specific selector | Architecture Patterns, Pattern 3 | An overly generous ceiling under-fills RPCs (more round trips than necessary, not a correctness risk); an overly tight one is caught immediately by `scrollOrderedPage`'s existing byte-measurement path (`proto.Size`), so this is a performance-only risk, never a correctness one |

**Both entries above are LOW risk and self-correcting at implementation/test time** (the byte
measurement is empirical at runtime regardless of the estimated constant) — neither blocks
planning; the plan should ask the executor to pick Option A over Option B unless a live
confirmation of `OrderValue`'s encoding is obtained first.

## Open Questions

1. **Should the two-phase search fetch helper live in a new file (`searchfetch.go`) or colocated
   in `store.go` beside `Search`/`SearchDiscovery`?**
   - What we know: Phase 3 placed its new primitives in dedicated files (`orderedpage.go`,
     `boundedread.go`); `store.go` is already 3472 lines.
   - What's unclear: no CONTEXT.md preference either way (explicitly Claude's discretion).
   - Recommendation: a new file, mirroring Phase 3's pattern, for the same reviewability reasons.

2. **Exact naming for the renamed `maxListLimit` constant (D-02 asks it fit both list and search).**
   - What we know: today's name is `maxListLimit` (`store.go:1497`); D-02 wants one name serving
     both `limit` and `k`.
   - What's unclear: no locked name; `maxRecallLimit`/`maxPageLimit`/`maxResultLimit` are all
     plausible.
   - Recommendation: `maxRecallLimit` — reads correctly at every one of D-02's call sites (List,
     ListScheduled, Search, SearchDiscovery, list_rules).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Qdrant (real, via testcontainers or CI `services:`) | Every regression test this phase adds (`storetest.Dial`, `SeedOversized`) | ✓ (per Phase 1-3's own successful runs, `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/...` passing) | pinned via `storetest.QdrantImage` | none needed — tests skip gracefully without `ENGRAM_REQUIRE_QDRANT=1` per existing storetest fail-open/closed behavior |
| `github.com/qdrant/go-client` | All Qdrant proto/client symbols cited above | ✓ | v1.19.2 (`go.mod:47`) | none needed |

No missing dependencies — this phase adds no new external dependency.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib), real Qdrant via `internal/store/storetest` |
| Config file | none — `Taskfile.yaml`'s `test:*` tasks are the entry points |
| Quick run command | `go test -short ./internal/store/... ./internal/server/... ./cmd/engram/...` |
| Full suite command | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1` (real-Qdrant regressions); `go test ./... -count=1` (whole repo) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-list-bounded | `Store.List` offset `limit:0`/deep-offset/cursor all stay under `storetest.RecvLimit`, both fixture shapes | integration | `go test ./internal/store/... -run TestStoreListOffsetBounded -v` (name illustrative — Wave 0 authors it) | ❌ Wave 0 |
| REQ-list-scheduled-bounded | `ListScheduled` large limit stays bounded, both fixture shapes | integration | `go test ./internal/store/... -run TestListScheduledBounded -v` | ❌ Wave 0 |
| REQ-search-k-bounded | `search_memory`/`SearchReranked` (full=true, k well above 2) and `SearchDiscovery` stay bounded | integration | `go test ./internal/store/... -run TestSearchTwoPhaseBounded -v` | ❌ Wave 0 |
| REQ-list-contract-unchanged | `total`/`next_cursor`/ordering/recall-gate invariance; a `CutByBudget` page never reports `Exhausted` | integration | `go test ./internal/store/... -run TestListContractInvariant -v`; existing `go test ./internal/store/... -run TestSchemaVersionNeverGatesRecall -v` must stay green with NO count changes for search rows (Pitfall 1) | ❌ Wave 0 (new) / ✅ (existing gate) |
| REQ-list-limit-contract-decided | `out_of_range` rejection on every D-02 surface (MCP, Connect, CLI exit 2) | integration/unit | `go test ./internal/server/... -run TestOutOfRangeRejection -v`; `go test ./cmd/engram/... -run TestExitCodeBaseline -v` | ❌ Wave 0 (new case rows) / ✅ (existing baseline, extended) |

### Sampling Rate

- **Per task commit:** `go test -short ./internal/store/... ./internal/server/... ./cmd/engram/...`
- **Per wave merge:** `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1` (must include
  `TestSchemaVersionNeverGatesRecall` and `TestRecallEmissionSetIsCompleteAndClassified`)
- **Phase gate:** `task` (lint + full test suite) green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/store/liststorebounded_test.go` (or similarly named) — covers REQ-list-bounded
  (offset `limit:0`, deep offset, cursor), both `storetest.SeedOversized` shapes
- [ ] `internal/store/listscheduledbounded_test.go` — covers REQ-list-scheduled-bounded
- [ ] `internal/store/searchtwophase_test.go` — covers REQ-search-k-bounded (Search full=true +
  large k; SearchDiscovery), both fixture shapes, plus the empty-batch-skips-fetch property
  (Pitfall 1)
- [ ] `internal/store/listcontractinvariant_test.go` — covers REQ-list-contract-unchanged
  (D-06 cut-page-at-shrunken-budget test via `SetByteBudgets`, per Phase 3's precedent)
- [ ] `internal/server/outofrange_test.go` (or extend `argattribution_test.go`) — covers the
  `out_of_range` hint on every D-02 surface
- [ ] Framework install: none — `storetest`/`go test` infrastructure already exists from Phase 1-3

*(No test framework gap — Phase 1-3 already built and proved the harness; Wave 0 here is entirely
new TEST FILES against existing infrastructure, not new tooling.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | Unaffected — callers already authenticated upstream of `Store.*` |
| V3 Session Management | no | Unaffected |
| V4 Access Control | yes | The two-phase search fetch and the deep-offset keys-only walk MUST re-apply (never widen) the caller's existing `ownerScopeFilter`/`ownerOrSharedCondition`/recall-gate conditions — verified pattern: `excludeSeen`'s narrow-only guarantee (`orderedpage.go:76-92`), which this phase's `includeIDs` sibling must match structurally |
| V5 Input Validation | yes | `out_of_range` rejection for `limit`/`k` above 1000 (D-10), via the existing `argErrf`/`connectError` single-mapper pattern — never hand-rolled |
| V6 Cryptography | no | Unaffected |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| A record superseded/archived/expired BETWEEN the two search phases still surfacing in results | Tampering / Information Disclosure | D-09's design: phase 2 re-applies the IDENTICAL filter phase 1 used (including the recall-gate `IsEmpty` conditions), so a record that transitioned out of visibility between phases simply drops out of the fetch result — never surfaces stale |
| A malformed/oversized `has_id` batch used to probe another owner's records | Information Disclosure | `includeIDs`'s `Must` wraps the caller's authz filter as a nested condition (never `Should`/`MustNot` at the top level) — an id belonging to another owner cannot pass the AND even if included in the batch, mirroring `excludeSeen`'s already-audited narrow-only shape |
| A crafted `limit`/`k` value driving unbounded internal work before rejection | Denial of Service | The `out_of_range` check runs BEFORE scope resolution/embedding in `deps.*` (Claude's discretion on exact position, but recommended: before any Qdrant/embed call) — this phase's server-boundary rejection is exactly the discipline REQ-recv-limit-backstop's later production backstop complements, not replaces |

## Sources

### Primary (HIGH confidence — read directly this session)

- `internal/store/orderedpage.go`, `internal/store/boundedread.go` (Phase 3's built primitives,
  full file reads)
- `internal/store/store.go` (List/listByCursor/ListScheduled/Search/SearchReranked/SearchDiscovery/
  fromPayload/memoriesFromPoints, targeted section reads spanning lines ~1100-1780)
- `internal/store/rerank.go` (candidateK/RerankHits/lexicalOverlap, full read)
- `internal/store/cursor.go` (listCursor/encodeCursor/decodeCursor, full read)
- `internal/store/schemaversion_recallgate_test.go` (recallEntryPointSeeds, recallTransmitters,
  otherNonRecallEmitters, operatorMigrationEmitters, recallInvocationRows — read in full)
- `internal/server/tools.go` (deps.listMemory/listScheduled/searchMemory/searchDiscovery,
  coreListRequest/coreSearchRequest, listArgs/searchArgs, MCP tool closures — targeted reads)
- `internal/server/connectapi.go` (ListMemories/SearchMemories/SearchDiscoveries handlers,
  shapeProtoMemories — targeted reads)
- `internal/server/summary.go` (shapeRecall/toRecallView/summaryOrTruncation, full read)
- `internal/server/rules.go` (listRules/toRuleView, targeted read)
- `internal/server/argerror.go`, `internal/server/connecterror.go`,
  `internal/server/hintcodedocs_test.go` (HintCode catalog, connectError mapping, the mechanical
  errors.md gate — targeted reads)
- `cmd/engram/client_common.go` (exitCodeForConnectErr, classOutOfRange/classMalformed exit-code
  grouping — targeted read)
- `internal/store/redevidence_harness_test.go` (redEvidenceDirs, Phase 2/3 entries — targeted read)
- `docs-site/src/content/docs/reference/errors.md`, `reference/tools.md`, `guides/cli.md`,
  `guides/upgrade.md`, `proto/engram/v1/engram.proto` — exact grep + targeted line reads for every
  file:line cited above
- `/Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.19.2/qdrant/{points.pb.go,
  qdrant_common.pb.go, oneof_factory.go, conditions.go}` — `RetrievedPoint`, `ScoredPoint`,
  `OrderValue`, `PointId`, `HasIdCondition`, `NewWithPayload*`, `NewHasID`, `NewStartFromDatetime`,
  `NewFilterAsCondition` (all read verbatim from the pinned module cache this session)
- `.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/{03-CONTEXT.md,
  03-INVENTORY.md,03-02-SUMMARY.md,03-04-SUMMARY.md,03-RESEARCH.md}` — full reads
- `.planning/phases/04-list-listscheduled-search-bounded-reads/04-CONTEXT.md`,
  `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` (Phase 4 section) — full/targeted reads

### Secondary (MEDIUM confidence)

- Qdrant's documented `order_by.start_from` + `must_not: has_id` pagination guidance, as already
  cited and verified by Phase 3's own research (`03-RESEARCH.md` line ~156, quoting
  qdrant.tech/documentation/manage-data/points) — re-confirmed by this session's direct read of the
  same `listByCursor`/`scrollOrderedPage` code implementing it.

### Tertiary (LOW confidence)

- `RetrievedPoint.OrderValue`'s exact numeric encoding for a datetime-indexed `OrderBy` field
  (Assumption A1) — not empirically confirmed this session; flagged, and Option A (payload-include)
  recommended instead precisely to avoid relying on this.

## Metadata

**Confidence breakdown:**
- Standard stack / mechanism reuse: HIGH — every primitive already exists, built, and tested by
  Phase 3; this phase's code claims were read directly from the working tree.
- Full-flag threading gap (Pattern 6) and the 100-candidate SearchReranked finding (Pattern 5):
  HIGH — both derived from direct reads of the exact call chains, not inference.
- Deep-offset keys-only view sizing (Pattern 3, Option B / Assumption A1): MEDIUM — the safe
  Option A path is HIGH confidence; the alternative is explicitly flagged LOW and not recommended.
- Pitfalls (recall-gate exact-count fragility, D-12 retarget, D-10 classification, doc gate
  count-word sweep, proto comment absence): HIGH — each grounded in a direct source/docs read this
  session.

**Research date:** 2026-09-19
**Valid until:** 30 days (stable internal codebase; the pinned `go-client@v1.19.2` proto shapes are
not expected to change mid-milestone)
