# Phase 3: Shared Bounded-Read Mechanism & Content Cap Decision - Context

**Gathered:** 2026-09-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Build the two shared bounded-read primitives every read-site migration in this milestone reuses —
an ordered-page helper for `List`-shaped reads and a byte-budget extension of `scrollAllPoints` for
whole-spine sweeps — record an inventory of every `WithPayload(true)` / unbounded `Scroll` /
`ScrollAndOffset` / `Query` call site with each assigned to its migrating phase (Phase 4: recall
paths; Phase 5: sweeps) or a justified exemption, and settle decision A (memory `content` size cap)
including its enforcement. The call-site MIGRATIONS land in Phases 4 and 5 (each updating the
recall-gate AST test's classifications in the same change); this phase ships the primitives, their
own oversized-fixture proofs, the inventory, and the content cap. Covers REQ-byte-budget-pages and
REQ-content-cap-decided (and contributes the inventory half of REQ-bounded-read-mechanism, which
closes in Phase 5).

</domain>

<decisions>
## Implementation Decisions

### Carried forward

- **D-00 (user preference `1w3h5sy56m`):** choose by idiom and long-term maintenance, never by effort.
- Phase 1: `store.NewQdrantClient` is the one constructor; tests pin `storetest.RecvLimit` (4 MiB);
  `storetest.SeedOversized` provides many-small and few-large fixtures.
- Phase 2: an over-limit response surfaces as `store.ErrResponseTooLarge` (classifier interceptor),
  mapped to `field=response hint=too_large` / `resource_exhausted` / exit 10.

### Decision A — content size cap (REQ-content-cap-decided)

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

- **D-09 (plan-time, user-confirmed 2026-09-19):** `ENGRAM_MEMORY_MAX_CONTENT_BYTES` is ALWAYS enforced — config validation rejects `0` and non-positive values (a deliberate, documented divergence from `ENGRAM_MEMORY_MAX_SUMMARY_BYTES`'s `0 disables` escape hatch), because D-02 derives the per-RPC record count from this cap and a disabled cap would silently remove the provable bound. The content check lives where EVERY lane reaches it — including inside `deps.updateMemory` (gated on `contentChanged`), since Connect's `UpdateMemory` bypasses `validateUpdateArgs` (RESEARCH finding; same shape as #360). — **Reversibility:** one-way (user-chosen; do not re-gate).
- **D-10 (plan-time, user-confirmed 2026-09-19):** Cap memory `tags` in THIS phase (same failure class as content): registry-declared `ENGRAM_MEMORY_MAX_TAGS` (default **128**) and `ENGRAM_MEMORY_MAX_TAG_BYTES` (default **128**), always enforced and > 0, on every memory write path that accepts tags (store/schedule/supersede/update, MCP + Connect + CLI), rejected with the EXISTING hints — `field=tags hint=too_many` (count) and `field=tags hint=too_long` (a tag's bytes). Defaults give ~2x headroom over this repo's heaviest real records (~60 tags of ~60 bytes). Existing records stay readable. The tag bound (16 KiB) folds into `maxRecordBytes`, making the per-record ceiling fully provable. Document both variables beside the content cap. — **Reversibility:** one-way (user-chosen; do not re-gate).

### Byte-budget mechanism (REQ-byte-budget-pages)

- **D-02:** Pages end on an accumulated-byte budget AS WELL AS a record count, built as **small
  RPCs + a running byte total**: each Qdrant RPC fetches `floor(rpcBudget / maxRecordBytes)`
  records — a count derived arithmetically from the per-record payload ceiling — so no single RPC
  can overflow; the logical page accumulates the MEASURED bytes of what it received and stops at
  the page byte budget or the record limit, whichever comes first. `maxRecordBytes` must be the
  TRUE per-record payload ceiling under the caps (content cap + summary cap + citations bound
  (count × excerpt cap) + tags bound (D-10) + every other payload field + protobuf overhead),
  derived PER VIEW — summary view (content and citations projected out) vs full/sweep view
  (citations dominate: 50 × 16 KiB ≈ 800 KiB) — per 03-RESEARCH.md.
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

### Budget values

- **D-06:** **Fixed constants** in `internal/store`: a per-RPC byte budget and a caller-facing
  page byte budget, both comfortably under the 4 MiB the tests pin (order of 2 MiB — exact values
  derived by research from `maxRecordBytes` and headroom for protobuf/gRPC framing), each with an
  unexported `var` seam (the `spineScrollBatch` precedent) so tests can shrink them. The mechanism
  is independent of Phase 5's `MaxCallRecvMsgSize` backstop (which stays pure defense in depth),
  and the page budget also bounds the Connect/MCP responses callers receive.

### Legacy oversized records

- **D-07:** A pre-cap record that exceeds the per-RPC capacity is handled by a **batch-of-1
  fallback**; if that single record STILL exceeds the receive limit, the request fails with the
  named `store.ErrResponseTooLarge` (→ `field=response hint=too_large` / exit 10) — it is never
  silently skipped or truncated (silent truncation is milestone Out of Scope). With D-04
  projection, summary-view recall of a summarized legacy record never fetches its content, so this
  path mostly concerns `full=true` and sweeps. "Existing oversized records stay readable" means
  `get_memory` returns them within the Phase 5 production backstop, and an owner can trim one via
  `update_memory`.

### Inventory (REQ-bounded-read-mechanism, inventory half)

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

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & research
- `.planning/REQUIREMENTS.md` — REQ-byte-budget-pages, REQ-content-cap-decided,
  REQ-bounded-read-mechanism (Phase 5 closes it), REQ-list-contract-unchanged (Phase 4, constrains
  the ordered-page helper's API)
- `.planning/ROADMAP.md` — Phase 3 goal (two-phase TOCTOU note; decision A)
- `.planning/research/SUMMARY.md`, `ARCHITECTURE.md`, `PITFALLS.md` (Pitfall 1 per-site patches,
  Pitfall 2 count-only caps, pitfall 3 cursor tie-safety is v2/deferred)
- `.planning/phases/01-test-harness-fixture-helper/01-CONTEXT.md` (storetest, 4 MiB named limit)
- `.planning/phases/02-error-classification-resourceexhausted-mapping/02-CONTEXT.md` (sentinel,
  envelope, exit 10)

### Code
- `internal/store/spine.go:30-70` — `scrollAllPoints` (the one whole-spine iterator,
  `spineScrollBatch` var seam) and its "never `Scroll` for a sweep" doc
- `internal/store/store.go:1400-1560` — `Store.List` (offset mode scrolls offset+limit full
  payloads; `limit 0` = all) and `listByCursor` keyset paging, `maxListLimit = 1000`
- `internal/store/store.go:1721` `scanCap`/`ListScopes` (projected `scope` only — the #583 fix);
  `:1174` `Search` (k, full payload); `:3117` `reindexBatch`; `internal/store/migrate.go:22`
  `migrateBatch`; `internal/store/spine.go:374` `nearDuplicateBatchSize`
- `internal/store/schemaversion_recallgate_test.go` — `recallTransmitters` (function-name
  set-equality; add new helpers)
- `internal/server/tools.go:769` `maxDiscoveryContentBytes` (64 KiB precedent);
  `internal/server/rules.go:21` `maxRuleContentBytes`; citation excerpt 16 KiB / 50 citations
  (`internal/server/connectapi.go:149` note)
- `internal/config/config.go:98-102` `MaxSummaryBytes` + `internal/config/validate.go:79` — the
  registry/validation pattern for `ENGRAM_MEMORY_MAX_CONTENT_BYTES`; `internal/server/tools.go:292`
  `maxMemorySummaryBytes` parsing precedent
- `internal/server/argerror.go` — `HintTooLong` for the content rejection
- `internal/server/summary.go:68-73` — `truncateForRecall(m.Content, …)`: the fallback snippet that
  needs content when a record has no summary (D-04)
- `internal/store/storetest/` — `Dial`, `SeedOversized`, `RecvLimit`

### Cross-phase notes
- Phase 2's store/Connect/MCP overflow regression tests and three of its red-evidence patches
  depend on today's unbounded `Store.List`; Phase 3 must not break them (it adds primitives only).
  Phase 4 retargets them when it bounds `Store.List`.

### Rules & memories
- Rules `m45p2b4bp7`, `xvqj44e5mk`; memories `1w3h5sy56m`, `7r10s08k9q`, `xb8y5pk6eh`
  (read-path facts: count-only bounds, no package-wide Scroll gate, recallTransmitters)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `listByCursor`: keyset paging over `created_at` with a boundary-id `seen` set and a `+1`
  over-fetch for forward progress — the base for the ordered-page helper.
- `scrollAllPoints`: the single whole-spine `ScrollAndOffset` loop — the base for the byte-budget
  sweep primitive.
- `qdrant.NewWithPayloadInclude` / `NewWithPayloadExclude`: payload projection (#583 precedent).
- `store.ErrResponseTooLarge` (Phase 2): the batch-of-1 fallback trigger and the final named error.

### Established Patterns
- Caps are declared once (registry field + validation) and rejected with `field=<name> hint=too_long`.
- Test-overridable package `var` seams for batch sizes (`spineScrollBatch`).
- Name-keyed AST gates (`recallTransmitters`) must be updated in the same change as a new helper.

### Integration Points
- `internal/config` registry + validation (content cap); every memory write path in
  `internal/server` (MCP + Connect) and `cmd/engram` (CLI) for enforcement; `internal/store` for
  the primitives.

</code_context>

<specifics>
## Specific Ideas

- `ENGRAM_MEMORY_MAX_CONTENT_BYTES`, default 65536, rejection `field=content hint=too_long`.
- Byte budgets are package constants with `var` seams, order of 2 MiB.

</specifics>

<deferred>
## Deferred Ideas

- Two-phase ids→payload paging with a stored `payload_bytes` field (considered, not adopted — D-05).
- `listByCursor` tie-safety under > a page of identical `created_at` values with concurrent inserts
  (REQ-cursor-tie-safety, v2).

</deferred>

---

*Phase: 03-shared-bounded-read-mechanism-content-cap-decision*
*Context gathered: 2026-09-19*
