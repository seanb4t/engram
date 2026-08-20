# Phase 7: Console & CLI State Surfacing - Research

**Researched:** 2026-08-20
**Domain:** Go Connect RPC service extension + protobuf codegen + Svelte 5 console + cobra CLI, over an existing typed-core/store architecture
**Confidence:** HIGH (every claim below was verified by reading the cited file/line this session; no library research was needed — this phase adds zero new dependencies and extends four already-mapped subsystems)

## Summary

This phase has no new-technology risk — every piece of machinery it needs already exists and is
already used in an analogous way somewhere in this codebase. The work is entirely "extend an
established seam in a way that doesn't fold two deliberately-separate things together." The
seams are: (1) `internal/store`'s recall gate, where two `Must` conditions per call site become
conditional on three new bools; (2) the Connect service, which gains a sixth read RPC over an
existing store computation; (3) the CLI's typed operator-view renderer, which already accepts
`json.RawMessage` and therefore already works over a protobuf message with zero new
serialization code; and (4) the console, whose vendored TS types already carry every field this
phase renders — the only vendoring event required is triggered by the *new* proto fields (the
six request bools and the migration-status RPC), not the ones Phase 5 already shipped.

The highest-risk mechanical detail is **not** in the store or the proto — it is in
`cmd/engram/cmdwalk.go`/`toolclass.go`/`catalog.go`: **every new CLI command must get a row in
`internal/surfaces/toolclass.go`'s `operations` table or `buildCatalog` panics at test time.**
This is easy to miss because the panic only fires when `TestCatalogBlastRadiusMatchesToolClasses`
(or any test invoking `buildCatalog`) runs, not at compile time. `engram get` and the client-tier
migration verb both need one row apiece.

The second-highest risk is the store-level detail the UI-SPEC already resolved but that is easy
to under-implement: `include_scheduled=true` must relax **both halves** of
`activeWindowConditions` (`not_before` AND `not_after`), not just the `not_before` Should-clause —
otherwise an expired record revealed by the flag renders with no state marker at all, which is
exactly the bug the UI-SPEC's "Resolved — `expired`" section exists to prevent.

**Primary recommendation:** Thread the three bools through `store.SearchOptions`/`store.ListOptions`
(already parameter structs on the exact two methods in scope — no new positional params, no store
signature break), gate `Store.Search`/`Store.List` only (not `SearchDiscovery`/`ListScheduled` —
confirmed as four gate sites but only two are in D-01's scope), add one Connect-only RPC
(`MigrateStatus`) that calls the existing `Store.MigrateStatus` directly with no owner filter, and
reuse `renderOperatorView`'s `json.RawMessage(protojsonBytes)` adapter verbatim for `engram get`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Recall-gate relaxation (3 bools) | API / Backend (`internal/store`) | API / Backend (Connect handler + typed core) | The gate is a Qdrant filter condition built in `internal/store`; the bools travel Connect proto -> Connect handler -> `coreListRequest`/`coreSearchRequest` -> `store.ListOptions`/`SearchOptions`. No browser-tier logic decides gating — the flags are opaque booleans by the time they leave the sidebar. |
| Migration histogram transport | API / Backend (`internal/store` + Connect) | — | `Store.MigrateStatus` already computes the histogram; this phase adds one Connect RPC over it. No new computation. |
| State-word derivation (archived/superseded/expired/scheduled) | Frontend Server / Browser (console) **and** CLI process (Go) | — | D-13 locks this as a PURE, field-presence-only derivation duplicated independently on both surfaces — never a shared generated table (rejected explicitly in D-13). Each surface reads the wire fields it already has. |
| `engram get` rendering | CLI process (Go) | — | `renderOperatorView` + the `json.RawMessage` adapter; no server change (`GetMemory` is already ungated). |
| Migration banner / sidebar toggles / state badges | Browser (Svelte components) | Frontend Server (SvelteKit static adapter — no SSR data fetching in this codebase) | `ui/` uses `@sveltejs/adapter-static` — there is no SSR tier fetching data server-side; every query in `observe/+page.svelte` runs client-side via `@tanstack/svelte-query` against the Connect Web transport. |
| CLI STATE column / advisory footer | CLI process (Go) | — | Text-lane-only, per D-08's explicit rejection of a wire-level count field. |

## User Constraints

<user_constraints>

### Locked Decisions (from 07-CONTEXT.md, verbatim)

- **D-01:** The phase relaxes the recall gate behind an explicit opt-in, rather than rendering
  state only where a record is already reachable. Reversibility: one-way (published wire fields).
- **D-02:** The opt-in is three orthogonal plain bools — `include_archived`, `include_superseded`,
  `include_scheduled` — false meaning today's behavior, on both `ListMemoriesRequest` and
  `SearchMemoriesRequest`. Each flag maps 1:1 onto one gate condition. Reversibility: one-way (six
  published field numbers).
- **D-03:** The flags are exposed on Connect and the CLI only. MCP tool schemas are unchanged.
  Reversibility: reversible (additive later).
- **D-04:** Authorization stays orthogonal to state. The three flags relax only the state gates;
  the owner/shared authz filter is untouched. Reversibility: reversible.
- **D-05:** Pending-migration state reaches the console through a new Connect read RPC returning
  the histogram `Store.MigrateStatus` already computes. Client-side derivation from listed records
  was rejected (misses the absent-key legacy bucket, wrong page-only scope). Reversibility:
  one-way (published RPC).
- **D-06:** The RPC is readable by any authenticated caller, returning the whole-collection
  histogram: version buckets plus totals, no owner filter. An admin-only gate is NOT a cheap
  alternative — `internal/authz/entities.go:42-47` intentionally omits `roles` from the principal
  entity so `tenant_isolate` stays vacuous; populating roles is reserved work. Reversibility:
  costly (narrowing later breaks callers).
- **D-07:** In the console, migration state renders as a banner in `AppShell.svelte`, on every
  route, only when there is something to say — silent at zero. Two distinct conditions: records
  BEHIND current version (advisory) vs records AHEAD of it (stronger warning — binary too old).
  Reversibility: reversible.
- **D-08:** The CLI gets both a client-tier verb for the full histogram AND a one-line advisory
  footer on `search`/`list` when a backlog exists. Accepted cost: one extra RPC per call for the
  footer. Piggybacking a count onto the hot response messages was rejected. Reversibility:
  reversible.
- **D-09:** `engram get <id>` is added over the existing `GetMemory` RPC, accepting a full UUID or
  a `short_id` exactly as the MCP tool does. `GetMemory` is the only ungated read path.
  Reversibility: reversible.
- **D-10:** `engram get` renders one record through `renderOperatorView`; `search`/`list` keep
  `renderMemoryTable`. The adapter is one line: `viewFields(doc any)` calls `json.Marshal(doc)`,
  and `encoding/json` returns a `json.RawMessage` verbatim, so `json.RawMessage(protojsonBytes)`
  reuses the whole view while preserving protojson's Timestamp and `optional` semantics.
  Reversibility: reversible.
- **D-11:** The headline sanitization hole (#505) is closed structurally, in this phase. Route
  `renderOperatorView`'s headline through `sanitizeViewValue` so the property holds by
  construction. Reversibility: reversible.
- **D-12:** `renderMemoryTable` gains an always-present STATE column, blank for live records
  (Sean's choice over the conditional-column recommendation). This changes today's default text
  output for every user — contractually free per Phase 6 D-03 (text is not a stable interface).
  `schema_version` stays OUT of the table. Reversibility: reversible.
- **D-13:** The wire is the vocabulary. Both surfaces derive a record's state label from the same
  proto field being set: `archived_at` present -> `archived`, `superseded_by` present ->
  `superseded`, `not_before` in the future -> `scheduled` (UI-SPEC extends this with a fourth word,
  `expired`, when `not_after` is in the past — see below). No shared string table, no codegen. Gate
  with a test per surface asserting the derivation. Reversibility: reversible.
- **D-14:** `MemoryDetail.svelte` gets a dedicated State section, rendered only when the record
  carries non-default state, plus `schema_version` as an always-present operator field (fourth
  meta chip). Reversibility: reversible.
- **D-15:** `MemoryRow.svelte` uses an explicit state marker in the meta line, plus a dimmed row
  treatment for archived/superseded rows (Sean's choice over marker-only). The marker carries the
  signal to a screen reader; dimming stays strictly decorative. "Archived AND superseded" needs a
  DEFINED dimming rule (see UI-SPEC precedence table below — resolved, not open). Reversibility:
  reversible.
- **D-16:** The three include toggles live in `ScopesSidebar.svelte`, beside the existing
  categories and visibility filters — inherits the existing URL round-trip
  (`parseObserveParams`/`observeSearch`) and query-cache keying (`listMemoriesKey`) for free.
  Reversibility: reversible.

### Claude's Discretion (from 07-CONTEXT.md, verbatim)

- The exact names of the new RPC and its request/response messages, and the shape of the
  histogram bucket message (`Store.MigrateStatus`'s existing result type is the obvious source).
- The client-tier migration verb's name (`engram status`, `engram migration-status`, or a
  client-tier sibling of `engram migrate status`) — resolve against the existing verb naming and
  the toolclass registry rather than by preference.
- Field numbers for the six new request fields, and whether `cross_spine` composition with the
  three flags needs an explicit guard or is naturally orthogonal.
- Whether `engram get` accepts multiple ids, and whether it needs a `--full` flag or always
  returns full content.
- How the footer's extra RPC is scheduled — sequential, concurrent, or best-effort. A failed
  footer lookup must never fail the command.
- Whether the banner polls, refetches on route change, or fetches once per session.
- The exact wording of the two banner conditions and the footer line.
- Whether the STATE column renders one composed value or a fixed-width flag set for a compound
  state.
- Migration/task ordering across plans, and whether the store/proto/CLI/console work splits by
  tier or by capability.

### Deferred Ideas (OUT OF SCOPE, from 07-CONTEXT.md, verbatim)

- The three include flags on the MCP tool schemas (`list_memory`, `search_memory`) — D-03 keeps
  agent recall zero-junk by default; additive later.
- Porting `search`/`list` off `renderMemoryTable` onto the view mechanism — D-10 adopts the view
  for `get` only.
- Piggybacking a pending-migration count onto the hot list/search responses — rejected in D-08.
- An admin/operator role for the migration-status RPC — D-06 opens it to any authenticated caller;
  `roles` is reserved forward-compat work.
- Hardening the red-evidence harness (`internal/store/redevidence_harness_test.go`) — inherited
  from Phase 6, unrelated test infrastructure this phase does not cause.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-console-record-state | The operator console shows a record's archived, superseded, and scheduled state, which it cannot render at all today. | `MemoryRow.svelte`/`MemoryDetail.svelte`/`ScopesSidebar.svelte` mapped below with exact insertion points; vendored TS types already carry every field (line-cited); D-13/UI-SPEC state-derivation rules confirmed against the actual `optional` presence semantics in the generated TS. |
| REQ-cli-record-state | `engram search`/`list`/`get` surface the same state, so the CLI and console agree on what a record is. | `renderMemoryTable`'s exact column-build code cited; `engram get` template (`client_list.go`) cited in full; Go-side field-presence derivation confirmed against `gen/go/engram/v1/engram.pb.go`'s pointer-typed fields. |
| REQ-migration-state-visible | Pending-migration state is visible to an operator through the same surfaces, not only by running the migrate command. | `Store.MigrateStatus` and `MigrateStatusResult` read in full; existing `migrate status` operator command's report-doc/summary pattern cited as the direct template for the new client-tier verb; `warnPendingMigrations`'s two-condition split cited as the banner's semantic precedent. |

</phase_requirements>

## Standard Stack

No new dependency is required or recommended. This phase's "stack" is the four already-adopted
technologies it extends:

### Core (already in go.mod / package.json — versions confirmed this session)

| Library | Version (confirmed) | Purpose | Why no change needed |
|---------|---------|---------|--------------|
| `connectrpc.com/connect` | v1.20.0 (`go.mod:8`) | Connect RPC transport | New RPC is a 12th service method on the same `EngramService`; no new interceptor, no new auth chain. |
| `google.golang.org/protobuf` | v1.36.11 (`go.mod:40`) | protojson / proto codegen | `protojson.MarshalOptions{UseProtoNames:true, EmitDefaultValues:true}` (already used by `renderJSON`, `client_common.go:380-391`) is the exact marshal shape D-10's `engram get` adapter needs. |
| `github.com/spf13/cobra` | v1.10.2 (`go.mod:22`) | CLI framework | `engram get` and the migration verb are new leaf commands following `client_list.go`'s exact shape. |
| `@connectrpc/connect` / `@connectrpc/connect-web` | ^2.1.1 (`ui/package.json:37-38`) | Console RPC client | `createClient(EngramService, transport)` (`ui/src/lib/client.ts:13`) auto-gains the new RPC as a typed method once `task proto:gen` re-vendors — no client code change beyond calling it. |
| `@bufbuild/protobuf` | ^2.12.0 (`ui/package.json:36`) | TS proto runtime (protobuf-es) | Generated `optional` fields already type as `field?: T \| undefined` (confirmed at `ui/src/lib/gen/engram/v1/engram_pb.ts:153-190`) — presence-checkable with a bare `!== undefined` / truthiness check, no extra wrapper library needed. |
| `svelte` | 5.56.6 (`ui/package.json:26`) | Console UI framework | Runes (`$derived`, `$props`) already the idiom in every touched component. |
| `@tanstack/svelte-query` | ^6.1.34 (`ui/package.json:40`) | Console data fetching | `createQuery(() => ({...}))` options-function idiom already used in `observe/+page.svelte:26-33`; the migration banner's mount-time fetch and the sidebar's extra params both fit this pattern with no new library. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Three orthogonal bools (D-02, locked) | A `repeated RecordState` enum | Rejected in CONTEXT: introduces a flag->condition mapping table the 1:1 bool form does not need. Not re-litigated here. |
| New Connect RPC for migration status (D-05, locked) | Client-side derivation from listed records / piggyback on list responses | Rejected in CONTEXT: misses the absent-key legacy bucket and the whole-collection scope. Not re-litigated here. |

**Installation:** none — zero new Go or TS dependencies, matching the "zero new Go dependencies"
invariant held across three consecutive milestones (confirmed in 07-CONTEXT.md's Established
Patterns section).

## Package Legitimacy Audit

**Not applicable.** This phase installs no external packages in any ecosystem — every library
used above is already a direct dependency, confirmed present in `go.mod`/`ui/package.json` this
session. No `npm view`/`pip index`/`cargo search` verification is needed because nothing new is
being added to either lockfile.

## Architecture Patterns

### System Architecture Diagram

```
                         CONSOLE (Svelte, browser-only — adapter-static, no SSR fetch)
   ScopesSidebar ──include flags──▶ observe/+page.svelte ──listMemories(includeX)──┐
   MemoryRow/MemoryDetail ◀──state fields (already-fetched Memory)──────────────────┤
   AppShell (mount) ──migrateStatus()───────────────────────────────────────────────┤
                                                                                     │
                                    Connect-Web transport (same-origin, /  baseUrl)  │
                                                                                     ▼
                    ┌───────────────────────────── engramAPI (internal/server/connectapi.go) ─────────────────────────────┐
                    │  ListMemories(req)         SearchMemories(req)        GetMemory(req)        MigrateStatus(req)      │
                    │      │ req.Msg.IncludeArchived/Superseded/Scheduled       │                     │  (no request      │
                    │      ▼                        ▼                          │                     │   fields needed)  │
                    │  coreListRequest          coreSearchRequest              │                     │                   │
                    │      │                        │                          │                     │                   │
                    │      ▼                        ▼                          ▼                     ▼                   │
                    │  deps.listMemory ──────▶ store.ListOptions      deps.getMemory          a.d.st.MigrateStatus(ctx)  │
                    │  deps.searchMemory ─────▶ store.SearchOptions    (already ungated:            (no owner filter,    │
                    │                                                   GetReadable, no             any authenticated   │
                    │                                                   state gate at all)           caller — D-06)     │
                    └──────────────────────┬──────────────────────────────────────────────────────────────┬─────────────┘
                                            ▼                                                              ▼
                          internal/store: Store.Search / Store.List                          internal/store: Store.MigrateStatus
                          (2 of 4 gate sites relaxed by the 3 bools;                          (existing, unmodified computation —
                           SearchDiscovery/ListScheduled NOT touched — D-03 scope)             Facet + 2 Count RPCs, D-08 upstream)


                                            CLI (cmd/engram, cobra)
   engram search/list ──renderMemoryTable──▶ STATE column (blank if live, else canonical-order csv)
   engram search/list ──extra RPC, best-effort──▶ renderCoverageFooter-shaped advisory line
   engram get <id> ──GetMemory RPC──▶ protojson.Marshal ──json.RawMessage──▶ renderOperatorView (D-10 adapter)
   engram <migration-verb> ──new MigrateStatus RPC──▶ renderOperator (same doc shape as `migrate status`'s operator report)
```

### Recommended Change Map (files touched, not a new directory structure — this phase extends
existing files; no new packages)

```
proto/engram/v1/engram.proto        # +6 bool fields (2 messages), +1 RPC, +2 new messages (request/response), +1 shared VersionBucket-equivalent message
gen/go/, gen/ts/, ui/src/lib/gen/   # regenerated + re-vendored via `task proto:gen` (never hand-edited)

internal/store/store.go             # SearchOptions/ListOptions gain 3 bool fields; Store.Search/Store.List (lines ~1086-1097, ~1328-1333) gate conditionally; activeWindowConditions call becomes conditional on IncludeScheduled
internal/server/tools.go            # coreListRequest/coreSearchRequest gain 3 bool fields (MCP call sites at 2385/2432 leave them unset — zero value = today's behavior); +MigrateStatus is server-side unchanged (already exists)
internal/server/connectapi.go       # ListMemories/SearchMemories map req.Msg.IncludeX into the new struct fields; +MigrateStatus handler (new, follows ListScopes's shape exactly)

cmd/engram/client_get.go            # NEW — engram get, modeled on client_list.go
cmd/engram/client_search.go         # STATE column wiring (renderMemoryTable call site) + footer call
cmd/engram/client_list.go           # same as above, + 3 new --include-* flags
cmd/engram/client_common.go         # renderMemoryTable gains STATE column + a state-derivation helper; footer helper reused as-is
cmd/engram/operator_view.go         # D-11: renderOperatorView's headline line routed through sanitizeViewValue
cmd/engram/<migration-verb>.go      # NEW — client-tier verb, modeled on migrate_family.go's migrateStatusCmd but calling the new Connect RPC
internal/surfaces/toolclass.go      # +2 rows: "get" and the new migration verb's CLICommand key — REQUIRED or buildCatalog panics

ui/src/lib/queries.ts               # ObserveParams + parseObserveParams/observeSearch gain 3 include-flags; listMemoriesKey gains 3 params
ui/src/lib/components/ScopesSidebar.svelte   # +3 checkboxes (D-16)
ui/src/lib/components/MemoryRow.svelte       # +state badges in meta line, +dimming (D-15)
ui/src/lib/components/MemoryDetail.svelte    # +schema chip, +State section (D-14)
ui/src/lib/components/AppShell.svelte        # +banner mount fetch + 2 conditional strips (D-07)
ui/src/routes/observe/+page.svelte           # listQ queryFn passes the 3 new params through
```

### Pattern 1: Extending an options struct instead of adding positional parameters

**What:** `store.SearchOptions`/`store.ListOptions` are already the request-supplied filter
extension point for `Store.Search`/`Store.SearchReranked`/`Store.List` (`internal/store/store.go:1044-1059`
and `:1207-1231`). Adding `IncludeArchived`, `IncludeSuperseded`, `IncludeScheduled bool` fields
to these two structs is a pure additive change — no call site's positional arguments change.

**When to use:** Any time a store method already takes an `Options` struct and the new behavior is
opt-in (zero value = unchanged behavior).

**Example (verified this session, exact lines):**
```go
// Source: internal/store/store.go:1064-1097 (current, before this phase's edit)
func (s *Store) Search(ctx context.Context, scope string, subj Subject, vec []float32, k uint64, opts SearchOptions) (out []Memory, err error) {
	...
	f := s.ownerScopeFilter(ctx, scope, subj)
	f.Must = append(f.Must, activeWindowConditions(s.now())...)          // <- becomes conditional on !opts.IncludeScheduled
	f.Must = append(f.Must, qdrant.NewIsEmpty("superseded_by"))          // <- becomes conditional on !opts.IncludeSuperseded
	f.Must = append(f.Must, qdrant.NewIsEmpty("archived_at"))            // <- becomes conditional on !opts.IncludeArchived
	...
```

### Pattern 2: The typed-core middle layer is where Connect-only exposure is enforced (D-03)

**What:** `coreListRequest`/`coreSearchRequest` (`internal/server/tools.go:1394-1445`) are the
transport-neutral structs both the MCP tool handlers and the Connect handlers build before calling
`deps.listMemory`/`deps.searchMemory`. D-03 ("flags Connect+CLI only, MCP unchanged") is satisfied
structurally: add the three bool fields to these two structs and to `store.ListOptions`/
`SearchOptions`, then only the Connect handler (`connectapi.go` `ListMemories`/`SearchMemories`)
populates them from `req.Msg.IncludeX`. The two MCP `mcp.AddTool` closures
(`internal/server/tools.go:2385` and `:2432`) build `coreSearchRequest{...}`/`coreListRequest{...}`
literals that simply never mention the new fields — they default to `false`, reproducing today's
gated behavior with **zero code change** at those two call sites.

**When to use:** Any "expose on one transport, not another" requirement where both transports
already funnel through one shared typed-core function.

### Pattern 3: `json.RawMessage` as a zero-cost adapter between a typed renderer and a protobuf message

**What:** `viewFields(doc any)` (`cmd/engram/operator_view.go:45-49`) calls `json.Marshal(doc)`.
`encoding/json`'s own contract for `json.RawMessage` is to return its bytes verbatim from
`MarshalJSON()`. So wrapping already-marshaled protojson bytes in `json.RawMessage` before passing
to `renderOperatorView` reuses the entire Phase 6 rendering pipeline with no new serialization
code and no loss of protojson's `Timestamp`/`optional` semantics (which a naive `json.Marshal`
over the Go struct would NOT preserve — protojson and encoding/json render `*timestamppb.Timestamp`
differently).

**Example (the exact adapter `engram get` needs):**
```go
// Pattern confirmed against cmd/engram/client_common.go:380-391 (renderJSON's existing options)
// and cmd/engram/operator_view.go:45-49 (viewFields' json.RawMessage passthrough).
b, err := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}.Marshal(resp.Msg.GetMemory())
if err != nil {
	return err
}
return renderOperatorView(cmd.OutOrStdout(), headline, json.RawMessage(b))
```

### Pattern 4: Field-presence state derivation over pointer-typed generated fields (D-13)

**What:** Every field D-13's derivation reads is a Go pointer (`*string`/`*timestamppb.Timestamp`)
in the generated struct — confirmed at `gen/go/engram/v1/engram.pb.go` (struct tags include
`,oneof` for the two `optional` scalar fields; the three `Timestamp` fields are message-typed,
inherently nilable in proto3). Presence is `field != nil`, never a zero-value comparison on the
dereferenced value (an empty string is not the same as "unset" and — while `superseded_by` never
legitimately holds `""` — checking the pointer keeps the derivation correct by construction rather
than by the current absence of a counter-example).

```go
// Go (CLI) — confirmed field types: gen/go/engram/v1/engram.pb.go (Memory struct,
// SupersededBy *string, NotBefore/NotAfter/ArchivedAt *timestamppb.Timestamp)
m.GetArchivedAt() != nil       // -> archived
m.GetSupersededBy() != ""      // "" and unset are the same non-state in practice, but prefer the pointer/Has-style check if a helper is introduced
m.GetNotAfter() != nil && m.GetNotAfter().AsTime().Before(now)  // -> expired
m.GetNotBefore() != nil && m.GetNotBefore().AsTime().After(now) // -> scheduled
```
```ts
// TS (console) — confirmed field types: ui/src/lib/gen/engram/v1/engram_pb.ts:153-190
// (supersededBy?: string | undefined; notBefore/notAfter/archivedAt?: Timestamp | undefined)
const isArchived = $derived(!!memory?.archivedAt);
const isSuperseded = $derived(!!memory?.supersededBy);
const isExpired = $derived(!!memory?.notAfter && timestampDate(memory.notAfter) < new Date());
const isScheduled = $derived(!!memory?.notBefore && timestampDate(memory.notBefore) > new Date());
```

### Anti-Patterns to Avoid

- **Folding `superseded_by`/`archived_at`/window IsEmpty conditions into one combined condition.**
  `store.go`'s own comments at every gate site explicitly refuse this ("never folded into either").
  The three new bools must gate three *independently toggleable* `f.Must` appends, not one combined
  branch — an "archived OR superseded" single flag would make "just the archive tier" (SC1's own
  wording) unrepresentable.
- **Widening `SearchDiscovery` or `ListScheduled` "for consistency."** Both share the exact same
  `IsEmpty("superseded_by")`/`IsEmpty("archived_at")` idiom (confirmed at `store.go:1191/1195` and
  `:1563/1568`), but neither is in D-01's scope (D-01 names only Search/List). `ListScheduled` also
  builds its filter inline, not via `listFilter`, so it will not accidentally inherit the new
  `ListOptions` fields' gating behavior unless someone explicitly wires it — do not wire it.
- **Relaxing only `not_before` for `include_scheduled`.** `activeWindowConditions`
  (`store.go:1006-1018`) is a single function returning TWO `Should`-wrapped conditions (one per
  window bound). The UI-SPEC's resolved `expired` vocabulary gap depends on `include_scheduled`
  relaxing BOTH — skip the `not_after` half and a genuinely expired record becomes reachable with
  no state word to render, reproducing the exact bug the UI-SPEC's "Resolved — `expired`" section
  fixes.
- **Adding a new CLI command without a `internal/surfaces/toolclass.go` row.** `buildCatalog`
  (`cmd/engram/catalog.go:100-107`) panics — not a soft warning — when `surfaces.ClassForCommand`
  returns `!ok` for any command the live cobra tree contains. This will not surface until a test
  invoking `buildCatalog` runs (e.g. `TestCatalogBlastRadiusMatchesToolClasses`), so it is easy to
  write and locally "run" the new command successfully while leaving this gap.
- **Forgetting `addClientFlags` on the new client-tier commands.** `operatorCommands()`
  (`cmd/engram/cmdwalk.go:101-116`) excludes any command whose flag set declares `"server"` — the
  flag `addClientFlags` registers. `engram get` and the new migration verb MUST call
  `addClientFlags` (client-tier), not `addOperatorOutputFlag` alone, or they will be
  misclassified as operator-tier commands by every gate that derives its work-list from
  `operatorCommands()`.
- **Restating the D-13 vocabulary as a second, hand-maintained string list anywhere.** Both the Go
  and TS derivations above are meant to be the ONLY place each surface computes the word — no
  shared constant, per D-13's explicit rejection of a generated string table.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Rendering one protobuf record as a field table | A bespoke `engram get` formatter | `renderOperatorView` + `json.RawMessage(protojsonBytes)` adapter (D-10, Pattern 3 above) | Already exists, already handles nested arrays/objects/humanized labels, already the text/json-identity-by-construction property Phase 6 built. |
| Migration histogram computation | A CLI-side or console-side scroll-and-count | `Store.MigrateStatus` via the new RPC | Already handles the non-atomic three-RPC reconciliation, the facet-limit truncation detection, and the absent-key legacy bucket (`internal/store/migrate_status.go:65-101`) — reimplementing any part of this client-side reproduces a documented and already-fixed correctness bug class (REVIEWS.md C6-M12/C7-M1 in that file's own comments). |
| CLI blast-radius / self-describe classification | A second hand-written command-safety table | `internal/surfaces/toolclass.go`'s `operations` slice + `ClassForCommand` | The catalog and destructive-command gate both already derive from this one table; a second one would silently diverge from `engram --help`'s own MCP-tool-annotation-parity guarantee. |
| Console filter state -> URL round-trip | New URL-param plumbing for the 3 include flags | `ObserveParams`/`parseObserveParams`/`observeSearch` (`ui/src/lib/queries.ts`) | Already the single shared shape every other filter (scope, categories, visibility, offset, selection) round-trips through; adding 3 fields here is additive, a parallel mechanism is not needed. |
| CLI advisory footer shape | A bespoke printf line for pending-migration counts | `renderCoverageFooter`'s `key: value` shape (`client_common.go:291-320`) as the literal template | D-08's copywriting contract explicitly requires the SAME `key: value` two-space-joined shape "so the two footer lines already precedented in this codebase read as one family" — this is a stated design constraint, not an implementation shortcut. |

**Key insight:** every "don't hand-roll" item in this phase is really the same insight restated
five times: this phase is a consumption pass over machinery Phases 2-6 already built and hardened.
Anywhere this phase's implementation looks like it needs new plumbing, check whether an existing
struct/function/table already has the extension point — in every case audited this session, one
did.

## Common Pitfalls

### Pitfall 1: `buildCatalog` panics on an unclassified new command
**What goes wrong:** `engram get` or the new migration verb ships with no
`internal/surfaces/toolclass.go` row; `TestCatalogBlastRadiusMatchesToolClasses` (or any code path
invoking `buildCatalog`) panics with `"catalog: command %q has no internal/surfaces blast-radius
classification"`.
**Why it happens:** The panic is a runtime/test-time check over the LIVE cobra tree
(`cmd/engram/catalog.go:98-107`), not a compile-time check — a command can be written, wired into
`rootCmd`, and manually exercised successfully with the gap still present.
**How to avoid:** Add one `Operation{MCPTool: "", CLICommand: "get", Class: Class{ReadOnly: true,
Destructive: false, Idempotent: true, OpenWorld: false}}`-shaped row per new command (mirroring
`search`/`list`'s existing rows at `internal/surfaces/toolclass.go:79-85`) in the SAME change that
adds the command.
**Warning signs:** `go test ./cmd/engram/...` fails with a panic naming the exact new command.

### Pitfall 2: New client-tier commands misclassified as operator-tier
**What goes wrong:** A gate that derives its work-list from `operatorCommands()`
(help-golden regeneration, timeout-group tests, the operator `--output` flag conventions) either
silently excludes the new command from operator-tier checks it should be exempt from, or — if
`addClientFlags` is omitted and `addOperatorOutputFlag` used instead — silently INCLUDES a
client-tier command in operator-tier gates it should never be subject to.
**Why it happens:** `operatorCommands()`'s structural filter is "drops any command whose flag set
declares `server`" (`cmdwalk.go:107-109`) — a purely structural signal, not a name-based one.
**How to avoid:** `engram get` and the migration verb must call `addClientFlags(cmd)`
(`client_common.go:42-55`), never `addOperatorOutputFlag`, and follow `client_list.go`'s exact
skeleton (dial via `clientFromFlags`, `wrapRPCError` on failure, `renderCoverageFooter`/
`renderJSON` on the two output paths).
**Warning signs:** `TestOperatorCommands` (asserts `operatorCommandExclusions` is exactly
`{"serve","version"}`) unexpectedly needs a new exclusion entry, or a help-golden diff appears in
an unrelated operator command's output.

### Pitfall 3: `include_scheduled` relaxing only half of `activeWindowConditions`
**What goes wrong:** A record whose `not_after` has already passed becomes reachable via
`include_scheduled=true` but renders with no state marker (invisible expiry) — the exact bug the
UI-SPEC's "Resolved — `expired`" section documents as its own flagged-and-fixed gap.
**Why it happens:** `activeWindowConditions` (`store.go:1006-1018`) is one function returning two
independent `Should`-wrapped conditions; it is easy to skip the whole function call only when
BOTH bounds should stay gated, and forget that D-13 (as extended by the UI-SPEC) needs the SAME
single flag to relax both bounds together — there is no separate `include_expired` flag (that
split was explicitly considered and rejected in the UI-SPEC).
**How to avoid:** Gate the entire `activeWindowConditions(...)` append on `!opts.IncludeScheduled`
as one unit, never split its two `Should` conditions across two different flags.
**Warning signs:** A test seeds a record with `not_after` in the past, calls
`Search`/`List{IncludeScheduled: true}`, and the record is present in results but the CLI/console
derivation produces neither `expired` nor `scheduled` for it (both would be legitimately absent
per D-13's rules, which is the silent-failure signature to test for).

### Pitfall 4: Recall-gate reachability re-derivation drifts from what actually shipped
**What goes wrong:** `internal/store/schemaversion_recallgate_test.go`'s `recallEntryPointSeeds`
(6 entries: `Store.Search`, `Store.SearchReranked`, `Store.SearchDiscovery`, `Store.List`,
`Store.ListScheduled`, `Store.ListScopes` — confirmed at `schemaversion_recallgate_test.go:357-364`)
and `migratebacklog.go`'s `backlogFilter` doc-comment claim ("never reachable from any
recallEntryPointSeeds member", confirmed at `migratebacklog.go:44`) are both AST-derived-reachability
assertions scoped to `schema_version`, not to `superseded_by`/`archived_at`. This phase does not
change which functions are reachable (Search/List remain the same entry points), so this specific
test file needs no structural edit — but a plan that assumes it therefore needs NO attention at
all risks missing that the SAME reasoning pattern (recall gates are enumerable and testable by
AST walk) is exactly what `TestSupersedeRecallGate`/`TestSupersedeMultiRecallGate`
(`store_test.go:3195`, `:3784`) already do for `superseded_by` specifically — THOSE are the tests
that must gain new sub-cases (`SearchOptions{IncludeSuperseded: true}` should surface the
previously-hidden record).
**Why it happens:** The two test families look similar (both are "recall gate" tests) but check
different things — one is a static AST reachability proof, the other is a live-Qdrant behavioral
assertion.
**How to avoid:** Extend `TestSupersedeRecallGate` and its Multi-variant with new sub-assertions
per new flag combination; leave `schemaversion_recallgate_test.go`'s seed list alone unless a NEW
store method is introduced (this phase introduces none).
**Warning signs:** A plan task titled "update recallEntryPointSeeds" with no clear new store
method driving it is very likely conflating the two test families.

### Pitfall 5: `EmitDefaultValues` + `optional` interacts differently for `engram get`'s adapter than for `renderJSON`
**What goes wrong:** `renderJSON`'s existing `protojson.MarshalOptions{UseProtoNames: true,
EmitDefaultValues: true}` (`client_common.go:380-391`) is documented (Phase 5 D-14 §3, confirmed
in `connectapi.go:98-101`'s comment on `memoryToProto`) to still OMIT an unset `optional` field
even with `EmitDefaultValues` set. `superseded_by` is the ONE field among the 8 that is a genuine
pointer copy where nil legitimately means unset (`connectapi.go:93`: `SupersededBy: m.SupersededBy`
— no unconditional-assign comment, unlike `SchemaVersion`/`SummaryModel`). If `engram get`'s
headline-parenthetical logic checks `resp.Msg.GetMemory().GetSupersededBy() != ""` instead of the
pointer, it is CORRECT today (empty string never legitimately occurs) but is a latent trap if a
future change ever makes empty-string a valid `superseded_by` sentinel.
**How to avoid:** Prefer the protobuf-generated pointer field or an explicit `Has`-shaped check
where the generated API offers one; document the "empty-string-never-occurs" assumption inline if
the getter form is used instead, so a future reader does not have to re-derive it.
**Warning signs:** N/A at ship time (behavior is correct either way today) — this is a
maintainability note, not a functional bug.

### Pitfall 6: `Store.MigrateStatus`'s three-RPC reconciliation error path has no Connect error mapping precedent
**What goes wrong:** `Store.MigrateStatus` can return a genuine `error` (facet truncation detected,
or a concurrent-writer reconciliation failure after one retry — `migrate_status.go:65-101`'s doc
comment). The new Connect handler must map this through `connectError` (the shared classifier
every other handler uses) rather than a bespoke `connect.NewError(connect.CodeInternal, ...)`,
matching every other handler's discipline in `connectapi.go`.
**How to avoid:** Follow `ListScopes`'s exact error-handling shape (`connectapi.go:170-184`):
`connectError(ctx, err)` on failure, nothing bespoke.
**Warning signs:** A hand-rolled `connect.NewError` call anywhere in the new handler that does not
go through `connectError`.

## Code Examples

### `ListMemories` request field additions (proto — next available field numbers confirmed by reading `engram.proto` in full)

```protobuf
// Source: proto/engram/v1/engram.proto:71-92 (current — fields 1-12 in use, cross_spine=12)
// This phase's additive edit appends fields 13-15:
message ListMemoriesRequest {
  // ... existing fields 1-12 unchanged ...
  bool include_archived = 13;   // false (default) = today's behavior: soft-hidden
  bool include_superseded = 14; // false (default) = today's behavior: soft-hidden
  bool include_scheduled = 15;  // false (default) = today's behavior: window-gated (both bounds)
}
```
```protobuf
// Source: proto/engram/v1/engram.proto:110-125 (current — fields 1-9 in use, cross_spine=9)
// This phase's additive edit appends fields 10-12:
message SearchMemoriesRequest {
  // ... existing fields 1-9 unchanged ...
  bool include_archived = 10;
  bool include_superseded = 11;
  bool include_scheduled = 12;
}
```

### New RPC skeleton (name/messages are Claude's Discretion per CONTEXT.md — `MigrateStatus`/`MigrateStatusRequest`/`MigrateStatusResponse` shown as the naming candidate that mirrors the existing store method and operator command name)

```protobuf
// Source: proto/engram/v1/engram.proto:292-305 (current EngramService — 5 read + 6 write RPCs)
// Appended as a 6th read RPC:
message VersionBucket {
  int32 version = 1;
  uint64 count = 2;
}
message MigrateStatusRequest {}
message MigrateStatusResponse {
  repeated VersionBucket buckets = 1;
  uint64 absent = 2;
  repeated VersionBucket future = 3;
  uint64 future_total = 4;
  uint64 total = 5;
  uint32 current_version = 6; // mirrors migrateStatusReportDoc's CurrentVersion (cmd/engram/migrate_family.go:312)
}
service EngramService {
  // ... 5 existing read RPCs unchanged ...
  rpc MigrateStatus(MigrateStatusRequest) returns (MigrateStatusResponse);
  // ... 6 existing write RPCs unchanged ...
}
```
Note: `proto:lint` bans `idempotency_level = NO_SIDE_EFFECTS` on any RPC
(`Taskfile.yaml:234-236`, "GET-reachable + CSRF risk") — do not add that option to this RPC or any
other.

### Connect handler for the new RPC (follows `ListScopes` exactly — simplest existing handler, no request fields, subject-only auth)

```go
// Source pattern: internal/server/connectapi.go:170-184 (ListScopes, verbatim shape to follow)
func (a *engramAPI) MigrateStatus(ctx context.Context, _ *connect.Request[engramv1.MigrateStatusRequest]) (*connect.Response[engramv1.MigrateStatusResponse], error) {
	if _, err := subjectFromConnectContext(ctx); err != nil { // D-06: any AUTHENTICATED caller — auth check only, no owner filter passed to the store call
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	res, err := a.d.st.MigrateStatus(ctx) // internal/store/migrate_status.go:102 — whole-collection, no Subject param exists on this method
	if err != nil {
		return nil, connectError(ctx, err) // Pitfall 6
	}
	// map res.Buckets/Future ([]store.VersionBucket) to []*engramv1.VersionBucket,
	// mirroring statusReportDoc's normalize-nil-to-empty-slice discipline
	// (cmd/engram/migrate_family.go:320-337) so json/proto emit [] never null.
	return connect.NewResponse(&engramv1.MigrateStatusResponse{ /* ... */ }), nil
}
```

### `store.SearchOptions`/`ListOptions` field additions and the two gate-site edits

```go
// Source: internal/store/store.go:1049-1059 (current struct)
type SearchOptions struct {
	Tags []string
	Categories []string
	CreatedAfter, CreatedBefore time.Time
	IncludeArchived   bool // D-02: relaxes the archived_at IsEmpty gate
	IncludeSuperseded bool // D-02: relaxes the superseded_by IsEmpty gate
	IncludeScheduled  bool // D-02: relaxes BOTH halves of activeWindowConditions — see Pitfall 3
}
```
```go
// Source: internal/store/store.go:1086-1097 (current Store.Search body — the edit shape)
f := s.ownerScopeFilter(ctx, scope, subj)
if !opts.IncludeScheduled {
	f.Must = append(f.Must, activeWindowConditions(s.now())...)
}
if !opts.IncludeSuperseded {
	f.Must = append(f.Must, qdrant.NewIsEmpty("superseded_by"))
}
if !opts.IncludeArchived {
	f.Must = append(f.Must, qdrant.NewIsEmpty("archived_at"))
}
```
The identical shape applies to `Store.List` at `store.go:1328-1333` against `ListOptions`'s three
new fields (added the same way to the struct at `store.go:1210-1231`).

### `engram get` command skeleton (follows `client_list.go` exactly)

```go
// Pattern source: cmd/engram/client_list.go:33-136 (full file read this session)
var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Fetch one memory by id or short_id, rendering full state via the operator view",
	Args:  cobra.ExactArgs(1), // multi-id support is Claude's Discretion
	RunE: func(cmd *cobra.Command, args []string) error {
		client, format, timeout, err := clientFromFlags(cmd) // client_common.go:133-171
		if err != nil { return err }
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.GetMemory(ctx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: args[0]}))
		if err != nil { return wrapRPCError(err) } // client_common.go:324-326
		headline := "record " + resp.Msg.GetMemory().GetShortId() // + parenthetical per D-13 vocabulary, sanitized (D-11)
		if format == formatText {
			b, err := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}.Marshal(resp.Msg.GetMemory())
			if err != nil { return err }
			return renderOperatorView(cmd.OutOrStdout(), headline, json.RawMessage(b)) // D-10 adapter (Pattern 3)
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg.GetMemory()) // json lane unchanged, client_common.go:380-391
	},
}
func init() {
	addClientFlags(getCmd) // client_common.go:42-55 — REQUIRED for correct operatorCommands() exclusion (Pitfall 2)
	rootCmd.AddCommand(getCmd)
	// internal/surfaces/toolclass.go needs a matching {CLICommand: "get", ...} row (Pitfall 1)
}
```

### D-11's headline sanitization fix (the exact one-line change)

```go
// Source: cmd/engram/operator_view.go:255-258 (current)
func renderOperatorView(w io.Writer, headline string, doc any) error {
	if _, err := fmt.Fprintln(w, headline); err != nil { // <- change to sanitizeViewValue(headline)
		return err
	}
	...
```

## State of the Art

Not applicable in the "library changed its recommended API" sense — this is an internal-only
extension of code all shipped within the last 2-6 phases of the same milestone. There is no
external "old approach vs. new approach" axis; the only relevant history is this repo's own,
already captured in the Locked Decisions above (e.g., D-10 notes "Phase 6 assumed this port would
be more expensive than it is" — the `json.RawMessage` adapter was discovered to be cheap only
during this milestone's own Phase 6/7 sequence).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Proposed RPC/message names `MigrateStatus`/`MigrateStatusRequest`/`MigrateStatusResponse`/`VersionBucket` | Code Examples, Architecture Patterns | Low — explicitly Claude's Discretion per CONTEXT.md; the plan may rename freely as long as the RPC exists and follows the shape. |
| A2 | Proposed client-tier CLI verb name is left as the UI-SPEC's stated proposal, `engram status` | Summary, Code Examples | Low — UI-SPEC itself flags this as "a naming proposal, not a lock"; resolve against the live toolclass registry and existing verb names at plan time. |
| A3 | `Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false}` is the correct `toolclass.go` classification for both `engram get` and the new migration verb | Pitfall 1 | Low — mirrors the existing `search`/`list`/`version` rows exactly (all read-only, non-destructive, idempotent CLI reads); no plausible alternative classification exists for a pure GET-shaped command. |
| A4 | Multi-id support for `engram get` is out of the recommended v1 shape (`cobra.ExactArgs(1)`) | Code Examples | Low — explicitly Claude's Discretion in CONTEXT.md; a plan may choose `cobra.MinimumNArgs(1)` instead with no research implication either way. |

**If this table is empty:** N/A — see rows above; all are low-risk naming/shape choices explicitly
left to planning discretion by CONTEXT.md, not load-bearing technical claims.

## Open Questions

1. **Does the STATE column's compound value need a fixed-width form for `tabwriter` alignment, or is comma-joined-no-space (already specified in UI-SPEC) sufficient?**
   - What we know: UI-SPEC locks the format (`archived,superseded`, no space, canonical order) and explicitly reasons through the `tabwriter` ANSI/wide-character hazards already (Component Specs, CLI section).
   - What's unclear: Nothing technical — this is fully resolved in the UI-SPEC. Listed here only so the planner does not re-open it.
   - Recommendation: Treat as locked; no further research needed.

2. **Should the migration-status verb's Connect call site live in a new file or join `client_list.go`/a new small file?**
   - What we know: Every other client-tier verb is one file (`client_search.go`, `client_list.go`).
   - What's unclear: Purely a file-organization choice with no correctness implication.
   - Recommendation: New file (e.g. `client_migrate_status.go` or matching whatever verb name is chosen), for consistency with the one-verb-one-file pattern.

## Environment Availability

Not applicable — this phase adds no new external tool, service, runtime, or CLI dependency. Qdrant
(via testcontainers for Go tests) and the existing `task` build tooling are unchanged prerequisites
already required by every prior phase in this milestone.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Go framework | stdlib `testing` + `testcontainers-go/modules/qdrant` (confirmed: `internal/store/store_test.go:22-26`) |
| Go quick run | `go test ./internal/store/... -run TestSearchRecallGate` / `go test ./cmd/engram/... -run TestCatalog` (name TBD at plan time — see below) |
| Go full suite | `go test ./...` (== `task` per CLAUDE.md's "`task` = lint + test") |
| UI framework | Vitest 4.1.10 + `vitest-browser-svelte` 2.2.1 (Playwright-backed browser project) — confirmed `ui/package.json:9-33`; existing `.browser.test.ts` files exist for all four touched components (`MemoryRow.browser.test.ts`, `MemoryDetail.browser.test.ts`, `ScopesSidebar.browser.test.ts`, `AppShell.browser.test.ts`) |
| UI quick run | `cd ui && npm run test:browser -- MemoryRow` (component-scoped) |
| UI full suite | `cd ui && npm run test` (== `vitest run`, both projects) |
| Proto/codegen drift | CI job `buf` (`.github/workflows/ci.yaml:244-264`): `buf lint`, `buf breaking (vs main)`, `git diff --exit-code -- gen/`, `git diff --exit-code -- ui/src/lib/gen/`, plus the `idempotency ban` step |
| CLI golden drift | CI job `surfaces` (`.github/workflows/ci.yaml:273-`): regenerates `--help`/catalog goldens from the live cobra tree via `task surfaces:gen` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-cli-record-state (recall gate) | `Search`/`List{IncludeSuperseded:true}` surfaces a previously-hidden superseded record | integration (Qdrant testcontainer) | `go test ./internal/store/... -run TestSupersedeRecallGate` | ✅ exists (`store_test.go:3195`) — needs new sub-cases, not a new file |
| REQ-cli-record-state (recall gate, archived) | Same, for `IncludeArchived` | integration | `go test ./internal/store/... -run TestArchive` (name TBD) | ❌ Wave 0 — no existing archived-recall-gate positive-relaxation test found this session (only `TestPruneExpiredExcludesArchived`, a different concern) |
| REQ-cli-record-state (recall gate, scheduled/expired) | `IncludeScheduled` relaxes BOTH window bounds (Pitfall 3) | integration | `go test ./internal/store/... -run TestSearchIncludeScheduled` (name TBD) | ❌ Wave 0 |
| REQ-console-record-state / REQ-cli-record-state | D-13 vocabulary derivation is identical on both surfaces | unit (Go) + component (Vitest browser) | `go test ./cmd/engram/... -run TestStateWord` + `npm run test:browser -- MemoryRow` (names TBD) | ❌ Wave 0 — both are new, per-surface, per D-13's own requirement ("a test per surface asserting the derivation") |
| REQ-migration-state-visible | New RPC handler returns the same histogram `Store.MigrateStatus` already computes, mapped correctly (nil->[] normalization per Pitfall 6/statusReportDoc precedent) | integration (Qdrant testcontainer) | `go test ./internal/server/... -run TestMigrateStatus` (name TBD) | ❌ Wave 0 |
| REQ-cli-record-state (`engram get`) | Text/json identity holds for the new command, mirroring Phase 6's `assertViewIdentity` fixture discipline | unit (Go) | `go test ./cmd/engram/... -run TestGetCommand` (name TBD) | ❌ Wave 0 |
| catalog completeness (Pitfall 1) | New commands classified in `toolclass.go` | unit (Go, existing gate, no new test needed) | `go test ./cmd/engram/... -run TestCatalogBlastRadiusMatchesToolClasses` | ✅ exists — this IS the gate; just needs the new rows to pass |

### Sampling Rate
- **Per task commit:** the scoped `-run` command from the row being implemented.
- **Per wave merge:** `go test ./internal/store/... ./internal/server/... ./cmd/engram/...` (Go) + `cd ui && npm run test` (UI).
- **Phase gate:** `task` (full Go suite + lint) AND `cd ui && npm run test` AND `task chart:validate`/`task ui:build` if either is touched (neither is expected to be) green before `/gsd-verify-work`, per STATE.md's standing note that these two CI gates run outside the normal phase-gate lifecycle and must be run locally.

### Wave 0 Gaps
- [ ] Archived-record recall-gate positive-relaxation test (`internal/store`) — covers REQ-cli-record-state/console-record-state
- [ ] Scheduled/expired-record recall-gate positive-relaxation test, asserting BOTH window bounds relax together (`internal/store`) — covers REQ-cli-record-state/console-record-state, directly gates Pitfall 3
- [ ] Go-side D-13 state-word derivation unit test — covers REQ-cli-record-state
- [ ] TS-side (Vitest browser) D-13 state-word derivation test per component — covers REQ-console-record-state
- [ ] New-RPC integration test against a real histogram (multiple version buckets + absent + future) — covers REQ-migration-state-visible
- [ ] `engram get` identity/adapter test (protojson bytes through `renderOperatorView` render correctly, including an `optional` field's correct omission/presence) — covers REQ-cli-record-state
- [ ] D-11 headline-sanitization regression test (a state-word parenthetical containing no control characters passes through unchanged; a synthetic control-character input, if ever reachable, is neutralized) — covers the phase's own stated precondition fix

## Security Domain

`security_enforcement` is absent from `.planning/config.json` -> treated as enabled per the
instruction contract.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (unchanged) | Existing OIDC bearer-token verifier (`internal/auth`); the new RPC requires the SAME `subjectFromConnectContext` check every other authenticated RPC uses — no new auth mechanism. |
| V4 Access Control | yes | D-04 (state gates orthogonal to authz) and D-06 (any-authenticated-caller migration RPC, explicitly NOT owner-scoped) are both access-control decisions already made and locked by CONTEXT.md — this phase implements them, does not re-decide them. The Cedar policy files (`internal/authz/policies/*.cedar`) are explicitly untouched (D-04). |
| V5 Input Validation | yes | The three new request bools need no validation (plain bools, no invalid values possible). `--include-*` CLI flags similarly need no validation beyond cobra's own bool parsing. No new `buf.validate` rules required. |
| V6 Cryptography | no | Not applicable — no new secrets, tokens, or crypto primitives introduced. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Terminal/report injection via record-derived free text reaching an operator's console unsanitized (T-06-03, this repo's own named threat, already mitigated for the field-table path) | Tampering / Information Disclosure (spoofed report content) | D-11's fix: route `renderOperatorView`'s headline through the SAME `sanitizeViewValue` (`operator_view.go:223-234`) the field-table path already uses. This phase is the FIRST to put record-derived content (a short_id — server-minted, but the parenthetical logic reads live record state) into that headline, so the fix is a precondition of shipping `engram get`, not an optional hardening pass — confirmed as this session's understanding matches CONTEXT.md's own framing of D-11 exactly. |
| Aggregate information disclosure via the new any-authenticated-caller migration RPC (D-06) | Information Disclosure | Accepted and explicitly reasoned through in CONTEXT.md D-06: the RPC discloses only aggregate version-bucket counts across owners, no record content, no scopes, no owners — judged acceptable for a self-hosted operator console. Not re-litigated here; implement as locked. If this needs revisiting, `internal/authz/entities.go:42-47`'s reserved (never-populated) `roles` attribute is the documented forward-compat seam, not a phase-7 task. |
| Over-broad recall exposure via a caller supplying `include_superseded`/`include_archived`/`include_scheduled` for a scope/owner they should not see hidden-state records in | Elevation of Privilege | Explicitly NOT a new risk per D-04: the three flags relax only the STATE `Must` conditions; `ownerScopeFilter`/`listFilter`'s owner/shared authz conditions remain unconditionally in `f.Must` at every gate site (confirmed by reading `store.go:1086` and `:1263-1266` — the authz condition is built BEFORE the state-gate appends and is never itself made conditional). A shared record that becomes superseded/archived remains readable by exactly the callers its live predecessor was (D-04's own stated property) — verify this stays true by construction when implementing, not by a new authz mechanism. |

## Sources

### Primary (HIGH confidence — every citation below was read directly this session, current repo state)
- `internal/store/store.go` (full read of lines 1-1300, 1500-1600) — `SearchOptions`/`ListOptions`, all four recall-gate sites, `activeWindowConditions`, `Store.Search`/`SearchReranked`/`SearchDiscovery`/`List`/`ListScheduled`
- `internal/store/migrate_status.go` (full read) — `Store.MigrateStatus`, `MigrateStatusResult`, `VersionBucket`
- `internal/store/migratebacklog.go` (targeted read) — `backlogFilter` reachability claim
- `internal/store/schemaversion_recallgate_test.go` (targeted read, lines 280-400) — `recallEntryPointSeeds`, the AST-reachability test shape
- `internal/store/store_test.go` (targeted read, `TestSupersedeRecallGate` at line 3195) — the live behavioral test precedent to extend
- `internal/server/connectapi.go` (full read of lines 1-350) — `memoryToProto`, `ListMemories`, `SearchMemories`, `GetMemory`, `ListScopes` handler shapes
- `internal/server/tools.go` (targeted reads: 480-540 `warnPendingMigrations`; 1394-1560 `coreListRequest`/`coreSearchRequest`/`listMemory`/`searchMemory`; 1776-1794 `getMemory`; 2370-2444 MCP tool call sites)
- `internal/authz/entities.go` (full read of lines 1-60) — `principalEntity`, the reserved `roles` omission
- `internal/surfaces/toolclass.go` (targeted reads: 1-230) — `Class`, `Operation`, the full `operations` table
- `internal/surfaces/rules.go` (targeted read, lines 100-235) — `RuleWindowOrdering`
- `cmd/engram/client_common.go` (full read) — `addClientFlags`, `clientFromFlags`, `renderJSON`, `renderMemoryTable`, `renderCoverageFooter`, `wrapRPCError`, `exitCodeForConnectErr`
- `cmd/engram/client_list.go` (full read) — the exact `engram get` template
- `cmd/engram/operator_view.go` (full read) — `viewFields`, `sanitizeViewValue`, `renderOperatorView`, the D-11 fix site
- `cmd/engram/operator_output.go` (full read) — `renderOperator`, `addOperatorOutputFlag`, `operatorOutputFormat`
- `cmd/engram/cmdwalk.go` (full read) — `operatorCommands()`, `commandWalkSkip`, `walkCommands`
- `cmd/engram/catalog.go` (targeted read, lines 1-115) — `buildCatalog`'s panic-on-unclassified-command behavior
- `cmd/engram/migrate_family.go` (targeted read, lines 260-360) — `migrateStatusCmd`, `migrateStatusReportDoc`, `statusReportDoc`, `statusSummary` (the exact template for the new client-tier verb's report shape)
- `cmd/engram/operator_output_test.go` (targeted read, lines 1-60) — existing operator-view test conventions
- `proto/engram/v1/engram.proto` (full read of lines 1-150, 280-306) — `Memory`, `ListMemoriesRequest`/`Response`, `SearchMemoriesRequest`/`Response`, `GetMemoryRequest`/`Response`, `EngramService`
- `gen/go/engram/v1/engram.pb.go` (targeted reads) — confirmed `Memory` struct field types (pointer semantics for `optional`/message fields)
- `ui/src/lib/gen/engram/v1/engram_pb.ts` (targeted reads, lines 140-200) — confirmed TS field types and their `optional` presence typing
- `ui/src/lib/components/MemoryRow.svelte`, `MemoryDetail.svelte`, `ScopesSidebar.svelte`, `AppShell.svelte` (all full reads) — current structure, insertion points, confirmed absence of any fixed-row-height/virtualization mechanism
- `ui/src/lib/components/MemoryList.svelte`, `ui/src/lib/client.ts`, `ui/src/lib/queries.ts`, `ui/src/routes/observe/+page.svelte` (all full or near-full reads) — query wiring, client construction, URL-param round-trip
- `Taskfile.yaml` (targeted read, lines 220-260) — `proto:gen`'s exact re-vendor commands, `proto:lint`'s `NO_SIDE_EFFECTS` ban, `surfaces:gen`'s chaining
- `.github/workflows/ci.yaml` (targeted read, lines 1-435) — the `buf`, `surfaces`, and `ui tests` CI jobs and their exact drift-check commands
- `.planning/phases/07-console-cli-state-surfacing/07-CONTEXT.md`, `07-UI-SPEC.md`, `.planning/REQUIREMENTS.md`, `.planning/STATE.md`, `.planning/config.json` (all read in full)

### Secondary (MEDIUM confidence)
- None — no web search or Context7 lookup was needed; every technology involved is already adopted and documented in-repo.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; every version cited was read from the actual `go.mod`/`package.json` this session.
- Architecture: HIGH — every integration point (store gate sites, typed-core structs, Connect handlers, CLI templates, console components) was read in full or targeted detail this session, not inferred from CONTEXT.md's summary alone.
- Pitfalls: HIGH — five of six pitfalls are backed by a specific panic/test-failure mechanism read directly in source (catalog.go's panic, operatorCommands()'s structural filter, activeWindowConditions' two-condition shape, the recall-gate test family split, MigrateStatus's own documented error path); the sixth (protojson optional-presence nuance) is backed by the exact struct-tag/comment evidence in `connectapi.go`.

**Research date:** 2026-08-20
**Valid until:** No external time pressure — this is an internal-only extension of code within the same milestone; re-validate only if Phase 5/6 CONTEXT.md decisions are revisited or if store.go/connectapi.go/toolclass.go are touched by an unrelated change between now and plan execution.
