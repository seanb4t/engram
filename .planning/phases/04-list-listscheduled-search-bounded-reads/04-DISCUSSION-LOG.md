# Phase 4: List, ListScheduled & Search Bounded Reads - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-19
**Phase:** 4-list-listscheduled-search-bounded-reads
**Areas discussed:** Decision B: Connect limit:0, Cursorless byte-cut pages, Search k ceiling, Over-max coercion

---

## Decision B: Connect limit:0

### What should Connect `ListMemories` `limit: 0` mean going forward?

| Option | Description | Selected |
|--------|-------------|----------|
| 0 = server default page | 0 → 20 (same as MCP and the existing `engram list --limit` help); explicit limits coerced to one documented max; offset-for-UI stays; breaking note in upgrade.md | (picked, then retracted) |
| 0 = all, bounded to the max | Keep the name "all" but page internally and stop at the documented max | |
| Hard cap + cursor everywhere | Retire offset paging in console and CLI; supersede LOCKED ADR engram-1frj | |

**User's choice:** Retracted the first pick. Free text: "Limit = 0 should equate to 'all', drop the 'all' word so that it's always numeric." After a clarification of the four different "limits" in play (gRPC receive limit, Phase 3 byte budgets, write caps, the record-count paging knob), the user settled it as: **"0 should resolve to page max."**
**Notes:** Resolved as `0 → 1000` (the existing `maxListLimit`), stated numerically on every surface; `total` stays exact; offset-for-UI untouched. MCP `list_memory`'s `0 → 20` default is unchanged.

### Under `0 = page max`, how does `list_rules` keep its "complete rule set" promise?

| Option | Description | Selected |
|--------|-------------|----------|
| Complete up to the page max | Inherit the 1000 ceiling; doc says "complete, up to 1000 rules per scope" | ✓ |
| Page internally until exhausted | Loop cursor pages until Exhausted — the one read with no numeric bound | |

**User's choice:** Complete up to the page max.

### How should the wire-visible `0 → 1000` change be announced?

| Option | Description | Selected |
|--------|-------------|----------|
| Breaking note in guides/upgrade.md | Follows the `--timeout` zero-semantics precedent; proto comment, tools.md, cli.md, PROJECT.md Key Decisions in the same change | ✓ |
| Docs only, not called breaking | Treat as a bug fix | |

**User's choice:** Breaking note in guides/upgrade.md.

---

## Cursorless byte-cut pages

### When the page byte budget would cut an offset-mode List page or a list_scheduled page short, what happens?

| Option | Description | Selected |
|--------|-------------|----------|
| Assemble the full count | Keep pulling primitive pages (each RPC ≤ 2 MiB) until `limit` records are in hand; response count-bounded; console/CLI/list_rules untouched; revises Phase 3 D-06 to cursor mode only | ✓ |
| Cut + additive wire signal | `next_offset` on ListMemoriesResponse, `next_cursor` on list_scheduled, console pager → next/prev, CLI resume hint | |
| Reject when it cannot fit | ErrResponseTooLarge when `limit × ceiling` exceeds the page budget — unusable in full view (2 records) | |

**User's choice:** Assemble the full count.

### Any ceiling on how deep an offset may go?

| Option | Description | Selected |
|--------|-------------|----------|
| No offset ceiling | Walk the prefix keys-only (ids + created_at), then fetch the page in the caller's view; O(offset) tiny reads | ✓ |
| Cap offset + limit at the page max | Offset paging stops at record 1000; beyond that, cursor paging | |

**User's choice:** No offset ceiling.

---

## Search k ceiling

### What is the documented server-side maximum for `k`?

| Option | Description | Selected |
|--------|-------------|----------|
| Same 1000 as limit | One number for every recall count knob | ✓ |
| 100 for k, 1000 for limit | Tighter search-specific ceiling; two numbers to keep in sync | |
| Derive k from the view ceiling | 61 (summary) / 2 (full) — full-view search effectively unusable | |

**User's choice:** Same 1000 as limit.

### How does a search whose payloads exceed one RPC fetch its results and stay bounded?

| Option | Description | Selected |
|--------|-------------|----------|
| Two-phase: Query ids, then budgeted fetch | Query returns ids + scores; payloads via byte-budgeted Scrolls filtered by has_id AND the same authz filter, re-sorted by held scores; D-05's List objections do not apply | ✓ |
| Re-run the Query per page with offset | k/n re-executions; ranked set not guaranteed stable across re-runs | |
| Cap k at the view's per-RPC count | full=true search returns at most 2 hits | |

**User's choice:** Two-phase: Query ids, then budgeted fetch.

---

## Over-max coercion

### A caller passes `limit: 5000` or `k: 5000`. What happens?

| Option | Description | Selected |
|--------|-------------|----------|
| Clamp silently to 1000 | AIP-158 shape and the REQ's "coerce" wording; documented; no new hint | |
| Reject with a named hint | Loud and unambiguous; noted that reusing `too_large` would collide with the response-overflow meaning | ✓ |
| Clamp and say so on the wire | Additive `limit_applied` / `k_applied` fields | |

**User's choice:** Free text: "2 - reject, but with a hint that's not ambiguous, even if overflow needs renaming/fixing".

### Which hint names?

| Option | Description | Selected |
|--------|-------------|----------|
| out_of_range + rename to response_too_large | `field=limit hint=out_of_range` / `field=k hint=out_of_range`, invalid_argument, exit 2; Phase 2's `too_large` → `response_too_large` (unreleased, no compat cost) | ✓ |
| out_of_range, keep too_large | Rely on `field=response` to disambiguate | |
| above_maximum + rename to response_too_large | Alternative input-hint name | |

**User's choice:** out_of_range + rename to response_too_large.

---

## Claude's Discretion

- Keys-only `readView` mechanics and the prefix-boundary handoff into `scrollOrderedPage`.
- Placement of the maximum check in the argError chain; `maxListLimit` naming for list + search.
- Per-site test design; recall-gate reclassifications; the no-summary content fallback mechanics under projection; Phase 4 red-evidence patches.

## Deferred Ideas

- `next_offset` / truncation flag on offset-mode responses (not adopted).
- Cursor-only paging for console and `engram list` (would supersede ADR engram-1frj; not adopted).
- Capping the still-uncapped payload fields — GitHub #589.
- `listByCursor` tie-safety under concurrent inserts (REQ-cursor-tie-safety, v2).
