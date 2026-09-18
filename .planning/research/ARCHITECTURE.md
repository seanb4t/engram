# Architecture Research

**Domain:** Go + Qdrant memory server — bounded-read integration for milestone 2026-09-18.01 "Bounded Reads"
**Researched:** 2026-09-18
**Confidence:** HIGH (all findings read directly from source at the commit on `feat/2026-09-18.01`, HEAD `ec79d4bd`; GitHub issue bodies for #585/#456/#347/#457/#497 read verbatim via `gh issue view`)

## System Overview

```
┌────────────────────────────────────────────────────────────────────┐
│ cmd/engram (cobra)                                                  │
│  client tier: get/search/list/store  --limit uint64 (0=all,        │
│    engram/client_list.go:20,58,122)                                 │
│  operator tier: reindex / migrate / summarize-missing /             │
│    spine-review / prune-expired (act on Qdrant directly)            │
└───────────────────────────┬──────────────────────────────────────┬─┘
                             │ Connect (HTTP)                        │ direct
┌────────────────────────────▼──────────────┐   ┌───────────────────▼─────┐
│ internal/server                            │   │ internal/store           │
│  connectapi.go: ListMemories, SearchMemories│   │  6 recall-gated entry   │
│    -> connectError(ctx, err) (SINGLE mapper,│   │  points (Search, Search-│
│    connecterror.go:55) — no ResourceExhausted│  │  Reranked, SearchDisc., │
│    arm today, falls to CodeInternal          │  │  List, ListScheduled,   │
│  tools.go: MCP tool closures                 │   │  ListScopes) — an AST   │
│    -> raw Go error return (no equivalent     │   │  gate proves schema_    │
│    single mapper; MCP SDK renders it)        │   │  version never gates   │
│  tools.go: deps.searchedScopes (#456 site)   │   │  any of their filters  │
└───────────────────────────┬──────────────┘   │  (schemaversion_       │
                             │                    │  recallgate_test.go)   │
┌────────────────────────────▼──────────────────┴─────────────────────┐
│ Qdrant go-client (qdrant.NewClient, internal/server/tools.go:123)     │
│  GrpcOptions: only otelgrpc stats handler — NO MaxCallRecvMsgSize set │
│  => grpc-go default 4 MiB client receive cap applies to every RPC     │
└────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Bounded-reads relevance |
|-----------|-----------------|--------------------------|
| `internal/server/tools.go:123` `storeFromConfig` | Constructs the one `*qdrant.Client` the whole process shares | Sole site to add `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(...))` as defense-in-depth (milestone explicitly says this alone is not the fix — #583 rejected it) |
| `internal/store/store.go` | `Store.List`, `Store.listByCursor`, `Store.ListScheduled`, `Store.ListScopes`, `Store.Search`, `Store.SearchDiscovery`, `Store.Reindex` | Every one of these calls `WithPayload(true)` (or, for `ListScopes`, a scoped `WithPayloadInclude`) against Qdrant with no per-page byte ceiling |
| `internal/store/spine.go` | `scrollAllPoints` — the package's one paginated whole-spine iterator (256/page); `ScanSpine`, `EnumerateCitations`, `NearDuplicates`, `derivePurgeEligible` route through it | Already paginates by *record count* (256), not bytes — same unbounded-content risk as the others, just already batched |
| `internal/store/migrate.go`, `revert.go`, `summarize.go` | Independent `ScrollAndOffset` loops at 256/page (`migrateBatch`, own literal `256`) — NOT routed through `scrollAllPoints` | Three more full-payload paginated loops outside the shared helper; a byte-aware fix touching only `scrollAllPoints` would miss them |
| `internal/server/connecterror.go:55` `connectError` | The **single** production mapper from a `deps.*`/`store.*` error to a Connect code | Correct place to add the `ResourceExhausted` arm for the Connect lane — everything else already routes through it |
| `internal/server/tools.go` (MCP closures) | Each MCP tool returns a raw `error`; there is **no MCP-side equivalent of `connectError`** | A second mapping site is needed (or the store-layer classification must be lane-agnostic so both lanes read the same sentinel) |
| `internal/server/tools.go:1639` `(*deps).searchedScopes` | Cross-spine coverage report — `Store.ListScopes` scan, called *after* a successful search/list | #456: its error today discards the already-computed hits at both call sites |
| `internal/embed/embed.go`, `internal/summarize/summarize.go` | OpenAI-compatible HTTP clients | Non-2xx error body is **already** bounded (`maxErrorBodyBytes` / 4096-byte `io.LimitReader`) from a prior milestone; the drain (`io.Copy(io.Discard, resp.Body)`) after both the error path and the success path is **not** bounded — relies entirely on `http.Client.Timeout`, which `WithTimeout(0)` explicitly disables |
| `internal/store/store_test.go:117` `TestMain` | Per-package ephemeral Qdrant testcontainer, or `ENGRAM_QDRANT_TEST_ADDR` | #497: `internal/store`'s container (the largest suite, last of several concurrent Qdrant containers to start) has died mid-run 3× in ~2h under CI resource pressure, unrelated to code |

## Full-payload Qdrant read site inventory

Found via `rg -n 'NewWithPayload(true)|Scroll(|ScrollAndOffset(|\.Query('` across `internal/store/*.go` (non-test). "Bound today" is the per-RPC record/byte ceiling as coded; "worst case" is what actually reaches the 4 MiB grpc-go client receive cap given **content is currently unbounded** (`ENGRAM_MEMORY_MAX_CONTENT_BYTES` does not exist — confirmed absent from `internal/config/registry.go`; only `ENGRAM_MEMORY_MAX_SUMMARY_BYTES`, default 512 B, caps `summary`).

| Site (file:line) | Method | Bound today | Worst case | Recall-gated? |
|---|---|---|---|---|
| `store.go:1435` `Store.List` offset-mode, `Limit:0` ("all") | `Scroll` | `fetch = total` — literally every matching record in one RPC | Unbounded — a scope with N large-content records always overflows past some N | Yes — `recallTransmitters` |
| `store.go:1435` `Store.List` offset-mode, deep `Offset` | `Scroll` | `fetch = opts.Offset + opts.Limit` | Grows with page depth — console page 50 at page-size 50 already fetches+discards 2500 records' full payload | Yes |
| `store.go:1493` `Store.listByCursor` | `Scroll` | `fetch = limit + len(seen) + 1`, `limit` clamped to `maxListLimit = 1000` (store.go:1458) | 1000 full payloads; overflows once average payload exceeds ~4 KiB (already the #583 shape) | Yes |
| `store.go:1641` `Store.ListScheduled` | `Scroll` | `Limit: qdrant.PtrOf(uint32(limit))`, `limit` defaults to 20 but is **caller-supplied via `opts.Limit` with no ceiling** | Unbounded if a caller passes a large `Limit` | Yes |
| `store.go:1689` `Store.ListScopes` | `Scroll` | `scanCap = 1000`, but `WithPayloadInclude("scope")` only (fixed by #583) | Bounded already — kept as `Scroll` deliberately for the recall-gate AST test (comment at store.go:1665-1672) | Yes |
| `store.go:1139` `Store.Search` | `Query` | `Limit: qdrant.PtrOf(k)` — **`k` is caller-controlled with no maximum anywhere** (no `MaxK` constant found in `internal/store` or `internal/server`) | Unbounded — MCP `search_memory`/Connect `SearchMemories` accept any `k` | Yes |
| `store.go:1230` `Store.SearchDiscovery` | `Query` | Same shape as Search, `k` uncapped | Unbounded | Yes |
| `store.go:3199` `Store.Reindex` | `ScrollAndOffset` | `batch` defaults to `reindexBatch = 256` (store.go:3084) | 256 full payloads/page — overflows once average content exceeds ~16 KiB | No (operator tier) |
| `spine.go:49` `scrollAllPoints` (shared iterator) | `ScrollAndOffset` | `spineScrollBatch = 256` (var, spine.go:28) | Same 256-record/~16 KiB-average ceiling; feeds `ScanSpine`, `EnumerateCitations`, `NearDuplicates`' id-enum (payload-frugal, low risk), `derivePurgeEligible`, and `revert.go`'s `previewRevertWithSteps` | No |
| `migrate.go:314`, `:377`, `:531` (three sites) | `ScrollAndOffset` | `batch` defaults to `migrateBatch = 256` (migrate.go:22) | Same 256-record ceiling — **not** routed through `scrollAllPoints`, an independent loop | No |
| `revert.go:431` (`revertWithSteps` write loop) | `ScrollAndOffset` | 256/page (mirrors migrate.go) | Same ceiling; `previewRevertWithSteps` (revert.go:278) instead uses `scrollAllPoints` | No |
| `summarize.go:145` `Store.SummarizeMissing` | `ScrollAndOffset` | Hardcoded `uint32(256)` literal (not a named const) | Same ceiling | No |
| `store.go:3418` `reindexTargetContents` | `Get` (batch by ids from `pts`) | Bounded by the calling page's `batch` (256) | Same ceiling, one `Get` per reindex page | No |

Every row above requests `qdrant.NewWithPayload(true)` — full `content`/`summary`/`tags`/`citations` — except `ListScopes` (already payload-scoped) and the `NearDuplicates` id-enumeration step (`WithPayloadInclude("short_id","scope")`, spine.go:558, already payload-frugal by design).

## Architectural Patterns

### Pattern 1: Two-phase ids-then-payload read

**What:** Scroll/Query with `WithPayload(false)` (ids + order key only) to establish the page's identity set, then a single `Get`/batched fetch for just that page's payloads.
**When to use:** Offset-mode `Store.List` (deep-offset skip) and `Store.ListScopes`-style aggregation, where most of the scrolled range is discarded anyway.
**Trade-offs:** Removes the "scroll N to skip to page K" cost entirely for the skipped prefix; adds a second round trip for the page actually returned. Does not by itself bound a *single* page's payload size if the page's own records are individually huge — needs pairing with Pattern 2 or a content cap.
**Where it already exists in this codebase:** `Store.Reindex`'s `--resume` lookup (`reindexTargetContents`, store.go:3407) is exactly this shape today (id-keyed page, then a scoped payload lookup) — reuse its structure rather than inventing a new one.

### Pattern 2: Byte-aware page loop

**What:** A page loop that keeps requesting/accumulating records but stops a page (returns a partial page + continuation cursor, or errors clearly) once accumulated payload size crosses a configured ceiling — independent of record count.
**When to use:** Any of the five 256(or 1000)-record-count loops in the inventory above, since record count alone cannot bound bytes while `content` is unbounded.
**Trade-offs:** Needs a size estimate per record (sum of `len(content)+len(summary)+...` from the already-decoded `Memory`, or the payload's own wire size) computed *after* the RPC already returned — it cannot prevent a single over-large RPC from itself exceeding 4 MiB. It only helps choose the **next** page's size; the current page can still overflow if one page's oversized *first* record already blew the receive cap.
**Implication:** Byte-aware paging alone does not fix the fundamental problem — it reduces steady-state risk (average record size) but does not cap worst case (one pathological record). A content-size cap (`ENGRAM_MEMORY_MAX_CONTENT_BYTES`, open per PROJECT.md) is the complementary fix that bounds the worst case; byte-aware paging bounds the aggregate.

### Pattern 3: Single bounded-scroll helper, reused across all recall-gated methods

**What:** One `Store` method — e.g. `boundedScroll` or a `scrollAllPoints`-with-byte-budget variant — that every recall-gated read (`Search`/`Query` excluded, since vector queries have no page-count knob other than `k`) and every operator sweep calls, parameterized by page size, byte budget, and payload selector.
**When to use:** This is the recommended default over five independent per-site fixes, because:
- `scrollAllPoints` (spine.go:30-69) already proves the shape works and is already unit-tested against forced single-record pagination (`spineScrollBatch` is a `var`, not a `const`, specifically so a test can force page size to 1 — spine.go:23-28).
- The AST completeness gate (`schemaversion_recallgate_test.go`) asserts by **enclosing function name**, not call count or line number — routing `Store.List`/`Store.ListScheduled` through a shared helper only requires adding new entries to `recallTransmitters` (moving `scrollAllPoints` itself out of `operatorMigrationEmitters`, since it would newly become reachable from the recall entry-point seeds) with a fresh justification. This is explicitly anticipated in the test's own comment (schemaversion_recallgate_test.go:546-549: "if a future recall path ever routes through it... the suite goes RED" — a deliberate tripwire, not a hard prohibition).
- The other AST gate (`TestSchemaVersionNeverGatesRecall`, Task 3) proves correctness by capturing the actual wire-transmitted `*qdrant.Filter` via a gRPC interceptor — it is agnostic to which internal function issued the call, so consolidating call sites behind a shared helper cannot break it as long as the caller still builds and passes the same filter.
**Trade-offs:** Touches five files (`store.go`, `spine.go`, `migrate.go`, `revert.go`, `summarize.go`) and both `spineScrollBatch`-style pagination and `Store.List`'s two different modes (offset vs. cursor) need to converge on one signature — `Store.List`'s offset-mode `Scroll` (single RPC with `OrderBy`+`Limit`) is a different Qdrant call shape than `scrollAllPoints`'s `ScrollAndOffset` loop, so "one helper" really means two: one for `ORDER BY`-ranked bounded pages (List/ListScheduled/ListScopes) and one for unordered whole-spine sweeps (the existing `scrollAllPoints`, extended with a byte budget). Recommend NOT trying to unify these two into a single call shape — the ordering semantics differ and forcing them into one function would make the diff far larger than the actual defect.
**Recommendation:** Prefer this consolidated approach over five independent per-site patches. Per-site fixes duplicate the byte-budget logic five times and give five chances to under-fix one site (exactly the failure #585 predicts for `Store.List` after #583 fixed only `ListScopes`).

## Where the ResourceExhausted mapping belongs

Three lanes need to agree, and the codebase's own documented discipline ("declared once, checked everywhere" — e.g. `PurgeFilterPathActive`, `expiredFilter`) says the mapping must live at the lowest common layer that both lanes already call through:

1. **`internal/store`** should classify the raw gRPC `ResourceExhausted` status into a **typed sentinel** (e.g. `store.ErrResponseTooLarge`, mirroring the existing `store.ErrInvalidArgument`/`store.ErrNotFound`/`store.ErrAmbiguousShortID` pattern) at the point each read RPC's error is returned — or, more simply, as a small `classifyQdrantErr(err) error` helper called uniformly by `Store.List`, `listByCursor`, `Search`, `SearchDiscovery`, `ListScheduled`, `ListScopes`, and the operator sweeps, using `status.Code(err) == codes.ResourceExhausted` (grpc-go's own `google.golang.org/grpc/status` / `codes` packages, already an indirect dependency via qdrant-go-client).
2. **`internal/server/connecterror.go`'s `connectError`** already has the exact right shape for the Connect lane: a `case errors.Is(err, store.ErrResponseTooLarge): return connect.NewError(connect.CodeResourceExhausted, err)` arm, added to the switch **before** the `default` — this is the single mapper every Connect handler already calls (per its own doc comment, "every Connect... handler calls it instead of hand-rolling its own per-handler mapping"). The CLI side is *already* ready for this: `cmd/engram/client_common.go`'s `exitCodeForConnectErr` already has a `{connect.CodeResourceExhausted, exitGeneric}` test row — the client-side exit-code taxonomy anticipates this arriving; only the server-side emission is missing.
3. **The MCP lane has no equivalent single mapper.** MCP tool closures in `tools.go` return the raw Go `error` from `deps.*`/`store.*` and the mcp-go SDK renders it as a tool-call error; there is no `connectError`-style chokepoint today. Two options: (a) let the raw `store.ErrResponseTooLarge` sentinel's `Error()` text alone carry the clear message (cheapest, consistent with today's undifferentiated MCP error surface, but the milestone's "clear, named error" bar is stronger than "readable string"); (b) introduce a matching MCP-side mapper analogous to `argErrf`/`conditionalErrf` (already used for validation errors, e.g. tools.go:1602) so `ErrResponseTooLarge` renders with a stable, greppable hint code, mirroring the existing `field=<name> hint=<code>` envelope described in CLAUDE.md's "Memory contract" section. Recommend (b) for consistency with the existing diagnosability discipline (`internal/server/argerror.go`), even though it is new surface, not a location fix.

**Recommendation:** classify once in `internal/store` (sentinel), map once per lane at the existing single chokepoints (`connectError` for Connect; a new, equally singular MCP-side mapper for MCP) — never inline `status.Code(err)` checks scattered across `tools.go`/`connectapi.go` call sites.

## #456 partial-result semantics: how they should flow to searched_scopes/scopes_truncated

Confirmed by reading both call sites verbatim:

- MCP: `tools.go:2414-2417` (`search_memory`) and `tools.go:2469-2472` (`list_memory`) — both call `d.searchMemory`/`d.listMemory` (succeeds, `ms`/`res` populated), then call `d.searchedScopes(ctx, c, a.CrossSpine)`, and on error `return nil, nil, err` — **discarding the already-computed hits**.
- Connect: `connectapi.go:301-304` (`ListMemories`) and `connectapi.go:364-367` (`SearchMemories`) — identical shape: `res`/`ms` populated, then `searchedScopes` error causes `return nil, connectError(ctx, err)`, discarding `res`/`ms`.
- The GitHub issue (#456, filed as an *accepted, documented trade-off* during a prior phase review, not a fresh defect) explicitly proposes the fix already scoped for this milestone: **return the already-computed hits with an explicit sentinel distinguishing "coverage unknown" from "coverage empty"** — a third state, not reusing `scopes_truncated` (whose meaning is already fixed: "the scope list itself is a bounded/truncated sample", from `ListScopes`' `scanCap` semantics, store.go:1667).

Recommended flow, consistent with `recallResultMap`'s existing on/off-by-`crossSpine` shape (tools.go:1663-1669):

1. On `searchedScopes` failure, callers should still return `hits`/`mems` (the real, already-fetched, already-authorized results) rather than discarding them.
2. `searched_scopes` becomes `nil`/absent (not an empty slice — proto3 already treats `nil` as absent per the existing `(nil, false, nil)` scope-confined-call convention noted at connectapi.go:296-300 and :361-363) when the coverage query itself failed.
3. Add a **new** boolean (not a reused field) — e.g. `scopes_unknown` (Connect) / `"scopes_unknown"` (MCP result map) — set `true` only on this failure path, alongside `searched_scopes` absent and `scopes_truncated` **also absent/false**, so a consumer can distinguish three states: (a) non-cross-spine call → neither key present; (b) cross-spine call, coverage known → `searched_scopes` populated, `scopes_truncated` present; (c) cross-spine call, coverage query failed → `scopes_unknown: true`, `searched_scopes` absent. This is additive on the wire (new proto field, non-breaking) and additive in the MCP result map (D-14's existing "only added when crossSpine is true" discipline extends naturally).
4. `searchedScopes` itself should **not** silently swallow the error into a zero value — its own doc comment (tools.go:1636-1638) is correct that "an error from ListScopes fails the call rather than degrading to an empty list" was right for the *coverage claim*; it was wrong only in also discarding the *hits*. The fix separates these: keep failing loudly on the coverage claim, stop discarding the hits.

This requires a **proto change** (new field on `ListMemoriesResponse`/`SearchMemoriesResponse`) plus MCP result-map and `recallResultMap` changes — both call sites (`tools.go` MCP closures, `connectapi.go` handlers) need the same restructuring from "propagate `searchedScopes`'s error" to "capture it, still return hits, set the new flag."

## What the structural gates / AST tests constrain

Two AST-based gates already exist in `internal/store` and both are name-keyed, not line-number-keyed — safe to refactor around as long as names/classifications are updated in the same change:

1. **`schemaversion_recallgate_test.go`** — derives every `Query`/`QueryBatch`/`Scroll`/`ScrollAndOffset`/`Count` call site in the package via `go/ast`, closes a same-package call graph from six named recall entry-point seeds (`recallEntryPointSeeds`, schemaversion_recallgate_test.go:357-364: `Store.Search`, `Store.SearchReranked`, `Store.SearchDiscovery`, `Store.List`, `Store.ListScheduled`, `Store.ListScopes`), and requires every derived emission site to land in exactly one of three hand-maintained, justified lists (`recallTransmitters`, `operatorMigrationEmitters`, `otherNonRecallEmitters`). **Constraint for this milestone:** introducing a new helper function reachable from any of the six seeds (e.g. a shared bounded-page helper called by `Store.List`) makes that new function name newly appear in the derived reachable set — it must be added to `recallTransmitters` with a justification, or the "reachable emission completeness" subtest goes RED by design. Reusing the *existing* `scrollAllPoints` from a recall-gated method reclassifies it from `operatorMigrationEmitters` to `recallTransmitters` — anticipated explicitly in that function's own current justification text.
2. **`TestSchemaVersionNeverGatesRecall`** (same file, Task 3) — a real gRPC interceptor captures the actual `*qdrant.Filter` transmitted by the six entry points against a live Qdrant and asserts `schema_version` never appears (via the recursive `walkFilterKeys`/`walkFilter` walker covering all seven `qdrant.Condition` oneof variants). **Constraint:** any refactor of filter-building code inside `List`/`Search`/etc. must still pass the identical, already-built `*qdrant.Filter` value to whatever RPC-issuing call replaces the current `Scroll`/`Query` — this test is agnostic to *how* the RPC is issued, only to *what filter reaches the wire*, so it does not block consolidating call sites.
3. **`TestListScopesFullPayloadsOverGRPCLimit`** (`store_test.go:1798`, the #583 regression pattern named explicitly in PROJECT.md's "Done means") — real-Qdrant, `testing.Short()`-skipped, seeds `n=40` records of `128<<10` (128 KiB) content each (`>4 MiB` total, asserted by a self-check `t.Fatalf` if the fixture ever shrinks below the 4 MiB threshold) into one scope, then calls the method under test and expects success. **This exact pattern — real Qdrant, oversized fixture, RED before the fix — is what the milestone's "Done means" section requires for every one of #585's remaining paths** (`List` in all three modes, `ListScheduled`, `Search`'s `k`, and the five 256-record operator sweeps). A shared byte-aware helper should get **one** such fixture-and-proof test per call site it replaces (proving the specific caller's request shape stays under the cap), not one test for the helper in isolation — the existing `TestListScopesFullPayloadsOverGRPCLimit` proves the *caller's* shape, not `Scroll` in the abstract (its own doc comment: "This test covers OUR request shape, not Qdrant's own behavior").
4. There is **no** enforced "exactly one `ScrollAndOffset` call site" AST test in the repository today — that phrasing in the milestone's `<required_reading>` context is a paraphrase of spine.go's *doc-comment* discipline (spine.go:40-45, and repeated at multiple call sites, e.g. spine.go:319-320, 489-491, 966-967: "internal/store/spine.go must carry exactly one `client.ScrollAndOffset` call site" — enforced by convention/comment/code-review, not by a compiled/tested gate). `rg` confirms four *other* non-test `ScrollAndOffset` call sites already exist outside `spine.go` (`migrate.go`×3, `revert.go`×1, `store.go`'s `Reindex`×1, `summarize.go`×1) — the doc comment's claim is scoped to *within spine.go itself*, not the whole package, and the `schemaversion_recallgate_test.go` file's own comment independently confirms this ("cycle-2 review found four non-test ScrollAndOffset call sites"). **Do not add a new hard "count == 1" AST assertion as part of this milestone's fix** — it would be a novel invented gate with no existing template, and the milestone's actual done-bar (a real-Qdrant regression test per fixed path) is the correct, already-precedented gate instead.

## Data Flow: a bounded read today (List, offset mode, `Limit=0`)

```
cmd/engram list --limit 0            engram console (offset paging)
        │                                    │
        ▼                                    ▼
  ListMemoriesRequest{Limit:0}  ──Connect──▶  connectapi.go ListMemories
                                                    │
                                                    ▼
                                          deps.listMemory → Store.List(ctx, scope, subj, opts)
                                                    │
                                    total,_ := Count(...)      [store.go:1407]
                                    fetch := total  (Limit==0) [store.go:1421-1424]
                                                    │
                                    Scroll(Limit:fetch, WithPayload(true)) [store.go:1435]
                                                    │
                                    ── ResourceExhausted if Σ payload > 4 MiB ──▶
                                                    │
                                    connectError(ctx, err) → CodeInternal (today)
```

### Key Data Flows After the Fix

1. **Bounded List (all three modes):** `Store.List` gains a byte budget threaded into whichever bounded-page helper (Pattern 2/3) replaces its single `Scroll`/`listByCursor`'s `Scroll` — `Limit:0` ("all") specifically needs either a hard cap or internal re-paging that still returns the full logical result to the caller (Connect's `ListMemories` contract keeps "0 = all", per PROJECT.md's still-open discuss-phase question) or, per the "Open for discuss-phase" note, is renegotiated to a hard cap + cursor paging.
2. **Cross-spine coverage with partial failure (#456):** `deps.searchMemory`/`listMemory` succeeds → `searchedScopes` fails → hits still flow to the caller, `scopes_unknown` (new field) signals the coverage gap, per the section above.
3. **Provider response draining (#457):** `embed.go`/`summarize.go`'s `io.Copy(io.Discard, resp.Body)` calls (four sites total: embed.go:295 error path, :309 success path; summarize.go:182 error path, :191 success path) gain `io.LimitReader(resp.Body, N)` wrapping, independent of `c.http.Timeout`.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Raising `MaxCallRecvMsgSize` and calling it done

**What people do:** Set `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64<<20))` on the Qdrant client and consider #585 closed.
**Why it's wrong:** #583's own PR explicitly rejected this as *the* fix — it only moves the ceiling to a bigger number; the same unbounded-`Limit`/unbounded-content shapes still overflow at a larger N, and PROJECT.md's "Done means" section states this outright ("Raising `MaxCallRecvMsgSize` alone only moves the ceiling... it is defense in depth at most").
**Do this instead:** Add it anyway (defense in depth, one-line change at `internal/server/tools.go:123`), but treat every row in the read-site inventory above as needing its own real-Qdrant regression test and bounded-page fix regardless.

### Anti-Pattern 2: A second, independently-written pagination loop

**What people do:** Fix `Store.List`'s offset mode with a new bespoke bounded loop, then fix `ListScheduled` with a *different* bespoke bounded loop, and so on for five call sites — exactly the failure mode `expiredFilter`'s and `derivePurgeEligible`'s doc comments elsewhere in this codebase explicitly warn against ("two independently constructed conditions... could silently drift").
**Why it's wrong:** Five independent implementations of "stop before N bytes" is five independent chances to get the boundary condition (a single oversized record) wrong, and five sets of tests to keep in sync — the exact problem #583→#585 already demonstrates (fixing `ListScopes` alone left `List` open).
**Do this instead:** Pattern 3 — one ordered-page helper (List/ListScheduled/ListScopes) and one extension to the existing `scrollAllPoints` (whole-spine sweeps), each proven once, reused everywhere.

## Suggested Build Order

Ordered by hard dependency, not by issue number. Each phase names what it unblocks.

**Phase 1 — Test harness stability (#497) and a >4 MiB fixture helper.**
Every other phase's "done" bar is "a real-Qdrant regression test holding more than 4 MiB of payload, RED before the fix" (PROJECT.md, citing `TestListScopesFullPayloadsOverGRPCLimit`). If `internal/store`'s testcontainer keeps dying mid-run under CI resource pressure (#497), every subsequent phase's CI signal is unreliable before it even reaches the new tests. This phase should also extract `TestListScopesFullPayloadsOverGRPCLimit`'s fixture-seeding shape (`n` records × `contentBytes`, with the `n*contentBytes <= 4<<20` self-check) into a small reusable test helper in `internal/store`, since the next five phases each need their own copy of it. Do not touch production code in this phase beyond what #497 requires (e.g., shared container, resource caps, or diagnostic capture per the issue's "Possible directions").

**Phase 2 — Store-layer error classification + `connectError` `ResourceExhausted` arm.**
Independent of the read-site fixes themselves (it classifies whatever error a Qdrant RPC returns today, oversized or not) and is a prerequisite for writing any of Phase 1's regression tests to assert *the right failure mode* pre-fix (RED should show a clear `ResourceExhausted`-mapped error once this lands, not a bare `internal` — makes the "RED before the fix" state itself legible). Also unblocks the CLI's already-anticipated `exitCodeForConnectErr` row. Add the MCP-side equivalent mapper in the same phase, since both lanes need it before any read-path fix can be verified end-to-end.

**Phase 3 — Shared bounded-read mechanism (Pattern 3).**
Build the ordered-page helper (for `List`/`ListScheduled`/`ListScopes`-shaped reads) and extend `scrollAllPoints` with a byte budget (for the whole-spine sweeps). This is the single largest phase and should land with the AST-gate updates (`recallTransmitters` reclassification) in the same change, since the gate will go RED the moment the new helper is wired in — never a separate follow-up commit.

**Phase 4 — Per-site migration onto the shared mechanism, one caller at a time, each with its own real-Qdrant regression test.**
Order within this phase by risk/exposure, matching #585's own ordering: `Store.List` (all three modes — highest exposure, hit live per #585's report) → `Store.ListScheduled` → `Store.Search`/`SearchDiscovery` (cap `k` server-side in addition to bounding pages) → the five 256-record operator sweeps (`migrate.go`×3, `revert.go`, `summarize.go`; `reindex` and `scrollAllPoints`'s existing callers get the byte budget "for free" from Phase 3's extension, but each still needs its own oversized-fixture proof per the #583 pattern's own stated scope). Raise `MaxCallRecvMsgSize` (Anti-Pattern 1's "add it anyway") in this phase too, since it is a one-line, low-risk addition alongside the real fixes.

**Phase 5 — #456 partial-result semantics.**
Independent of the byte-bounding work (it is a pure error-handling/proto-shape change to `searchedScopes` and its two call sites in each lane) — could run in parallel with Phase 3/4, but is sequenced after Phase 2 so `connectError`'s classification discipline (single mapper, typed sentinels) is already the established pattern to extend for the new `scopes_unknown` field's error path.

**Phase 6 — Bounded provider responses (#457; #347 is effectively already shipped).**
Fully independent of the Qdrant read-path work (`internal/embed`/`internal/summarize` share no code with `internal/store`). Confirmed by reading the current source: #347's "discard the error body" defect is **already fixed** (bounded `io.LimitReader` on the error path in both clients, landed in a prior milestone per PROJECT.md's v0.12.x changelog — the GitHub issue was simply never closed). The only remaining work is #457: wrap all four `io.Copy(io.Discard, resp.Body)` drain calls (embed.go:295,309; summarize.go:182,191) in `io.LimitReader(resp.Body, N)`. Can run at any point — placed last only because it has zero dependency on and zero risk to the rest of the milestone, so it should not block the Qdrant-side work's critical path.

## Sources

- `/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go` (List, listByCursor, ListScheduled, ListScopes, Search, SearchDiscovery, Reindex, reindexTargetContents — line numbers as cited above)
- `/Volumes/Code/github.com/seanb4t/engram/internal/store/spine.go` (scrollAllPoints and every whole-spine sweep built on it)
- `/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go`, `revert.go`, `summarize.go` (independent ScrollAndOffset loops)
- `/Volumes/Code/github.com/seanb4t/engram/internal/store/schemaversion_recallgate_test.go` (the recall-gate AST completeness + wire-capture gates)
- `/Volumes/Code/github.com/seanb4t/engram/internal/store/store_test.go:1793-1839` (`TestListScopesFullPayloadsOverGRPCLimit`, the #583 regression pattern)
- `/Volumes/Code/github.com/seanb4t/engram/internal/server/connecterror.go`, `connectapi.go`, `tools.go` (connectError, searchedScopes, recallResultMap, MCP tool closures)
- `/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go`, `internal/summarize/summarize.go` (bounded error body already present; unbounded drain confirmed)
- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go` (`exitCodeForConnectErr`'s existing `CodeResourceExhausted` row)
- GitHub issues #585, #456, #347, #457, #497 (bodies read verbatim via `gh issue view --json body`)
- `/Volumes/Code/github.com/seanb4t/engram/.planning/PROJECT.md` (milestone goal, "Done means", open discuss-phase questions)

---
*Architecture research for: engram bounded reads (milestone 2026-09-18.01)*
*Researched: 2026-09-18*
