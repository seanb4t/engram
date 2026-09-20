# Phase 4: List, ListScheduled & Search Bounded Reads - Context

**Gathered:** 2026-09-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Migrate the five recall-path read sites Phase 3's inventory assigned to this phase — `Store.List`
(offset mode incl. `limit: 0` and deep offset, and `listByCursor`), `Store.ListScheduled`,
`Store.Search` (and through it `SearchReranked`) and `Store.SearchDiscovery` — onto Phase 3's
shared bounded-read primitives (`scrollOrderedPage`, the per-view `readView` ceilings, the
`rpcByteBudget`/`pageByteBudget` seams), each with its own real-Qdrant regression proving that
caller's request shape stays under `storetest.RecvLimit` on both `storetest.SeedOversized` shapes.
Keep `total`, `next_cursor` (empty = last page), result ordering, and recall gating independent
of the internal batching, and never report a byte-cut page as the last page. Settle decision B
(REQ-list-limit-contract-decided) and record it in PROJECT.md Key Decisions. Reclassify the
recall-gate AST test in the same change. Covers REQ-list-bounded, REQ-list-scheduled-bounded,
REQ-search-k-bounded, REQ-list-contract-unchanged, REQ-list-limit-contract-decided. The five
operator sweeps, the production `MaxCallRecvMsgSize` backstop, and the CI-green proof are
Phase 5; cross-spine partial results are Phase 6.

</domain>

<decisions>
## Implementation Decisions

### Carried forward

- **D-00 (user preference `1w3h5sy56m`):** choose by idiom and long-term maintenance, never by effort.
- Phase 1: `store.NewQdrantClient` is the one constructor; tests pin `storetest.RecvLimit` (4 MiB);
  `storetest.SeedOversized` provides the many-small and few-large fixtures.
- Phase 2: an over-limit response surfaces as `store.ErrResponseTooLarge` → `resource_exhausted` /
  exit 10 (its hint code is renamed by D-11 below).
- Phase 3: D-02 small RPCs + running byte total; D-03 the two primitives; D-04 project payloads by
  view (`summaryView` excludes `content` and `citations` — already cleared on the wire today by
  `shapeRecall`/`shapeProtoMemories`, so the projection is invisible to callers except the
  no-summary truncation fallback, which must still see content); D-05 two-phase ids→payload NOT
  adopted for List; D-06 budgets are fixed 2 MiB seams; D-07 batch-of-1 fallback → named error,
  never a silent skip; `maxListLimit = 1000` already caps a cursor page.
- ADR `engram-1frj` (LOCKED): boundary id-set cursor for recall paging, offset-for-UI kept. This
  phase does not touch it.

### Decision B — Connect `ListMemories` limit contract (REQ-list-limit-contract-decided)

- **D-01:** `limit: 0` resolves to the **page maximum, 1000** (the existing `maxListLimit`) — on
  Connect `ListMemories`, therefore on `engram list --limit 0` and the console, and at the store
  (`ListOptions.Limit == 0` resolves to `maxListLimit` in offset mode so every lane shares one
  rule). The contract is stated **numerically everywhere** — proto field comment, code comments,
  `tools.md`, `cli.md`, `engram list --limit` help — and the word "all" is removed from each
  (`store.go:1461` "limit 0 = all", `connectapi.go:274` "0 = all", `tools.md:195-196` "an unset
  limit means all"). `total` stays exact; a scope with more than 1000 matching records is paged
  from there by `offset` or by `page_token`. MCP `list_memory`'s documented `0 → 20` default is
  already numeric and is unchanged. Offset-for-UI stays; nothing moves to cursor-only.
  — **Reversibility:** one-way — a published Connect wire contract (`0` no longer returns a
  >1000-record scope in one call); the user chose this exact shape in discuss-phase (2026-09-19,
  after first considering `0 → 20`) — do not insert a checkpoint to re-ask.
- **D-02:** **One documented maximum, 1000**, for every recall count knob: `limit` on Connect
  `ListMemories` (offset and cursor modes), MCP `list_memory`, MCP `list_scheduled`; and `k` on
  Connect `SearchMemories`/`SearchDiscoveries`, MCP `search_memory`/`search_discovery`, and
  `engram search --k`. One store-level constant (`maxListLimit`, renamed or aliased at Claude's
  discretion so the name fits both list and search); docs cite one number.
- **D-03:** `list_rules` inherits the same ceiling: its contract becomes "the complete rule set,
  up to 1000 rules per scope" (tool description, `rules.go:206` comment, `tools.md`, CLAUDE.md
  memory-contract line). No exemption, no internal paging past the maximum.
- **D-04:** The `0 → 1000` change is announced as **BREAKING** in
  `docs-site/src/content/docs/guides/upgrade.md` (precedent: the `--timeout` zero-semantics
  entry), naming Connect `ListMemories` and `engram list`, the old and new meaning of `0`, and how
  to page the remainder (offset or `--page-token`). The proto comment, `tools.md`, `cli.md`, and
  PROJECT.md Key Decisions (REQ-locked) are updated in the same change.

### Cursorless byte-cut pages (REQ-list-contract-unchanged, REQ-list-bounded, REQ-list-scheduled-bounded)

- **D-05:** Offset-mode `Store.List` (console pager, `engram list --offset`, `list_rules`) and
  `Store.ListScheduled` **assemble the full requested count**: they iterate `scrollOrderedPage`
  pages (following `Next`) until `limit` records are in hand or the primitive reports
  `Exhausted`. Every Qdrant RPC stays under `rpcByteBudget`; the response is bounded by count
  (≤ 1000 × the view's per-record ceiling). Offset arithmetic, `total`, the console's numbered
  pager, and the CLI are untouched. This **revises Phase 3 D-06**: `pageByteBudget` bounds
  cursor-mode responses only — state that explicitly in the plan and in `boundedread.go`'s
  doc comment.
- **D-06:** Cursor mode (`page_token` / `cursor_mode` / MCP `list_memory`'s default) honors
  `pageByteBudget`: a page the primitive cuts short (`CutByBudget`) is returned with a **non-empty
  `next_cursor`** and is never the last page; `next_cursor` is empty only when the primitive
  reports `Exhausted`. This is the explicit test REQ-list-contract-unchanged asks for — write it
  against the few-large fixture at a shrunken `pageByteBudget`, not by assumption.
- **D-07:** **No offset ceiling.** Deep offset walks the skipped prefix with a **keys-only
  budgeted view** (ids + `created_at`, tens of thousands per RPC), then fetches the page in the
  caller's view by keyset resume from the prefix boundary. Cost is O(offset) tiny reads; it never
  overflows and never widens the authz filter. The exact handoff into `scrollOrderedPage`'s
  `Seen`/boundary is Claude's discretion.

### Search `k` (REQ-search-k-bounded)

- **D-08:** `k == 0` keeps today's per-surface defaults (Connect 20; MCP `search_memory` 8;
  `search_discovery` 8). `k > 1000` is rejected (D-10). The documented maximum is stated on every
  surface (`tools.md`, `cli.md`, proto comments, `--k` help).
- **D-09:** **Two-phase search.** The `Query` requests **no payload** (ids + scores, one small
  RPC at any `k ≤ 1000`); payloads are then fetched through byte-budgeted `Scroll`s filtered by
  `has_id(batch)` **AND the identical authz/recall filter the Query used**, in the caller's view
  (`summaryView` by default, `fullView` on `full=true`), and re-ordered by the held scores. A
  record absent from the fetch (deleted or hidden between the two phases) simply drops out of the
  hits. Phase 3 D-05 stands for List; two-phase is adopted for **Search only** because the order
  is held by the caller and the authz filter is re-applied, so the TOCTOU and `GetPoints`-order
  risks that ruled it out for List do not apply — say so in the plan so the verifier does not
  look for those criteria. Applies to `Store.Search` (hence `SearchReranked`) and
  `Store.SearchDiscovery`; the new fetch helper is reachable from recall seeds and joins
  `recallTransmitters`.

### Over-maximum rejection

- **D-10:** A `limit` or `k` above 1000 is **rejected, never clamped**, with a **new hint
  `out_of_range`** — "a numeric field exceeds its documented maximum" — as
  `field=limit hint=out_of_range` / `field=k hint=out_of_range`, classified `classMalformed` →
  Connect `invalid_argument` → CLI exit 2, like every other rejected input. Added to the
  `HintCode` catalog (`argerror.go`), the `errors.md` hint table, and whatever conformance/
  attribution tests enumerate hints. Applies at the server boundary on every surface named in
  D-02; the CLI does not duplicate the check (correct-by-reading `4aksmneehh`: the `--limit`/`--k`
  help names the maximum, the server is the single source of truth). The store rejects the same
  condition with `ErrInvalidArgument` as a backstop (today `maxListLimit` is a silent cursor-page
  cap — that becomes a rejection too).
  — **Reversibility:** one-way — a published rejection contract on every recall surface; the user
  chose reject-over-clamp explicitly in discuss-phase (2026-09-19) — do not re-ask.
- **D-11:** Phase 2's overflow hint is **renamed `too_large` → `response_too_large`**
  (`HintTooLarge` → `HintResponseTooLarge`; envelope `field=response hint=response_too_large`;
  Connect `resource_exhausted` and exit 10 unchanged) so the two codes are self-describing and
  cannot be read as the same failure. The milestone branch is unreleased, so there is no wire
  compatibility cost. Update in one change: `internal/server/{argerror,connecterror,responsetoolarge}.go`
  and their tests, `cmd/engram/exitcode_baseline_test.go`, `internal/store/storetest/storetest_test.go`,
  `docs-site/.../reference/errors.md` (hint table + code table), `guides/cli.md` (exit 10 row),
  `guides/upgrade.md:376`, `internal/store/redevidence_harness_test.go`'s Phase 2 mapping and the
  patch `02-03-errors-doc-drops-too-large.patch` (regenerate against the renamed text and commit
  it with the rename — playbook `f7zdc18tn3` #13), and Phase 2's VERIFICATION covered files
  (legitimate staleness: re-prove, re-fingerprint, `## Re-fingerprint` section — playbook #17).
  `02-CONTEXT.md` is historical and is not edited.

### Phase 2 regression retargets

- **D-12:** Phase 2's overflow regressions that rely on today's unbounded `Store.List`
  (`internal/server/responsetoolarge_test.go:166` "`Limit: 0` — one full-payload Scroll", the
  Connect/MCP siblings, and the red-evidence patches that assert them) are **retargeted in this
  phase** to a shape that still overflows after the fix — the D-07 single-record-over-limit
  fixture through a recall entry point, or an explicitly unbudgeted raw path in a `package store`
  test — so `ErrResponseTooLarge` stays reachable and RED-provable on every lane. Record each
  retarget in the plan's SUMMARY deviations.

### Claude's Discretion

- The keys-only `readView` for the prefix walk (its selector and ceiling) and the boundary handoff
  into `scrollOrderedPage`; whether offset mode and `ListScheduled` share one assembly loop.
- Where in `deps.*`/`connectapi.go` the maximum check sits relative to the existing scope and
  window checks, and the exact `argErrf` detail text.
- The rename/alias of `maxListLimit` so one identifier reads correctly for list and search.
- Test design per site (List offset `limit: 0`, deep offset, cursor; `ListScheduled` large limit;
  `Search` with `full=true` and `k` well above 2; `SearchDiscovery`), each on both fixture shapes
  at `storetest.RecvLimit`; the `total`/`next_cursor`/ordering/recall-gate invariance tests; the
  D-06 cut-page test at a shrunken page budget; the new `out_of_range` rejections on every
  surface (MCP, Connect, CLI exit 2).
- Recall-gate reclassification: `Store.scrollOrderedPage` → `recallTransmitters` (inventory check
  (d)), the new search fetch helper and the keys-only walk added with justifications.
- Phase 3 D-04's no-summary truncation fallback under projection: keep today's visible behavior
  (fetch content only for records whose summary is empty, in a second budgeted has_id fetch) —
  the mechanics are Claude's, the visible behavior is not to change.
- Phase 4 red-evidence patches (one per RED direction; registered in `redEvidenceDirs` after the
  last plan, before verification).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements, roadmap, research
- `.planning/REQUIREMENTS.md` — REQ-list-bounded, REQ-list-scheduled-bounded, REQ-search-k-bounded,
  REQ-list-contract-unchanged, REQ-list-limit-contract-decided (this phase); REQ-exhausted-connect /
  REQ-exhausted-mcp / REQ-exhausted-cli-docs (Phase 2 — D-11 renames their hint)
- `.planning/ROADMAP.md` — Phase 4 goal and success criteria 1–5
- `.planning/research/SUMMARY.md` §"Open questions" (decision B framing), `ARCHITECTURE.md`
  (`ListMemoriesRequest{Limit:0}` flow, AST-gate constraints), `PITFALLS.md` (Pitfall 8
  `total`/exhaustion semantics)
- `.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-CONTEXT.md` — D-02..D-08
- `.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-INVENTORY.md` — the
  five Phase 4 rows and closing check (d)
- `.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-04-SUMMARY.md` —
  `orderedPage{Items, Next, Exhausted, CutByBudget, Bytes}` contract and test precedents
- `.planning/phases/02-error-classification-resourceexhausted-mapping/02-CONTEXT.md` — D-04/D-05
  (the hint D-11 renames; historical, do not edit)
- `.planning/PROJECT.md` — Key Decisions table (record decision B); ADR `engram-1frj` (offset-for-UI
  stays LOCKED)

### Code — store
- `internal/store/store.go:1283` `ListOptions`; `:1388` `Store.List` (offset mode `:1453-1495`,
  "limit 0 = all" at `:1461`); `:1497` `maxListLimit`; `:1502` `listByCursor`; `:1638`
  `ListScheduled` (`limit == 0 → 20` at `:1659`); `:1123` `Search` (`Query` full payload `:1180`);
  `:1217` `SearchReranked`; `:1235` `SearchDiscovery` (`:1271`)
- `internal/store/orderedpage.go` — `scrollOrderedPage`, `excludeSeen`, `orderedPage`
- `internal/store/boundedread.go:129-238` — `rpcByteBudget`, `pageByteBudget`, `fullRecordCeiling`
  (970240), `summaryRecordCeiling` (34304), `perRPCLimit`, `readView`, `fullView`, `summaryView`
- `internal/store/spine.go:30-70` — `scrollAllPoints` (Phase 5's primitive; not this phase's)
- `internal/store/schemaversion_recallgate_test.go` — `recallTransmitters`,
  `otherNonRecallEmitters`, `recallEntryPointSeeds`
- `internal/store/redevidence_harness_test.go:106` — `redEvidenceDirs` (Phase 2 mapping to
  update for D-11; Phase 4 entries to add)
- `internal/store/storetest/` — `Dial`, `SeedOversized`, `RecvLimit`; `storetest_test.go`
  (mentions `too_large`)
- `internal/store/store_test.go:238-260` — the `Limit:0` empty-scope pin (`engram-3jo0.4`)

### Code — server / CLI / console
- `internal/server/connectapi.go:235-311` `ListMemories` (`Limit: req.Msg.Limit // 0 = "all"` at
  `:274`); `:326-336` `SearchMemories` `k == 0 → 20`; `:420-424` `SearchDiscoveries`;
  `:142-145` non-full shaping clears content + citations
- `internal/server/tools.go:788` `listArgs`; `:800` `listScheduledArgs`; `:779`/`:914` `K`;
  `:1652` `listScheduled` default 20; `:1843` `searchDiscovery` default 8; `:2559` `search_memory`
  default 8; `:2605-2636` `list_memory` closure (`CursorMode: true`); `:2767` `list_rules` tool
  description ("COMPLETE rule set")
- `internal/server/rules.go:206-208` — `list_rules` → `Store.List` `Limit: 0`
- `internal/server/summary.go:66-110` — `summaryOrTruncation`, `toRecallView` (the no-summary
  content fallback D-04 must keep feeding)
- `internal/server/argerror.go:34-44` — `HintCode` catalog (add `out_of_range`; rename
  `HintTooLarge`); `internal/server/connecterror.go`, `responsetoolarge.go` (+ tests)
- `cmd/engram/client_list.go:122-146` (`--limit` "0 = server default", `--offset`, `--page-token`,
  flag group); `cmd/engram/client_search.go:110` (`--k`); `cmd/engram/exitcode_baseline_test.go`
- `ui/src/lib/queries.ts:1` `PAGE_LIMIT = 50`; `ui/src/routes/observe/+page.svelte:34,95` offset
  pager; `ui/src/routes/+page.svelte:19` root list (`limit 50, offset 0, crossSpine`)

### Docs
- `docs-site/src/content/docs/reference/tools.md:175,193-197,210` (limit rows; "an unset limit
  means all" paragraph); `reference/errors.md:118-128,227` (hint + code tables);
  `guides/cli.md:74-84` (paging flags), `:375` (exit 10 row); `guides/upgrade.md:376` and its
  `--timeout` zero-semantics entry (the breaking-note precedent); `proto/engram/v1/*.proto:71-125`
  (`ListMemoriesRequest`/`Response` comments)
- `CLAUDE.md` memory-contract paragraph (`list_rules` "complete set" wording; `list_memory`
  pagination sentence)

### Rules & memories
- Rules `m45p2b4bp7` (never gate third-party behavior), `xvqj44e5mk` (idiomatic over hand-rolled),
  `8dfdhfs5nn`, `2rjnv8sc9a`; memories `1w3h5sy56m`, `7r10s08k9q`, `xb8y5pk6eh`, `y02a9ft3gy`
  (storetest usable only from `package store_test`), `4aksmneehh` (correct-by-reading CLI),
  playbook `f7zdc18tn3` (#1 key_links escaping, #12–13 red-evidence, #16–17 verification
  staleness), `2tb2ew756h` (progress-table corruption)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `scrollOrderedPage` + `orderedPage.{Next, Exhausted, CutByBudget}`: the ordered primitive every
  List-shaped read in this phase composes — cursor mode maps one primitive page to one wire page;
  offset mode and `ListScheduled` loop it (D-05).
- `summaryView`/`fullView` + `perRPCLimit`: the per-view ceilings (2 vs 61 records per RPC at
  default caps) that size every RPC; a keys-only view is the one new `readView` (D-07).
- `excludeSeen` and `listByCursor`'s cursor codec (`cursor.go`): keyset resume and the opaque
  `next_cursor` shape stay as they are.
- `store.ErrResponseTooLarge` / D-07 batch-of-1: the single-record-over-limit path that stays
  reachable for D-12's retargeted regressions.
- `argErrf(classMalformed, Hint…, field, …)` + `connectError` single mapper: the rejection path
  `out_of_range` joins.

### Established Patterns
- Name-keyed AST gates (`recallTransmitters`, inventory check (d)) updated in the same change as
  any new helper reachable from a recall seed.
- Hint codes are declared once (`HintCode` const + `errors.md` row + attribution tests); exit
  codes come from the Connect code, never a second table.
- Wire-visible changes get an `upgrade.md` entry; proto changes are additive (no new field is
  needed in this phase — D-01/D-10 change semantics and comments only).
- Red-evidence: one patch per RED direction, registered after the last plan; a later phase that
  breaks an earlier patch regenerates it with the change.

### Integration Points
- `internal/store` (List/ListScheduled/Search/SearchDiscovery bodies, one new keys-only view, one
  search fetch helper, recall gate); `internal/server` (limit/k maximum rejection on MCP + Connect,
  hint catalog rename, `list_rules` description); `cmd/engram` (help text, exit-code baseline);
  docs-site + proto comments + CLAUDE.md + PROJECT.md Key Decisions. The console needs no code
  change (offset pages keep their size; root page uses `limit 50`).

</code_context>

<specifics>
## Specific Ideas

- "Drop the word 'all' — it's always numeric": `0 = 1000` everywhere the contract is written.
- One maximum (1000) for `limit` and `k`; over it → `field=<limit|k> hint=out_of_range`, exit 2.
- Overflow hint reads `response_too_large` from now on.

</specifics>

<deferred>
## Deferred Ideas

- Adding `next_offset` / a truncation flag to offset-mode responses (considered under the
  byte-cut question; not adopted — D-05 assembles the full count instead).
- Moving the console and `engram list` to cursor-only paging (would need an ADR superseding
  `engram-1frj`; not adopted).
- Capping the still-uncapped payload fields — GitHub #589 (Phase 3 D-11).
- `listByCursor` tie-safety under concurrent inserts at one `created_at` (REQ-cursor-tie-safety, v2).

</deferred>

---

*Phase: 04-list-listscheduled-search-bounded-reads*
*Context gathered: 2026-09-19*
