# Phase 25: Supersession with History - Context

**Gathered:** 2026-07-19
**Status:** Ready for planning
**Mode:** `--auto` (gray areas auto-resolved to recommended options; see Discussion Log)

<domain>
## Phase Boundary

A memory can **supersede** another via additive `supersedes` / `superseded_by` payload
links. Correction is explicit and preserves history — the superseded record is never
deleted or silently overwritten; its content and vector stay intact. Superseded records
are **soft-hidden from recall** (excluded from `search_memory` / `list_memory` at the
recall gate) but remain **fully fetchable by id** via `get_memory`. The supersede
operation routes through the **ownership write gate** (`getWritable` / `OwnedOrAbsent`),
never a read/shared grant.

**In scope:** the supersede write path (new record + back-stamp of the target), the
recall-gate exclusion of superseded records, the write-gate authorization, and write-time
validation (single-hop, no cycles, no auto-supersede).

**Out of scope:** un-supersede / restore, multi-parent merges, similarity-triggered or
write-through automatic superseding, surfacing supersession chains in a UI, and any
Connect/HTTP lane change (MCP tool surface only unless the planner finds parity is required).
</domain>

<decisions>
## Implementation Decisions

### Stamp mechanism (how the target's `superseded_by` is written)
- **D-01:** Back-stamp the target with **`SetPayload` (partial payload update)**, following the
  existing `SetVisibility` pattern (`store.go:1693`): `getWritable(target, subj, ActionWrite)`
  then a payload-only, vector-preserving `SetPayload` of the single `superseded_by` key.
  **Rationale / deviation note:** ROADMAP.md phrases the Phase 24 dependency as "reuses
  idempotency's payload-only **re-Upsert** mechanism." We refine that to `SetPayload` because a
  full re-Upsert *replaces the entire payload* and therefore inherits the CR-01 lost-write hazard
  documented in engram memory `m43h2yt97m` (a concurrent same-point Upsert drops a co-written
  key). `SetPayload` merges the one key and is race-safe for this stamp. The payload-only intent
  is preserved; only the primitive changes. **Planner/researcher: confirm this against the
  Phase 24 mechanism and flag if the roadmap intended a literal re-Upsert.**
- **D-02:** `SetVisibility`'s TOCTOU property carries over: if the target is deleted between the
  ownership gate and `SetPayload`, Qdrant returns NotFound and the error propagates unchanged
  (fail-closed) — no extra re-fetch needed.

### Entry point (tool surface)
- **D-03:** Expose a **dedicated MCP tool/verb** (`supersede_memory`), NOT an overloaded field on
  `store_memory`. Follows the DEC-90w precedent (`schedule_memory` is a distinct verb rather than
  a `store_memory` flag). The tool stores the new/correcting memory (normal write path, caller
  owns it) AND back-stamps the target — one atomic-intent operation from the caller's view.
- **D-04:** The new record carries `supersedes = <target_id>`; the target gets `superseded_by =
  <new_record_id>`. The new record is created through the normal write path (already write-gated
  as the caller's own record); only the target back-stamp needs the additional `getWritable` gate.

### Single-hop model & cycle rejection (write-time validation)
- **D-05:** Reject at write time if the target is **already superseded** (`superseded_by`
  non-empty) → a typed error (e.g. `store.ErrAlreadySuperseded`). This keeps a single live head
  and makes cycles structurally impossible.
- **D-06:** **Forward chains are allowed** — superseding the current live head is how history
  accumulates (C supersedes A which superseded B → chain C→A→B, all fetchable by id). Only
  superseding a non-head (already-superseded) record is rejected.
- **D-07:** **No automatic superseding** — supersession never fires on a similarity threshold or
  any write-through path (SC4). It is only ever the explicit `supersede_memory` call.

### Link representation (payload shape)
- **D-08:** Add two **optional `*string`** payload keys to `store.Memory`: `supersedes` and
  `superseded_by` (single-id each, matching the single-hop linear model; not arrays). Optional
  pointer follows the repo's `*time.Time`-for-optional convention (never zero-value +
  `omitempty`). Absent on every pre-feature record → unchanged behavior.
- **D-09:** Recall-gate integration: add a `superseded_by IS EMPTY` condition to the
  Search/List recall filter, alongside `activeWindowConditions` (`store.go:727`). `get_memory`
  (`Store.Get`, id-addressed, ungated) is deliberately left untouched so superseded records stay
  fetchable (SC2). Same soft-hide-at-recall-gate shape as the DEC-ufz scheduling window.

### Claude's Discretion
- Exact error type names/wire mapping (`ErrAlreadySuperseded` → Connect/MCP code), tool argument
  names, and whether `supersede_memory` shares `storeArgs` with `store_memory` — left to the
  planner to fit existing conventions.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` § "Phase 25: Supersession with History" — goal, dependency on Phase 24,
  4 success criteria, referenced decisions (DEC-y1g/DEC-ufz soft-hide, DEC-90w dedicated verb,
  DEC-kyz write gate, DEC-xa6).
- `.planning/REQUIREMENTS.md` § REQ-supersession-links (lines 68–72) — the normative requirement.

### Code seams (verified during scout)
- `internal/store/store.go:1693` `SetVisibility` — the payload-only, vector-preserving,
  write-gated mutation template to mirror for the target back-stamp (D-01/D-02).
- `internal/store/store.go:727` `activeWindowConditions` — the recall-gate filter assembly to
  extend with the `superseded_by IS EMPTY` soft-hide condition (D-09).
- `internal/store/store.go:1426` `getWritable` / `store.go:1446` `OwnedOrAbsent` — owner-only
  write gate (SC3: shared/read access must NOT permit supersede).
- `internal/store/store.go` `Memory` struct (~line 150–172) + payload encode/decode
  (`p["not_after"]` at `store.go:442`/`529`) — the pattern for adding the two new optional keys.

### Prior-art / hazards (engram memory)
- `m43h2yt97m` — CR-01 lost-write hazard: deterministic Upsert fully replaces payload; explicitly
  flagged as inherited by Phase 25's re-Upsert. Basis for the D-01 `SetPayload` refinement.
- `3230ff9e` / Phase 24 SUMMARY (`.planning/phases/24-idempotent-capture/24-*-SUMMARY.md`) — the
  idempotency re-Upsert mechanism this phase's dependency refers to.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`SetVisibility` (store.go:1693):** near-exact template — `getWritable(id, subj, ActionShare)`
  → `SetPayload(single key, PointsSelectorIDs)`. Supersede back-stamp swaps `ActionShare` →
  `ActionWrite` and the `visibility` key → `superseded_by`.
- **`activeWindowConditions` (store.go:727) + recall filter:** the existing soft-hide-at-recall
  mechanism (`not_after`/`not_before` windowing). Adding one `IsEmpty("superseded_by")` condition
  reuses this exact gate shape.
- **`getWritable` / `OwnedOrAbsent` (store.go:1426/1446):** owner-only write authorization already
  wired to the Cedar PDP with an explicit `authz.Action` verb (Phase 22/23). Reuse directly.

### Established Patterns
- Optional payload fields use `*T` pointers (e.g. `NotAfter *time.Time`), encoded only when
  non-nil (`store.go:442`), decoded defensively (`store.go:529`). New links follow this.
- Recall gate excludes; id-addressed `Get` does not (D-02/D-08 recall vs fetch split).
- Dedicated verbs over flag-overloading `store_memory` (DEC-90w / `schedule_memory`).

### Integration Points
- New `supersede_memory` MCP tool in `internal/server/` delegating to a new `Store.Supersede`
  (or similar) store method.
- New `Store` method composing: normal store of the new record + `getWritable(target)` +
  `SetPayload(superseded_by)` on the target, with the already-superseded / self-ref guard.
- Recall filter assembly (Search + List) gains the `superseded_by IS EMPTY` condition.
</code_context>

<specifics>
## Specific Ideas

The Phase 24 dependency ("reuses idempotency's payload-only re-Upsert") is honored in spirit
(payload-only, vector-preserving, write-gated) but implemented with `SetPayload` rather than a
literal full re-Upsert, specifically to avoid the CR-01 lost-write race that a whole-payload
replace would reintroduce. This is the single most important thing for the planner to validate.
</specifics>

<deferred>
## Deferred Ideas

- **Un-supersede / restore** — reversing a supersession link is a separate capability; not in
  REQ-supersession-links. Note for a future phase if needed.
- **Multi-parent / merge supersession** (one record superseding several) — the single-hop
  `*string` model excludes it by design; revisit only if a real need appears.
- **Surfacing supersession chains in the operator console / Connect read lane** — this phase is
  MCP-store-only; UI/Connect exposure is its own phase.

</deferred>

---

*Phase: 25-supersession-with-history*
*Context gathered: 2026-07-19*
