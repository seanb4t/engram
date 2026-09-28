# Phase 2: Recall-First Search - Research

**Researched:** 2026-09-26
**Domain:** SvelteKit 2/Svelte 5 operator console — recall entry-point resolution, dense virtualized
results list, honest recall-gate reporting, header search + ⌘K split, detail pane rework
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

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
  reference get the new field. This is an output-only count: it does NOT add the
  `include_archived/superseded/scheduled` inputs to MCP and does not relax the MCP recall gate —
  the 2026-08-20 decision `fenpnam8ah` (MCP input schemas unchanged, agent recall zero-junk)
  still holds. No record content or id is exposed through the count.
- **D-04:** An **ambiguous short_id** (GetMemory → `failed_precondition`, legacy data) shows an
  honest warning-tone message only: `short_id <x> is ambiguous — paste the full id to be exact`.
  No candidate list, no server change.
- **D-05:** **Jev** is a status, not a toggle. There is no request field for reranking, so there is
  no `jev rerank` facet chip. When hits carry `relevance`, the header says `· reranked by jev` and
  the `rel` column appears; otherwise `· ranked by cosine`. Score and relevance are never blended.
- **D-06:** **Per-scope coverage** is derived client-side by grouping the returned hits by scope,
  labelled `in top k`. Every `searched_scopes` entry is listed (zeros included);
  `scopes_truncated` / `scopes_unknown` are shown verbatim, never inferred.
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

**Carried forward (decided earlier, do not re-ask):**
- Sketch winners (memory `6akjphx3k7`): header inline search with anchored dropdown (002 B), fixed
  rows + 250ms hover card + right pane where a second click on the open row closes it (001 B′).
- One shared classifier module for every id-accepting box: UUID → `GetMemory`; 10-char Crockford
  base32 → short_id lookup (via `GetMemory`); else `SearchMemories` with `cross_spine` on;
  `scope:`/`in:`, `#tag`/`tag:`, `is:<category>` become chips. A short_id-shaped miss says it was
  searched as text instead — never a silent reinterpretation.
- Rejections render the server envelope `field=<f> hint=<code>: <text>` with fix rows.
- "The entry point must not lie" (memory `st74vdk0gh`); `shouldFilter={false}` on the palette.
- State chips in canonical order archived › superseded › expired › scheduled, dim only past
  states, collapse to `first +N`. This **replaces**, for the fixed-height dense row, the
  2026-08-20 rule (`cb5dajw7qp`) that the state meta line wraps and never truncates.
- Authz lives only in `internal/store`; zero new Go dependencies.

### Claude's Discretion

- Exact debounce interval, overscan, and hover-card placement math (within the sketch's timings).
- ListMemories hidden-count mechanism (D-02) as long as it is bounded and documented.
- Proto field shape for the per-state hidden count (message vs four scalars), subject to
  `buf breaking` staying green.
- Content and structure of the `engram-console-conventions` and `engram-connect-client` skills,
  sourced from the sketch findings and the existing `ui/src/lib` client/query code.

### Deferred Ideas (OUT OF SCOPE)

- Candidate list for an ambiguous short_id (would need server work) — not planned.
- A request-level Jev rerank toggle — not planned; reranking stays server config.
- Tags section of the header dropdown with `ListTags` counts — arrives with Phase 3's ListTags RPC.
- "Security-review then install the three design/a11y registry skills" — belongs to DSYS-03
  (WCAG / Web Interface Guidelines audit), mapped to Phase 4.
</user_constraints>

## Project Constraints (from CLAUDE.md)

- **VCS/PRs:** git, branch + PR, never push to `main` (protect-main ruleset); Conventional Commits
  required, PR titles CI-validated.
- **License headers:** every in-scope Go/Markdown file needs the Apache-2.0 SPDX header
  (`task license:check`/`task license:add`) — but `.planning/**` (this file), `skill/**/SKILL.md`,
  and slash-command markdown are excluded; do not add a header to any new `.claude/skills/*/SKILL.md`.
- **Lint/format:** `task lint` (golangci-lint, yamlfmt, actionlint, rumdl) and `task fmt` (gofmt,
  dprint, yamlfmt) must be clean before shipping.
- **Authorization:** enforced only in `internal/store` (DEC-cgb, refined by ADR `engram-cdr1`) —
  never in a Connect/MCP handler. The hidden-count comparison call must go through
  `Store.Search`/`Store.List` unchanged (own filter-building path), never a hand-rolled second
  filter, to avoid silently dropping the owner/scope authz condition (see Security Domain).
- **Capture is explicit and zero-junk:** no auto-extraction — not relevant to this phase's
  read-only surfaces, but binding on any future write-surface work this phase might touch (it does
  not add any write RPC).
- **Proto field numbers:** a `deprecated = true` field still occupies its number; append new fields
  at the next unused number (confirmed this session: `SearchMemoriesResponse` next free = 5,
  `ListMemoriesResponse` next free = 8). `buf breaking` (`breaking.use: [FILE]`) gates this in CI.
- **CLI is correct-by-reading** (`4aksmneehh`): any new `engram search`/`engram list` output line
  (the hidden-count footer) needs help-text/docs treatment, not just a validation backstop.
- **Not used here:** viper, cocogitto — not relevant to this UI-heavy phase but binding project-wide.
- **Durable project memory** goes into the engram MCP store, not `MEMORY.md` files — applies to any
  decisions this phase's plan/execution wants to record for later phases.
- **Issue tracking:** GitHub Issues (label `from-beads` for migrated items); no markdown TODO lists
  for durable tracking — file follow-ups (e.g., the Open Questions above) as GitHub issues if they
  survive planning.

## Summary

Phase 2 is a design-and-code job on an already-current stack, not a stack migration. Every
capability except the new hidden-count field reuses an existing Connect RPC
(`SearchMemories`/`ListMemories`/`GetMemory`/`ListScopes`), so the highest-value research this
session went into (a) how to compute the D-01/D-02 recall-gate hidden count cheaply and
authz-correctly without new `internal/store` surface, (b) confirming the exact library APIs the
sketch findings named against their real, currently-installed versions, and (c) closing the
milestone's one open cross-cutting question (TanStack Query v6's `keepPreviousData` behavior) by
reading the installed package rather than trusting a migration guide's silence.

The hidden-count mechanism turns out to be nearly free: `store.Search`/`store.List` already expose
`IncludeArchived`/`IncludeSuperseded`/`IncludeScheduled` as `SearchOptions`/`ListOptions` fields
(`internal/store/store.go:1172-1174`, `:1546-1548`), so "the same query with the recall gate
lifted" is just a second call to the *same, already-classified* `Store.Search` entry point with
those three flags forced `true` — no new `internal/store` function, and therefore no new row to
add to `schemaversion_recallgate_test.go`'s three classification lists (`recallTransmitters` /
`operatorMigrationEmitters` / `otherNonRecallEmitters`) or its `recallEntryPointSeeds` list,
because the AST-reachability scan classifies emission *sites*, not call *sites*, and `Store.Search`
is already `recallTransmitters`' first entry. The comparison must use `Store.Search` directly
(never `Store.SearchReranked`), or the hidden-count query silently doubles the Jev reranker cost
per search. `Store.List`'s unranked equivalent has no such free ride — its total is computed with
a private `s.client.Count(...)` call inside `List` itself (`store.go:1806-1811`), so a bounded,
per-state count needs either a second full `Store.List` call (simplest, no new store code, doubles
one bounded page fetch) or a new `Store` method issuing up to four `Count` calls with a `MustNot`
condition per state (cheaper on a large collection, but *is* new `internal/store` code reachable
from a seed and *would* need a `recallTransmitters` row).

Every sketch-named library API was verified live against the pinned/installed version rather than
training memory: `@humanspeak/svelte-virtual-list@0.5.14` (already the newest release, peer
`svelte: ^5.0.0`, compatible with the pinned `5.57.1`) has **no discrete "fixed-height mode"** —
its whole model is dynamic-height-by-measurement, and the sketch's "fixed-height mode" is achieved
behaviorally by giving every row identical content, not by a prop switch. Its scroll-to-index API
is `scroll({ index, align, smoothScroll })`, not `scrollToIndex`, and its scrollable viewport
renders with a hardcoded `role="region"` (a focusable, labeled landmark) — **not**
`role="listbox"`, which is a real, unresolved tension against the sketch's WAI-ARIA APG Listbox
requirement that needs a design decision, not an assumption, before implementation. bits-ui's
`Command.Root shouldFilter={false}` threads straight through the shadcn-svelte wrapper
(`ui/src/lib/components/ui/command/command.svelte` spreads `...restProps` onto
`CommandPrimitive.Root`), and the existing `hover-card.svelte`/`resizable-pane-group.svelte`
wrappers are already thin passes onto bits-ui `LinkPreview` and paneforge `PaneGroup` respectively
— so `openDelay`/`closeDelay`/`customAnchor` and `keyboardResizeBy`/`autoSaveId` are already
reachable, no new wrapper needed. `@tanstack/svelte-query@6.1.48` (the version actually installed,
inside the `^6.1.34` range) resolves `@tanstack/query-core@5.102.8` as its dependency — directly
confirmed by reading the installed package's `keepPreviousData` source
(`node_modules/.pnpm/@tanstack+query-core@5.102.8/.../utils.cjs:149-151`) — so v6's Svelte adapter
is a thin runes-API wrapper over the **same** v5 query-core, and `placeholderData: keepPreviousData`
behaves byte-identically to v5. This closes `questions.md`'s open item with a direct read, not an
inference.

**Primary recommendation:** build the hidden-count as a second `Store.Search` call (search) plus a
second `Store.List` call (list) from `internal/server` only — zero `internal/store` diff, zero
recallgate-test-vocabulary changes — ship it as one new proto message
(`RecallGateHiddenCount{archived,superseded,expired,scheduled}`) reused as one field on both
`SearchMemoriesResponse` (next free number 5) and `ListMemoriesResponse` (next free number 8), and
treat the header search's role-vs-landmark conflict on `@humanspeak/svelte-virtual-list` as an
explicit build-and-verify spike in the first plan wave, not an assumption.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| UUID/short_id/text classification | Browser / Client | — | Pure string-shape decision (regex + one shared TS module); no server round trip needed to decide *which* RPC to call |
| id-shaped resolution (GetMemory) | API / Backend | Database / Storage | `GetMemory` already exists; store enforces not-found-for-unauthorized (DEC-xa6) |
| Free-text / cross_spine search | API / Backend | Database / Storage | `SearchMemories` + `Store.Search`; authz composed into the Qdrant filter only inside `internal/store` (DEC-cgb) |
| Recall-gate hidden count (D-01/D-02) | API / Backend | Database / Storage | Second bounded `Store.Search`/`Store.List` call with the recall-gate `Include*` flags forced true; server-side diff/bucket, never a browser-computed guess |
| Facet chips (category/tags/window/state/scope) | Browser / Client | API / Backend | Chips are pure URL-state; the server already accepts every field as a request parameter — no new server work |
| Scope autocomplete with counts | API / Backend | Browser / Client | `ListScopes` already returns counts; client renders via `Combobox` |
| Dense virtualized row list, hover card, keyboard model | Browser / Client | — | `@humanspeak/svelte-virtual-list` + hand-rolled `aria-activedescendant` listbox layered on top; no server involvement |
| Detail pane (stacked sections, action buttons) | Browser / Client | API / Backend (fetch) | Rendering is client-only; content comes from the same `Memory` already fetched for the row |
| Site-wide text-size preference | Browser / Client | — | `localStorage` + a Svelte 5 `.svelte.ts` store; SvelteKit is adapter-static, so there is no SSR path that could apply it earlier than first paint |
| `engram-console-conventions` / `engram-connect-client` skills | (n/a — documentation) | — | Project-local `.claude/skills/` artifacts, not runtime code |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `@humanspeak/svelte-virtual-list` | `0.5.14` | Virtualizes the dense results list (bounded to `k`/page size ≤ 1000 rows) | `[VERIFIED: npm registry]` Only Svelte-5-runes-native, actively maintained (published 2026-08-10) virtualizer; `@tanstack/svelte-virtual`'s Svelte 5 support issue (#866) is still open. `[ASSUMED]` per package-name-provenance rule (discovered via the milestone's prior WebSearch/STACK.md research, not official docs) — registry existence alone does not upgrade this to VERIFIED; `npm view` and the legitimacy check below confirm it *exists* and is *not slopsquatted*, which is a different claim. |
| `bits-ui` | `2.19.3` (bump from installed `2.18.1`) | `Command`, `Popover`, `Combobox`, `LinkPreview` primitives | `[VERIFIED: npm registry]` for the version number and publish date (`npm view bits-ui version`/`time.modified`, this session); `[ASSUMED]` for the package identity itself per the same provenance rule — it is already a project dependency, discovered originally via training knowledge/WebSearch, not first read from an official doc this session. |
| `@tanstack/svelte-query` | `^6.1.34` (installed: `6.1.48`) | Debounced, cancellable, previous-data-preserving search queries | `[VERIFIED: ui/node_modules]` — read directly this session; no version bump needed. |
| paneforge | `1.0.2` (installed, unchanged) | Resizable/collapsible detail pane | `[VERIFIED: ui/package.json]` — already a dependency; `ui/src/lib/components/ui/resizable/resizable-pane-group.svelte:1-24` (read this session) already forwards every prop to the primitive. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| shadcn-svelte `popover` | n/a (not yet installed locally) | Header search dropdown anchor, Aa text-size popover, scope-picker chip | Run `pnpm dlx shadcn-svelte@latest add popover` — `[VERIFIED: ui/src/lib/components/ui/*]` directory listing this session shows no `popover/` folder exists yet, only `hover-card`/`resizable`/`dialog`/`dropdown-menu`/`command`/`tooltip`/`select`. |
| shadcn-svelte `combobox` | n/a (not yet installed locally) | Scope autocomplete with `ListScopes` counts | Same as above — `[VERIFIED: ui/src/lib/components/ui/*]`, no `combobox/` folder present. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `@humanspeak/svelte-virtual-list` | `svelte-tiny-virtual-list` | Fallback per D-07 if the library's dynamic-height remeasurement or its `role="region"` viewport proves unworkable against the listbox requirement; loses the built-in keyboard-accessible-viewport wiring (STACK.md, still current). |
| Second `Store.Search`/`Store.List` call for the hidden count | A new `Store.HiddenRecallCounts` method with 3-4 `Count` calls per state | Cheaper on very large collections/pages, but is new `internal/store` code reachable from a `recallEntryPointSeeds` member and needs a `recallTransmitters` row in `schemaversion_recallgate_test.go` plus its own unit tests — a real cost the "second call" option avoids entirely. |
| One shared `RecallGateHiddenCount` message field on both responses | Four separate scalar fields (`hidden_archived`, `hidden_superseded`, ...) duplicated on both messages | Both are additive and `buf breaking`-safe (`buf.yaml: breaking.use: [FILE]`, confirmed this session); the message form is one type reused twice instead of eight scalar fields — Claude's Discretion per CONTEXT.md, recommend the message form for symmetry with the MCP/CLI JSON shape (`{"archived":0,"superseded":0,"expired":0,"scheduled":0}`). |

**Installation:**
```bash
pnpm --dir ui add @humanspeak/svelte-virtual-list@0.5.14
pnpm --dir ui add bits-ui@2.19.3
pnpm dlx shadcn-svelte@latest add popover combobox
```

**Version verification:** confirmed live this session —
`npm view @humanspeak/svelte-virtual-list version` → `0.5.14` (published 2026-08-10, peer
`svelte: ^5.0.0`); `npm view bits-ui version` → `2.19.3` (published 2026-09-22); installed
`ui/package.json` still pins `bits-ui@^2.18.1` and `node_modules/.pnpm` still resolves `2.18.1` —
the bump has not yet been applied.

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `@humanspeak/svelte-virtual-list` | npm | published 2026-08-10 (~1.5 mo) | 3,947/wk | `github.com/humanspeak/svelte-virtual-list` | `[OK]` | Approved |
| `bits-ui` (bump to 2.19.3) | npm | latest version published 2026-09-22 (~4 days old at research time) | 1,135,668/wk | `github.com/huntabyte/bits-ui` | `[SUS]` (`reasons: ["too-new"]`) | Flagged — planner must add a `checkpoint:human-verify` before the version bump, though this is an established dependency (1.1M weekly downloads, already `^2.18.1` in this project) and the flag is solely about the **2.19.3 point release's** age, not the package's legitimacy. Wait a few more days or verify the 2.19.3 GitHub release/tag matches the npm publish before bumping if the checkpoint is declined. |

**Packages removed due to `[SLOP]` verdict:** none.
**Packages flagged as suspicious `[SUS]`:** `bits-ui` (version-recency flag only, see disposition above — the planner must gate the `2.19.3` bump behind a `checkpoint:human-verify` task per the Package Legitimacy Gate protocol).

`@humanspeak/svelte-virtual-list` and `bits-ui` were both discovered via the milestone's own prior
WebSearch-based research (STACK.md) and training knowledge, not first read from an official doc
this session — both carry `[ASSUMED]` package-identity provenance per the provenance rule even
though the legitimacy check and `npm view` both return clean/expected results.

## Architecture Patterns

### System Architecture Diagram

```
Browser (any search surface: header box · /search · ?q= · palette hand-off)
   │
   ▼
Shared TS classifier (one module)
   │  UUID?            10-char Crockford base32?      else
   ▼                    ▼                              ▼
GetMemory(id)      GetMemory(short_id lookup)     SearchMemories / ListMemories
   │                    │                              │
   │                    │ FailedPrecondition:           │
   │                    │ store.ErrAmbiguousShortID     │
   │                    │ ("ambiguous short id: <id>")  │
   │                    ▼                              ▼
   │             UI: warning-tone message,    ┌────────────────────────────┐
   │             no candidates, paste full id  │ internal/server/connectapi │
   │                                           │  - effectiveSearchScope    │
   ▼                                           │  - a.d.searchMemory/       │
UI: fetched-by-id record, or                   │    listMemory (deps.*)     │
"no memory with id … that you can read"        │  - a.d.searchedScopes      │
                                                └──────────────┬─────────────┘
                                                                │
                                          ┌─────────────────────┴──────────────────────┐
                                          ▼                                            ▼
                             1st call: Store.Search/List             2nd call: SAME Store.Search/List
                             (recall gate ON, caller's own            (Include{Archived,Superseded,
                              Include* flags) → shown results          Scheduled}=true, same k/filters)
                                          │                                            │
                                          └──────────────────┬─────────────────────────┘
                                                              ▼
                                          diff by id, bucket extras by state
                                          (ArchivedAt/SupersededBy/NotBefore/NotAfter)
                                                              │
                                                              ▼
                              RecallGateHiddenCount{archived,superseded,expired,scheduled}
                                          │
                     ┌────────────────────┼─────────────────────────┐
                     ▼                    ▼                         ▼
              Connect response      MCP result map              CLI stdout footer
          (SearchMemoriesResponse  (search_memory/list_memory   (renderCoverageFooter-
           / ListMemoriesResponse   result map, alongside        style helper, ALWAYS
           new field)               searched_scopes/…)           printed, not gated on
                                                                  cross_spine)
                                                              │
                                                              ▼
                                          Results header: "12 hits … · 2 hidden by recall gate"
                                          (tooltip splits by state; header ALSO reports
                                           searched_scopes/scopes_truncated/scopes_unknown
                                           independently, per the existing cross-spine contract)

Dense row list (client-only, no new server calls):
  SearchMemories/ListMemories response
        │
        ▼
  @humanspeak/svelte-virtual-list (uniform-height rows; NOT its dynamic-height feature)
        │  renderItem snippet renders <div role="option" id="opt-N">
        ▼
  App-owned wrapper carrying role="listbox" tabindex="0" aria-activedescendant
  (see Pitfall: role="region" vs role="listbox" below — needs a spike)
        │
   j/k/arrows/Home/End → move activeId, call listRef.scroll({index, align:'nearest'})
   pointer mousemove (not mouseenter) → 250ms-delayed HoverCard (bits-ui LinkPreview,
     customAnchor = active row's DOM node, controlled `open`)
   Enter/click → toggle paneforge Pane (collapse()/expand() via bind:this ref)
```

### Recommended Project Structure

```
ui/src/lib/
├── search/
│   ├── classify.ts          # shared UUID/short_id/text + operator-chip classifier
│   ├── classify.test.ts     # node-project unit tests (regex edge cases)
│   ├── recall-header.ts     # pure formatter: hits/scopes/hidden-count → header string
│   └── recall-header.test.ts
├── components/
│   ├── HeaderSearch.svelte      # replaces CommandPalette's search half (Popover+Command)
│   ├── CommandMenu.svelte       # the new ⌘K: static nav items + hand-off row (D-11)
│   ├── ResultsList.svelte       # wraps @humanspeak/svelte-virtual-list + listbox semantics
│   ├── ResultRow.svelte         # one dense row (fixed height, grid columns)
│   ├── ResultHoverCard.svelte   # LinkPreview-based overlay, controlled open
│   ├── FacetStrip.svelte        # category/scope/cross_spine/include_*/tags chips
│   ├── ScopeCombobox.svelte     # ListScopes-backed autocomplete with counts
│   └── DetailPane.svelte        # rebuilt: stacked sections, inline action buttons (D-14/D-15)
├── display.svelte.ts        # site-wide text-size store ($state, localStorage, DESIGN per foundations.md)
└── errors/
    └── connect-error.ts     # parseConnectError(err): {kind: 'not-found'|'ambiguous'|'rejected'|'opaque', ...}
```

### Pattern 1: Recall-gate hidden count via a second, unmodified `Store.Search`/`Store.List` call

**What:** In `internal/server/tools.go`'s `deps.searchMemory`/`deps.listMemory` (the shared core both
MCP and Connect already call), after computing the caller's real result set, issue a second call to
the same store method with `IncludeArchived: true, IncludeSuperseded: true, IncludeScheduled: true`
(every other option — scope, tags, categories, window, `k`/limit — held identical), diff the
returned ids against the first call's ids, and bucket every id present only in the second call by
inspecting `Memory.ArchivedAt`/`Memory.SupersededBy`/`Memory.NotBefore`/`Memory.NotAfter`
(`internal/store/store.go:218-246`, read this session) using the SAME precedence
`cmd/engram/memory_state.go:39-67`'s `memoryStateWords` already encodes (expired evaluated first,
suppresses scheduled) — reimplemented against `store.Memory`'s fields since `memoryStateWords`
lives in `package main` and operates on the proto type, not `store.Memory`, so it cannot be
imported from `internal/server`.

**When to use:** Both `SearchMemories`/`search_memory` (compare against `Store.Search` — never
`Store.SearchReranked`, which would double the Jev rerank cost) and, per D-02's discretion,
`ListMemories`/`list_memory` (compare against a second bounded `Store.List` call at the same
page window — the simplest option that needs no new `internal/store` code).

**Example:**
```go
// Source: internal/store/store.go:1146-1192 (SearchOptions, read this session)
// and internal/server/tools.go:1917-1946 (deps.searchMemory, read this session)
gated, err := d.st.SearchReranked(ctx, scope, c.Subj, req.Query, vec, req.K, store.SearchOptions{
    Tags: req.Tags, Categories: req.Categories,
    CreatedAfter: req.CreatedAfter, CreatedBefore: req.CreatedBefore,
    IncludeArchived: req.IncludeArchived, IncludeSuperseded: req.IncludeSuperseded,
    IncludeScheduled: req.IncludeScheduled, RankHook: d.rankHook, RankAudit: d.rankAudit,
})
// ... existing behavior unchanged above; NEW below:
ungated, err := d.st.Search(ctx, scope, c.Subj, vec, req.K, store.SearchOptions{
    Tags: req.Tags, Categories: req.Categories,
    CreatedAfter: req.CreatedAfter, CreatedBefore: req.CreatedBefore,
    IncludeArchived: true, IncludeSuperseded: true, IncludeScheduled: true,
})
hidden := diffAndBucketByState(gated, ungated) // new, small, internal/server-local helper
```

### Pattern 2: Header search classifier and dropdown (bits-ui `Command` in a `Popover`, not a `Dialog`)

**What:** `Command.Root shouldFilter={false}` inside `Popover.Content customAnchor={inputEl}` — NOT
`Command.Dialog`, which is what `ui/src/lib/components/CommandPalette.svelte:9-22` (read this
session) currently uses and is exactly the "What to Avoid: ⌘K modal palette" case the sketch
findings name. `Command.svelte`'s wrapper (`ui/src/lib/components/ui/command/command.svelte:18-25`,
read this session) spreads `...restProps` onto `CommandPrimitive.Root`, so `shouldFilter={false}`
threads through unmodified.

**When to use:** The header search dropdown only (ENTRY-04). The new ⌘K command menu (D-11) keeps
`Command`'s default client-side filtering — its items are static navigation, so filtering them
locally is honest — but must still end in an unfiltered "Search memories for …" hand-off row that
is never itself subject to `Command`'s filter (give it a `value` matching every keystroke, or
render it outside the filtered `Command.Group`).

**Example:**
```svelte
<!-- Source: ui/src/lib/components/ui/command/command.svelte:18-25 (confirms restProps passthrough) -->
<!-- and Context7 /huntabyte/bits-ui "Disable Command Filtering" + "Custom Anchor for Popover Content" -->
<Popover.Root bind:open>
  <Popover.Content customAnchor={inputEl}>
    <Command.Root shouldFilter={false}>
      <Command.List>
        <!-- items driven from live SearchMemories/ListScopes/ListTags responses -->
      </Command.List>
    </Command.Root>
  </Popover.Content>
</Popover.Root>
```

### Pattern 3: Hover card as a controlled `LinkPreview`, anchored to the keyboard-active row

**What:** `ui/src/lib/components/ui/hover-card/hover-card.svelte:1-7` (read this session) is
already `bits-ui`'s `LinkPreview` with `open` bindable and `...restProps` spread — confirms the
sketch's stack mapping directly. Drive `open` as app state (not the primitive's own hover
detection) so keyboard focus can open it instantly (0ms) while pointer hover keeps the 250ms
`openDelay`, and set `customAnchor` to the currently-hovered/active row's DOM node
(`bind:this`), confirmed available via Context7's "Configure Custom Anchor Element" for
`LinkPreview.Content`.

**When to use:** ROW-02's overlay hover card. Never let this same `open` boolean double as the
`j`/`k` "active row" state (Pitfall 4) — track them as two separate variables that happen to be
set from the same row index during pointer-driven opens.

### Pattern 4: Resizable, collapsible detail pane via paneforge's imperative ref

**What:** `ui/src/lib/components/ui/resizable/resizable-pane-group.svelte:1-24` (read this session)
already forwards `autoSaveId`/`keyboardResizeBy`/everything else to paneforge's `PaneGroup`.
Toggle-close (a second click/Enter on the open row closes the pane) is `pane.collapse()`/
`pane.expand()` via `bind:this={pane}`, confirmed via Context7's "Collapsible Panes with
Programmatic Control" example (`collapsible`, `collapsedSize`, `onCollapse`/`onExpand`).

**When to use:** ROW-07's detail pane. **Gotcha:** paneforge's `keyboardResizeBy` is a
**percentage of the group**, confirmed via Context7's "Keyboard Navigation for Pane Resizing"
example (`keyboardResizeBy={5}` steps by 5%), not a pixel count — the sketch's "32px steps" cannot
be passed to `keyboardResizeBy` directly; either accept a percentage step or compute one from the
live container width in a `$effect`/`ResizeObserver` and pass a per-keypress `pane.resize(pct)`
call instead of relying on the built-in prop.

### Anti-Patterns to Avoid

- **Calling `Store.SearchReranked` a second time for the hidden count.** Doubles the Jev reranker
  call (cost + `ENGRAM_SEARCH_RERANK_AUDIT` log volume) for a count that only needs ids and state
  fields, not a re-ranked order. Call `Store.Search` directly.
- **Gating the hidden count inside `recallResultMap`'s `crossSpine` conditional.** The existing
  `internal/server/tools.go:2067-2078` `recallResultMap` only adds `searched_scopes`/
  `scopes_truncated`/`scopes_unknown` when `crossSpine` is true (by design — a scope-confined call
  issues no `ListScopes` round trip). The hidden count has no such precondition — a
  scope-confined search can still have archived/superseded/expired/scheduled records hidden in it
  — so it must be computed and reported unconditionally, as its own field, never nested inside that
  existing conditional.
- **Reading `ConnectError.message` to detect the `field=/hint=` envelope or the ambiguous-short_id
  case.** `message` is prefixed with the bracketed code (confirmed by reading
  `node_modules/@connectrpc/connect/dist/cjs/connect-error.js:40` this session: `super(createMessage(message, code))`) — the classifier must read `.rawMessage` (the unprefixed
  server text) and `.code` (the numeric `Code` enum), exactly the pattern `ui/src/lib/client.ts:30-35`'s `mapAuthError` already uses (`err.code === Code.Unauthenticated`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Virtualized scrolling, resize handling | A custom windowed `{#each}` | `@humanspeak/svelte-virtual-list` | Scroll-math edge cases (resize, overscan, scroll-into-view) are exactly what this library exists to absorb; STACK.md already rejected the hand-rolled alternative as a "legitimate but not preferred" fallback. |
| Popover/floating positioning for the hover card and dropdown | Custom `getBoundingClientRect` + manual flip logic | bits-ui `Popover`/`LinkPreview` `customAnchor` | Both already ship viewport-flip/clamp behavior; hand-rolling reintroduces exactly the placement bugs Floating-UI-based primitives solve. |
| Pane resize/collapse and persisted layout | Custom flex-basis + localStorage code | paneforge `PaneGroup`/`Pane` + `autoSaveId` | Already installed, already wired via `ui/src/lib/components/ui/resizable/*`; `autoSaveId` is a one-line persisted-layout feature. |
| Text-size scaling math | Ad hoc `rem`/`px` overrides per component | The `display.js` contract's single `--ui-font`/`--u` token, per `foundations.md` | A second scaling mechanism invented locally will drift from the Aa popover's global preference the first time someone forgets to derive from `--ui-font`. |

**Key insight:** every "don't hand-roll" item in this phase already has a wired, forwarding wrapper
component in `ui/src/lib/components/ui/*` — the work is composition and ARIA wiring on top of
primitives that are already one `import` away, not building new primitives.

## Common Pitfalls

(See also `.planning/research/PITFALLS.md` Pitfalls 1-5, which this phase must still address —
palette lying, search-box race, flash-to-empty, focus/hover conflation, virtualization breakage.
The three below are new findings from this session's direct verification.)

### Pitfall A: `@humanspeak/svelte-virtual-list`'s viewport hardcodes `role="region"`, not `role="listbox"`

**What goes wrong:** The library's scrollable viewport renders as a focusable, labeled
`role="region"` element (its own accessibility feature, "Keyboard-accessible viewport") — there is
no prop to override this role. Naively wrapping rows in `role="option"` underneath a `role="region"`
ancestor (instead of `role="listbox"`) produces a DOM the WAI-ARIA APG Listbox pattern does not
describe: a screen reader may not announce the region as a listbox with option children.

**Why it happens:** The sketch findings and STACK.md both describe the intended shape
(`role="listbox" tabindex="0"` on the scroll container) as if it were a property the consuming code
sets on the library's own element. Confirmed via Context7 this session
(`/humanspeak/svelte-virtual-list`, "viewportLabel prop type" + full props table): `role="region"`
is baked into the component, controllable only via `viewportLabel` (the accessible *name*, not the
*role*).

**How to avoid:** Spike this in the first plan wave, live, in a browser accessibility tree
inspector, before committing to the row/listbox architecture. Two candidate resolutions to test:
(1) an action that runs after mount and rewrites the rendered viewport element's `role` attribute
from `region` to `listbox` (fragile — could be undone on any internal re-render) or (2) accept the
library's own `region` role for the scroll container and place the actual `listbox`/`option` ARIA
roles on an inner wrapper the `renderItem` snippet fully controls, verifying with a real screen
reader (or the accessibility tree panel) that the nesting still reads as a listbox. Do not assume
either resolution works without a live check — this is exactly the class of claim the milestone's
own Pitfall 4 (focus/hover conflation) warns is easy to believe is "done" without inspecting the
accessibility tree.

**Warning signs:** `getByRole('listbox')` queries fail in a vitest-browser test even though the
visual list renders correctly; a screen reader announces "region" instead of "list, N items."

**Phase to address:** This phase, as a design-verification spike before the row-list component is
built, not after.

### Pitfall B: paneforge's `keyboardResizeBy` is a percentage, not the sketch's pixel step

**What goes wrong:** Wiring `PaneGroup keyboardResizeBy={32}` expecting 32px arrow-key steps
actually produces 32% steps — a single arrow-key press could swing the detail pane from 30% to 62%
of the body width.

**Why it happens:** Confirmed via Context7 this session (`/svecosystem/paneforge`, "Keyboard
Navigation for Pane Resizing in Svelte" — `keyboardResizeBy={5}` is captioned "adjust pane sizes...
by 5% increments"). The sketch's CSS-driven mock (`resizer` with `aria-valuemin="280"
aria-valuemax="900" aria-valuenow="440"`, pixel-based) does not map onto paneforge's own
percentage-based prop.

**How to avoid:** Either accept a percentage step (compute what percentage ≈ 32px is for the
current container width once, and re-derive on resize) and pass that to `keyboardResizeBy`, or
skip the prop and implement custom `keydown` handling on `PaneResizer` that calls the pane's
`resize(pct)` imperative method with a percentage computed from a live container width.

**Warning signs:** Arrow-key resizing feels wildly oversensitive in manual testing; a UAT script
asserting "the pane grows by roughly 32px" fails by an order of magnitude.

**Phase to address:** This phase, in the detail-pane plan wave.

### Pitfall C: Ambiguous short_id and the field/hint envelope are both `CodeFailedPrecondition` with different message shapes

**What goes wrong:** A classifier that branches only on `err.code === Code.FailedPrecondition`
cannot distinguish D-04's "short_id is ambiguous" case from ENTRY-05's "field=/hint=" rejected-envelope
case — both map to the same Connect code.

**Why it happens:** Confirmed via `internal/server/connecterror.go:84-85` (read this session):
`store.ErrAmbiguousShortID` maps to `connect.NewError(connect.CodeFailedPrecondition, err)` with
message `"ambiguous short id: <shortid>"` (the raw sentinel text,
`internal/store/store.go:82,2151`) — NOT the `field=<f> hint=<code>: <detail>` grammar
`argerror.go:99-101`'s `renderHintEnvelope` produces for `*argError`-classed rejections (e.g.
`scope-required-unless-cross-spine`). Both share `CodeFailedPrecondition`.

**How to avoid:** The shared TS classifier (`connect-error.ts`) must pattern-match on
`err.rawMessage` shape, not `err.code` alone: `/^field=([^ ]+) hint=([^:]+): (.+)$/` for the
rejected-envelope case (D-04's "Rejections use the real envelope"), `/^ambiguous short id: (.+)$/`
for the ambiguous-short_id case, `Code.NotFound` for not-found, and everything else as an opaque
error (still shown with a request id and Retry per the sketch's error-state spec — never silently
swallowed).

**Warning signs:** A UAT for D-04 (ambiguous short_id message) accidentally renders the generic
`field=/hint=` fix-row UI instead of the plain warning-tone message, or vice versa.

**Phase to address:** This phase, in the header-search/classifier plan wave.

## Runtime State Inventory

Not applicable — Phase 2 is new UI/API-additive work, not a rename/refactor/migration phase.

## Code Examples

### Diffing a gated vs. ungated `Store.Search` result set by state

```go
// Source: internal/store/store.go:218-246 (store.Memory field shapes, read this session)
// and cmd/engram/memory_state.go:39-67 (precedence order to mirror; not importable
// from internal/server since it lives in package main and operates on the proto type)
type hiddenBucket struct{ Archived, Superseded, Expired, Scheduled uint64 }

func diffAndBucketByState(gated, ungated []store.Memory) hiddenBucket {
	shown := make(map[string]bool, len(gated))
	for _, m := range gated {
		shown[m.ID] = true
	}
	var b hiddenBucket
	now := time.Now()
	for _, m := range ungated {
		if shown[m.ID] {
			continue
		}
		if m.ArchivedAt != nil {
			b.Archived++
		}
		if m.SupersededBy != nil {
			b.Superseded++
		}
		expired := false
		if m.NotAfter != nil && !m.NotAfter.After(now) {
			b.Expired++
			expired = true
		}
		if !expired && m.NotBefore != nil && m.NotBefore.After(now) {
			b.Scheduled++
		}
	}
	return b
}
```

### `ConnectError` classifier respecting `.rawMessage`/`.code`

```typescript
// Source: node_modules/@connectrpc/connect/dist/cjs/connect-error.js (read this session:
// `.message` is prefixed with the bracketed code; `.rawMessage` is the raw server text)
// and internal/server/connecterror.go:84-85, internal/server/argerror.go:99-101 (read this session)
import { ConnectError, Code } from '@connectrpc/connect';

const ENVELOPE_RE = /^field=([^ ]+) hint=([^:]+): (.+)$/;
const AMBIGUOUS_RE = /^ambiguous short id: (.+)$/;

export function parseConnectError(err: unknown) {
  if (!(err instanceof ConnectError)) return { kind: 'opaque' as const, detail: String(err) };
  if (err.code === Code.NotFound) return { kind: 'not-found' as const };
  const ambiguous = AMBIGUOUS_RE.exec(err.rawMessage);
  if (ambiguous) return { kind: 'ambiguous-short-id' as const, shortId: ambiguous[1] };
  const envelope = ENVELOPE_RE.exec(err.rawMessage);
  if (envelope) return { kind: 'rejected' as const, field: envelope[1], hint: envelope[2], detail: envelope[3] };
  return { kind: 'opaque' as const, detail: err.rawMessage, code: err.code };
}
```

### `keepPreviousData` wired with a per-query `AbortSignal`

```typescript
// Source: ui/src/routes/search/+page.svelte:18 (current, no signal — read this session)
// and node_modules/.pnpm/@tanstack+query-core@5.102.8/.../query.cjs:194-198 (signal support,
// read this session) — keepPreviousData confirmed byte-identical to v5's own implementation
import { createQuery, keepPreviousData } from '@tanstack/svelte-query';

const query = createQuery(() => ({
  queryKey: ['searchMemories', text, scope, crossSpine, tags, categories, includeArchived, includeSuperseded, includeScheduled, k],
  queryFn: ({ signal }) => engram.searchMemories({ query: text, scope, crossSpine, k: BigInt(k) /* ... */ }, { signal }),
  placeholderData: keepPreviousData,
  enabled: !!text
}));
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `Command.Dialog` (modal ⌘K) driving both search and navigation | Header-anchored `Command.Root` (Popover) for search; a separate ⌘K command menu for navigation | This phase (D-10/D-11) | Fixes the motivating "no matches for github" bug at its root — the palette stops being the thing that searches |
| Tabs (Summary/Content/Meta) in the detail pane | Stacked sections (sticky head → title → actions → State → Content → Tags → Metadata) | This phase (D-14) | Supersedes ROW-07's tab wording; every field ROW-07 lists must still appear, just not behind a tab click |
| `⋯` row action menu | Inline action buttons under the title | This phase (D-15) | Supersede/Archive render disabled with a tooltip until Phase 3's RPCs land (D-16) |

**Deprecated/outdated:** `keepPreviousData: true` boolean + `isPreviousData` flag (pre-v5 API) —
already gone in the installed `6.1.48`/`query-core@5.102.8`; only `placeholderData: keepPreviousData`
exists.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `@humanspeak/svelte-virtual-list` and `bits-ui` are the correct, non-slopsquatted package names (registry existence + legitimacy check both pass, but package identity itself was originally sourced via WebSearch/training, not an official doc read this session) | Standard Stack | Low — both are the milestone's already-adopted STACK.md picks and `bits-ui` is already a live project dependency; a wrong name would have failed `npm view`/the legitimacy check outright, which it did not |
| A2 | Comparing against a second `Store.List` call (rather than a new `Count`-based method) is an acceptable "bounded" reading of D-02's ListMemories discretion clause | Architecture Patterns / Pattern 1 | Low-medium — if a page/limit is large, doubling the `List` call doubles that fetch's cost; acceptable per D-02's own wording ("keep it bounded and document the choice"), but the planner should confirm this reading is what "bounded" was meant to license before committing, since the Count-based alternative is more work but cheaper at scale |
| A3 | `role="region"` vs `role="listbox"` on `@humanspeak/svelte-virtual-list`'s viewport is resolvable by one of the two approaches in Pitfall A | Common Pitfalls / Pitfall A | Medium — if neither resolution reads correctly to a screen reader, the fallback (`svelte-tiny-virtual-list`, which builds its own container from scratch) may be needed, which is a larger rework than a config change |
| A4 | Splitting the hidden count into per-state counts computed via the state-field diff (rather than any other bucketing scheme) satisfies D-01's "split per state" requirement without double-counting a record carrying more than one state word simultaneously | Architecture Patterns / Pattern 1 | Low — `memoryStateWords`' own precedence rule (expired suppresses scheduled) already resolves the one real double-count case; archived+superseded+expired can co-occur on one record and would legitimately increment more than one bucket, matching the header's own "1 archived, 1 superseded" tooltip precedent in the sketch |

**If this table is empty:** N/A — see rows above.

## Open Questions

1. **Which of Pitfall A's two resolutions (role-rewrite action vs. inner-wrapper listbox) actually
   satisfies a real screen reader?**
   - What we know: the library's viewport hardcodes `role="region"`; there is no prop to change it.
   - What's unclear: whether a nested `role="listbox"` inside a `role="region"` ancestor reads
     correctly, or whether an action must rewrite the rendered `role` attribute post-mount (and
     whether that survives the library's own re-renders).
   - Recommendation: spike this first, with a real accessibility-tree inspection (or an
     `agent-browser`/screen-reader pass), before the row-list component's shape is locked in.

2. **Does the plan want the List hidden count to be a second `Store.List` call (A2, no new store
   code) or a new `Count`-based `internal/store` method (cheaper, but needs a `recallTransmitters`
   row + new tests)?**
   - What we know: both are D-02-compliant ("bounded... document the choice").
   - What's unclear: whether Sean's "keep it bounded" intent tolerates doubling a large `List`
     page's fetch cost, or expects the cheaper Count-based approach despite its larger diff.
   - Recommendation: default to the second-`Store.List`-call approach (zero new store surface,
     ships faster) unless a plan-review checkpoint surfaces a performance objection.

3. **Message-vs-scalar-fields shape for `RecallGateHiddenCount` (Claude's Discretion per
   CONTEXT.md) — final call belongs to the plan, not this research.**
   - What we know: both are additive and `buf breaking`-safe.
   - What's unclear: none — this is a pure style call, included here only so the plan does not
     re-research it.
   - Recommendation: one shared message type, reused on both responses (see Alternatives
     Considered).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node/pnpm toolchain | `ui/` build, new npm deps | ✓ (existing CI/dev setup; not re-probed this session — no change from prior phases) | — | — |
| Chromium (Playwright) | `vitest-browser` component tests (`*.browser.test.ts`) | Assumed ✓ (already the test harness for existing `*.browser.test.ts` files, e.g. `MemoryDetail.browser.test.ts`) | — | — |
| Chrome/chromedp | `internal/e2e/console_browser_test.go` (`task test:e2e`) | Assumed ✓ (existing e2e harness; `task test:strict` sets `ENGRAM_REQUIRE_BROWSER=1` to fail closed rather than skip) | — | Skips gracefully outside `test:strict` if genuinely unavailable, per the existing harness convention (not re-verified this session — no new e2e dependency introduced by this phase beyond what already runs) |

**Missing dependencies with no fallback:** none identified — this phase adds no new external
service dependency, only two npm packages and two shadcn-svelte component installs, all of which
install into the existing toolchain.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest 5.0.1, two projects: `node` (`*.test.ts`) and `browser` (`*.browser.test.ts`, `vitest-browser-svelte` + Playwright Chromium) — `[VERIFIED: ui/vite.config.ts:19-53]`, read this session |
| Config file | `ui/vite.config.ts` |
| Quick run command | `pnpm --dir ui vitest run --project node <file>` (pure-logic changes); `pnpm --dir ui vitest run --project browser <file>` (component changes) |
| Full suite command | `pnpm --dir ui test` (both projects) and `task test:go` (`go test ./...`); phase gate also includes `task test:e2e` (`go test ./internal/e2e/ -count=1 -v`) for the chromedp console e2e |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ENTRY-01 | UUID/short_id/text classification routes to the right RPC on every surface | unit | `pnpm --dir ui vitest run --project node src/lib/search/classify.test.ts` | ❌ Wave 0 |
| ENTRY-02 | Free text with no scope defaults to `cross_spine` | unit + browser | `classify.test.ts` (logic) + a `HeaderSearch.browser.test.ts` assertion on the outgoing request | ❌ Wave 0 |
| ENTRY-03 | Honest header text (hits/scopes/`scopes_truncated`/`scopes_unknown`/hidden count) and empty/error states | unit | `pnpm --dir ui vitest run --project node src/lib/search/recall-header.test.ts` | ❌ Wave 0 |
| ENTRY-04 | Palette issues a real `SearchMemories` call for a term absent from static labels; never "no matches" for a real hit | browser | `pnpm --dir ui vitest run --project browser src/lib/components/HeaderSearch.browser.test.ts` | ❌ Wave 0 |
| ENTRY-05 | Rejected search shows `field=<name> hint=<code>` from `.rawMessage`, never opaque | unit | `pnpm --dir ui vitest run --project node src/lib/errors/connect-error.test.ts` | ❌ Wave 0 |
| ENTRY-06 | URL state round-trip, debounce, `AbortSignal`, `keepPreviousData` (stale-response race test) | unit + browser | A race test firing two overlapping `queryFn` calls and asserting final state matches the later one (per PITFALLS.md Pitfall 2) | ❌ Wave 0 |
| ROW-01 | Dense one-line rows, ~30 visible, smooth at 1000 rows | browser | `pnpm --dir ui vitest run --project browser src/lib/components/ResultsList.browser.test.ts` | ❌ Wave 0 |
| ROW-02 | Hover/focus overlay card, no row-height change, no keyboard-selection mutation | browser | Same file; assert `aria-activedescendant` is unchanged by a `mousemove` over a different row | ❌ Wave 0 |
| ROW-03 | `j`/`k`/arrows traversal, `aria-activedescendant`, action keys | browser | Accessibility-tree assertion in `ResultsList.browser.test.ts` (see Pitfall A — write this test only after the role spike resolves) | ❌ Wave 0 |
| ROW-04 | `score` always, `relevance` only when reranker ran, visually distinct | browser | `ResultRow.browser.test.ts` with a fixture carrying/omitting `relevance` | ❌ Wave 0 |
| ROW-05 | Category/tags/window/state/scope filter chips round-trip through the URL | unit | Extend `queries.ts`-style parse/encode tests (`parseObserveParams`/`observeSearch` precedent) for the new fields | ❌ Wave 0 (extends existing pattern) |
| ROW-06 | Scope combobox with `ListScopes` counts | browser | `ScopeCombobox.browser.test.ts` | ❌ Wave 0 |
| ROW-07 | Detail pane stacked sections show every listed field, both id forms copyable | browser | `DetailPane.browser.test.ts` (extends/replaces existing `MemoryDetail.browser.test.ts`) | Partial — `MemoryDetail.browser.test.ts` exists today for the tabbed version |
| DSYS-01 | `engram-console-conventions` skill exists, cited by this phase's UI-SPEC | manual/doc check | N/A — grep `.claude/skills/engram-console-conventions/SKILL.md` exists and is referenced | ❌ Wave 0 |
| DSYS-02 | `engram-connect-client` skill exists, cited by this phase's UI-SPEC | manual/doc check | N/A — grep `.claude/skills/engram-connect-client/SKILL.md` exists and is referenced | ❌ Wave 0 |
| (Go-side) D-01/D-02/D-03 | Hidden count computed correctly, symmetric across Connect/MCP/CLI | Go unit + parity | `go test ./internal/server/... -run HiddenCount` (new); extend the MCP↔Connect parity pattern (`internal/server/connectapi_write_parity_test.go`) with a read-side row, or a sibling parity test if none exists for read RPCs | ❌ Wave 0 |
| Success criterion 2 (palette regression) | Palette never says "no matches" for a real hit | browser | Same as ENTRY-04 | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** the relevant `vitest run --project {node|browser} <file>` or
  `go test ./internal/server/... -run <Test>`.
- **Per wave merge:** `pnpm --dir ui test` + `task test:go`.
- **Phase gate:** `pnpm --dir ui test`, `task test:go`, and `task test:e2e` all green, plus
  `task lint`/`task fmt:check` (per CLAUDE.md) before `/gsd:verify-work`.

### Wave 0 Gaps

- [ ] `ui/src/lib/search/classify.ts` + `classify.test.ts` — the shared id/short_id/text/operator
      classifier (ENTRY-01/02, D-09's operator-token parsing).
- [ ] `ui/src/lib/search/recall-header.ts` + `recall-header.test.ts` — pure header-string formatter
      (ENTRY-03).
- [ ] `ui/src/lib/errors/connect-error.ts` + `connect-error.test.ts` — the `.rawMessage`/`.code`
      classifier (ENTRY-05, D-04; see Pitfall C).
- [ ] `internal/server/*_hiddencount_test.go` — Go unit tests for the diff/bucket helper (Pattern 1)
      and its wiring into `SearchMemoriesResponse`/`ListMemoriesResponse`/MCP result maps/CLI
      footers.
- [ ] A spike/prototype resolving Pitfall A (`role="region"` vs `role="listbox"`) before
      `ResultsList.svelte`'s architecture is finalized.
- [ ] `.claude/skills/engram-console-conventions/SKILL.md` and
      `.claude/skills/engram-connect-client/SKILL.md` (DSYS-01/DSYS-02) — do not exist yet; only
      `sketch-findings-engram` and `spike-findings-engram` are present under `.claude/skills/`.

## Security Domain

`security_enforcement` is absent from `.planning/config.json` — treated as enabled per default.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No (unchanged this phase) | Existing OIDC/session cookie flow untouched |
| V3 Session Management | No (unchanged this phase) | No new session surface |
| V4 Access Control | Yes — indirectly | The hidden-count computation MUST use the caller's own `Subject`/scope in both the gated and ungated `Store.Search`/`Store.List` calls (never a subject-less/operator-tier call) — the ungated call still passes through `s.ownerScopeFilter`/`s.ownerOrSharedCondition`, only the recall-gate `Include*` flags are relaxed. This is DEC-cgb's store-layer-only enforcement, unmodified: no new authz path, no handler-level check. |
| V5 Input Validation | Yes | Existing `argError`/`connectError` machinery unchanged; no new request field beyond what the CONTEXT.md D-01 additive response field requires (no new request field at all). |
| V6 Cryptography | No | Not applicable to this phase. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Hidden-count query silently widens what a caller can *learn about* (record existence via count) beyond their own read predicate | Information Disclosure | The ungated comparison call MUST still apply `ownerScopeFilter`/`ownerOrSharedCondition` — only the archived/superseded/scheduled `IsEmpty`/window conditions are lifted, never the owner/scope authz conditions. This is exactly D-03's "read predicate must still apply — only the recall-gate conditions are lifted" instruction; a handler that builds a second filter from scratch (rather than reusing `Store.Search`/`Store.List`'s own filter-building path unchanged) risks dropping the authz condition by omission. |
| A new proto field silently reused/misnumbered | Tampering (wire contract) | `buf breaking` (`breaking.use: [FILE]`, confirmed this session) gates every proto change in CI; append at the next free field number (`SearchMemoriesResponse` → 5, `ListMemoriesResponse` → 8), never reuse a `deprecated` field's number. |

## Sources

### Primary (HIGH confidence)

- Direct reads this session: `internal/store/store.go` (Memory struct :190-254, SearchOptions
  :1141-1192, Store.Search :1194-1329, SearchReranked :1354-1377, ListOptions :1508-1553, List
  :1746-1852, listFilter :1601-1620, activeWindowConditions :1098-1117), `internal/store/boundedread.go`
  (:255-300, summaryView excludes only content/citations), `internal/store/verdictstate.go`,
  `internal/store/schemaversion_recallgate_test.go` (:1-100, :340-510), `internal/server/connectapi.go`
  (:230-448), `internal/server/tools.go` (:1838-2100, :2790-2930), `internal/server/connecterror.go`
  (:1-105), `internal/server/argerror.go` (:80-129), `cmd/engram/memory_state.go`,
  `cmd/engram/client_common.go` (:342-356, renderCoverageFooter), `proto/engram/v1/engram.proto`
  (:100-197), `buf.yaml`, `ui/vite.config.ts`, `ui/src/lib/client.ts`, `ui/src/lib/queries.ts`,
  `ui/src/lib/resume.ts` (:1-70), `ui/src/lib/components/CommandPalette.svelte`,
  `ui/src/lib/components/ui/command/command.svelte`, `ui/src/lib/components/ui/command/command-dialog.svelte`,
  `ui/src/lib/components/ui/hover-card/hover-card.svelte`,
  `ui/src/lib/components/ui/resizable/resizable-pane-group.svelte`,
  `ui/src/routes/search/+page.svelte`, `Taskfile.yaml`, `internal/e2e/console_browser_test.go`,
  `ui/package.json`, `node_modules/.pnpm/*` listing (bits-ui@2.18.1, @tanstack+query-core@5.102.8,
  @tanstack+svelte-query@6.1.48), `node_modules/@connectrpc/connect/dist/cjs/connect-error.js`,
  `node_modules/.pnpm/@tanstack+query-core@5.102.8/.../utils.cjs` and `query.cjs`.
- `npm view @humanspeak/svelte-virtual-list version/peerDependencies/time.modified` and
  `npm view bits-ui version/time.modified` (live registry queries, this session).
- `gsd_run query package-legitimacy check --ecosystem npm @humanspeak/svelte-virtual-list bits-ui`
  (this session).
- Context7 `/humanspeak/svelte-virtual-list` (scroll/scrollToOffset API, props table, viewportLabel/
  role="region", onLoadMore/hasMore/loadMoreThreshold) and `/huntabyte/bits-ui` (Command
  shouldFilter, Popover customAnchor, LinkPreview controlled open + customAnchor) and
  `/svecosystem/paneforge` (collapse/expand imperative API, keyboardResizeBy percentage semantics,
  autoSaveId) — quoted directly from each project's own docs source, this session.
- `.planning/phases/02-recall-first-search/02-CONTEXT.md`, `.planning/REQUIREMENTS.md`,
  `.planning/STATE.md`, `.claude/skills/sketch-findings-engram/{SKILL.md,references/recall-surface.md,references/foundations.md}` — read this session.

### Secondary (MEDIUM confidence)

- `.planning/research/{SUMMARY,STACK,PITFALLS,FEATURES,ARCHITECTURE}.md` (2026-09-25, HIGH
  confidence per their own metadata; carried forward here rather than re-derived) — milestone-level
  research this phase builds on directly per the instructions not to redo it.

### Tertiary (LOW confidence)

- WebSearch results on TanStack Query v6 changelog specifics (no first-party v6-specific changelog
  page was found; superseded by the direct `node_modules` read above, which is the authoritative
  finding — the WebSearch results are retained here only as the negative-result trail, not as a
  claim source).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every version/API claim verified live against the registry, the installed
  `node_modules` tree, or Context7's first-party docs this session.
- Architecture (hidden-count mechanism): HIGH — grounded in direct reads of the exact
  `internal/store`/`internal/server` functions and line ranges that implement the recall gate today.
- Pitfalls: HIGH for the three new findings (A/B/C) — each is a direct read of an installed
  package's real behavior/docs, not an inference. MEDIUM for the five carried-forward PITFALLS.md
  items (already HIGH per that file's own research, re-cited here without re-verification this
  session since they were dated one day prior with file:line evidence).

**Research date:** 2026-09-26
**Valid until:** 2026-10-10 (14 days — the npm ecosystem/bits-ui version pins are fast-moving; the
Go-side store/proto findings are stable and do not expire on this timeline)

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ENTRY-01 | Type a UUID/short_id and land on that record, fetched by id, never sent as a query — same on every surface | Shared classifier module (Architecture Patterns, Recommended Project Structure); D-04 ambiguous-short_id handling (Pitfall C, connecterror.go findings) |
| ENTRY-02 | Free text with no scope defaults to `cross_spine`, scope as a facet | `effectiveSearchScope` (tools.go:1975-1992) already implements this contract server-side; client must default the `cross_spine` chip on |
| ENTRY-03 | Honest statement of what was searched, including `scopes_truncated`/`scopes_unknown` and hidden count; honest empty state | Pattern 1 (hidden count), existing `searched_scopes`/`scopes_truncated`/`scopes_unknown` precedent (connectapi.go:309-329, tools.go:1994-2078) |
| ENTRY-04 | Server-driven palette, `shouldFilter={false}`, never "no matches" for a real hit | Pattern 2, PITFALLS.md Pitfall 1, `command.svelte` restProps-passthrough finding |
| ENTRY-05 | Rejected search shows `field=<name> hint=<code>`, never opaque | Pitfall C, `argerror.go:99-101`/`connecterror.go` findings, `connect-error.ts` code example |
| ENTRY-06 | URL state, debounce, `AbortSignal`, `keepPreviousData` | `keepPreviousData`/query-core@5.102.8 finding (Summary, State of the Art, code example); PITFALLS.md Pitfalls 2/3 |
| ROW-01 | Dense virtualized rows, ~30 visible, smooth at 1000 rows | `@humanspeak/svelte-virtual-list` verified API (Standard Stack, Pattern discussion, Pitfall A) |
| ROW-02 | Hover/focus overlay, no height change, no keyboard-selection mutation | Pattern 3 (LinkPreview controlled hover card), PITFALLS.md Pitfall 4 |
| ROW-03 | `j`/`k`/arrows traversal, `aria-activedescendant`, action keys | Pitfall A (role conflict to resolve first), `scroll({index})` API |
| ROW-04 | `score` always, `relevance` when reranked, visually distinct | Existing `Memory.score`/`relevance` wire fields (unchanged this phase); rendering is pure UI work |
| ROW-05 | Filter chips round-trip through URL | Existing `queries.ts` `parseObserveParams`/`observeSearch` pattern to extend (Recommended Project Structure) |
| ROW-06 | Scope combobox with `ListScopes` counts | `Combobox` (STACK.md, unopinionated/no shouldFilter needed), shadcn-svelte install gap noted (Supporting stack table) |
| ROW-07 | Detail pane: stacked sections, every listed field, both id forms copyable | Pattern 4 (paneforge collapse/expand), D-14 supersedes tabs wording (State of the Art) |
| DSYS-01 | `engram-console-conventions` skill exists and is cited | Wave 0 Gaps — does not exist yet; content sourced from sketch findings + this research's patterns |
| DSYS-02 | `engram-connect-client` skill exists and is cited | Wave 0 Gaps — does not exist yet; content sourced from `client.ts`/`connect-error.ts`/CSRF conventions read this session |

</phase_requirements>
