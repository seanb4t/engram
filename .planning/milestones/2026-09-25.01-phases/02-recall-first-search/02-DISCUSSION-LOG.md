# Phase 2: Recall-First Search - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-26
**Phase:** 2-Recall-First Search
**Areas discussed:** Sketch vs. server gaps, Results list scale, Shell/routes/scope, Detail pane & actions

---

## Sketch vs. server gaps

| Question | Options | Selected |
|----------|---------|----------|
| Recall-gate hidden count | Drop the count (honest subset) / Add additive field now | Add additive field now |
| Ambiguous short_id | Honest message only / Carry candidates in error | Honest message only |
| Jev rerank chip | Status, not a toggle / Drop jev mention | Status, not a toggle |
| Per-scope coverage | Client-derived, labelled "in top k" / Scope list only | Client-derived, labelled |
| Hidden-count lanes | All lanes / Connect only | All lanes |
| Hidden-count semantics | Per-state, query-matching / Single total | Per-state, query-matching |
| Hidden-count bound | Would-have-ranked in top-k / Filter-matching, unranked | Would-have-ranked in top-k |
| ListMemories counts | Yes, same field / Search only | Yes, same field |

**Notes:** Adding the field moves additive proto/store/MCP/CLI work into a phase the roadmap
described as "uses existing RPCs".

---

## Results list scale

| Question | Options | Selected |
|----------|---------|----------|
| Virtualization | @humanspeak/svelte-virtual-list / Hand-rolled windowed list / No virtualization | @humanspeak/svelte-virtual-list |
| Default k | k=50 + Show more / k=20 / k=100 fixed | k=50 + Show more |
| Listing paging | Infinite scroll on cursor / Explicit Load more | Infinite scroll |

---

## Shell, routes & scope

| Question | Options | Selected |
|----------|---------|----------|
| Home route | / stays, header search everywhere / / becomes recall surface | / stays |
| ⌘K | ⌘K and / focus header box / Keep modal as alias | Other (free text) |
| /observe | Shared row list only / Leave alone / Fold into /search | Shared row list only |
| Foundations | Tokens + text size in Phase 2 / Tokens only | Both in Phase 2 |

**User's choice (⌘K):** "why wouldn't we keep cmd k for navigation _other_ than search?"
**Notes:** Proposed split — `/` focuses the server-driven header search; ⌘K is a command menu for
navigation/actions that always ends in an unfiltered "Search memories for …" hand-off row (or
"Open record …" for an id), so it never reports "no matches" for unsearched text. Sean: "that works".

---

## Detail pane & actions

| Question | Options | Selected |
|----------|---------|----------|
| Pane layout | Stacked sections / Keep tabs / Tabs + pinned summary | Stacked sections |
| Phase-3-dependent actions | Omit until wired / Show disabled + tooltip | Show disabled + tooltip |
| Action buttons | Inline buttons / Keep ⋯ menu | Inline buttons |
| Row keys | e/s/#/c/⇧C / You decide | e/s/#/c/⇧C |

---

## Claude's Discretion

- Debounce, overscan, hover-card placement math; ListMemories hidden-count mechanism; proto field
  shape for per-state counts; content of the two project-local skills.

## Deferred Ideas

- Ambiguous-short_id candidate list; request-level Jev toggle; ListTags-backed tags section (Phase 3).
- Todo "Security-review then install the three design/a11y registry skills" → Phase 4 (DSYS-03).
