# Phase 2: Recall-First Search - Context

**Gathered:** 2026-09-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Every console search surface (the header search box, `/search`, the URL `?q=`) accepts a UUID,
a short_id, or free text, resolves which it is, runs the right RPC, defaults to `cross_spine`, and
reports honestly what it searched. Results render as a dense, virtualized, keyboard-driven
listbox with a hover card and a pinned right detail pane, narrowed by URL-persisted facet chips.
Phase 2 also lands the console's design foundations (new tokens, site-wide text size) and the two
project-local skills `engram-console-conventions` and `engram-connect-client`.

Requirements: ENTRY-01..06, ROW-01..07, DSYS-01, DSYS-02.

**One boundary change from ROADMAP.md:** the roadmap says this phase "uses existing RPCs". Sean
chose (D-01) to add an additive recall-gate hidden-count field to the search and list responses in
this phase, so Phase 2 carries a small, additive proto + store + MCP + CLI change. No new RPC,
no new mutating procedure. Supersede, Archive/Restore, ListTags, RelatedMemories stay in Phase 3.

</domain>

<decisions>
## Implementation Decisions

### Honest reporting where the sketch outran the server
- **D-01:** Add an additive recall-gate **hidden count** to `SearchMemoriesResponse` and
  `ListMemoriesResponse`, split **per state** (archived / superseded / expired / scheduled). The
  results header shows `N hidden by recall gate` with a per-state tooltip, as in the sketch.
  — **Reversibility:** one-way — a published proto field on two responses plus the MCP and CLI
  output contracts; removal later is a breaking change on all three lanes.
- **D-02:** Hidden-count semantics for **search** = "would have ranked": run the same vector query
  with the recall gate lifted (every other filter kept) at the same `k`, and count the gated
  records that would have appeared in that top-k, by state. Bounded by `k`; costs one extra query.
  For **ListMemories** (unranked), the equivalent is the count of gated records matching the same
  non-vector filters (planner decides whether a Qdrant `Count` or the same page window; keep it
  bounded and document the choice).
- **D-03:** The hidden count ships on **all lanes**: Connect, the MCP `search_memory` /
  `list_memory` results, and the `engram search` / `engram list` CLI output, following the
  `searched_scopes` / `scopes_truncated` / `scopes_unknown` precedent. The MCP↔Connect parity and
  cross-spine coverage tests stay symmetric. CLAUDE.md's memory contract and the docs-site
  reference get the new field.
- **D-04:** An **ambiguous short_id** (GetMemory → `failed_precondition`, legacy data) shows an
  honest warning-tone message only: `short_id <x> is ambiguous — paste the full id to be exact`.
  No candidate list, no server change.
- **D-05:** **Jev** is a status, not a toggle. There is no request field for reranking, so there is
  no `jev rerank` facet chip. When hits carry `relevance`, the header says `· reranked by jev` and
  the `rel` column appears; otherwise `· ranked by cosine`. Score and relevance are never blended.
- **D-06:** **Per-scope coverage** is derived client-side by grouping the returned hits by scope,
  labelled `in top k`. Every `searched_scopes` entry is listed (zeros included);
  `scopes_truncated` / `scopes_unknown` are shown verbatim, never inferred.

### Results list scale
- **D-07:** Virtualize with **`@humanspeak/svelte-virtual-list`** in fixed-height mode (the
  milestone's allowed small UI dependency). Our own WAI-ARIA listbox sits on top: container
  `role="listbox" tabindex="0"`, rows `role="option"` with stable ids, `aria-activedescendant`,
  `j`/`k`/arrows/Home/End calling the virtualizer's scroll-to-index. Hover card is an overlay
  outside the virtualizer's flow, never a dynamic row height. Fallback if it misbehaves:
  `svelte-tiny-virtual-list` (STACK.md). — **Reversibility:** costly — the listbox, hover-card
  anchoring and scroll-to-index wiring are built around the library's API.
- **D-08:** Default search `k = 50`. A final **Show more** row re-queries at 100 → 250 → 1000
  (vector search has no cursor; more hits means a bigger `k`). `k` is part of the URL state.
- **D-09:** Operator-only input (no text, e.g. `scope:x #tag is:gotcha`) is an unranked
  `ListMemories` listing that **infinite-scrolls** on its cursor as the virtual list nears the end.
  Header: `Latest N memories across M scopes · unranked (list — no score)`.

### Shell, routes and palette
- **D-10:** `/` stays (scope tiles + recent feed, restyled onto the new row list). The header
  search box lives on every route; Enter on free text opens `/search?q=…` with chips applied.
- **D-11:** **Split `/` and ⌘K.** `/` focuses the header search, which is the server-driven
  palette (bits-ui `Command` with `shouldFilter={false}`, `SearchMemories`/`GetMemory`-backed) and
  the only place memories are searched. **⌘K** is a **command menu**: navigation, theme, text size,
  copy the current record's id, and (Phase 4) row actions. Its items are static, so client-side
  filtering is honest there — but it **always** ends in an unfiltered `Search memories for "…" ↵`
  row that hands the text to the header search, and a pasted UUID/short_id shows `Open record …`.
  ⌘K never shows "no matches" for text it did not send to the server. ENTRY-04 is satisfied by the
  header dropdown; ⌘K is a command menu, not the palette. Both get tests: the header palette's
  server-call test (success criterion 2) and a ⌘K test that a memory term yields the hand-off row.
- **D-12:** `/observe` adopts the shared dense row list and detail pane in this phase and keeps its
  `ScopesSidebar` until the curation phase (Phase 4) reshapes it.
- **D-13:** Design foundations ship in Phase 2: the sketch's new tokens (surface-2, hover,
  selected, border-subtle, text-faint, primary-soft, warning, success, `--cat-rule`) in
  `ui/src/app.css` (dark → `.dark`, light → `:root`), and the **site-wide text-size preference**
  (12–16px, default 15, persisted per viewer, applied on every route, **Aa** popover in the header,
  `⌘+`/`⌘-`/`⌘0`) per the `display.js` contract in the sketch findings.

### Detail pane and actions
- **D-14:** The pane uses the sketch's **stacked sections** (sticky head → title → actions →
  State → Content → Tags → Metadata), not tabs. This supersedes ROW-07's "Summary / Content / Meta
  tabs" wording; every field ROW-07 lists must still appear (supersession links, schedule window,
  archive stamp, schema version, citations, usage counters, both id forms copyable).
- **D-15:** Actions are **inline buttons** under the title (Edit, Supersede…, Archive/Restore,
  Share/Make private, Delete in danger colour), replacing today's `⋯` menu.
- **D-16:** Supersede and Archive/Restore render **disabled with a tooltip** ("Arrives with
  curation — Phase 4") until their RPCs land. Their row keys are reserved, not bound.
- **D-17:** Row keys on the active row: `e` edit · `s` share / make private · `#` delete (through
  the existing confirm dialog) · `c` copy short_id · `⇧C` copy full UUID. Listed in the listbox
  legend.

### Carried forward (decided earlier, do not re-ask)
- Sketch winners (memory `6akjphx3k7`): header inline search with anchored dropdown (002 B), fixed
  rows + 250ms hover card + right pane where a second click on the open row closes it (001 B′).
- One shared classifier module for every id-accepting box: UUID → `GetMemory`; 10-char Crockford
  base32 → short_id lookup (via `GetMemory`); else `SearchMemories` with `cross_spine` on;
  `scope:`/`in:`, `#tag`/`tag:`, `is:<category>` become chips. A short_id-shaped miss says it was
  searched as text instead — never a silent reinterpretation.
- Rejections render the server envelope `field=<f> hint=<code>: <text>` with fix rows.
- "The entry point must not lie" (memory `st74vdk0gh`); `shouldFilter={false}` on the palette.
- State chips in canonical order archived › superseded › expired › scheduled, dim only past
  states, collapse to `first +N`.
- Authz lives only in `internal/store`; zero new Go dependencies.

### Claude's Discretion
- Exact debounce interval, overscan, and hover-card placement math (within the sketch's timings).
- ListMemories hidden-count mechanism (D-02) as long as it is bounded and documented.
- Proto field shape for the per-state hidden count (message vs four scalars), subject to
  `buf breaking` staying green.
- Content and structure of the `engram-console-conventions` and `engram-connect-client` skills,
  sourced from the sketch findings and the existing `ui/src/lib` client/query code.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Design direction (locked by Phase 01.1 sketches)
- `.claude/skills/sketch-findings-engram/SKILL.md` — design direction index
- `.claude/skills/sketch-findings-engram/references/recall-surface.md` — results list, hover card,
  detail pane, header search, classifier, honest states, keyboard model, stack mapping
- `.claude/skills/sketch-findings-engram/references/foundations.md` — tokens, text-size
  `display.js` contract, state chips, timings
- `.claude/skills/sketch-findings-engram/sources/001-results-surface/index.html`,
  `sources/002-command-palette/index.html`, `sources/themes/` — runnable winners

### Milestone scope and research
- `.planning/REQUIREMENTS.md` — ENTRY-01..06, ROW-01..07, DSYS-01, DSYS-02
- `.planning/ROADMAP.md` — Phase 2 goal and success criteria 1–7
- `.planning/notes/console-overhaul-exploration.md` — the two motivating live bugs, API gaps
- `.planning/research/SUMMARY.md`, `.planning/research/STACK.md` (virtualization verdict),
  `.planning/research/PITFALLS.md` (pitfalls 1–5: palette lying, search race, flash-to-empty,
  focus/hover conflation, virtualization breakage), `.planning/research/FEATURES.md`
- `.planning/research/questions.md` — open: TanStack Query v6 `keepPreviousData` behaviour

### API contract
- `proto/engram/v1/engram.proto` — `SearchMemoriesRequest/Response`, `ListMemoriesResponse`,
  `GetMemoryRequest`, `ListScopesResponse`
- `internal/server/connecterror.go` — error envelope mapping (`ErrAmbiguousShortID` →
  `failed_precondition`)
- `CLAUDE.md` §Memory contract — cross_spine coverage fields, recall gate; must be updated for D-01

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ui/src/lib/client.ts`: `engram` (read) and `engramWrite` (CSRF write) clients, `mapAuthError`.
- `ui/src/lib/queries.ts`: `PAGE_LIMIT`, `CATEGORIES`, `INCLUDE_STATES`, `listMemoriesKey`,
  `parseObserveParams`/`observeSearch` — the URL-state and query-key pattern to extend.
- `ui/src/lib/memorystate.ts`: `STATE_WORD_ORDER`, `memoryStateWords`, `isPastState`.
- `ui/src/lib/errors.ts`: `describeError`, `handleQueryError` — to carry the `field=/hint=` envelope.
- `ui/src/lib/components/`: `MemoryRow`, `MemoryList`, `MemoryDetail` (tabs + `⋯` menu today),
  `CommandPalette` (the lying palette), `ScopeChip`, `DeleteConfirmDialog`, `MemoryFormSheet`.
- `ui/src/lib/resume.ts` — re-auth resume envelope; destination routes must keep consuming it.

### Established Patterns
- svelte-query v6 options wrapped in a function; results are runes objects.
- `cross_spine` sent explicitly, never inferred from an empty scope (D-04 of an earlier phase, #500).
- vitest-browser tests with screenshots beside each component.
- Server: cross-spine coverage fields computed in `internal/server/connectapi.go` (~L327, ~L383),
  mirrored in `internal/server/tools.go` (`searchMemory`, ~L1917) and `cmd/engram/client_search.go`
  / `client_list.go` — the path D-01..D-03 follow.

### Integration Points
- `ui/src/routes/+layout.svelte` mounts `CommandPalette` (⌘K) — becomes header search + ⌘K menu.
- `ui/src/routes/search/+page.svelte` (68 lines) — rebuilt as the recall surface.
- `ui/src/routes/+page.svelte`, `ui/src/routes/observe/+page.svelte` — adopt the shared row list.
- Recall-gate test vocabularies in `internal/store/schemaversion_recallgate_test.go`: the
  deliberately ungated counting query for D-02 must be registered there (memory `ba1st8kzwz` —
  four lists, not two), or the static AST scan will not see it.
- Vendored SPA + `ui-drift` gate; regenerate `gen/` and `ui/src/lib/gen` after the proto change.

</code_context>

<specifics>
## Specific Ideas

- ⌘K split came from Sean: keep ⌘K for navigation and actions other than search, with the
  hand-off row so it can never repeat the "no matches for github" bug.
- Header wording from the sketch, e.g. `12 hits across 3 scopes for query · ranked by cosine ·
  2 hidden by recall gate · scopes_truncated: 1 readable scope not searched`.

</specifics>

<deferred>
## Deferred Ideas

- Candidate list for an ambiguous short_id (would need server work) — not planned.
- A request-level Jev rerank toggle — not planned; reranking stays server config.
- Tags section of the header dropdown with `ListTags` counts — arrives with Phase 3's ListTags RPC.

### Reviewed Todos (not folded)
- "Security-review then install the three design/a11y registry skills" — belongs to DSYS-03
  (WCAG / Web Interface Guidelines audit), mapped to Phase 4.

</deferred>

---

*Phase: 02-recall-first-search*
*Context gathered: 2026-09-26*
