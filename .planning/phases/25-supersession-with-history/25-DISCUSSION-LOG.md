# Phase 25: Supersession with History - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-19
**Phase:** 25-supersession-with-history
**Mode:** `--auto` (recommended option auto-selected for each area; no interactive prompts)
**Areas discussed:** Stamp mechanism, Entry point (tool surface), Single-hop & cycle model, Link representation

---

## Stamp mechanism (writing the target's `superseded_by`)

| Option | Description | Selected |
|--------|-------------|----------|
| SetPayload partial-update | Mirror `SetVisibility`: `getWritable` + payload-only `SetPayload` of one key; race-safe, vector-preserving | ✓ |
| Full re-Upsert | Literal reuse of Phase 24 idempotency re-Upsert; replaces whole payload | |

**Auto-selected:** SetPayload partial-update (recommended default).
**Notes:** Full re-Upsert replaces the entire payload and reintroduces the CR-01 lost-write hazard
(engram memory `m43h2yt97m`) that Phase 24 explicitly flagged as inherited here. `SetPayload`
merges the single key and is race-safe. ROADMAP.md wording ("payload-only re-Upsert") is honored
in spirit; planner must confirm the deviation.

---

## Entry point (tool surface)

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated `supersede_memory` verb | New MCP tool; stores new record + back-stamps target | ✓ |
| Field on `store_memory` | Optional `supersedes` arg overloading the existing store tool | |

**Auto-selected:** Dedicated `supersede_memory` verb (recommended default).
**Notes:** Follows DEC-90w precedent (`schedule_memory` is a distinct verb, not a `store_memory`
flag). ROADMAP.md explicitly cites this precedent for Phase 25.

---

## Single-hop model & cycle rejection

| Option | Description | Selected |
|--------|-------------|----------|
| Reject already-superseded target + self-ref; forward chains allowed | Single live head; cycles structurally impossible; history accumulates forward | ✓ |
| Allow re-superseding any record | Permits multiple `superseded_by` / cycles | |
| Reject all chains (one hop ever) | No history accumulation past a single supersession | |

**Auto-selected:** Reject already-superseded target + self-ref; forward chains allowed (recommended).
**Notes:** Superseding the current live head is how "with History" accumulates (C→A→B, all
fetchable by id). Only superseding a non-head (already-superseded) record is rejected, which makes
cycles impossible. No similarity/auto-supersede path (SC4).

---

## Link representation (payload shape)

| Option | Description | Selected |
|--------|-------------|----------|
| Two optional `*string` keys | `supersedes` / `superseded_by`, single-id each, `*T`-optional convention | ✓ |
| Arrays of ids | Multi-parent / multi-child links | |

**Auto-selected:** Two optional `*string` keys (recommended default).
**Notes:** Single-id matches the single-hop linear model; `*string` follows the repo's
`*time.Time`-for-optional convention. Recall gate gains a `superseded_by IS EMPTY` condition;
`get_memory` stays ungated.

---

## Claude's Discretion

- Error type names and Connect/MCP wire-code mapping for the already-superseded rejection.
- Tool argument names and whether `supersede_memory` shares `storeArgs` with `store_memory`.

## Deferred Ideas

- Un-supersede / restore (reversing a link) — future phase.
- Multi-parent / merge supersession — excluded by the single-hop `*string` model by design.
- Surfacing supersession chains in the operator console / Connect read lane — separate phase.
