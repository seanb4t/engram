# Phase 3: Shared Bounded-Read Mechanism & Content Cap Decision - Research

**Researched:** 2026-09-19
**Domain:** Go + Qdrant memory server — bounded-read primitives and a content-size cap decision
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-00 (user preference `1w3h5sy56m`):** choose by idiom and long-term maintenance, never by effort.
- Phase 1: `store.NewQdrantClient` is the one constructor; tests pin `storetest.RecvLimit` (4 MiB);
  `storetest.SeedOversized` provides many-small and few-large fixtures.
- Phase 2: an over-limit response surfaces as `store.ErrResponseTooLarge` (classifier interceptor),
  mapped to `field=response hint=too_large` / `resource_exhausted` / exit 10.
- **D-01:** Cap memory `content`: a registry-declared `ENGRAM_MEMORY_MAX_CONTENT_BYTES`
  (`internal/config` field registry, the single source of truth for `ENGRAM_` vars) with a
  documented default of **64 KiB** (65536 bytes) — matching the existing discovery-content bound
  (`maxDiscoveryContentBytes`, `internal/server/tools.go:769`). Every memory write path rejects
  oversized content — `store_memory`, `schedule_memory`, `supersede_memory`, and `update_memory`
  when it changes content, on MCP, Connect, and the `engram` CLI — with the EXISTING
  `field=content hint=too_long` envelope (no new hint code). Existing records larger than the cap
  stay stored and readable (the cap never rewrites or deletes). Record the decision in PROJECT.md
  Key Decisions (REQ-locked) and document the variable (docs-site configure/errors pages). —
  **Reversibility:** one-way — a published write-rejection contract agents and scripts will hit;
  the user chose this exact option in discuss-phase (2026-09-19) — do not insert a checkpoint to
  re-ask.
- **D-02:** Pages end on an accumulated-byte budget AS WELL AS a record count, built as **small
  RPCs + a running byte total**: each Qdrant RPC fetches `floor(rpcBudget / maxRecordBytes)`
  records — a count derived arithmetically from the per-record payload ceiling — so no single RPC
  can overflow; the logical page accumulates the MEASURED bytes of what it received and stops at
  the page byte budget or the record limit, whichever comes first. `maxRecordBytes` must be the
  TRUE per-record payload ceiling under the caps (content cap + summary cap + citations bound
  (count × excerpt cap) + tags + every other payload field + protobuf overhead) — research must
  derive it from the actual schema, not assume content dominates.
- **D-03:** Two primitives, both in `internal/store`:
  1. an **ordered-page helper** for `List`-shaped reads (OrderBy `created_at`), generalizing
     `listByCursor`'s existing keyset paging (`created_at` `start_from` + boundary-id `seen` set) so
     multiple small RPCs compose one logical page without skips or duplicates;
  2. a **byte-budget extension of `scrollAllPoints`** (`spine.go:46`, the one whole-spine
     iterator) for unordered sweeps (point-id offset paging).
  Each primitive takes the caller's payload selector (D-04) and is proven on its own against
  `storetest.SeedOversized` (both shapes) at the named 4 MiB limit. The recall-gate AST test's
  classifications (`recallTransmitters`, `schemaversion_recallgate_test.go`) are updated in the
  same change that introduces any new transmitting helper.
- **D-04:** **Project payloads by view.** The primitives accept a payload selector; recall callers
  (wired in Phase 4) skip `content` in summary view (the default) for records that already have a
  summary, and fetch content only where the response needs it (`full=true`, or the fallback
  snippet for a record with no summary yet). This extends #583's payload-selector pattern.
- **D-05:** The two-phase ids→payload design is **NOT** adopted (no stored `payload_bytes` field,
  no schema-version step, no GetPoints re-fetch), so the TOCTOU-on-delete/supersede/archive and
  `GetPoints`-order acceptance criteria the ROADMAP names for that design do not apply — state this
  explicitly in the plan so the verifier does not look for them.
- **D-06:** **Fixed constants** in `internal/store`: a per-RPC byte budget and a caller-facing
  page byte budget, both comfortably under the 4 MiB the tests pin (order of 2 MiB — exact values
  derived by research from `maxRecordBytes` and headroom for protobuf/gRPC framing), each with an
  unexported `var` seam (the `spineScrollBatch` precedent) so tests can shrink them. The mechanism
  is independent of Phase 5's `MaxCallRecvMsgSize` backstop (which stays pure defense in depth),
  and the page budget also bounds the Connect/MCP responses callers receive.
- **D-07:** A pre-cap record that exceeds the per-RPC capacity is handled by a **batch-of-1
  fallback**; if that single record STILL exceeds the receive limit, the request fails with the
  named `store.ErrResponseTooLarge` (→ `field=response hint=too_large` / exit 10) — it is never
  silently skipped or truncated (silent truncation is milestone Out of Scope). With D-04
  projection, summary-view recall of a summarized legacy record never fetches its content, so this
  path mostly concerns `full=true` and sweeps. "Existing oversized records stay readable" means
  `get_memory` returns them within the Phase 5 production backstop, and an owner can trim one via
  `update_memory`.
- **D-08:** Record an inventory of every `WithPayload(true)` / `NewWithPayload(true)` / unbounded
  `Scroll` / `ScrollAndOffset` / `Query` call site in `internal/store` (non-test), each assigned to
  its migrating phase — Phase 4: `Store.List` (offset/limit-0/deep-offset/cursor), `ListScheduled`,
  `Search`/`SearchDiscovery`; Phase 5: `migrate`, `migrate revert`, `summarize-missing`,
  `spine-review` scan/verify/purge, `reindex` — or a written justification for exemption (e.g.
  `ListScopes` already projects `scope` only; single-record `Get`/`GetPoints` by id). The inventory
  lives in the phase's planning artifacts (and is what Phase 5's closing check compares against).

### Claude's Discretion

- Exact identifiers, file placement, the exact budget constant values (within D-06), and the
  overhead formula for `maxRecordBytes` (within D-02).
- How the ordered-page helper exposes "page cut short by budget" to its caller so Phase 4 can keep
  REQ-list-contract-unchanged (a budget-short page is never reported as the last page).
- Test design for the primitives (both fixture shapes; a many-small page that would overflow as one
  RPC; a few-large page; a legacy over-cap record via a direct raw write in a `package store` test;
  keyset correctness across RPC boundaries with equal `created_at` values).
- Phase 3 red-evidence patches (register in `redEvidenceDirs` after the last plan).

### Deferred Ideas (OUT OF SCOPE)

- Two-phase ids→payload paging with a stored `payload_bytes` field (considered, not adopted — D-05).
- `listByCursor` tie-safety under > a page of identical `created_at` values with concurrent inserts
  (REQ-cursor-tie-safety, v2).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-byte-budget-pages | Pages end on an accumulated-byte budget as well as a record count, so a page of a few very large records stays under the receive limit. | "Payload Ceiling Derivation" and "Budget Constants" sections derive `maxRecordBytes` (two views) and the `rpcByteBudget`/`pageByteBudget` constants from the actual payload schema (content+citations+tags+fixed fields), not content alone; "Ordered-Page Helper" and "Byte-Budget Sweep Primitive" sections give the concrete mechanism for each of the two required primitives (D-03). |
| REQ-content-cap-decided | Whether memory `content` gets a size cap is decided and recorded in PROJECT.md Key Decisions; if adopted, a registry-declared `ENGRAM_MEMORY_MAX_CONTENT_BYTES` rejects an oversized write on every write path with a named hint, and existing oversized records stay readable. | "Content Cap Enforcement (D-01)" section maps every write path (MCP/Connect/CLI) to its exact enforcement call site, identifies the one non-obvious placement pitfall (`deps.updateMemory` vs. `validateUpdateArgs`), and gives the exact registry/validation/documentation pattern to copy from `MaxSummaryBytes`. |
</phase_requirements>

<claude_md_constraints>
## Project Constraints (from CLAUDE.md)

- **VCS discipline:** branch + PR only; `main` is protected. Planning/workflow via GSD
  (`.planning/`, `/gsd-*`). This research/plan work happens on `feat/2026-09-18.01`.
- **Migrations:** payload migrations are schema-version-driven, additive-only, and only via the
  registered sweep — this phase introduces NO migration step (it adds read-side primitives and a
  write-side validation, neither of which changes stored payload shape), so `internal/migrate`'s
  registry is not implicated.
- **License headers:** every in-scope Go file needs the Apache-2.0 SPDX header (`task
  license:check`); `.planning/**` files (including this one) are explicitly EXCLUDED — do not add
  a header to this file.
- **Config:** `internal/config`'s field registry is the single source of truth for `ENGRAM_`
  variables — D-01's new `ENGRAM_MEMORY_MAX_CONTENT_BYTES` MUST be added there (`registry.go`),
  never as a bespoke `os.Getenv` read.
- **Lint/format:** `task lint` (golangci-lint, yamlfmt, actionlint, rumdl) and `task fmt` must stay
  clean; this phase's new Go files/functions must pass gofmt/golangci-lint as part of normal
  development, no special exemption needed.
- **Not used here:** viper, cocogitto — do not introduce either.
- **CLI is correct-by-reading:** any new/changed help text this phase touches (unlikely, since no
  new CLI flags are introduced) must follow the existing self-describe-catalog discipline; expected
  to be a non-issue since D-01's enforcement is server-side only and the CLI has no update/supersede
  command to begin with.
- **Commits:** Conventional Commits; PR titles CI-validated; branch + PR only, never push to
  `main` directly.
</claude_md_constraints>

## Summary

This phase builds two reusable, standalone primitives in `internal/store` (an ordered-page
helper generalizing `listByCursor`'s keyset paging, and a byte-budget extension of
`scrollAllPoints`), records a full call-site inventory, and enforces `D-01`'s memory `content`
cap. It does **not** wire either primitive into `Store.List`/`ListScheduled`/`Search` (Phase 4)
or into the five operator sweeps (Phase 5) — this phase's own tests exercise the primitives
directly against `storetest.SeedOversized` fixtures, not through the existing call sites.

The single most consequential finding this research surfaces, not previously visible in
`ARCHITECTURE.md`/`PITFALLS.md`, is that **`content` is not the dominant unbounded field once
capped**. Reading `internal/server/tools.go`'s `validateCitations` (called with `minCount 0` from
the three curated-memory write paths, not just `store_discovery`) shows a memory record can carry
up to **50 citations × 16 KiB `Excerpt` each ≈ 800 KiB** — over 12× the proposed 64 KiB content
cap — plus unbounded `Ref`/`Locator`/`Pin` strings per citation, and `Tags` has **no length or
count bound anywhere in the codebase** `[VERIFIED: internal/server/tools.go]`. `connectapi.go:149`'s
own comment already flags the citations risk for the **response-shaping** layer (`shapeProtoMemories`
clears citations for non-`full` Connect responses) — but that clearing happens **after** Qdrant has
already returned the full payload; it does nothing for the gRPC receive-limit risk this milestone
is about. `maxRecordBytes` (D-02) must therefore be derived from content + citations + tags +
fixed fields, not content alone, and the derivation differs by payload view (summary vs. full).

A second load-bearing, directly-confirmed finding: Qdrant's own documentation states plainly that
point-id `Offset`-based pagination cannot be combined with `OrderBy` on a non-unique sort key —
"When sorting is based on a non-unique value, it is not possible to rely on an ID offset... you
can still do pagination by combining `order_by.start_from` with a `must_not: has_id` filter"
`[CITED: qdrant.tech/documentation/manage-data/points]`. This is exactly the documented shape of
`listByCursor`'s existing keyset technique (client-side "seen"-set drop instead of a server-side
`has_id` filter) — confirming the existing pattern is the *correct*, Qdrant-sanctioned mechanism to
generalize, not a workaround to replace.

**Primary recommendation:** build the ordered-page helper as a generalization of `listByCursor`'s
existing `StartFrom` + boundary-`seen`-set idiom (never combine `OrderBy` with point-id `Offset`);
extend `scrollAllPoints`'s existing signature with a byte-budget parameter rather than introducing
a differently-named function; derive `maxRecordBytes` per view (summary-view excludes
content+citations; full-view/sweep-view must budget for the ~800 KiB citations worst case); and
enforce the content cap inside the three shared `deps.*` methods (`storeMemory`, `scheduleMemory`,
`supersedeMemory`) plus `deps.updateMemory`'s `contentChanged` branch, so both the MCP and Connect
lanes (and, transitively, the `engram` CLI, which is a Connect client) get it from one call site
each.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Ordered-page bounded scroll (List/ListScheduled-shaped) | API/Backend (`internal/store`) | — | Qdrant read-shaping is a store-layer concern; no client or server-handler tier touches paging mechanics |
| Byte-budget whole-spine sweep extension | API/Backend (`internal/store`) | — | `scrollAllPoints` is already store-tier; extending it stays store-tier |
| Content size cap enforcement | API/Backend (`internal/server` `deps.*`) | Database/Storage (write-time validation before Qdrant Upsert) | Validation must run before the embed call and the Upsert, in the shared `deps` methods both MCP and Connect route through |
| Content cap documentation | CDN/Static (docs-site) | — | `docs-site/src/content/docs/guides/configure.md` + `reference/errors.md` are static-built pages |
| Call-site inventory | API/Backend (`internal/store`) | — | The inventory is a property of `internal/store`'s own source, recorded in planning artifacts, not runtime code |

## Package Legitimacy Audit

**No external packages are installed by this phase.** Every capability used
(`google.golang.org/protobuf/proto`, `github.com/qdrant/go-client/qdrant`'s
`NewWithPayloadInclude`/`NewWithPayloadExclude`) is an existing, already-vendored dependency
`[VERIFIED: go.mod:47 — "google.golang.org/protobuf v1.36.12"; already imported at
internal/server/connectapi.go:17 and internal/surfaces/surfaces.go:11]`. The Package Legitimacy
Gate protocol is not applicable — table omitted per "no new packages" convention.

## Standard Stack

No new libraries. This phase composes existing, already-vendored primitives:

| Capability | Symbol | Already used at | Confidence |
|---|---|---|---|
| Payload projection | `qdrant.NewWithPayloadInclude(...)` / `NewWithPayloadExclude(...)` | `store.go:1726` (`ListScopes`, `WithPayloadInclude("scope")`) | `[VERIFIED: internal/store/store.go:1721-1727]` |
| Exact wire-size measurement | `proto.Size(p)` (`google.golang.org/protobuf/proto`) against a `*qdrant.RetrievedPoint` | Not yet used in `internal/store`; `RetrievedPoint` is a genuine `protoimpl.MessageState`-backed proto message | `[VERIFIED: qdrant-go-client@v1.19.2 points.pb.go:9876-9888]` |
| Keyset resume token | `qdrant.NewStartFromDatetime` / `qdrant.OrderBy.StartFrom` | `store.go:1516,1533` (`listByCursor`) | `[VERIFIED: internal/store/store.go:1504-1536]` |
| Test-overridable batch-size seam | package-level `var`, not `const` | `spineScrollBatch` (`spine.go:28`) | `[VERIFIED: internal/store/spine.go:23-28]` |
| Oversized fixture seeder | `storetest.SeedOversized(t, st, spec)` | Phase 1 | `[VERIFIED: internal/store/storetest/seed.go:179-252]` |
| Named receive limit | `storetest.RecvLimit` (4 MiB) | Phase 1 | `[VERIFIED: internal/store/storetest/storetest.go:37-43]` |

**Installation:** none — no `go.mod` change.

## Architecture Patterns

### System Architecture Diagram

```
                     store_memory / schedule_memory / supersede_memory / update_memory
                                (MCP tool closures)     (Connect RPCs, connectapi.go)
                                          │                        │
                                          └───────────┬────────────┘
                                                       ▼
                                     deps.storeMemory / .scheduleMemory /
                                     .supersedeMemory / .updateMemory (tools.go)
                                                       │
                                     validateStoreArgs(a, maxSummaryBytes)  ◄── ADD content-cap check HERE
                                     (storeMemory/scheduleMemory/supersedeMemory share this ONE call)
                                                       │
                                     deps.updateMemory: contentChanged branch ◄── ADD content-cap check HERE
                                     (Connect's field-mask lane bypasses validateUpdateArgs,
                                      so the check must live inside updateMemory itself)
                                                       │
                                                       ▼
                                          Store.Upsert / Store.Update (internal/store)
                                                       │
                                                       ▼
                                              Qdrant (payload() encodes Memory)


   ─────────────────────── READ SIDE (this phase builds, does not wire) ───────────────────────

   Store.List / listByCursor / ListScheduled          ScanSpine / EnumerateCitations /
   (ordered, OrderBy-ranked reads — Phase 4 wires)     NearDuplicates / derivePurgeEligible /
              │                                        migrate / revert / summarize-missing
              ▼                                                    │
      [NEW] orderedPageScroll-shaped helper                        ▼
      (generalizes listByCursor's keyset:                 scrollAllPoints (spine.go:46)
       StartFrom + boundary seen-set,                      [EXTENDED with a byte-budget param —
       several small RPCs per logical page,                 same function name, same call sites,
       accepts a WithPayloadSelector)                        Phase 5 wires the extended signature]
              │                                                    │
              ▼                                                    ▼
      s.client.Scroll (small Limit, WithPayloadInclude/Exclude)   s.client.ScrollAndOffset
      per-RPC record count = floor(rpcBudget / maxRecordBytes)     (existing shape, byte-aware stop)
```

### Recommended Project Structure

No new files are required by D-03's discretion items (exact placement is Claude's discretion);
the natural placement mirrors existing organization:

```
internal/store/
├── store.go              # listByCursor stays; the new ordered-page helper can live
│                         #   alongside it, or in a new boundedread.go (Claude's discretion)
├── spine.go              # scrollAllPoints gains a byte-budget parameter here, in place
├── boundedread.go         # (optional new file) maxRecordBytes derivation + the ordered-page
│                         #   helper, if not colocated with listByCursor
├── responsetoolarge.go    # (Phase 2, existing) store.ErrResponseTooLarge — this phase's
│                         #   batch-of-1 fallback (D-07) reuses this sentinel, does not add one
└── schemaversion_recallgate_test.go  # recallTransmitters/operatorMigrationEmitters/
                          #   otherNonRecallEmitters — see "Gates This Phase Touches" below
```

### Pattern 1: Ordered-page helper generalizing `listByCursor`'s keyset idiom

**What:** `listByCursor` (`store.go:1493-1579`) already implements exactly the pattern Qdrant's
own docs prescribe for paging a non-unique `OrderBy` key: `qdrant.NewStartFromDatetime(c.C)`
resumes at the boundary `created_at` value, and a client-side `seen` id set (built from the prior
page's boundary-timestamp ids) drops already-emitted records at that exact timestamp on the next
page (`store.go:1541-1552`). D-03's ordered-page helper generalizes this to **several small RPCs
per logical page** instead of one RPC per page: repeat the same `Scroll` call with a shrinking
per-RPC `Limit` (derived from `maxRecordBytes`, D-02) and accumulate emitted records + measured
bytes across RPCs until either the page's record limit or its byte budget is hit.
**When to use:** Any `List`/`ListScheduled`-shaped read ordered by `created_at` (Phase 4 wires
`Store.List`'s cursor mode, `ListScheduled`; `Store.List`'s offset mode can also route through
this helper internally — see "Offset-mode viable shapes" below).
**Confirmed correctness of the base idiom:** Qdrant's official docs state: *"When sorting is
based on a non-unique value, it is not possible to rely on an ID offset. Thus, `next_page_offset`
is not returned within the response. However, you can still do pagination by combining
`"order_by": { "start_from": ... }` with a `{ "must_not": [{ "has_id": [...] }] }` filter."*
`[CITED: https://qdrant.tech/documentation/manage-data/points]`. `listByCursor` implements the
functional equivalent client-side (drop already-seen ids at the exact boundary) rather than via a
server-side `must_not`/`has_id` filter — both are valid; the client-side form is simpler to extend
to a byte budget since it needs no filter mutation between RPCs.
**Confirmed: `Offset` (point-id) and `OrderBy` cannot be combined for pagination purposes.** The
`ScrollPoints` proto carries `Offset *PointId` ("Start with this ID") and `OrderBy *OrderBy` as
independent fields `[VERIFIED: qdrant-go-client@v1.19.2 points.pb.go:4425-4435,4491-4511]`, but
Qdrant's own docs (quoted above) confirm the server does not return a usable continuation via
`Offset` once `OrderBy` on a non-unique key is in play — `next_page_offset` isn't even returned.
This is why `store.go` never combines the two: `List`'s offset mode (`store.go:1468-1474`) uses
`OrderBy` with **no** `Offset` field (single Scroll, trim client-side); `listByCursor` uses
`OrderBy.StartFrom` with **no** `Offset` field either. The new helper must preserve this: never
set `ScrollPoints.Offset` when `OrderBy` is set.
**Example (existing idiom to generalize):**
```go
// Source: internal/store/store.go:1526-1536 (listByCursor, current single-RPC-per-page shape)
fetch := limit + uint64(len(seen)) + 1
pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
    CollectionName: s.collection,
    Filter:         f,
    Limit:          qdrant.PtrOf(uint32(fetch)),
    OrderBy: &qdrant.OrderBy{
        Key:       "created_at",
        Direction: qdrant.PtrOf(qdrant.Direction_Desc),
        StartFrom: startFrom,
    },
    WithPayload: qdrant.NewWithPayload(true),
})
```
The generalization: instead of one `Scroll` with `Limit: fetch`, loop calling `Scroll` with a
smaller `Limit` (sized from `maxRecordBytes`), re-deriving `StartFrom`/`seen` from the LAST
sub-fetch's boundary each time — the same resume logic `listByCursor` already uses between
top-level pages, applied one level deeper, between RPCs within one logical page.

**Tie-safety across RPC boundaries (many records sharing one `created_at` second):** RFC3339
second-precision `created_at` (`payload()` writes `m.CreatedAt.Format(time.RFC3339)`,
`store.go:651`) means a burst of writes in the same second collide at the same sort key. The
existing `listByCursor` already handles this WITHIN one page (the `seen` set), and `maxListLimit
= 1000` bounds how large that set can get (`store.go:1512-1514`). The new helper's added risk is a
tie spanning an RPC boundary **within** one logical page (not just across pages): if the small
per-RPC `Limit` cuts off mid-tie, the next sub-RPC must resume with the SAME `StartFrom` and an
updated `seen` set covering every id already emitted at that boundary timestamp across ALL
sub-RPCs so far in the current logical page — not just the last sub-RPC's own emissions. This is
a structural generalization of the existing `seen`-carry-forward logic
(`store.go:1566-1570`, "carry forward prior seen ids if the boundary did not advance"), applied at
sub-RPC granularity rather than only at page granularity.
**Explicitly out of scope (per REQ-cursor-tie-safety, v2):** correctness under CONCURRENT inserts
during a scan, and ties exceeding `maxListLimit` (1000) — those are the pre-existing, documented
limitations `PITFALLS.md` Pitfall 3 already describes and the milestone REQUIREMENTS.md defers to
v2. This phase's ties-across-RPC-boundary requirement is narrower: correctness for a STATIC
(no-concurrent-write) oversized fixture, which is exactly what `storetest.SeedOversized`'s
`ManySmall` shape produces (1000 records, one per second, deterministic — `seed.go:44-48,163-168`
— so ties do not even arise in the SHIPPED fixture unless a test deliberately constructs one with
identical timestamps).

### Pattern 2: Byte-budget extension of `scrollAllPoints`, same function name

**What:** `scrollAllPoints` (`spine.go:46-69`) is the package's one whole-spine `ScrollAndOffset`
iterator, already parameterized by `filter`, `withPayload`, and a per-page callback `fn`, with a
test-overridable `var spineScrollBatch uint32 = 256` (`spine.go:28`). D-03.2's byte-budget
extension should ADD a byte-budget parameter to this SAME function (not introduce a differently
named sibling) — this keeps its "one whole-spine iterator" doc-comment claim
(`spine.go:30-38`) literally true and requires **no** `recallTransmitters` reclassification, since
the AST gate keys on function NAME, not signature (`schemaversion_recallgate_test.go`'s own
`enclosingFuncDisplayName`-keyed identity — see "Gates This Phase Touches" below).
**When to use:** Any of the five operator sweeps Phase 5 migrates (`migrate`, `migrate revert`,
`summarize-missing`, `spine-review` scan/verify/purge, `reindex`) — but Phase 3 only needs the
signature change proven against `storetest.SeedOversized`, not the five call sites re-pointed.
**Signature shape (illustrative, exact identifiers are Claude's discretion):**
```go
// Illustrative signature — extends spine.go:46 in place, adds two params.
// maxRecordBytes and pageByteBudget are D-02/D-06's derived constants.
func (s *Store) scrollAllPoints(ctx context.Context, filter *qdrant.Filter,
    withPayload *qdrant.WithPayloadSelector, maxRecordBytes, pageByteBudget int,
    fn func(*qdrant.RetrievedPoint) error) error {
    var offset *qdrant.PointId
    for {
        perRPCLimit := pageByteBudget / max(maxRecordBytes, 1) // floor division, D-02
        pts, next, err := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
            CollectionName: s.collection, Filter: filter,
            Limit: qdrant.PtrOf(uint32(min(perRPCLimit, int(spineScrollBatch)))),
            Offset: offset, WithPayload: withPayload,
        })
        // ... measure bytes via proto.Size(p) per point, batch-of-1 fallback (D-07) on overflow ...
    }
}
```
Note: `scrollAllPoints` uses POINT-ID `Offset` (unordered sweep — no `OrderBy` at all), so it is
NOT subject to the "Offset+OrderBy can't combine" constraint above; that constraint is specific
to Pattern 1's ordered reads.

### Anti-Patterns to Avoid

- **Truncating a response after the oversized RPC already succeeded or failed:** the shrink must
  happen BEFORE the Qdrant call (a smaller `Limit`, or a narrower `WithPayloadSelector`) — you
  cannot locally truncate a response that already failed to arrive, or already overflowed the
  wire `[CITED: PITFALLS.md Performance Traps table]`.
- **Combining `OrderBy` with point-id `Offset`:** confirmed above this does not work as a
  pagination mechanism for a non-unique sort key — `next_page_offset` is not even returned by
  Qdrant in this mode.
- **Introducing a second, differently-named whole-spine iterator** instead of extending
  `scrollAllPoints` in place — this is the exact anti-pattern `ARCHITECTURE.md` Anti-Pattern 2
  already names for this milestone, and it would also force an avoidable `recallTransmitters`/
  `operatorMigrationEmitters` reclassification churn.
- **Assuming Qdrant compression shrinks the effective byte budget:** the client's receive-limit
  check applies to the DECOMPRESSED size; `storetest.SeedOversized`'s own low-entropy filler
  (`strings.Repeat("x", ...)`, `seed.go:212`) is fine because decompressed size is what's measured,
  not compressibility `[CITED: PITFALLS.md Pitfall 6, grpc/grpc-go#4761]`.

## Payload Ceiling Derivation (D-02, Q1/Q2)

### Every payload field, its cap, and its provenance

Enumerated by reading `payload()`/`fromPayload()` verbatim (`store.go:633-830`, quoted inline
above at "Memory struct" citation) against every write-side validator that bounds it:

| Payload key | Go field | Cap today | Cap source | Bounded? |
|---|---|---|---|---|
| `content` | `Content` | **none** (subject of D-01, this phase) | — | **NO — this phase adds one (64 KiB)** |
| `summary` | `Summary` | `ENGRAM_MEMORY_MAX_SUMMARY_BYTES`, default 512 B, **"0" disables the bound entirely** | `internal/config/config.go:97-103`, `validate.go:79-81`, enforced in `validateStoreArgs`/`validateUpdateArgs` (`tools.go:849-886`) | Conditionally — an operator can set the env var to `0` and make it unbounded |
| `citations` (list) | `Citations []Citation` | `maxDiscoveryCitations = 50` entries; each `Excerpt` ≤ `maxCitationExcerptBytes = 16 * 1024` | `internal/server/tools.go:768-771`, enforced by `validateCitations` (`tools.go:915-937`), called with `minCount 0` from `storeMemory`/`scheduleMemory`/`supersedeMemory` (`tools.go:1154, ...`, confirmed by reading `validateCitations`'s doc comment at `tools.go:907-914`: *"the memory write handlers (store_memory/schedule_memory/supersede_memory) call it with minCount 0"*) | Count and Excerpt bounded; `Ref`/`Locator`/`Pin` per citation are **NOT** length-bounded anywhere `[VERIFIED: no length check on citationArg.Ref/Locator/Pin found in internal/server/tools.go]` |
| `tags` (list) | `Tags []string` | **none** — no per-tag length cap, no count cap found anywhere in `internal/server` or `internal/store` | — | **NO — genuinely unbounded** |
| `supersedes` (list) | `Supersedes []string` | Each entry ≤ `maxSupersedeTargetBytes = 256` bytes; **no cap on the NUMBER of targets** (PD-07, `tools.go:781-793` doc comment: *"deliberately declined a cap on the NUMBER of targets in the set"*) | `internal/server/tools.go:781-793` | Per-entry length bounded; count unbounded |
| `scope`, `repo`, `workspace`, `worktree_path`, `base_dir`, `source`, `category`, `actor`, `owner`, `visibility`, `kind` | various `string` | No explicit length cap found | — | Server-set or short enum-like strings in practice; not attacker-adjustable content, low priority |
| `created_at`, `not_before`, `not_after`, `archived_at`, `last_accessed_at`, `summary_egress_at` | `time.Time`/`*time.Time` | Fixed-size (RFC3339 string or int64 unix seconds) | — | Bounded by construction |
| `superseded_by` | `*string` | One UUID, fixed size | — | Bounded by construction |
| `access_count` | `uint64` | Fixed 8 bytes | — | Bounded by construction |
| `schema_version` | `int` | Fixed, small integer | — | Bounded by construction |
| `short_id` | `string` | ~10-char Crockford base32, fixed | — | Bounded by construction |
| `embedder_identity`, `idempotency_fingerprint` | `string` | Config-derived / sha256 hex (64 chars), fixed | — | Bounded by construction |
| `summary_source`, `summary_model` | `string` | Small enum/config-derived string | — | Bounded in practice |

**The two genuinely unbounded fields this phase must account for (Q1's explicit ask):**
1. **`tags`** — no cap exists. D-02's `maxRecordBytes` derivation cannot assume a small
   contribution from tags. **Recommendation:** either (a) treat this as an accepted, documented
   gap the byte-budget mechanism papers over via measured-bytes accounting (the primitive stops a
   page on ACTUAL measured size regardless of which field caused it, so an unbounded-tags record
   still gets caught — see "Legacy Over-Cap Fallback" below), or (b) flag a follow-up issue to add
   a tags cap in a later milestone. This phase does not need to ADD a tags cap — D-01 only scopes
   `content` — but the RESEARCH must not let `maxRecordBytes` silently assume tags are small.
2. **Citations, when the summary bound is disabled (`ENGRAM_MEMORY_MAX_SUMMARY_BYTES=0`)** — the
   worst case per record is not bounded by a single "content cap" alone.

### The true per-record worst case, by view

Two distinct ceilings, because D-04's projection means summary-view reads never fetch `content`
or `citations` for records that already have a summary:

**Full-view / sweep-view worst case** (content + citations + everything, matches what `full=true`
recall and every Phase 5 sweep must survive):
```
content:            65,536 bytes   (D-01's proposed cap)
citations:          51×(16,384 + ~300 overhead for kind/ref/locator/pin) ≈ 850,000 bytes  (50 × 16 KiB Excerpt + per-entry overhead)
summary:               512 bytes   (default; UNBOUNDED if operator sets the env var to 0 — flag as a risk, not a guarantee)
tags:                    ? bytes   (UNBOUNDED — no cap; assume a few hundred bytes realistically, but not provably bounded)
fixed fields:         ~500 bytes   (timestamps, ids, flags, schema_version, etc.)
──────────────────────────────────
≈ 917,000 bytes (~896 KiB) per record, EXCLUDING unbounded tags
```
This is **~14x the proposed 64 KiB content cap** — citations dominate the worst case, not
content. `maxRecordBytes` for the full-view/sweep path must be derived from this number (or
measured empirically against a `storetest.SeedOversized`-style fixture that also seeds max
citations), not from the content cap alone.

**Summary-view worst case** (content and citations both excluded via `WithPayloadExclude`, per
D-04 — see "Payload Projection by View" below):
```
summary:               512 bytes   (or the fallback truncation of content, which summary-view
                                     ITSELF avoids fetching per D-04 — see below)
tags:                    ? bytes   (unbounded, same caveat)
fixed fields:         ~500 bytes
──────────────────────────────────
≈ 1,000 bytes per record (order of magnitude), EXCLUDING unbounded tags
```
This confirms D-04's projection is not a nice-to-have — it changes `maxRecordBytes` by roughly
three orders of magnitude (1 KB vs ~900 KB), which directly determines the per-RPC record count
(`floor(rpcBudget / maxRecordBytes)`): at a 2 MiB `rpcBudget` (D-06), summary-view could fetch
~2000 records per RPC while full-view/sweep-view could fetch only ~2-3 records per RPC before
hitting the budget. **This asymmetry is the single most important number for D-06's constants** —
a single `maxRecordBytes` constant shared across both views would either starve summary-view
paging (tiny pages) or under-protect full-view/sweep reads (still overflow-prone).
**Recommendation:** derive TWO `maxRecordBytes`-shaped constants (or a function of the payload
selector), not one.

### Measuring received bytes (Q2)

`*qdrant.RetrievedPoint` is a genuine protobuf message (`protoimpl.MessageState`-backed)
`[VERIFIED: qdrant-go-client@v1.19.2 points.pb.go:9876-9888]`, so
`proto.Size(p)` (`google.golang.org/protobuf/proto`, already a direct dependency
`[VERIFIED: go.mod:47]`, already imported elsewhere in this module
`[VERIFIED: internal/server/connectapi.go:17]`) gives the exact serialized byte size of one
received point, summable across a page to get the logical page total. **Cost:** `proto.Size`
performs a full marshal-size traversal (O(message size)) — cheap relative to the network I/O
already paid to receive the point, and it runs once per received record on the client side
(after the RPC has already succeeded), so it cannot itself cause an overflow; it is purely an
accounting mechanism for the NEXT page/RPC's sizing decision (Pattern 2 from `ARCHITECTURE.md`).
`internal/store` does not currently import `google.golang.org/protobuf/proto` — this is a new
import for the package, but not a new module dependency, so no `go.mod`/`go.sum` change and no
package-legitimacy concern.

## Budget Constants (D-06, Q2)

D-06 asks for a per-RPC byte budget and a page byte budget "on the order of 2 MiB," with an
unexported `var` seam mirroring `spineScrollBatch`. Given the derivation above:

| Constant (illustrative names) | Proposed value | Rationale |
|---|---|---|
| `rpcByteBudget` (`var`, not `const`) | 2 MiB (2,097,152) | Comfortably under `storetest.RecvLimit` (4 MiB), matching D-06's "order of 2 MiB" guidance; headroom absorbs protobuf/gRPC framing overhead (small per-message envelope bytes, negligible relative to 2 MiB) and leaves room for the response wrapper (`ListMemoriesResponse`, MCP tool-result envelope) on top of the raw Qdrant RPC |
| `pageByteBudget` (`var`, not `const`) | 2 MiB (can equal `rpcByteBudget`, or set independently — Claude's discretion) | Bounds what the CALLER (Connect/MCP response) receives, distinct from what any one Qdrant RPC receives; D-06 explicitly separates these two budgets |
| Full-view/sweep `maxRecordBytes` | ~900 KiB - 1 MiB (rounded up from the ~896 KiB derivation above, leaving headroom for unbounded tags) | Yields `floor(2 MiB / 1 MiB) = 2` records per RPC in the worst case — correctly conservative; the REALISTIC per-RPC count is much higher when citations/tags are small, since this bound only governs the request `Limit`, not actual measured bytes |
| Summary-view `maxRecordBytes` | ~2-4 KiB (rounded up from the ~1 KB derivation, leaving headroom for tags) | Yields `floor(2 MiB / 4 KiB) = 512` records per RPC — matches `maxListLimit = 1000`'s existing ergonomics reasonably well |

**These are illustrative starting points, not locked values** — Claude's Discretion (per
CONTEXT.md) covers the exact numbers. The important, RESEARCH-derived constraint is that the
full-view/sweep constant must be computed from content+citations+tags-worst-case (not content
alone), and the two views need materially different constants (not one shared value) given the
~3-order-of-magnitude gap.

**Measurement discipline:** whatever constants are chosen, prove them the same way
`TestListScopesFullPayloadsOverGRPCLimit` proves its own claim — a live `storetest.SeedOversized`
fixture that seeds citations/tags at their real caps (not just `content`) alongside the primitive
under test, asserting `proto.Size`-measured actual bytes cross the named limit.

## Ordered-Page Helper: Exposing "Page Cut Short by Budget" (Q3, Claude's Discretion)

REQ-list-contract-unchanged (Phase 4) requires that a page shortened by the byte budget is never
reported as the last page — this must be true regardless of the exact wire shape Phase 4 later
picks, so the PRIMITIVE (this phase) should return enough information for ANY of the following
Phase 4 shapes to be built on top of it without a primitive-level change:

**Viable return shapes for the primitive** (Claude's Discretion which to build; listed so Phase
4's decision doesn't require reworking Phase 3's primitive):
1. Return `(items []Memory, cutByBudget bool, resumeCursor listCursor, err error)` — an explicit
   boolean flag distinguishing "budget-truncated, more available at this exact position" from
   "exhausted, no more data" (mirrors Pitfall 8's explicit recommendation in `PITFALLS.md`: *"introduce
   an explicit signal... distinct from `nextCursor == \"\"`"*).
2. Return the SAME shape `listByCursor` already returns (`items []Memory, nextCursor string,
   err error`) but NEVER emit an empty `nextCursor` when the page was cut by budget rather than by
   exhaustion — i.e., internally keep fetching sub-pages until either `limit` distinct records
   are assembled OR the filtered set is truly exhausted, so budget-cutting is invisible to the
   caller and the EXISTING `len(out) < limit` invariant (`store.go:1554`) is preserved exactly.
   This is Pitfall 8's alternative (a): *"do not expose the shrink to the caller at all."*

**Recommendation:** prefer shape 2 (internally absorb the budget cut via more RPCs, never surface
it) when the record's caller-visible `limit` is itself sane (≤ `maxListLimit`) — this requires NO
Phase 4 wire change and preserves every existing invariant the Pitfalls research names (`total`
independent of paging; short-page-means-last-page). Only fall back to shape 1 (an explicit new
signal) if a single logical page, even after internal re-fetching, still cannot be assembled within
some bounded number of RPCs (e.g., `limit` itself times `maxRecordBytes` exceeds what's
achievable) — this is the genuinely-oversized-record edge D-07 covers.

**Offset-mode (`limit: 0`/deep offset) viable shapes**, for REQ-list-limit-contract-decided
(Phase 4's decision, informed by what the primitive can serve):
- **Shape A — internal re-paging, wire contract unchanged:** the primitive fetches
  `opts.Offset + opts.Limit` records via SEVERAL bounded RPCs (chained `StartFrom` resumes,
  descending `created_at`, same as today's single Scroll but split), then trims the leading
  `opts.Offset` client-side exactly as `List` does today (`store.go:1482-1485`). No wire change;
  same `O(offset)` cost profile as today (already a documented, accepted characteristic per
  `PITFALLS.md`'s Performance Traps table), but never overflows a single RPC.
- **Shape B — hard cap + cursor-only:** deprecate `limit: 0`/deep offset in favor of cursor
  paging exclusively. This is the REQ-list-limit-contract-decided "moves to a hard cap plus cursor
  paging" branch — a wire-breaking change Phase 4 must decide, not Phase 3.
- The primitive built in THIS phase should support Shape A trivially (it's the same keyset
  mechanism, just fetching more records before trimming) and does not preclude Shape B (Phase 4
  could simply stop calling the offset-mode entry point). **No primitive-level decision is forced
  by Phase 3** — this is the correct outcome, since REQ-list-limit-contract-decided is explicitly
  a Phase 4 requirement.

## Byte-Budget Sweep Primitive: Extending `scrollAllPoints` (D-03.2, Q4)

`scrollAllPoints`'s single-call-site property (`spine.go:30-38`'s doc comment: *"ScanSpine... and
every later whole-spine sweep this phase adds... route through this single call site"*) is
preserved by extending its EXISTING signature in place (Pattern 2 above) rather than adding a
parallel function. Its existing callers — `ScanSpine`, `EnumerateCitations`, `NearDuplicates`' id
enumeration, `derivePurgeEligible` (all within `spine.go`) — are NOT this phase's concern to
re-point (Phase 5 does that); this phase only needs the signature extension to compile and pass
its OWN new tests without breaking those four existing (test-covered) callers, which means the
byte-budget parameters likely need sane defaults or the extension must be additive (e.g., new
trailing parameters, or a `Options` struct with a zero-value = "no additional budget, existing
256-batch behavior only" default) so the four untouched callers need no code change in THIS phase.
**Recommendation:** thread the byte-budget/maxRecordBytes as new PARAMETERS (not a struct field
merge into existing behavior) so the four existing callers can be updated in a small,
mechanical follow-up within this same phase (passing the new full-view/sweep `maxRecordBytes` and
`rpcByteBudget` constants) rather than silently changing behavior via a hidden default — matching
this phase's discretion note ("Test design for the primitives... proven on its own against
`storetest.SeedOversized`").

## Payload Projection by View (D-04, Q5)

`qdrant.NewWithPayloadInclude(...)` / `NewWithPayloadExclude(...)` are already proven in this
codebase (`store.go:1726`, `ListScopes`) — the primitives accept a `*qdrant.WithPayloadSelector`
parameter (already the plan, per D-03/D-04) rather than hardcoding one.

**The summary-view-without-content problem, confirmed by reading the actual recall render path:**
`summaryOrTruncation` (`internal/server/summary.go:64-69`) reads `m.Content` ONLY when
`m.Summary == ""` — for a record that already has a summary, `m.Content` is never read at
render time. This means: for records WITH a summary, a summary-view fetch can safely
`WithPayloadExclude("content", "citations")` and skip roughly 900 KiB of worst-case payload per
record. But the caller does NOT know, before fetching, which of a page's records lack a summary
— so a record WITHOUT a summary, fetched with `content` excluded, would render an EMPTY
truncation fallback (a regression: `truncateForRecall` needs the actual content).

**Recommended two-step design for Phase 4 to wire** (Phase 3's primitives just need to accept a
selector — this decision belongs to Phase 4's wiring, documented here so Phase 3's signature
doesn't foreclose it):
1. First pass: fetch the page with `WithPayloadExclude("content", "citations")` (summary,
   tags, category, scope, created_at, score, access_count, last_accessed_at, short_id, id,
   summary_source all included/small).
2. Identify the subset of the page's records where `Summary == ""`.
3. Second, TARGETED fetch (`Store.Get`-shaped batch, or a small `WithPayloadInclude("content")`
   Scroll filtered to just those ids) for ONLY that subset's `content`, to compute
   `truncateForRecall`'s fallback snippet.
4. Merge: records with a summary render from step 1's fetch alone; records without one splice in
   step 3's content-derived truncation.

This is the option this phase's `<additional_context>` calls out as "a second targeted fetch for
those ids" — it is RECOMMENDED over alternatives (e.g., "always include content but truncate
client-side after fetch," which does not avoid the oversized RPC in the first place — see
Anti-Pattern 1 above) because it is the only shape that actually avoids requesting the ~900 KiB
worst case for the (likely common, post-auto-summary-sweep) case where most records already carry
a summary. **This is explicitly a Phase 4 wiring decision** — Phase 3's acceptance criterion is
narrower: the primitive must ACCEPT a caller-supplied selector and be provably correct under BOTH
an all-fields selector and a content/citations-excluding selector, proven against
`storetest.SeedOversized`.

## Legacy Over-Cap Fallback (D-07, Q6)

The batch-of-1 fallback lives INSIDE the ordered-page helper's/`scrollAllPoints`'s per-RPC retry
logic: when a computed `Limit` (derived from `maxRecordBytes`) still overflows because a
PRE-CAP legacy record's actual content vastly exceeds `maxRecordBytes` (D-01's cap only rejects
NEW writes; `D-01`'s own text: *"Existing records larger than the cap stay stored and readable"*),
retry the SAME RPC position with `Limit: 1`. If that single-record RPC STILL overflows
`storetest.RecvLimit`, the request fails with the ALREADY-NAMED `store.ErrResponseTooLarge`
(Phase 2, `field=response hint=too_large`, exit 10) — never silently skipped or truncated (D-07's
explicit text, matching the milestone's Out-of-Scope table: *"Silent truncation of an oversized
result"* is out of scope entirely).

**Interaction with the ordered keyset:** a batch-of-1 retry at a given `StartFrom` position still
returns EXACTLY one record (or zero, if none remain) at that resume point — the keyset/`seen`-set
bookkeeping is unaffected, since the retry is a re-issue of the SAME logical position with a
smaller `Limit`, not a skip. The one subtlety: if that single record is itself part of a
`created_at` tie, the `seen` set must still record its id as emitted before advancing, exactly as
the normal-path emission loop does today (`store.go:1573-1577`).

**Interaction with the sweep primitive:** identical shape — `scrollAllPoints`'s point-id `Offset`
resume is unaffected by a batch-of-1 retry at the same offset position; a `Limit: 1` retry that
still overflows fails the whole sweep with `ErrResponseTooLarge`, consistent with D-07's "never
silently skipped."

**How to test this (D-07's specific test-design note):** a legacy over-cap record cannot be
produced through the normal write path once D-01's cap is enforced — `storetest.SeedOversized`
itself cannot produce one either, since it validates its own oversized-by-accumulation invariant
(`checkOversized` explicitly REJECTS `recordBytes >= limit`, i.e. a single record at or over the
limit — `seed.go:125-140`, the doc comment: *"oversized fixtures must overflow by accumulation,
never by one oversized record — the deferred single-large-record content-cap question"*). This
phase's test for the batch-of-1 fallback must therefore write a genuinely-oversized SINGLE record
via a RAW `store.Upsert` call in a `package store` test (bypassing both the new content-cap
validation, which lives in `internal/server`, not `internal/store`, and `SeedOversized`'s own
accumulation-only invariant) — exactly as this phase's `<additional_context>` names ("a raw
over-cap write in a `package store` test bypassing the new cap"). Since `Store.Upsert` has no
content-length validation of its own (`internal/server`'s `deps.storeMemory` is the enforcement
point, not `Store.Upsert`), a same-package test can freely construct a `Memory{Content:
strings.Repeat("x", storetest.RecvLimit*2)}` and `Upsert` it directly, then exercise the new
primitive against it.

## Content Cap Enforcement (D-01, Q7)

### Every write path that accepts `content`, and where it validates today

| Surface | Function | File:line | Today's validation | Where content-cap check must be added |
|---|---|---|---|---|
| MCP `store_memory` | `deps.storeMemory` | `tools.go:1150-1176` | `validateStoreArgs(a, maxSummaryBytes)` at `tools.go:1151` (summary cap only) | Inside `validateStoreArgs` (`tools.go:849-866`) — shared by all three of storeMemory/scheduleMemory/supersedeMemory |
| Connect `StoreMemory` | same `deps.storeMemory` | same | same (Connect routes through the SAME shared method, `connectapi.go:444`) | Same fix as above — no separate Connect-side code path exists |
| MCP `schedule_memory` | `deps.scheduleMemory` | `tools.go:1231-...` | `validateStoreArgs(a.storeArgs, maxSummaryBytes)` at `tools.go:1232` | Same fix (shared validator) |
| Connect `ScheduleMemory` | same `deps.scheduleMemory` | same | same (`connectapi.go:508`) | Same fix |
| MCP `supersede_memory` | `deps.supersedeMemory` | `tools.go:2112-...` | `validateStoreArgs(a.storeArgs, maxSummaryBytes)` at `tools.go:2113` | Same fix |
| Connect (no separate SupersedeMemory RPC confirmed — MCP-only tool per the memory contract) | — | — | — | N/A |
| MCP `update_memory` | MCP closure wrapping `deps.updateMemory` | closure at `tools.go:2491-2509`; `validateUpdateArgs` call at `tools.go:2504` | `validateUpdateArgs(a, maxSummaryBytes)` — summary cap only, MCP closure ONLY | Content-cap check must NOT go in `validateUpdateArgs` alone — see below |
| Connect `UpdateMemory` | `deps.updateMemory` directly | `connectapi.go:471-481` calls `a.d.updateMemory(...)` directly, bypassing `validateUpdateArgs` entirely `[VERIFIED: internal/server/connectapi.go:476]` | NONE today for content length | Must be added INSIDE `deps.updateMemory` itself, guarded by `a.Content != nil` |
| `engram store` CLI | Connect client → `StoreMemory` RPC | `cmd/engram/client_store.go` | Client-side: only presence (`--content is required`, `client_store.go:45`); no length check | None needed — the CLI is a Connect client; the server-side `deps.storeMemory` fix covers it automatically. **There is no `engram update`/`engram supersede` CLI command** `[VERIFIED: CLAUDE.md Layout table: client-tier commands are get/search/list/store/migration-status only]` — D-01's "CLI" surface is fully covered by `engram store` alone. |

**Critical finding: `validateUpdateArgs` is the WRONG place for the content-cap check.**
`updateArgs.Content`'s own doc comment (referenced at `tools.go:1702-1703`) and the MCP closure's
comment (`tools.go:2497-2503`) both explain why `validateUpdateArgs`'s content-REQUIRED check
lives ONLY in the MCP closure: Connect's field-mask lane legitimately calls `deps.updateMemory`
with a `nil` `Content` (no change requested). Since `connectapi.go:476` calls
`a.d.updateMemory(...)` directly — never `validateUpdateArgs` — a content-cap check placed
only inside `validateUpdateArgs` would NEVER run for Connect's `UpdateMemory` RPC, leaving the
Connect lane with zero content-cap enforcement on updates. The fix must instead live inside
`deps.updateMemory` (`tools.go:1700-1783`) itself, gated on `a.Content != nil` (which is
ALREADY the exact boolean `contentChanged` at `tools.go:1738` derives), so both lanes get it from
one call site.

### The registry pattern to follow (D-01's explicit precedent)

`ENGRAM_MEMORY_MAX_SUMMARY_BYTES` is the byte-for-byte precedent: a `koanf`-configurable registry
field (not a compile-time constant, per D-01's phrasing and the existing `MaxSummaryBytes`
doc comment: *"Sean approved this bound on the explicit condition that it be koanf-configurable
rather than a compile-time constant"* `[VERIFIED: internal/config/config.go:89-96]`), validated
unconditionally in `Config.Validate()`, with `"0"` as the documented disable convention.

```go
// Source: internal/config/registry.go:41-44 (existing MaxSummaryBytes registration —
// the pattern to copy for ENGRAM_MEMORY_MAX_CONTENT_BYTES)
{Key: "memory.max_summary_bytes", Env: "ENGRAM_MEMORY_MAX_SUMMARY_BYTES", Default: "512"},
```
```go
// Source: internal/config/validate.go:75-81 (existing unconditional validation —
// the pattern to copy)
if _, err := strconv.ParseUint(c.Memory.MaxSummaryBytes, 10, 64); err != nil {
    errs = append(errs, fmt.Errorf("ENGRAM_MEMORY_MAX_SUMMARY_BYTES %q: must be a non-negative integer: %w", c.Memory.MaxSummaryBytes, err))
}
```
```go
// Source: internal/server/tools.go:849-851 (existing enforcement call shape —
// the pattern to copy for content, reusing the EXISTING HintTooLong hint code
// per D-01's explicit "no new hint code" instruction)
if maxSummaryBytes > 0 && len(a.Summary) > maxSummaryBytes {
    return argErrf(classOutOfRange, HintTooLong, "summary", "summary too large: %d bytes (max %d)", len(a.Summary), maxSummaryBytes)
}
```

D-01 pins the default at 64 KiB (65536), matching `maxDiscoveryContentBytes`
(`tools.go:769`) — but unlike `MaxSummaryBytes`'s "0 disables" convention, D-01's text does not
mention a disable escape hatch; Claude's Discretion should decide whether "0 disables" is
inherited from the `MaxSummaryBytes` precedent or whether the content cap is always-enforced
(no disable value) — flag this as an open question for discuss-phase confirmation if not
already resolved, since D-01's context doesn't explicitly rule either way.

**Minor test-literal ripple:** exactly 3 files hand-construct `MemoryConfig{...}` in tests
(`internal/config/config_test.go:205`, `internal/config/validate_test.go:17`,
`internal/config/service_auth_test.go:271`) `[VERIFIED: rg count above]`, all three already set
`MaxSummaryBytes: "512"` explicitly — adding a new field validated unconditionally
(mirroring `MaxSummaryBytes`'s discipline) means these same 3 literals need the new field added
too, a small, bounded, already-precedented ripple (not the ~33-literal `ValidateClient` scope the
STATE.md gotcha warns about — that gotcha is about a DIFFERENT validation function).

### Documentation surfaces (D-01's "document the variable" instruction)

`docs-site/src/content/docs/guides/configure.md` already documents `ENGRAM_MEMORY_MAX_SUMMARY_BYTES`
in a table row (`configure.md:44`); add a parallel row for `ENGRAM_MEMORY_MAX_CONTENT_BYTES`.
`docs-site/src/content/docs/reference/errors.md` already documents the generic `too_long` hint
code (line 105) and shows a `field=summary hint=too_long` worked example (lines 24-27) — per D-01
("no new hint code"), no new table row is strictly required, but adding a
`field=content hint=too_long` worked example alongside the existing summary one keeps parity.
**Discovery/rule content is explicitly OUT of scope** — `maxDiscoveryContentBytes` (64 KiB,
`tools.go:769`) and `maxRuleContentBytes` (8 KiB, `rules.go:21`) already have their own,
independent caps; D-01 only names the four curated-memory write paths.

## Call-Site Inventory (D-08, Q8)

Every non-test `NewWithPayload(true)`/`NewWithPayload(false)`/`NewWithPayloadInclude`/unbounded
`Scroll`/`ScrollAndOffset`/`Query` site in `internal/store`, confirmed by reading each function
body directly this session (superset of `ARCHITECTURE.md`'s own inventory table, cross-checked
against it):

| Function | File:line | Current bound | Response shape risk | Assigned phase / exemption |
|---|---|---|---|---|
| `Store.List` (offset-mode, `Limit:0`) | `store.go:1382-1486`, Scroll at `:1468` | `fetch = total` — unbounded | Full payload, unbounded record count | **Phase 4** |
| `Store.List` (offset-mode, deep offset) | same, `fetch = opts.Offset+opts.Limit` | grows with page depth | Full payload | **Phase 4** |
| `Store.listByCursor` | `store.go:1496-1579`, Scroll at `:1526` | `limit` clamped to `maxListLimit=1000` | 1000 full payloads | **Phase 4** |
| `Store.ListScheduled` | `store.go:1632-1688`, Scroll at `:1674` | caller-supplied `opts.Limit`, no ceiling | Unbounded if caller passes a large Limit | **Phase 4** |
| `Store.ListScopes` | `store.go:1706-1741`, Scroll at `:1722` | `scanCap=1000`, `WithPayloadInclude("scope")` only | Already bounded (#583 fix) | **Exempt** — already payload-scoped; kept as plain `Scroll` deliberately per its own doc comment (`store.go:1701-1705`) for the recall-gate AST test |
| `Store.Get` | `store.go:1744-1771` | Single point, `WithPayload(true)` | ONE record's full payload — bounded by whatever content/citation caps exist (D-01 helps here); still the ~900 KiB full-view worst case for one record | **Exempt with caveat** — single-point fetch is a fundamentally different risk shape (one record, not a page); D-01's cap plus the eventual production `MaxCallRecvMsgSize` backstop (Phase 5) cover this. Not migrated to either new primitive since there is no "page" to bound here — flag as an EXEMPTION with justification (id-addressed, ungated by design, no page-size knob) |
| `Store.ResolvePointID` | `store.go:1778-1810`, Scroll at `:1798` | `Limit:2`, `WithPayload(false)` | Already payload-frugal (`WithPayload(false)`), fixed tiny limit | **Exempt** — no payload requested at all |
| `Store.Search` | `store.go:1117-...` (per `ARCHITECTURE.md`, Query at `:1139`/confirmed func start `:1117`) | `k` caller-controlled, no maximum | Unbounded `k` × full payload | **Phase 4** |
| `Store.SearchDiscovery` | `store.go:1229-...` (func start `:1229`, per `ARCHITECTURE.md` Query at `:1230`) | Same shape, `k` uncapped | Unbounded | **Phase 4** |
| `Store.Reindex` | `store.go:3166-...`, `ScrollAndOffset` at `:3199`, `WithVectors(false)` at `:3237` | `reindexBatch=256` default | 256 full payloads/page | **Phase 5** |
| `Store.scrollAllPoints` | `spine.go:46-69`, `ScrollAndOffset` at `:49` | `spineScrollBatch=256` var | 256/page | **Phase 3 EXTENDS signature here; Phase 5 re-points its callers' constants** |
| `Store.Migrate` (3 sites) | `migrate.go:314,377,531`, `ScrollAndOffset` | `migrateBatch=256` | 256/page each | **Phase 5** |
| `Store.revertWithSteps` | `revert.go:431`, `ScrollAndOffset` | 256/page (mirrors migrate.go) | 256/page | **Phase 5** |
| `Store.SummarizeMissing` | `summarize.go:145`, `ScrollAndOffset` | Hardcoded `uint32(256)` literal | 256/page | **Phase 5** |
| `Store.reindexTargetContents` | `store.go:3418-...`, `Get` batch, `WithVectors(false)` at `:3452` | Bounded by calling page's `batch` (256) | Same 256-record ceiling, one Get per reindex page | **Phase 5** (rides along with Reindex) |
| `Store.ScanSpine`/`EnumerateCitations`/`NearDuplicates` id-enum/`derivePurgeEligible` | `spine.go`, all route through `scrollAllPoints` | Inherits `scrollAllPoints`'s bound | Payload-frugal by design (`NearDuplicates` uses `WithPayloadInclude("short_id","scope")`, `spine.go:558`) | **Phase 5** rides along with `scrollAllPoints`'s extension — low individual risk |
| `Store.CountOwnerless`/`CountAnonymousBucket`/`MigrateSetOwner`/`RemapOwner`/`MigrateStatus`/`CountExpired`/`NearDuplicates` (QueryBatch) | various, `Count`/`QueryBatch` only | N/A — `Count` and `QueryBatch` (near-dup search) do not fetch full record payloads at scale the same way | **Exempt** — `Count` returns a number, not payloads; `NearDuplicates`' `QueryBatch` is payload-frugal per its own id-enum classification above |

**Vectors confirmed NOT requested by any of the above** except `Reindex`/`reindexTargetContents`,
which explicitly set `WithVectors(false)` (`store.go:3237,3452`) — meaning even THOSE two
explicitly suppress vectors. Every OTHER site (`List`, `listByCursor`, `ListScheduled`,
`ListScopes`, `Search`, `SearchDiscovery`, `Get`, `ResolvePointID`, `scrollAllPoints` and its
callers) leaves `WithVectors` unset on the request. `[ASSUMED]` — Qdrant's server-side default for
an unset `with_vectors` selector is `false` (vectors omitted) per general Qdrant API convention;
this was not falsified against a live server this session and should be spot-checked in Phase 3's
own tests (e.g., assert `RetrievedPoint.Vectors == nil` on a `List` result) before relying on it
for `maxRecordBytes`'s derivation — if vectors WERE returned by default, `embed.dim`-sized float32
vectors (e.g., 1024 dims × 4 bytes = 4 KiB per vector) would need to be added to every ceiling
above.

## Gates This Phase Touches (Q9)

**`schemaversion_recallgate_test.go`'s three-list classification (`recallTransmitters` /
`operatorMigrationEmitters` / `otherNonRecallEmitters`) requires EVERY `Query`/`QueryBatch`/
`Scroll`/`ScrollAndOffset`/`Count` emission site in the WHOLE package — not just ones reachable
from the six recall seeds — to appear in exactly one of the three lists** `[VERIFIED:
internal/store/schemaversion_recallgate_test.go:672-713, specifically the "union of the three
classification lists vs the full derived emission set" assertion at line 712]`. This has a
non-obvious consequence for Phase 3 specifically:

- If Phase 3's new ORDERED-PAGE HELPER is a genuinely new function (not a rename of
  `listByCursor`) that itself calls `Scroll`, it becomes a NEW entry in the derived emission set
  the moment it's merged — REGARDLESS of whether anything calls it yet. Since Phase 3 does NOT
  wire `Store.List`/`ListScheduled` to call this new helper (that's Phase 4's job), the helper is
  NOT reachable from any of the six `recallEntryPointSeeds` in Phase 3's own tree state. Per the
  gate's "reachable emission completeness" subtest (line 678: `assertNameSetEqual(...,
  reachableEmission, classificationNames(recallTransmitters))`), putting it in `recallTransmitters`
  THIS phase would make it an "extra" entry (classified but not yet reachable) and FAIL that
  subtest. **The new helper must instead be classified in `otherNonRecallEmitters` in Phase 3**,
  with a justification explicitly noting it is not yet wired into any recall entry point and will
  move to `recallTransmitters` in Phase 4, in the SAME change that wires the caller — mirroring
  `Store.scrollAllPoints`'s own existing justification text, which anticipates exactly this
  (`schemaversion_recallgate_test.go:546`: *"if a future recall path ever routes through it...
  the suite goes RED"*).
- `scrollAllPoints`'s byte-budget extension (Pattern 2) does NOT introduce a new function name —
  it is already classified in `operatorMigrationEmitters`, and extending its signature in place
  requires NO reclassification (the gate keys on `enclosingFunc` name, never signature).
- **D-05's explicit rejection of the two-phase ids→payload design means Pitfalls 4/5 (TOCTOU on
  `Get`/`GetPoints`, response-order non-guarantee) do not apply to this phase** — no new
  `Get`/`GetPoints` call site is introduced by either primitive, so the recall-gate's separate
  concern about `Get` bypassing soft-hide filters is not implicated. State this explicitly in the
  plan (per this phase's own context) so the verifier does not look for TOCTOU/ordering acceptance
  criteria that this phase's design does not need.
- **D-11 (repo-wide Qdrant client-construction convergence gate,
  `qdrant_client_convergence_test.go`) and D-13 (storetest write-boundary gate) are unaffected** —
  neither primitive constructs a `*qdrant.Client`; both call methods on the EXISTING
  `s.client`/`storetest`-dialed client. No new construction site, no gate risk.
- **The red-evidence harness (`redEvidenceDirs`, `internal/store/redevidence_harness_test.go`)**
  must gain this phase's own entry, registered AFTER the last plan (per CONTEXT.md's Claude's
  Discretion item), following the exact Phase 1/Phase 2 precedent: one hand-verified `.patch` per
  acceptance criterion, each independently confirmed RED (`git apply --check`/`apply`/`go test
  -run '^Target$'`/`apply -R`) before registration.

## Common Pitfalls

### Pitfall: Assuming `content` alone determines `maxRecordBytes`

**What goes wrong:** deriving the byte-budget constants from the 64 KiB content cap alone,
missing that citations (up to ~800 KiB) and unbounded tags dominate the full-view/sweep worst
case by an order of magnitude.
**Why it happens:** the milestone's own framing repeatedly centers "content" as the headline
unbounded field; citations' contribution is buried in a comment about a DIFFERENT concern
(`connectapi.go:149`'s response-shaping note, not the Qdrant-read-side receive-limit risk).
**How to avoid:** use the two-view derivation in "Payload Ceiling Derivation" above; write a test
fixture that seeds max citations AND max tags-length-in-practice alongside content, not content
alone.
**Warning signs:** a `maxRecordBytes` constant that, when you multiply by 50 (max citations) ×
16 KiB (max excerpt), doesn't obviously exceed it.

### Pitfall: Placing the content-cap check only in `validateUpdateArgs`

**What goes wrong:** the Connect `UpdateMemory` RPC calls `deps.updateMemory` directly, bypassing
`validateUpdateArgs` entirely (confirmed at `connectapi.go:476`) — a check placed only in
`validateUpdateArgs` silently never runs for Connect, the exact #360-shaped diagnosability
regression this codebase has already been burned by once (`validateStoreArgs`/`validateUpdateArgs`'s
own doc comments reference #360 explicitly).
**How to avoid:** put the check inside `deps.updateMemory`, gated on the already-existing
`a.Content != nil` / `contentChanged` boolean (`tools.go:1738`).

### Pitfall: Reclassifying a not-yet-wired helper into `recallTransmitters` preemptively

**What goes wrong:** adding the new ordered-page helper to `recallTransmitters` in THIS phase
(anticipating Phase 4's wiring) fails the gate's exact-set-equality assertion, since nothing
reaches the helper from a seed yet.
**How to avoid:** classify in `otherNonRecallEmitters` this phase; move it in Phase 4's own change
that wires the caller (see "Gates This Phase Touches" above).

### Pitfall: Treating `storetest.SeedOversized` as capable of producing a legacy over-cap SINGLE record

**What goes wrong:** `SeedOversized`'s own `checkOversized` REJECTS a fixture where a single
record is at or over the named limit (`seed.go:133-134`) — it can only produce oversized-by-
accumulation fixtures. D-07's batch-of-1 fallback needs a genuinely single-record-over-limit
fixture, which requires a raw `store.Upsert` in a `package store` test, not `SeedOversized`.
**How to avoid:** write this fixture by hand in an `internal/store` (not `storetest`) test file,
exactly as this phase's context instructs.

## Code Examples

### Existing keyset resume, the base to generalize
```go
// Source: internal/store/store.go:1541-1552 (listByCursor's emission loop —
// the per-page boundary-tie-drop this phase's per-RPC generalization must preserve)
out := make([]Memory, 0, limit)
for _, p := range pts {
    m := fromPayload(p.Id.GetUuid(), p.Payload)
    ts := m.CreatedAt.UTC().Format(time.RFC3339)
    if ts == boundary && seen[m.ID] {
        continue // already emitted at this exact timestamp
    }
    out = append(out, m)
    if uint64(len(out)) == limit {
        break
    }
}
```

### Existing oversized-fixture seeder, reused verbatim for this phase's own primitive tests
```go
// Source: internal/store/storetest/seed.go:179 (SeedOversized's signature) +
// storetest.go:37-43 (RecvLimit) — the exact call shape this phase's primitive
// tests should use, per Phase 1's established contract
fixture := storetest.SeedOversized(t, st, storetest.Spec{
    Limit:  storetest.RecvLimit,
    Shape:  storetest.FewLarge, // or storetest.ManySmall
    Vector: someFixedDimVector,
})
```

### Existing payload-projection precedent (D-04's base)
```go
// Source: internal/store/store.go:1721-1727 (ListScopes — the only existing
// payload-selector-parameterized-by-caller-intent read in this package)
const scanCap = 1000
pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
    CollectionName: s.collection,
    Filter:         &qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(ctx, subj)}},
    Limit:          qdrant.PtrOf(uint32(scanCap)),
    WithPayload:    qdrant.NewWithPayloadInclude("scope"),
})
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Fix one overflow site at a time (#583, `ListScopes`) | One shared bounded-read mechanism reused by every site | This milestone (2026-09-18.01) | Prevents the #583→#585 recurrence pattern documented in `PITFALLS.md` Pitfall 1 |
| Count-only page caps (`maxListLimit=1000`, `reindexBatch=256`) | Count AND measured-byte caps | This phase | Closes Pitfall 2 ("a small page of huge records overflows the same cap a large page of tiny records wouldn't") |
| `content` unbounded | `content` capped at 64 KiB (D-01) | This phase | Removes the single largest per-write growth vector, though NOT the largest per-record worst case (citations remain larger) |

**Not deprecated, still current:** `listByCursor`'s keyset pattern — this research CONFIRMS it
against Qdrant's own documentation as the correct, sanctioned mechanism, not a workaround.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Qdrant's server-side default for an unset `with_vectors` selector is `false` (no vectors returned) on every read site that leaves it unset | Call-Site Inventory (D-08) | If wrong, every `maxRecordBytes` derivation in this document undercounts by one embedding vector's worth of bytes per record (e.g., 4 KiB for a 1024-dim float32 vector) — would require adding vector bytes to every ceiling and re-deriving D-06's constants. Low-cost to falsify: assert `RetrievedPoint.Vectors == nil` in this phase's own fixture test. |
| A2 | `ENGRAM_MEMORY_MAX_CONTENT_BYTES` should follow `MaxSummaryBytes`'s "0 disables" convention | Content Cap Enforcement (D-01) | If the milestone intends an always-enforced cap with no disable escape hatch, following the wrong precedent would let an operator accidentally disable a REQ-locked guarantee; low risk since discuss-phase can confirm before implementation, and the decision is easily reversible (a one-line validation change) |
| A3 | The full-view/sweep `maxRecordBytes` should be derived from content+citations+tags-worst-case, and citations' `Ref`/`Locator`/`Pin` fields' realistic size is small (a few hundred bytes) despite having no hard cap | Payload Ceiling Derivation (D-02) | If a caller writes citations with very large `Ref`/`Locator`/`Pin` values in practice, the derived constant could still be an undercount; this is a genuinely unbounded field this research flags rather than resolves — the byte-budget mechanism's MEASURED-bytes accounting (not just the pre-computed `Limit`) is the actual safety net regardless of this assumption's accuracy |

## Open Questions (RESOLVED)

1. **Does `ENGRAM_MEMORY_MAX_CONTENT_BYTES` support a "0 disables" escape hatch?**
   - What we know: `MaxSummaryBytes` has one; D-01's text doesn't explicitly say either way for
     content.
   - What's unclear: whether the milestone's REQ-locked "content cap decided" intends an
     always-on cap or a configurable-off one.
   - Recommendation: default to NOT supporting disable (an always-enforced cap) unless
     discuss-phase explicitly confirms the `MaxSummaryBytes` convention should extend — a
     REQ-locked write-rejection contract (per D-01's "Reversibility: one-way" note) is a stronger
     guarantee if it cannot be silently turned off.
   - RESOLVED: always enforced, config rejects 0 and non-positive values (CONTEXT D-09, user-confirmed 2026-09-19).

2. **Should `tags` gain a cap in this phase, or purely be flagged as a follow-up?**
   - What we know: D-01 scopes only `content`; tags has zero validation anywhere.
   - What's unclear: whether the byte-budget mechanism's measured-bytes safety net is judged
     sufficient coverage for this milestone, or whether an unbounded field left unaddressed is a
     gap worth a tracked follow-up issue.
   - Recommendation: leave tags uncapped this phase (matches D-01's explicit scope), but the
     planner should record a follow-up issue/backlog note — this research surfaced a real,
     previously-undocumented gap.
   - RESOLVED: the user chose to cap tags THIS phase — `ENGRAM_MEMORY_MAX_TAGS` 128 and `ENGRAM_MEMORY_MAX_TAG_BYTES` 128, always enforced (CONTEXT D-10); the remaining uncapped fields get a 16 KiB allowance and follow-up GitHub #589 (CONTEXT D-11).

## Environment Availability

Skipped — this phase has no new external dependencies beyond the existing Docker/testcontainers
Qdrant setup already exercised by Phases 1–2 (`storetest.Run`), which prior phases already
confirmed available in this environment.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` package + `storetest` harness (Phase 1) |
| Config file | none — Go's native test runner; `internal/store/storetest` provides fixtures |
| Quick run command | `go test ./internal/store/... -short` (skips oversized fixtures per `SeedOversized`'s `testing.Short()` gate) |
| Full suite command | `task` (lint + full test suite, real Qdrant via testcontainers or `ENGRAM_QDRANT_TEST_ADDR`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-byte-budget-pages | Ordered-page helper stops a page on accumulated bytes as well as record count | integration (real Qdrant) | `go test ./internal/store/... -run 'Test<OrderedPageHelperName>' -v` (name TBD at plan time — re-resolve against `go test -list` per the STATE.md `bsbsvn4hbc` gotcha) | ❌ Wave 0 |
| REQ-byte-budget-pages | `scrollAllPoints`'s byte-budget extension stops a sweep page on accumulated bytes | integration (real Qdrant) | `go test ./internal/store/... -run 'TestScrollAllPoints.*Byte'` (name TBD) | ❌ Wave 0 |
| REQ-content-cap-decided | `store_memory`/`schedule_memory`/`supersede_memory`/`update_memory` reject oversized content on both MCP and Connect lanes | integration | `go test ./internal/server/... -run 'TestContentTooLarge'` (name TBD) | ❌ Wave 0 |
| REQ-content-cap-decided | Existing oversized records stay readable (cap never rewrites/deletes) | integration | `go test ./internal/server/... -run 'TestLegacyOversizedContentStillReadable'` (name TBD) | ❌ Wave 0 |
| D-07 (legacy fallback) | A single pre-cap oversized record triggers batch-of-1, and still-overflowing fails with `ErrResponseTooLarge` | integration, raw `package store` write | `go test ./internal/store/... -run 'TestBatchOfOneFallback'` (name TBD) | ❌ Wave 0 |

**IMPORTANT:** every `-run` pattern above is illustrative — this phase's PLAN must name real test
functions and this project's own recorded gotcha (`bsbsvn4hbc`, STATE.md) requires re-resolving
every `-run` command against `go test -list ./internal/store/...` (or the relevant package) at
verification time, never trusting a pattern written at plan time to still match.

### Sampling Rate
- **Per task commit:** `go test ./internal/store/... -short` (fast, skips oversized fixtures)
- **Per wave merge:** `go test ./internal/store/... ./internal/server/... ./internal/config/...`
  (full, includes oversized fixtures — requires Docker/Qdrant)
- **Phase gate:** `task` (full repo lint + test) green before `/gsd-verify-work`, matching
  Phase 1/2's own closing bar.

### Wave 0 Gaps
- [ ] The ordered-page helper's own test file (new, name TBD) — covers REQ-byte-budget-pages
- [ ] `scrollAllPoints`'s extended-signature test coverage (may extend an existing spine_test.go
      file rather than a new one) — covers REQ-byte-budget-pages
- [ ] A raw-Upsert legacy-oversized-record fixture helper in `package store` (not `storetest`,
      per D-07's design) — covers the batch-of-1 fallback
- [ ] Content-cap rejection tests across MCP + Connect for all four write paths — covers
      REQ-content-cap-decided
- Framework install: none — `go test`/`storetest` already fully present from Phase 1

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | Unaffected — this phase touches read-shaping and write-validation only, not auth |
| V3 Session Management | no | Unaffected |
| V4 Access Control | no | This phase does not touch the authz/recall-gate filter conditions themselves (confirmed: D-05's rejected two-phase design would have risked this via `Get`/`GetPoints`; this phase's actual design introduces no new `Get` call site, so the recall gate's soft-hide guarantees are structurally unaffected) |
| V5 Input Validation | **yes** | `argErrf(classOutOfRange, HintTooLong, "content", ...)` — the existing `internal/server/argerror.go` validation-envelope pattern; no hand-rolled validation |
| V6 Cryptography | no | Unaffected |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Resource exhaustion via unbounded read (the milestone's own root cause) | Denial of Service | This phase's byte-budget primitives, D-01's content cap, Phase 2's `ResourceExhausted` classification (already shipped) |
| Resource exhaustion via unbounded WRITE (citations/tags growing a record past any read-side cap) | Denial of Service | NOT fully closed by this phase — citations (count+excerpt) already capped; tags remains genuinely unbounded (flagged as Open Question 2 above); the byte-budget mechanism's measured-bytes accounting is the defense-in-depth backstop regardless |
| Information leakage via error message (a rejected content-too-long response echoing raw byte counts is fine per existing convention — compare Pitfall 9 in `PITFALLS.md`, which concerns the DIFFERENT `ResourceExhausted`-from-Qdrant case, not a validation rejection) | Information Disclosure | The existing `field=content hint=too_long: content too large: %d bytes (max %d)` shape already discloses ONLY the caller's own submitted length and the configured max — both already knowable/intended to be disclosed (unlike Qdrant's internal receive-limit implementation detail) — no new leak surface |

## Sources

### Primary (HIGH confidence)
- This repo's working tree, read directly this session: `internal/store/store.go` (Memory struct
  189-357, payload()/fromPayload() 633-830, List/listByCursor/ListScheduled/ListScopes/Get/
  ResolvePointID 1382-1810), `internal/store/spine.go` (1-100, scrollAllPoints 46-69),
  `internal/store/storetest/{seed,storetest}.go` (full), `internal/store/schemaversion_recallgate_test.go`
  (340-713), `internal/server/tools.go` (280-1176, 1700-1783, 2480-2509, 755-947),
  `internal/server/connectapi.go` (140-160, 444-520), `internal/server/summary.go` (full),
  `internal/server/argerror.go` (1-65), `internal/server/rules.go` (21-22),
  `internal/config/config.go` (70-120), `internal/config/registry.go` (1-45),
  `internal/config/validate.go` (60-100), `docs-site/src/content/docs/guides/configure.md` (42-50),
  `docs-site/src/content/docs/reference/errors.md` (24-119).
- `qdrant-go-client@v1.19.2` module cache, read directly: `qdrant/points.pb.go` (OrderBy,
  ScrollPoints, RetrievedPoint struct definitions), `qdrant/oneof_factory.go` (WithPayloadInclude/
  Exclude, WithVectors factory functions).
- Qdrant official documentation, fetched this session: `https://qdrant.tech/documentation/manage-data/points/`
  (order_by/start_from semantics, the non-unique-sort-key pagination caveat quoted verbatim).
- `.planning/research/{SUMMARY,ARCHITECTURE,PITFALLS}.md` (2026-09-18, this milestone's own prior
  research pass).
- `.planning/phases/01-test-harness-fixture-helper/01-05-SUMMARY.md`,
  `.planning/phases/02-error-classification-resourceexhausted-mapping/02-04-SUMMARY.md` (red-evidence
  registration precedent).

### Secondary (MEDIUM confidence)
- `https://github.com/qdrant/qdrant/issues/6322` (corroborating community report of the
  offset+order_by pagination inconsistency; the fetched issue body itself contained no
  maintainer-confirmed explanation, so this is corroborating context only, not a primary claim
  source — the primary claim source is the official docs page above).

### Tertiary (LOW confidence)
- The default `with_vectors` behavior when unset (Assumption A1) — not verified against a live
  Qdrant server this session; flagged explicitly in the Assumptions Log.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies, every capability verified against vendored source.
- Architecture (payload ceiling derivation): HIGH for the field enumeration (read directly from
  `payload()`/`fromPayload()` and every validator this session); MEDIUM for the exact numeric
  budget constants (D-06 explicitly leaves these to Claude's Discretion, and the protobuf/gRPC
  framing overhead component is estimated, not measured against a live wire capture).
- Pitfalls: HIGH — every pitfall in this document traces to a directly-read line of this repo's
  own source or a directly-quoted, dated official Qdrant doc page.

**Research date:** 2026-09-19
**Valid until:** 30 days (stable domain — Qdrant server v1.19.1 is pinned; this repo's own
`internal/store` source is the primary subject and changes only via this milestone's own phases)
