# Architecture Research — Console Overhaul (2026-09-25.01)

**Domain:** Adding write/graph/facet RPCs and a query-understanding decision to an existing Go + Qdrant + ConnectRPC + SvelteKit system
**Researched:** 2026-09-25
**Confidence:** HIGH for the Go/proto/authz integration points (read from the actual code); MEDIUM for the query-understanding decision shape (no precedent in this codebase for Choice/Score batching outside `internal/verdict`/`internal/relevance`, extrapolated from those two)

## Standard Architecture

### System Overview (existing, annotated with what this milestone touches)

```
┌───────────────────────────────────────────────────────────────────────────┐
│  Callers                                                                    │
│  MCP tools (tools.go)     Connect handlers (connectapi.go) ◄── SPA (ui/)   │
│  CLI operator tier (cmd/engram/spine_review_*.go) — subject-less, trusted  │
└──────────────┬───────────────────────┬──────────────────────┬─────────────┘
               │                       │                      │
               ▼                       ▼                      │
        ┌─────────────────────────────────────┐               │
        │ internal/server: *deps               │  ◄────────────┘ (Connect handlers
        │  - deps.getMemory/listMemory/         │                  call the SAME
        │    searchMemory/supersedeMemory/...   │                  deps.* funcs MCP
        │  - one deps.* fn per capability,      │                  tools call — D-07)
        │    called by BOTH MCP and Connect     │
        │  - NEW: deps.archiveMemory/           │
        │    restoreMemory (no MCP tool exists  │
        │    today — see Gap 1 below)           │
        └──────────────┬────────────────────────┘
                       ▼
        ┌─────────────────────────────────────┐
        │ internal/store: *Store               │
        │  - Search/List/Get/GetReadable/      │
        │    getWritable (authz.Action-gated)  │
        │  - ownerScopeFilter/ownerOrShared-    │
        │    Condition/ownerOnlyCondition       │  ← the ONE place Cedar (internal/authz)
        │    compile to *qdrant.Filter          │    policy becomes a Qdrant Must/Should
        │  - NearDuplicates (subject-less,      │    filter — never in a handler
        │    NewQueryID sub-query pattern)      │
        │  - Archive/Restore (subject-less      │
        │    TODAY — CLI-only, see Gap 1)       │
        │  - MigrateStatus (Facet, no filter —  │
        │    see Gap 2)                         │
        └──────────────┬────────────────────────┘
                       ▼
                  Qdrant (go-client v1.19.2)

        internal/decide (provider-neutral, advisory) ── internal/decide/jev
          consumed today by: internal/relevance (search rerank hook, 2s
          no-retry client) and internal/verdict (spine-review consolidate,
          larger-timeout client) — see decider.go for the two-client pattern
          this milestone's query-understanding decision must reuse.
```

### Component Responsibilities

| Component | Responsibility | Evidence |
|-----------|-----------------|----------|
| `internal/server/connectapi.go` (`engramAPI`) | Thin adapter: resolve caller → call the same `deps.*` fn MCP calls → map result/error via `connectError` (single mapper, D-11) | `connectapi.go:450-455` "The six Connect write RPCs below are thin adapters" |
| `internal/server/*deps` | The ONE capability-per-method core both lanes call (D-07 rewire) | `connectapi.go:231-236` |
| `internal/store/*Store` | Owns authz composition into Qdrant filters; the ONLY place `internal/authz` becomes a `*qdrant.Filter` | `store.go:1038-1045` `ownerScopeFilter`; `store.go:922-940` `ownerOrSharedCondition` |
| `internal/server/connectcsrf.go` | Hand-maintained allowlist of write Procedures requiring the CSRF double-submit token | `connectcsrf.go:33-39` `csrfWriteProcedures` |
| `internal/decide` + `internal/decide/jev` | Provider-neutral typed decisions; off unless `ENGRAM_DECISIONS_PROVIDER` set; a failure is never fatal to the caller | `decide.go:17-31` |
| `ui/src/lib/client.ts` | Two Connect clients: `engram` (read, no interceptors) and `engramWrite` (write, `[retryOnce, attachCsrf]`) | `client.ts:1-26` |
| `ui/src/lib/mutations/*.ts` | One `useXMutation()` hook per write RPC: `onMutate` optimistic patch + snapshot, `onError` rollback + toast, `onSettled` invalidate | `mutations/memory.ts:275-416` |

## New Connect RPCs: what each one wraps, and what it costs

All six proposed RPCs are additive on the wire (new `rpc` lines in `engram.proto`'s `EngramService`, new message types with unused-so-far field numbers) — none touches an existing message or field, so `buf breaking` stays green by construction as long as no existing field is renumbered or retyped.

| RPC | Wraps | Authz path | Write-interceptor stack? | MCP parity |
|---|---|---|---|---|
| `SupersedeMemory` | `deps.supersedeMemory(ctx, c, supersedeArgs)` — already exists, backs MCP `supersede_memory` (`tools.go:2563`) | Already authz-checked per target inside `resolveAndAuthorizeSupersedeTargets` → `store.go:2763` `getWritable(..., authz.ActionWrite)` | **Yes** — must be added to `csrfWriteProcedures` (`connectcsrf.go:33-39`) or it silently bypasses CSRF | Direct row addable to `connectapi_write_parity_test.go` (same shape as `StoreMemory` et al.) — MCP tool already exists, no asymmetry to document |
| `ArchiveMemory` / `RestoreMemory` | **No existing authz-checked wrapper.** `store.Archive`/`store.Restore` (`spine.go:773`, `:836`) are subject-less and today called ONLY by the CLI's trusted `spine-review archive/restore` (`spine_review_archive.go:211,242`, `st.Archive`/`st.Restore` direct, no `Subject`) | **Gap — new work, not a wrapper.** Must add a `subj Subject` parameter and gate through `getWritable(ctx, id, subj, authz.Action…)` (the same gate `Delete`/`Update`/`SetVisibility`/`Supersede` use — `store.go:2195-2208`, call sites at `:2268,:2558,:2763,:2973`) *before* the existing archived_at stamp/delete-key write. Keep the CLI's subject-less `st.Archive`/`st.Restore` for the trusted operator tier (it is exactly the same class of privileged bypass as `spine-review purge`/`migrate-remap-owner`) and add a NEW authz-gated method (or a `deps`-level check) for the per-caller Connect/console path. | **Yes** — new write RPCs, must be added to `csrfWriteProcedures` | No MCP tool named `archive_memory`/`restore_memory` exists (`tools.go` search confirms). Decide explicitly: either add matching MCP tools (so the parity harness has something to compare, and agents get the capability too) or document this as an intentional Connect/console-only asymmetry, mirroring the existing `SearchDiscoveries` `Scope==""→cross_spine` divergence pinned by `TestConnectCrossSpineNotInferred` (`connectapi.go:419-428`) |
| `ListRules` | `deps.listRules(ctx, c, a)` — already exists, backs MCP `list_rules` (`tools.go:3041`) | Rule scopes are `rule:repo:*`/`rule:project:*`, validated by `validRuleScope` (`rules.go:46-53`); read-shared, no owner filter beyond scope validity | **No** — pure read RPC, no CSRF/write interceptor | Direct parity row; MCP tool exists |
| `ListScheduled` | `deps.listScheduled(ctx, c, a)` — exists, backs MCP `list_scheduled` (`tools.go:1867`) but **requires `a.Scope` non-empty today** (`tools.go:1868-1872`, comment: "list_scheduled has no Connect RPC — MCP-only") | `store.ListScheduled` (`store.go:1966`) — check whether it composes `ownerOnlyCondition` (management view; a `shared`+scheduled record must stay hidden from non-owners per `store.go:942-946`) | **No** — read RPC | If the Connect RPC adds `cross_spine` (recommended, matching `ListMemories`/`SearchMemories`'s D-04 pattern) that is new behavior beyond the existing MCP tool's contract — decide whether to also widen the MCP tool or accept a second documented asymmetry |
| `RelatedMemories(id)` | **New store method**, `s.NewQueryID`-pattern reused from `NearDuplicates` (`spine.go:576-706`) but authz-composed (see below) | New: compose `ownerScopeFilter`/`ownerOrSharedCondition` into every sub-query's filter — see design below | **No** — read RPC | No MCP tool exists; same asymmetry decision as Archive/Restore |
| `ListTags(scope)` | **New store method** using `qdrant.Facet` on `tags`, modeled on `Store.MigrateStatus`'s Facet call (`migrate_status.go:143-148`) but WITH a filter (MigrateStatus's never carries one) | New: authz `Must` filter passed into `qdrant.FacetCounts.Filter` | **No** — read RPC | No MCP tool exists; same asymmetry decision |

**Additive proto sketch** (illustrative field numbers; use the next free number per message, not these literals):

```protobuf
message SupersedeMemoryRequest { /* mirrors supersedeArgs: storeArgs fields + repeated string supersedes + optional string idempotency_key */ }
message SupersedeMemoryResponse { string id = 1; string short_id = 2; }

message ArchiveMemoryRequest  { string id = 1 [(buf.validate.field).string.min_len = 1]; }
message ArchiveMemoryResponse { string id = 1; string short_id = 2; string outcome = 3; } // "changed"|"already" — mirrors store.ArchiveOutcome
message RestoreMemoryRequest  { string id = 1 [(buf.validate.field).string.min_len = 1]; }
message RestoreMemoryResponse { string id = 1; string short_id = 2; string outcome = 3; }

message ListRulesRequest  { repeated string scopes = 1; repeated string tags = 2; bool full = 3; }
message ListRulesResponse { repeated Memory rules = 1; string advisory = 2; }

message ListScheduledRequest  { string scope = 1; string state = 2; /* + cross_spine, created window, limit */ }
message ListScheduledResponse { repeated Memory memories = 1; }

message RelatedMemory { Memory memory = 1; string edge_type = 2; double score = 3; optional string label = 4; } // label: Jev same_subject, advisory
message RelatedMemoriesRequest  { string id = 1 [(buf.validate.field).string.min_len = 1]; uint64 k = 2; }
message RelatedMemoriesResponse { repeated RelatedMemory related = 1; }

message TagCount { string tag = 1; uint64 count = 2; }
message ListTagsRequest  { string scope = 1; bool cross_spine = 2; }
message ListTagsResponse { repeated TagCount tags = 1; bool approximate = 2; } // approximate mirrors MigrateStatus's truncation-vs-not distinction
```

## Gap 1: Archive/Restore have no authz-checked path today

`store.Archive`/`store.Restore` (`spine.go:773-813`, `:836-875`) take only `(ctx, id)` — no `Subject`. The CLI calls them directly (`spine_review_archive.go:211`, `:242`) as a trusted, subject-less administrative sweep, the same privilege class as `spine-review purge`/`migrate-remap-owner`. Exposing "archive/restore this record" as a per-caller Connect write RPC is **new authz surface**, not a thin wrapper — it needs the same `getWritable(ctx, id, subj, authz.Action)` gate every other per-record write already uses (`store.go:2195-2208`, used by Delete `:2973`, Update `:2268`, SetVisibility/Share `:2558`, Supersede-per-target `:2763`). Recommend: add a new `authz.Action` (e.g. `ActionArchive`) or reuse `ActionWrite`, and either (a) add `Store.ArchiveOwned(ctx, id, subj)`/`RestoreOwned` that gate-then-delegate to the existing unlocked-write internals `Archive`/`Restore` already use, or (b) thread `subj Subject` into `Archive`/`Restore` directly with a privileged bypass value for the CLI's existing call sites. Whichever is chosen, **this is Store-layer work that must land before the Connect RPC**, not console/proto work.

## Gap 2: `ListTags` needs a payload index AND widens a recall-safety gate

Two independent Qdrant-level facts, both confirmed from code:

1. **No `tags` payload index exists today.** `ensureIndexes` (`store.go:657-687`) creates indexes for `owner`, `scope`, `created_at`, `short_id`, `schema_version` only — `tags` is absent. `qdrant.NewMatch("tags", t)` filtering works today without an index (unindexed scan), but Qdrant's Facet API needs the field indexed to be efficient and is the standard way engram already adds indexes idempotently at boot (`store.go:638-641`, "Indexes are ensured on every boot... so existing collections gain them without a data migration"). Add `{"tags", qdrant.FieldType_FieldTypeKeyword, nil}` to `ensureIndexes`'s `idxs` slice — additive, boots clean on every existing deployment.
2. **`qdrant.FacetCounts` DOES support a `Filter` field** (`go-client@v1.19.2/qdrant/points.pb.go:7899-7973`, `Filter *Filter` + `GetFilter()`), and Qdrant's own field comment says the filter is honored ("return only those points that satisfy the specified conditions") — so **facet counts DO respect the filter**, which is exactly what authz-scoped tag counts need. BUT: `Store.MigrateStatus`'s existing Facet call carries no filter on purpose, and a companion test — `recallCaptureInterceptor` in `schemaversion_recallgate_test.go:868-907` — `t.Fatalf`s on ANY gRPC request that implements `GetFilter()` and returns non-nil unless its method name is in `recognizedFilterCarryingRequestMethods` (currently `{Query, Scroll, Count}` — no `Facet`). **`ListTags`'s filtered Facet call will fail this test until `Facet` is added to that map and to the interceptor's type switch** (`schemaversion_recallgate_test.go:891-907`). This is a real, name-it-in-the-plan integration point, not a hypothetical.

## Pattern: `RelatedMemories` as one RPC, authz composed inside the store

**What:** One RPC, one store method, returning a flat list of typed edges — not four separate RPCs per edge type. `Store.RelatedMemories(ctx, id, subj, opts)`:

1. `GetReadable(ctx, resolvedID, subj)` the source record first (authz-checked fetch, same not-found-echo discipline as `deps.getMemory` — `tools.go:2240-2258`). 404s here if the caller can't read the source at all.
2. Build the SAME authz filter `Search` builds: `f := s.ownerScopeFilter(ctx, "", subj)` (no scope restriction — related memories may legitimately live in a different scope the caller can also read) plus the same soft-hide defaults (`archived_at`/`superseded_by`/active-window IsEmpty conditions) `Search` applies (`store.go:1223-1248`).
3. Collect four edge-type candidate ID sets under that ONE filter:
   - **Supersession, both directions:** `source.SupersededBy` (forward) + `source.Supersedes` (backward) — these ids are already on the fetched `Memory` (proto fields 23/24; store fields), no extra query.
   - **Shared tags:** a `Scroll`/`Query` with `Must` = `f.Must` + one-or-more `NewMatch("tags", t)` per shared tag, `MustNot` self.
   - **Shared citations** (discovery-kind only): same shape, matching on citation `ref`.
   - **Vector neighbours:** `qdrant.NewQueryID(qdrant.NewID(id))` sub-query exactly like `NearDuplicates` (`spine.go:632-644`), but with `f` (the authz filter) as the sub-query's `Filter`, not `NearDuplicates`'s subject-less scope-only filter.
4. Union the candidate IDs and fetch payloads via the SAME `fetchPayloadsByID(ctx, f, view, ids)` helper `Search` already uses (`store.go:1256-1264,1280`, comment: "the fetch can never see a record the query's own filter would have excluded"). **This is the enforcement point** — a candidate the caller cannot read is never returned by the store call at all, so the handler has nothing to post-filter. This directly satisfies "neighbours must be filtered by the caller's read predicate inside the store, never post-filtered in the handler."
5. Attach `edge_type` per candidate (a candidate can legitimately appear via more than one edge type; either emit one row per edge type or dedupe-and-list types — decide in planning, not research) and a `score`: vector edges carry the real cosine score from the `QueryID` sub-query; supersession edges get a fixed 1.0 (deterministic, not a similarity); shared-tag/citation edges get a cheap overlap fraction (`|intersection| / |union|` or `/min(|A|,|B|)`), computed in Go, not Qdrant. The advisory Jev `same_subject` label (`internal/verdict`'s existing question, reused read-only — never a new decide call inside this hot path unless deliberately budgeted) is a SEPARATE optional `label` field, unset unless a decision provider is configured and the call succeeded — mirroring `Memory.relevance`'s "optional, unset unless ran and succeeded" contract (`engram.proto:58-62`).

**`k` semantics:** bound the vector-neighbour sub-query's `Limit` (mirror `SearchMemories`'s `k`: 0→20 default, `rejectOverMaximum` ceiling 1000 — `store.go:1219`); shared-tag/citation edge counts are naturally bounded by tag/citation cardinality and don't need a separate `k`.

**Why one RPC, not several:** every edge type needs the identical authz filter composed once; splitting into four RPCs would either duplicate that composition four times or force the UI to make four round-trips to render one graph view. A single `RelatedMemory{memory, edge_type, score, label}` repeated field is also what a force-directed/graph UI component wants as its one data source.

## Pattern: query-understanding as a server-side, budgeted `internal/decide` consumer

**Where it lives:** server-side only, mirroring `internal/relevance`'s rerank hook (`decider.go:198-247`) — never the browser. The Jev API key and base URL are server config (`ENGRAM_DECISIONS_*`); a client-side call would leak the key. This means a new Connect RPC (e.g. `UnderstandQuery(text) → {categories, created_after, created_before, scope, advisory_disclaimer}`), not a client library call.

**Question shapes:** `internal/decide`'s vocabulary is closed — `Noul` (binary), `Choice` (single-select over a finite option map), `Score` (ordered levels) (`decide.go:41-101`). This is a poor fit for open-vocabulary tag extraction (tags have no finite option set), so:
- **Categories** (`decision|preference|convention|gotcha`, OR-matched in the store — `engram.proto:147`): model as up to four `Noul` questions ("does this text ask about a `decision`/`preference`/`convention`/`gotcha`?") batched into ONE `decide.Request` (mirrors `internal/relevance`'s per-candidate `Noul` batching, `relevance.go:135`) — one `Decide` call, not `DecideMany` (there is only one State: the query text).
- **Time window:** one `Choice` question over a small finite bucket set (`{"none","today","this_week","this_month","this_year"}`), converted to `created_after`/`created_before` server-side — mirrors `internal/verdict`'s `QuestionRelation` `Choice` usage (`verdict.go:155`).
- **Scope:** one `Choice` question whose option map is built from the caller's REAL `ListScopes` result (so the model can only pick a scope the caller can actually read) — never a free-text scope guess.
- **Tags: out of scope for the decide call.** No typed-decision shape fits open vocabulary; if tag-chip suggestion is wanted at all, it is a separate non-decide heuristic (simple keyword tokenization against the caller's own known tag vocabulary from `ListTags`), kept explicitly outside the advisory/provider-neutral contract.

**Timeout budget:** reuse the search-rerank pattern exactly — a SECOND, dedicated Jev client built with its own short timeout and `jev.WithNoRetry()` (`decider.go:207-224`, the `searchDeciderFromConfig` precedent), NOT the longer sweep-path `deciderFromConfig` client. Default 2s (matching `searchRerankTimeout`'s default — `decider.go:156-166`), configurable via a new `ENGRAM_QUERY_UNDERSTANDING_TIMEOUT`-shaped knob following the exact same parse/validate/default pattern as `searchRerankTimeout`.

**Fallback = no chips:** a `Decide` error, timeout, or a nil/rejected answer map means the RPC returns zero suggested filters — never an error to the caller, mirroring `applyRankHook`'s "a hook error or rejected map never fails the surrounding search" contract (`rerank.go:172-204`). The RPC itself should probably never fail at all except on request validation — a decide-layer problem degrades to an empty suggestion, not a Connect error.

**How the UI keeps it advisory:** the RPC returns SUGGESTED chips; the SPA renders them as removable/toggleable UI state that, once confirmed, becomes ordinary `SearchMemoriesRequest` fields the user already controls (`categories`, `created_after/before`, `scope`) — never auto-applied to the live search. This is the same shape `spine-review consolidate`'s verdict already uses: "structurally unable to mutate" (PROJECT.md).

## Pattern: SPA integration — URL-as-state, TanStack Query keys, id resolution

**URL-as-state (existing, extend, don't replace):** `search/+page.svelte` already derives `q`/`scope`/`sel` from `page.url.searchParams` (`+page.svelte:13-23`) and calls `goto` on selection change (`:24-27`). Add `facets` (a serialized filter set — categories/tags/time window/state-words/scope) as additional query params using the same derive-then-`goto` shape; this is what makes a search shareable/back-button-able and is already the established pattern, not a new one.

**TanStack Query key convention (existing, confirmed across every route):** `[rpcCamelCaseName, ...args]` — `['searchMemories', query, scope]` (`+page.svelte:18`), `['getMemory', id]` (`:22`), `['listScopes']`, `['migrateStatus']`, `['listMemories', scope, categories, visibility, limit, offset]` (comment at `mutations/memory.ts:255-259` documents the positional key shape, including that `key[3]` is the visibility filter, read by `applyToMemoryCaches`). New RPCs must follow this positional-args convention exactly (`['relatedMemories', id, k]`, `['listTags', scope]`) so the existing cache-invalidation helpers (`applyToMemoryCaches`, `snapshotMemoryQueries`) can be extended rather than re-invented.

**Mutation hook shape (existing, confirmed, copy exactly):** `mutations/memory.ts:275-416` — every write RPC gets a `useXMutation()` returning a `createMutation` with `mutationFn` → `engramWrite.x(...)`, `onMutate` (cancel in-flight, snapshot via `snapshotMemoryQueries`, apply an optimistic patch via `applyToMemoryCaches`), `onError` (rollback via `restoreMemoryQueries`, toast), `onSuccess` (toast), `onSettled` (invalidate `listMemories`/`searchMemories`/`getMemory`/`listScopes` as relevant). `useArchiveMemory`/`useRestoreMemory` should mirror `useSetMemoryVisibility` (`memory.ts:364-395`) most closely: an optimistic patch to a derived-state field, with the SAME "drop from a filtered list page whose filter no longer matches" rule `applySetVisibilityOptimistic` already implements for `include_archived`-style filtering (`memory.ts:260-269`). `useSupersedeMemory` needs an invalidation fan-out across the survivor id AND every target id (no existing mutation invalidates more than one id today — new code, not a copy).

**id/short_id/text resolution: client-side, no new "resolve" RPC needed.** `Store.ResolvePointID` (`store.go:2109-2138`) already tries `uuid.Parse` first, then treats the input as a `short_id` via `shortid.Canonical` — and `GetMemory`/`DeleteMemory`/etc. already call it transparently (`tools.go:2244`, `getMemory`). **`GetMemory` already accepts either a full UUID or a short_id from the client today** — the 2026-09-25 "searching by an id errors" bug (`console-overhaul-exploration.md`) is a pure CLIENT ROUTING bug (the UI always calls `SearchMemories`, never `GetMemory`), not a missing server capability. Fix: client-side heuristic (UUID regex, or a bare 10-char Crockford-base32-shaped token per the memory contract's `short_id` spec) routes an id-shaped entry to `GetMemory` first, falling back to `SearchMemories` on `NotFound` (never the reverse — a `GetMemory` 404 is a legitimate "not that id" answer to try as free text; a `SearchMemories` empty result is not evidence the input wasn't a valid id). No proto change, no new RPC.

**CSRF/interceptor stack for any new write RPC:** every one of `SupersedeMemory`/`ArchiveMemory`/`RestoreMemory` MUST be added to `csrfWriteProcedures` (`connectcsrf.go:33-39`) — the interceptor's fallback for an unlisted procedure is "pass through untouched" (`connectcsrf.go:59-62`), i.e. an omission is a silent CSRF hole, not a build failure. The UI must call these three through `engramWrite` (the `[retryOnce, attachCsrf]` transport, `client.ts:21-26`), never the read-only `engram` client.

## Anti-Patterns (specific to this integration)

### Anti-Pattern: post-filtering related/facet results in the Connect handler
**What people do:** fetch candidate ids with a subject-less or under-filtered Qdrant query, then drop unauthorized ones in `engramAPI.RelatedMemories` before building the response.
**Why it's wrong:** the milestone's own stated constraint ("authz enforced in internal/store only") exists because a handler-level filter is trivially forgettable on the next change and is not covered by the same store-level test suite that proves `Search`/`List` never leak. It also means a caller can observe HOW MANY unauthorized neighbours existed (via timing, counts before truncation, etc.) even if the payloads are stripped.
**Do this instead:** compose the authz filter into every Qdrant request the store issues (`ownerScopeFilter` + `fetchPayloadsByID`, per the `RelatedMemories` pattern above) so an unauthorized record is never fetched, not merely never shown.

### Anti-Pattern: a client-side "resolve id vs text" RPC
**What people do:** add a `ResolveIdentifier(token) → {kind, id}` RPC because the client "needs to know" whether input is an id.
**Why it's wrong:** duplicates `ResolvePointID`'s logic in a second place (a UUID/short_id parser that must stay in sync with `internal/shortid`'s canonicalization rules), and adds a round-trip before the round-trip that actually fetches data.
**Do this instead:** try `GetMemory` first for id-shaped input; treat its `NotFound` as "wasn't an id, search instead." The server already does the real parsing.

### Anti-Pattern: computing the Jev query-understanding decision from the browser
**What people do:** call the decide provider (or a thin proxy of it) directly from `ui/` to avoid a server round-trip.
**Why it's wrong:** leaks `ENGRAM_DECISIONS_API_KEY`/base URL to the browser and breaks the provider-neutral, server-owned wiring every other decide consumer uses (`decider.go`).
**Do this instead:** a Connect RPC that wraps a server-side `decide.Decider`, exactly like the search-rerank hook.

### Anti-Pattern: forgetting the vendored-asset and generated-client drift gates
**What people do:** land the proto + Go handler in one PR, then a follow-up PR for the UI, without re-running `task proto:gen` + the `cp -R gen/ts/. ui/src/lib/gen/` step in between, or landing `ui/src` changes without `task ui:build`'s output committed.
**Why it's wrong:** CI's `buf` job diffs `ui/src/lib/gen/` against a fresh `cp -R gen/ts/.` and fails "ui/src/lib/gen/ is stale" (`ci.yaml:262-266`); the `ui-drift` job separately rebuilds the SPA and diffs `internal/webauth/static/`, self-healing ONLY on Renovate PRs and otherwise failing with "vendored SPA is stale — run `task ui:build` and commit" (`ci.yaml:302-401`).
**Do this instead:** every phase that changes `proto/engram/v1/engram.proto` runs `task proto:gen` and commits the regenerated `gen/go`, `gen/ts`, AND the copied `ui/src/lib/gen/` in the SAME commit; every phase that changes `ui/src` runs `task ui:build` and commits `internal/webauth/static/` in the same commit.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---|---|---|
| Qdrant Facet API | `qdrant.FacetCounts{CollectionName, Key, Filter, Limit, Exact}` via `s.client.Facet` | Requires a payload index on the faceted field (add `tags` to `ensureIndexes`, `store.go:657-687`); filter IS honored by Facet per the field's own doc comment; `MigrateStatus`'s existing call is the only precedent and deliberately carries no filter — do not copy its no-filter shape for `ListTags` |
| Qdrant Query-by-id sub-query | `qdrant.NewQueryID(qdrant.NewID(id))` as `QueryPoints.Query`, with `Filter` for both scope AND `MustNot(NewHasID(id))` self-exclusion | Precedent: `NearDuplicates` (`spine.go:632-644`) — subject-less; `RelatedMemories` must add the authz `Must`/`Should` conditions this precedent lacks |
| Jev / `internal/decide` | A dedicated short-timeout, no-retry `decide.Decider` built via `jev.New(..., jev.WithTimeout(shortDur), jev.WithNoRetry())` | Precedent: `searchDeciderFromConfig` (`decider.go:198-224`) — build a SECOND client, don't reuse the sweep-path one |

### Internal Boundaries

| Boundary | Communication | Notes |
|---|---|---|
| `proto/engram/v1/engram.proto` ↔ `gen/go`, `gen/ts` | `task proto:gen` (buf) | `buf breaking` gate; additive-only; a deprecated field number is never reused |
| `gen/ts` ↔ `ui/src/lib/gen/` | `cp -R gen/ts/. ui/src/lib/gen/` (manual copy step, checked by CI) | Drift gate: `ci.yaml:262-266` |
| `ui/src` ↔ `internal/webauth/static/` | `task ui:build` (SvelteKit build, copied in) | Drift gate: `ci.yaml:302-401`, self-heals ONLY on `renovate/*` PRs |
| `internal/server/tools.go` (MCP) ↔ `internal/server/connectapi.go` (Connect) | Both call the same `deps.*` method | Parity proven by `connectapi_write_parity_test.go`/`connectapi_parity_test.go`-family tests comparing store-call traces across two independently-seeded spy stores (`connectapi_write_parity_test.go:1-80`) |
| `internal/authz` (Cedar) ↔ `internal/store` | `decideBucket`/`decideRecord` → `ownerScopeFilter`/`ownerOrSharedCondition`/`ownerOnlyCondition`/`getWritable` | Never bypassed in a handler; every new per-record write RPC must gate through `getWritable` with an explicit `authz.Action`, never a hand-rolled ownership `==` check (CLAUDE.md: "Do NOT... an ownership comparison (DEC-cgb)" pattern echoed at `connectapi.go:455`) |

## Suggested Build Order

Respects "RPCs before the UI that consumes them" and "results row before curation and graph" from the milestone brief, plus the dependency chains this research surfaced:

1. **Store-layer prerequisites (no proto, no UI):**
   - Add `tags` to `ensureIndexes` (Gap 2).
   - Add `Facet` to `recognizedFilterCarryingRequestMethods` + the interceptor type switch (Gap 2) — this is a TEST-INFRASTRUCTURE change that must land before or alongside `ListTags`'s implementation, or that implementation cannot pass CI.
   - Design and land the authz-gated Archive/Restore path (Gap 1) — new `authz.Action`, `getWritable`-gated method(s), decide whether the CLI keeps the subject-less bypass.
   - Land `Store.RelatedMemories` and `Store.ListTags(scope, subj)` as pure store methods with unit/integration tests, no RPC yet.
2. **Proto + Connect + MCP-parity decisions, one RPC at a time (or batched, but proto-gen'd together):**
   - `SupersedeMemory` first (zero new store work, straightforward parity row, de-risks the CSRF-allowlist step).
   - `ArchiveMemory`/`RestoreMemory` (depends on step 1's authz work).
   - `ListRules`/`ListScheduled` (near-zero risk, existing `deps.*`, decide the `cross_spine`-widening question for `ListScheduled`).
   - `RelatedMemories`, `ListTags` (depend on step 1's store methods).
   - For each: regenerate (`task proto:gen`), vendor into `ui/src/lib/gen/`, add to `csrfWriteProcedures` if it mutates, add/skip an MCP tool (explicit decision per RPC), add a parity-test row or a documented asymmetry.
3. **UI — recall/results row first** (per the milestone's own audience ordering: developer recall → operator curation → newcomer browsing):
   - Entry-point fix (client-side id/short_id/text routing — no RPC dependency, can land independently and early).
   - Dense results row consuming already-existing `SearchMemoriesRequest` fields the UI doesn't yet send (tags/categories/time window/`cross_spine`/`include_*`) — zero new RPCs needed, pure UI work, should land BEFORE any of the new RPCs' UI to prove out the row/detail-pane/keyboard-nav shape on data that already flows.
4. **UI — curation surfaces** (needs step 2's write RPCs): supersede-with-chain view, archive/restore actions, rules/scheduled list views — each gets its own `useXMutation` per the established hook shape.
5. **UI — graph and tag cloud** (needs step 2's `RelatedMemories`/`ListTags`): visual related-memories view, tag cloud with counts, scope autocomplete (the last of these needs no new RPC — `ListScopes` already returns counts).
6. **Query understanding** last: it is the least load-bearing capability (advisory chips over filters steps 3 already renders), has the most design risk (Gap in this document: Choice/Noul batching has no direct precedent for this exact use), and benefits from every other filter/facet surface already existing to populate its option sets (scope choices from `ListScopes`, tag suggestions from `ListTags`).

## Sources

- `proto/engram/v1/engram.proto` (full read) — existing 12-RPC service, message shapes, additive-field conventions
- `internal/server/connectapi.go:1-579` (full read) — handler patterns, interceptor stack/order, write-RPC adapter shape
- `internal/server/connectcsrf.go:28-62` — hand-maintained write-procedure allowlist
- `internal/server/decider.go:1-443` (full read) — the two-Jev-client (sweep vs. search) pattern, timeout/budget resolution helpers
- `internal/decide/decide.go:1-163` (full read) — Noul/Choice/Score question vocabulary, advisory contract
- `internal/relevance/relevance.go:1-100+` — per-candidate `Noul` batching into one `decide.Request`, `Hook`/`Budget` shape
- `internal/verdict/verdict.go:155-156` — `Choice`/`Noul` usage for `same_subject`/`relation`
- `internal/store/store.go` — `ownerScopeFilter`/`ownerOrSharedCondition`/`ownerOnlyCondition` (:922-1045), `Search` (:1190-1304), `getWritable` (:2195-2208) and its call sites, `Get`/`GetReadable`/`ResolvePointID` (:2075-2208), `ensureIndexes` (:638-687)
- `internal/store/spine.go:540-875` — `NearDuplicates`'s `NewQueryID` sub-query pattern, `Archive`/`Restore` (subject-less)
- `internal/store/migrate_status.go:1-233` (full read) — `qdrant.Facet` call shape and its deliberate no-filter constraint
- `github.com/qdrant/go-client@v1.19.2/qdrant/points.pb.go:7899-7982` — `FacetCounts.Filter` field and `GetFilter()`
- `internal/store/schemaversion_recallgate_test.go:840-917` — `recognizedFilterCarryingRequestMethods`/`recallCaptureInterceptor`, the gate a filtered `Facet` call must widen
- `cmd/engram/spine_review_archive.go:1-262` (full read) — CLI's subject-less `Archive`/`Restore` call sites
- `internal/server/tools.go` (`getMemory` :2240, `listScheduled` :1867, `listRules`/rules.go, `supersedeMemory` :2563) — existing `deps.*` wrappers the new RPCs reuse
- `internal/server/connectapi_write_parity_test.go:1-80` — MCP↔Connect store-trace parity harness shape
- `ui/src/routes/search/+page.svelte:1-68` (full read) — URL-as-state, TanStack query-key convention
- `ui/src/lib/client.ts:1-35` (full read) — `engram`/`engramWrite` transport split, interceptor order
- `ui/src/lib/mutations/memory.ts:1-416` (full read) — mutation-hook shape, query-key convention, optimistic-cache helpers
- `.github/workflows/ci.yaml:258-401` — `buf`/`ui-drift` generated-artifact drift gates
- `Taskfile.yaml:20-33` — `ui:build`'s vendoring step
- `.planning/notes/console-overhaul-exploration.md` — milestone decisions, the two live entry-point bugs, API-gap table
- `.planning/PROJECT.md` — milestone goal/target features, prior-milestone decide/authz conventions (advisory-only, off-by-default)

---
*Architecture research for: engram console overhaul (2026-09-25.01)*
*Researched: 2026-09-25*
