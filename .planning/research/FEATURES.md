# Feature Research

**Domain:** operator console for a memory/knowledge store (recall-first search, curation, browse)
**Researched:** 2026-09-25
**Confidence:** HIGH for table-stakes UI mechanics (widely-documented, multi-source corroborated); MEDIUM for graph-view and NL-chip judgment calls (fewer independent sources, more editorializing); LOW nowhere — anything uncertain is flagged inline.

This research answers, for each of the milestone's six target capabilities, what is table
stakes, what is a differentiator worth the build cost, and what is an anti-feature to avoid —
grounded in how Raycast, Linear, Algolia DocSearch, GitHub code search, Notion, Obsidian,
Logseq, Roam, Tana, Kumu, Mem, Readwise, Gmail, Superhuman, and the Qdrant/Weaviate/Chroma
dashboards actually work today. Every claim below is sourced; `engram`-specific framing (what
already exists, what's a proto gap) is drawn from `.planning/PROJECT.md` and
`.planning/notes/console-overhaul-exploration.md`.

## Feature Landscape

### 1. Unified search entry (id / short_id / prose, facets, keyboard-first, honest empty/error states)

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Global keyboard shortcut opens focused search (⌘K pattern) | Universal across Raycast, Linear, Notion, GitHub, Vercel since ~2020; users carry the muscle memory between products and a missing shortcut reads as dated | LOW | engram already has a palette; the fix is behavioral (server-driven, not client-filtered), not new UI chrome |
| Arrow-key / j-k navigation through results, Enter to act, Esc to dismiss with focus returned to trigger | WAI-ARIA APG combobox/listbox pattern; every reviewed product (Raycast, Linear, Algolia) treats keyboard nav as non-negotiable, not a power-user extra | LOW–MEDIUM | Already decided in the exploration note: DOM focus stays on the container, `aria-activedescendant` points at the active row |
| Honest, specific empty state naming what was searched and why nothing matched | Raycast's own design docs literally warn against a "blank" empty state reading as broken vs. "No results for 'xyz'"; Algolia's Autocomplete v1 ships "Query Suggestions" specifically to avoid a dead-end no-results screen | LOW | This is exactly the "entry point must not lie" principle already adopted for this milestone — e.g. "no memories match `github` in any scope you can read" |
| Fuzzy/typo-tolerant matching with match highlighting | Algolia DocSearch and Raycast both foreground typo tolerance + highlighted matched substrings as the reason results feel "smart" | LOW (server) / MEDIUM (highlight rendering) | engram's search is already semantic (embedding-based), which subsumes literal typo tolerance for prose; short_id/id lookup needs its own exact-match path, not fuzzy |
| Debounced, cancelable async queries with a lightweight loading state, never a blocking spinner | Universal (Raycast, Algolia, SaaSFrame's CMD+K analysis of 5,000+ interfaces); sub-200ms perceived latency is the bar cited repeatedly | LOW | TanStack Query 6 + `placeholderData: keepPreviousData` (carried into the exploration note's dispositions) avoids flash-to-empty between keystrokes |
| Result grouping/sectioning once item count grows past ~20 | Raycast (Favorites section, root search sections), Algolia's federated grouping by content type | MEDIUM | For engram: group by category or by "exact id match" vs "search results", not by static menu category |
| Filters reflected in a shareable/persistent state (URL or view) | Linear's filters "reflected in the browser URL"; Algolia's Autocomplete v1 "Synchronized search state in the URL" | LOW–MEDIUM | Enables sharing a curation query (`?scope=...&tags=...`) — cheap given SvelteKit's URL-driven routing |
| Query resolution that tries id/short_id first, falls through to prose search, and says which path it took | Not one product does exactly this (id-vs-prose is engram-specific), but the *pattern* — GitHub's `is:`/`in:` qualifiers, Rover's building-block chips, Linear's `@`-mention quick filters — all resolve a typed token into a specific, named interpretation rather than guessing silently | MEDIUM | This is the #1 gap named in the exploration note (bug: "Searching by an id an agent mentioned errors"); the honest-feedback rule applies here directly — report "resolved as short_id, found 1 record" vs "searched as free text across 3 scopes" |

Sources: [Raycast Manual — Search Bar](https://manual.raycast.com/search-bar); [Raycast: The Art of Productivity UI](https://blakecrosley.com/guides/design/raycast); [DesignSystems.one — Search & command palette pattern](https://www.designsystems.one/design-systems/patterns/search-and-command); [137Foundry — command palette design](https://137foundry.com/articles/command-palette-interface-users-actually-reach-for); [Algolia — Take doc search to new heights with Autocomplete](https://www.algolia.com/blog/ux/taking-documentation-search-to-new-heights-with-algolia-and-autocomplete); [Algolia DocSearch — How It Works](https://algolia-docsearch.mintlify.app/concepts/how-it-works); [SaaSFrame — The Rise of CMD+K](https://www.saasframe.io/blog/the-rise-of-cmd-k-why-every-saas-needs-a-search-modal-in-2026); [Obvio Studio — Linear Searching](https://obviostudio.substack.com/p/linear-searching); [Rover docs — search terminal / CMD-K](https://docs.rover.inc/core/cmd-k).

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Faceted filters as first-class UI (category OR, tags AND, time window, derived state, scope) surfaced *before* the user types, not buried in a menu | Algolia DocSearch v5 added "facet filtering... breadcrumbs, and badges that let readers refine results" as a named upgrade over a flat result list; GitHub code search's qualifier vocabulary (`repo:`, `language:`, `is:`) is the reason its search reads as precise rather than best-effort | MEDIUM | engram's server already accepts all these facets (tags AND, categories OR, created_after/before, cross_spine, include_archived/superseded/scheduled) — this is a pure UI-exposure differentiator, "roughly eight capabilities are API-reachable and UI-invisible" per the exploration note |
| Boolean/qualifier syntax in the query itself (`tag:foo -tag:bar`, `is:archived`) as a power-user escape hatch alongside the facet UI | GitHub code search's qualifier language (`repo:`, `language:`, `is:`, boolean `AND`/`OR`/`NOT`, quoted exact match) is the reference implementation for "search that also reads like a query language"; Obsidian's graph-filter search box reuses the same query syntax as note search | MEDIUM–HIGH | Optional; a differentiator only if it doesn't fork behavior from the facet chips — best done by having facet chips *emit* the same underlying query the box would parse, not two independent code paths |
| `score`/`relevance` shown as a legible, comparable signal, not a raw float | See §2 below — this is the single highest-leverage differentiator, because engram already computes both signals server-side and today renders neither | MEDIUM | Directly named in the milestone's target features |
| Cross-spine-by-default search with an honest scope-coverage report | Not common elsewhere (most competitors are single-tenant/single-graph), but exactly matches engram's existing `cross_spine`/`searched_scopes`/`scopes_truncated`/`scopes_unknown` contract (CLAUDE.md memory contract) | LOW (already server-side) | This is engram's actual differentiator vs. every consumer product surveyed — none of them have a multi-scope isolation model to be honest about |

Sources: [Algolia — DocSearch UI library v5](https://www.algolia.com/blog/ai/docsearch-ui-library-v5); [GitHub Docs — Understanding GitHub Code Search syntax](https://docs.github.com/en/search-github/github-code-search/understanding-github-code-search-syntax); [GitHub Docs — Searching code](https://docs.github.com/en/search-github/searching-on-github/searching-code).

#### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| Client-side filtering of a server-authoritative list (bits-ui `Command` default `shouldFilter`) | Feels "instant" and is the library default, zero server round-trip | This is the literal root cause of the milestone's motivating bug ("Palette says 'no matches' for `github`") — the UI silently stops asking the server and starts lying about what it searched | `Command.Root shouldFilter={false}`, drive the list from `SearchMemories`, already decided in the exploration note |
| A command palette that grows to hundreds of entries because "we can expose everything" | Every server capability *should* be reachable somewhere | 137Foundry's design guidance explicitly names this failure mode: "A command palette with 400 entries... degrades the search experience for everyone" | Curate a small default action set (search, create, recent); put exhaustive capability access behind a secondary/advanced surface, not the default keystroke-to-result path |
| Silent fallback / guessed interpretation when a typed token is ambiguous between id, short_id, and prose | Feels smoother than asking or reporting | Violates "the entry point must not lie" directly — an agent pasting a short_id that happens to look like a word gets a confusing prose-search result with no explanation | Always report which resolution path was taken, even when it's the obviously-right one; ambiguity should say "tried as short_id (no match), searched as text" |
| Pagination of search results | Standard on most content sites | Algolia's own documentation team rejected it for DocSearch: "the best results should always be on the first page... pagination is taking them on an unsatisfying path of less relevant results" — for a recall tool, the honest move is to say "12 hits" and show them, not paginate a ranked list | A single bounded, honestly-labeled result set (a page-size ceiling already exists server-side via the `limit`/1000 max from the Bounded Reads milestone) rather than page 2/3/4 of decreasingly relevant hits |

Sources: as above (Raycast, 137Foundry, Algolia, exploration note).

---

### 2. Result presentation with a trustworthy score/relevance signal

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| One visible per-hit signal, always in the same place, always explained on hover/tooltip | Weaviate's hybrid search literally ships an `explainScore` metadata field alongside `score` for exactly this reason — a bare float is not trustworthy without provenance | LOW | engram already has `score` (cosine, always on) and `relevance` (Jev P(relevant), opt-in) per CLAUDE.md; today neither renders in the UI |
| Results ordered by the signal shown, never a mismatch between visual order and the number displayed | Qdrant's raw API contract is `"result": [{"id":10,"score":0.81}, ...]` ordered by score — the invariant is definitional in every vector DB surveyed (Qdrant, Weaviate) | LOW | Trivial to hold since the server already orders correctly; the risk is purely a rendering bug (e.g. re-sorting client-side after facet changes) |
| A qualitative label alongside (or instead of) the raw number for non-expert users | Notion's "Best Matches" default sort is described only in relative terms ("Pages that have been recently edited show up higher... page titles are more likely to show up than page contents"), not exposed as a number at all — the product philosophy is "trust the ranking, don't force users to interpret a float" | LOW–MEDIUM | For engram: a coarse bucket (e.g. "strong match" / "related" / "loose") derived from score thresholds is more legible than "0.812" to an operator who isn't a Qdrant user |

Sources: [Weaviate — Hybrid search, explain score](https://docs.weaviate.io/weaviate/search/hybrid); [Qdrant — Search](https://qdrant.tech/documentation/search/search/); [Notion Help — Search](https://www.notion.com/help/search).

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Two distinct signals shown side by side when both exist (`score` always, `relevance` when Jev is on), visually distinguished so a user never confuses "how close in vector space" with "how likely this answers your query" | No single competitor product needs this distinction (none of Raycast/Linear/Notion expose a raw vector score to end users at all; Weaviate/Qdrant dashboards expose scores only to developers, not curators) — this is a genuinely novel UI problem for engram because it straddles "developer tool" and "curation tool" | MEDIUM | Directly named in target features: "`score` and `relevance` rendered". Consider: `score` as a small dim numeric badge (always present, low-key), `relevance` as a colored/labeled chip only when the reranker actually ran (never fabricate a value on fallback — CLAUDE.md: "a reranker failure never fails a search" and falls back to lexical order silently) |
| `explain`-style disclosure ("why did this rank here") on demand, not by default | Weaviate's `explainScore` and Algolia's ranking-tie-break documentation both treat this as an expert/debug affordance, not a default-visible column | MEDIUM–HIGH | Good fit for the detail pane's Meta tab rather than the row — keeps the dense row honest without cluttering it |
| Honest "no relevance signal available" state when Jev is off or fell back, rather than omitting the field silently | Directly required by the standing constraint that a reranker failure/off-state must be visible-if-relevant, not silently absent — the "entry point must not lie" principle extends to result metadata | LOW | Was explicitly declined at the *response* level in the prior milestone (D-06: "the response-level 'nothing relevant' flag was declined") — that decision was about a different signal (nothing-relevant), not about per-row relevance-availability; worth re-confirming scope during REQUIREMENTS, not assuming reopened |

Sources: [Weaviate — Hybrid search](https://docs.weaviate.io/weaviate/search/hybrid); CLAUDE.md memory contract (search_memory score/relevance); `.planning/PROJECT.md` (2026-09-22.01 milestone, Jev reranker fallback behavior).

#### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| Showing a fabricated or estimated `relevance` when the reranker didn't run | "Consistency" — always show the same columns | Directly violates the standing advisory-only constraint (`internal/decide` is provider-neutral, advisory, and a reranker failure must fall back to exactly the lexical order, never a fake number) | Omit the column/badge entirely when `relevance` is absent on the wire; never backfill from `score` |
| Precision-heavy numeric display (`0.8142857`) as the primary signal | Looks rigorous | Nielsen/Algolia-style research on end-user-facing metrics consistently shows raw floats don't build trust for non-technical users — an operator curating junk doesn't benefit from 6 significant figures | Round to 2 decimals or a qualitative bucket; keep full precision available in a tooltip/Meta tab for the rare debugging need |

---

### 3. Curation workflows (supersede/merge, archive/restore, hidden-right-now views, rules)

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Merge/supersede shows a field-by-field or side-by-side preview *before* committing | Universal across every duplicate-merge UI surveyed: Reevo CRM's guided wizard ("Review and resolve field conflicts... Preview column"), Twenty CRM's merge dialog, Zotero's "select a master, then select alternative versions of mismatched fields" | MEDIUM | engram's `supersede_memory` already takes a target *set* server-side with history preserved — the console gap is purely the preview/confirm UI, not new server logic |
| A merge/supersede is reversible-by-history, not destructive | Zotero explicitly documents: "You should always resolve duplicate items by merging them, rather than deleting... Merges retain all of the collections and tags"; engram's own contract is stronger still (soft-hidden, fetchable by id, forward+backward links) | LOW (server already does this) | UI should visualize the chain (predecessor → successor), not just hide the old record |
| Archive is reversible, distinct from delete, and named as such | Reevo: "The duplicate record is archived, not deleted... history is preserved" as an explicit design callout distinguishing archive from destructive merge; engram's own contract: `archive`/`restore` is "always reversible, and never a delete" | LOW (server already does this via `spine-review archive/restore`; console needs the write RPCs) | This is exactly the `ArchiveMemory`/`RestoreMemory` RPC gap named in the milestone |
| A dedicated view for records currently hidden by a time window (scheduled-not-yet-visible, expired) | Not common in consumer note apps (none of Notion/Obsidian/Roam have temporal visibility windows), but the *pattern* — "surface what the normal view is suppressing" — matches Roam's Unlinked References pane ("mentions... that haven't been formally turned into a link... a safety net for memory") and Readwise's Themed Reviews (surface things you'd otherwise never see again) | LOW (server: `list_scheduled` already exists; console needs `ListScheduled` RPC + a route) | Named directly in target features |
| Rules rendered as a visually distinct, always-shared kind — never mixed into the same list/row treatment as ordinary memories | No consumer product has an exact analogue (closest: Notion's page vs. database-property distinction, or Linear's saved/shared views vs. personal filters), but the design principle — a normative, user-blessed kind must not look optionally-owned — is unambiguous from engram's own contract (`store_rule` is user-blessed only, `set_visibility` is rejected for rules) | LOW–MEDIUM | `ListRules` RPC + a distinct list treatment (index-first, full text on demand — matches CLAUDE.md's "progressive-disclosure index" description) |

Sources: [Reevo — Merging Records](https://help.reevo.ai/CRM-configurations/Merging-Records); [Zotero — Duplicate Detection](https://www.zotero.org/support/duplicate_detection); [Twenty CRM — feat: merge records PR](https://github.com/twentyhq/twenty/pull/13436); [Roam — Unlinked References](https://app.studyraid.com/en/read/93166/4230354/analyzing-the-unlinked-references-pane); [Readwise — Themed Reviews](https://docs.readwise.io/readwise/guides/themed-reviews); CLAUDE.md memory contract (supersession, archive/restore, rules).

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Multi-target merge (collapse N duplicates to one survivor in one operation) with per-field conflict resolution | Most merge UIs (Reevo, Zotero, Twenty) are pairwise; engram's server already supports a *set* of targets (`supersedes` takes a set — v0.13.x "Multi-target merge supersession") | MEDIUM | The console differentiator is exposing what competitors do one pair at a time as a genuine N-way operation, matching the server contract already shipped |
| Derived-state badges rendered consistently across every row and detail view, in the documented canonical order (`archived` › `superseded` › `expired` › `scheduled`) | This ordering-and-suppression rule (expired suppresses scheduled) is engram-specific and already implemented in `MemoryRow.svelte` per the exploration note — the differentiator is *not losing this consistency* when new curation surfaces (rules list, related-graph, tag cloud) are added | LOW (extend existing derivation, don't reinvent) | Risk to flag for REQUIREMENTS: any new list/graph view must reuse the existing `memoryStateWords` derivation, not grow a second one |
| A "what's hidden right now and why" cross-cutting view (one place that shows archived + superseded + expired + scheduled together, each labeled) rather than one screen per state | No competitor has this because none have four independent orthogonal hidden-states; it's a genuine product opportunity given engram's specific contract | MEDIUM | Useful for the operator-curation audience specifically — matches the milestone's build-order (curation is audience #2) |

Sources: `.planning/PROJECT.md` (v0.13.x "Multi-target merge supersession"); CLAUDE.md (derived-state ordering rule).

#### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| A one-click "undo merge" | Feels safer | Every merge UI surveyed (Reevo, Zotero) explicitly documents merge as *irreversible at the UI layer* specifically because the underlying operation is a real data consolidation — promising undo you can't deliver is worse than being honest that it can't be undone | Lean on what engram actually guarantees: supersession preserves history and predecessors stay fetchable by id — sell *that* honestly ("nothing is deleted, the old record is just soft-hidden") rather than promising a literal undo button |
| Auto-merge / auto-archive suggestions applied without confirmation | "Smart" cleanup sounds efficient | Directly contradicts the standing constraint that Jev verdicts are advisory-only and "structurally unable to mutate" — any curation action must stay a human (or skill-mediated, consent-gated) decision | Surface Jev's `same_subject`/`needs_review` verdict as a suggestion chip next to a candidate pair; the merge/archive action itself is always a separate, explicit user click |
| Treating rules like memories with a visibility toggle | Consistency of UI affordances | engram's contract explicitly rejects `set_visibility` for rules — a UI that offers a share/private toggle on a rule row would be offering an action the server will reject, producing a confusing error instead of an absent control | Render rules with no visibility control at all; the always-shared nature is a property of the kind, not a per-record setting |

---

### 4. Visual related-items graph

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| A **local/focused** graph (one record + its neighborhood), not just a global hairball | This is the single most consistent finding across every graph-view source: "The local graph is a query. The global graph is decorative" (Shadow.do); Obsidian's own docs and three independent community write-ups converge on exactly this; Roam's "Graph Overview" redesign added degrees-in/degrees-out sliders specifically to make exploration *local-first* | MEDIUM | engram's target feature is `RelatedMemories(id)` — i.e., local-graph-only by design, which is the *correct* choice per this research, not a scope-cut |
| Depth/degree control (1 hop vs 2 hops) | Obsidian ("depth 2 is often the sweet spot"), Roam ("degrees out" / "degrees in" sliders), Kumu ("out 1", "out 2" focus syntax) all converge on adjustable-but-bounded depth as the core interaction | LOW–MEDIUM | A simple depth toggle (1/2) is enough; unbounded depth on a real vector-neighbor graph risks the "dense hairball" failure mode named repeatedly |
| Click-to-navigate / click-to-recenter on a node | Universal (Obsidian: "Clicking on any node will instantly navigate you to that page" — quoted for Roam but true of every graph reviewed) | LOW | Standard graph library behavior |
| Visual distinction between edge *types* (not all edges look the same) | Logseq's graph builder explicitly models distinct edge classes (`:block/refs`, `:block/tags`, `:block/parent`, class-extension edges) and a long-running Logseq community thread argues block-level edges are invisible/underwhelming *precisely because* they're not visually distinguished from page-level edges — the lesson is: undifferentiated edges make a graph look richer than it is and then disappoint | MEDIUM | engram has 4 real edge types (supersession chain, shared tags, shared citations, vector neighbours) per the exploration note — each needs a distinct visual encoding (color/dash/arrow), or the graph will look like Logseq's "barely any connections" complaint |
| Honest "few or no connections" empty state on the graph itself, matching the drift-detection principle used everywhere else | Obsidian's own docs treat "Orphans" (nodes with zero links) as a first-class, filterable concept, not a bug | LOW | A record with no supersession/shared-tag/shared-citation/vector-neighbor edges should render as "no related memories found" on the graph, not an empty canvas that looks broken |

Sources: [Shadow.do — Obsidian Graph View for AI Meeting Notes](https://www.shadow.do/blog/obsidian-graph-view-ai-meeting-notes-2026); [Obsidian Help — Graph view](https://github.com/obsidianmd/obsidian-help/blob/5fb785ac/en/Plugins/Graph%20view.md); [Dan Holloran — Making Obsidian's Graph View Actually Useful](https://danholloran.me/posts/making-obsidians-graph-view-actually-useful); [Roam Research — New Graph Overview (YouTube)](https://www.youtube.com/watch?v=F-xnDulxvaw); [studyraid — Interpreting the Local Graph View on a Page](https://app.studyraid.com/en/read/44365/2063264/interpreting-the-local-graph-view-on-a-page); [Kumu — Focus](https://docs.kumu.io/guides/focus); [Logseq — agent-guide/graph-view](https://github.com/logseq/logseq/blob/3de7c751/docs/agent-guide/graph-view/001-graph-view.md); [Logseq forum — block references on the graphview](https://discuss.logseq.com/t/the-option-to-show-block-references-on-the-graphview/3814).

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Filter-by-edge-kind toggle (show/hide supersession vs shared-tag vs shared-citation vs vector-neighbor edges independently) | Kumu's entire "focus + filter by connection type" model (`connection-type = "donation"` selectors, `focus-direction: in/out/all`) exists precisely because mixed edge types without filters produce unreadable graphs; Obsidian's Filters panel (tags on/off, path filters) is the same idea applied to notes | MEDIUM | Directly named as wanted in the exploration note; should reuse the same category-accent/tag-chip visual language already established for rows, so the graph doesn't invent a second color system |
| Jev's `same_subject` verdict rendered as an edge label/style distinguishing "same fact" from "merely similar" | This is engram-specific — no competitor has an advisory typed-decision layer to draw on — but it directly extends the already-shipped `spine-review consolidate` verdict object (`relation`, probability map, `needs_review`) into a visual affordance | MEDIUM–HIGH | Must stay advisory-only and clearly labeled (e.g. a dashed/uncertain edge style below the 0.9 `needs_review` threshold) — never implied as ground truth, consistent with the standing "advisory, off by default" constraint |
| Hover-to-highlight-neighborhood, dim-the-rest (vs. click-to-navigate-away) | Roam's local graph: "Hovering... will highlight it and its direct connections, dimming everything else... fantastic for isolating a specific relationship... without distraction" — this preserves context better than immediately navigating | LOW–MEDIUM | Complements the milestone's existing hover-to-expand row pattern — same interaction vocabulary (hover reveals more, click commits) applied to the graph |

Sources: [Kumu — Selectors / Focus / Filter](https://docs.kumu.io/guides/selectors); [studyraid — Roam local graph hover](https://app.studyraid.com/en/read/44365/2063264/interpreting-the-local-graph-view-on-a-page); `.planning/PROJECT.md` (spine-review consolidate verdict).

#### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| A global, vault/store-wide graph as the primary or default view | "Show everything" feels comprehensive | Every single source converged independently on this being the *decorative* failure mode: "the global graph is decorative... the feature most people open once, stare at, and then never touch again"; at engram's real scale (potentially thousands of records across scopes) it would be the "dense hairball... nearly impossible to read" Obsidian's own docs warn about | Ship only the local/focused `RelatedMemories(id)` view the milestone already scoped; if a global overview is ever wanted later, treat it explicitly as a health-check/audit tool (Obsidian's actual successful use case for the global graph — finding orphans, not navigating) rather than a navigation surface |
| Physics/force-layout tuning exposed to end users (repel force, link distance sliders) | Obsidian ships this | It's explicitly named as a "procrastination trap disguised as PKM work" by one of the sourced critiques, and adds real implementation surface for a feature this milestone doesn't need to nail aesthetically | A fixed, sane default layout; no exposed physics controls |
| Treating the graph as a navigation replacement for the search/list view | The graph looks impressive in a demo | Dan Holloran's critique and Shadow.do's practical guide both land on the same conclusion: graphs are for *understanding a neighborhood*, lists/search are for *finding a specific thing* — conflating the two makes both worse | Keep the graph as a secondary, detail-pane-adjacent view; recall (the developer audience, built first) stays list/search-driven per the milestone's own audience ordering |

---

### 5. Tag clouds and scope/tag autocomplete with counts

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Autocomplete list capped to a manageable size (≤10 desktop, 4–8 mobile) | Baymard's e-commerce autocomplete research (80% of sites have autocomplete, only 19% implement it fully correctly) names list-length as best-practice #1 | LOW | engram's tag/scope autocomplete should never dump the full tag/scope universe into a dropdown |
| Highlighted matched substring + highlighted active/keyboard-selected item | Baymard best practices #3 and #6; matches the same pattern already required for the unified search entry (§1) | LOW | Reuse whatever highlight component the search box already needs |
| Counts shown next to each tag/scope option, not just the label | This is the entire *point* of a count-annotated autocomplete vs. a bare list — GitHub's own qualifier UI (`stars:>1000`, `topics:>=5`) exists because a bare label with no signal about size is not enough to decide what to filter by; Algolia's faceting model is built around returning facet *values with counts* precisely so users can gauge before committing | LOW–MEDIUM (server: Qdrant Facet API, already a dependency per the exploration note) | Directly named target feature: `ListTags(scope)` with counts |
| Keyboard navigable, arrow-key selectable, Enter to commit | Same combobox pattern as §1 | LOW | — |

Sources: [Baymard — Autocomplete design best practices](https://baymard.com/research-articles/autocomplete-design); [GitHub Docs — search qualifiers with numeric comparators](https://docs.github.com/en/enterprise-cloud@latest/search-github/getting-started-with-searching-on-github/understanding-the-search-syntax); [Algolia — Relevance overview / facets](https://www.algolia.com/doc/guides/managing-results/relevance-overview).

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| A genuinely useful tag *cloud* (size-by-count) rather than a plain alphabetical list, used for browsing rather than only filtering | The classic tag-cloud pattern (ui-patterns.com: "Use when your website has more than 10-20 different tags... size represents popularity") is real for the newcomer-browsing audience (audience #3, built third) — someone learning what the store holds benefits from an at-a-glance popularity view that a plain filter list doesn't give | LOW–MEDIUM (once `ListTags` counts exist server-side, this is pure rendering) | Must be paired with the anti-feature caveat below — Nielsen Norman Group's explicit critique (next section) is real and should shape the execution, not veto the feature |
| Clicking a tag in the cloud both filters the result list *and* highlights that tag's presence across other surfaces (e.g. the graph, if a "shared tags" edge is visible) | Obsidian's forum wishlist explicitly asked for exactly this cross-surface link ("clicking on a tag in the tag pane would select on the graph all nodes with that tag") as a still-unshipped feature request — implementing it for engram (where tag cloud, graph, and search are all first-class rather than plugin-dependent) is a genuine differentiator over what Obsidian itself has shipped | MEDIUM | Nice-to-have; do not let this couple the tag cloud's ship date to the graph's |

Sources: [ui-patterns.com — Tag Cloud pattern](https://ui-patterns.com/patterns/TagCloud); [Obsidian forum — Design talk about the Graph View](https://forum.obsidian.md/t/design-talk-about-the-graph-view/22594).

#### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| A tag cloud as the *primary* or only way to filter by tag | Looks visually rich, "shows off" the data | Jakob Nielsen's Nielsen Norman Group critique is blunt and specific: "tag clouds are overused... use screen space inefficiently, and many users don't know how to use them" — most normal users don't parse font-size-as-weight intuitively | Tag cloud is a *browse/discover* affordance for the newcomer audience; the actual filter mechanism should be the same autocomplete-with-counts chips used everywhere else in the search UI, with the cloud as an optional secondary entry point into that same filter |
| Font-size scaling with no floor/ceiling, so a single dominant tag visually drowns everything else | "Faithful" to raw counts | The academic critique of tag clouds (Hassan-Montero & Herrero-Solana) specifically identifies this: frequency-only weighting produces "high semantic density" where "very few different topics... dominate the whole cloud" | Cap the size range (e.g. Telerik's `MinFontSize`/`MaxFontSize` pattern) and/or use logarithmic rather than linear scaling |

Sources: [Nielsen Norman Group — Tag Cloud Examples](https://www.nngroup.com/articles/tag-cloud-examples/); [Hassan-Montero & Herrero-Solana — Improving Tag-Clouds as Visual IR Interfaces](https://doi.org/10.48550/arxiv.2401.04947); [Telerik RadTagCloud docs](https://www.telerik.com/products/aspnet-ajax/documentation/controls/tagcloud/overview).

---

### 6. Natural-language query understanding as removable, confirmed filter chips

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| The NL interpretation renders as *discrete, removable* chips mapped to real filter fields — never opaque prose the user can't inspect or undo | Gmail's search chips are the canonical version: "clickable chips appear... personalized and adapt to your term" (From:, Any time:, Has attachment:) — each maps 1:1 to a real query operator and can be toggled off; Linear's AI Filters explicitly resolve to the *same* structured filter menu items a manual filter would produce, not a separate freeform state | MEDIUM | This is exactly the milestone's stated design: "natural-language query understanding rendered as removable, user-confirmed filter chips" |
| A visible, honest signal when the NL interpretation is uncertain or wrong, with an easy path to fix it by rephrasing | Linear's own changelog for AI Filters states this plainly: "AI is powerful and at times can be unpredictable. If you're not getting the results you expect, try rephrasing your query" — shipped as user-facing guidance, not hidden | LOW | Matches the standing constraint that typed decisions stay advisory; the chip UI should make "this is a guess, confirm or edit it" the default framing, not "this is now applied" |
| User confirms before the interpretation becomes an actual applied filter (vs. auto-applying silently) | Rover's "natural language translator... converts it into a structured set of building blocks" that the user then sees as inline chips *before* running the search — translation and execution are two separate steps | MEDIUM | Directly matches the milestone's "user-confirmed" requirement; do not auto-execute the parsed filters the instant NL parsing returns |

Sources: [Gmail Help — search chips](https://support.google.com/mail/answer/6593); [9to5google — Gmail search chips launch](https://9to5google.com/2020/02/19/gmail-search-chips/); [Linear Docs — Filters / Filter with AI](https://linear.app/docs/filters); [Linear Changelog — AI Filters](https://linear.app/changelog/2023-06-01-ai-filters); [Rover docs — search terminal](https://docs.rover.inc/core/cmd-k).

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Chaining/refining multiple NL requests on top of each other, each producing/adjusting chips incrementally | Linear's changelog: "You can chain multiple requests together to quickly find the issues you need" — treats NL parsing as conversational refinement, not one-shot | MEDIUM–HIGH | Nice-to-have; the one-shot version (parse once, confirm, apply) already satisfies the milestone's stated scope and is much lower risk |
| Provider-neutral, fully off-by-default NL parsing that degrades to "just search the literal text" with zero UI difference when disabled | No competitor needs this framing (they all ship one committed AI vendor), but it is engram's actual standing constraint — the typed-decision layer must stay swappable and advisory, and "the default render, default config and default ranking are byte-identical to before" when the capability is off | LOW (governance, not UI complexity) | This is a constraint carried over from the 2026-09-22.01 milestone, not a new invention — treat it as non-negotiable in REQUIREMENTS |

Sources: [Linear Changelog — AI Filters](https://linear.app/changelog/2023-06-01-ai-filters); `.planning/PROJECT.md` (2026-09-22.01 standing constraints).

#### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| Auto-applying the parsed filters the moment NL parsing returns, with no confirm step | Feels faster, fewer clicks | Contradicts both the milestone's explicit "user-confirmed" requirement and the general principle (Rover, Gmail) that translation and execution should be separably inspectable steps — an agent-adjacent tool where "the entry point must not lie" cannot let an LLM silently decide what got searched | Always render chips first in an unapplied/pending state; require an explicit "apply" (or edit-then-apply) action |
| Free-text NL box replacing the structured facet UI rather than augmenting it | Simpler to build one text box | Every real-world example (Gmail, Linear, Rover) keeps NL as an *alternate entry path into the same structured filters*, never as a replacement — removing the structured UI would also remove the "removable chip" affordance the milestone specifically wants | NL parsing is a shortcut that produces the same chip objects the manual facet UI produces; both paths converge on one filter-state representation |
| Presenting the LLM's confidence as a precise percentage on each chip | Looks rigorous, mirrors the `relevance` score idea from §2 | Unlike `search_memory`'s `relevance` (a real per-hit probability from a specific typed-decision call), an NL-to-filter parse is a much blunter instrument; a false-precision confidence number here risks the same failure Notion/Algolia avoided by *not* surfacing raw floats to end users (§2), but worse because there's less basis for the number to mean anything specific | A binary/coarse signal is enough: "parsed" vs. "uncertain — check these filters", not `73.2%` |

---

## Feature Dependencies

```
[Score/relevance rendering §2]
    └──requires──> [Server already returns score+relevance] (done — CLAUDE.md contract)
    └──enhances──> [Unified search entry §1] (the signal only matters once results render)

[Related-memories graph §4]
    └──requires──> [RelatedMemories(id) RPC] (named gap)
    └──requires──> [Derived-state badge consistency] (existing MemoryRow.svelte derivation — reuse, don't fork)
    └──enhances──> [Curation workflows §3] (Jev same_subject verdict as an edge label)

[Tag cloud §5]
    └──requires──> [ListTags(scope) RPC with counts] (named gap; Qdrant Facet API, already a dependency)
    └──enhances──> [Scope/tag autocomplete §5] (shared counts source)

[Curation: supersede/merge §3]
    └──requires──> [SupersedeMemory RPC] (named gap; reuses MCP supersede_memory server logic)
    └──requires──> [Multi-target merge preview UI] (server already supports N-way; UI must not regress to pairwise)

[Curation: archive/restore §3]
    └──requires──> [ArchiveMemory, RestoreMemory RPCs] (named gap; reuses spine-review archive/restore store paths)

[Curation: rules / scheduled views §3]
    └──requires──> [ListRules, ListScheduled RPCs] (named gap; wrap existing store reads)
    └──conflicts-with──> [Treating rules like ordinary memories] (no visibility toggle — server rejects it)

[NL query understanding §6]
    └──requires──> [Facet/filter chip UI already built] (§1's facet exposure must ship first — NL chips are a shortcut into the same state, not a separate feature)
    └──requires──> [internal/decide advisory contract] (provider-neutral, off-by-default — already shipped in 2026-09-22.01)

[id/short_id/prose resolution §1]
    └──enhances──> [Honest empty/error states §1] (resolution path is part of what gets reported)
```

### Dependency Notes

- **Score/relevance rendering requires nothing new server-side** — this is the single lowest-risk, highest-visibility item in the whole milestone, since `score` and `relevance` are already on the wire (CLAUDE.md memory contract) and simply unrendered. It should land early to prove the "honest signal" pattern the rest of the milestone reuses (graph edge confidence, NL chip confidence).
- **The related-memories graph and the tag cloud both depend on new read RPCs** (`RelatedMemories`, `ListTags`) that are individually small (both explicitly scoped as "small; reuses existing... sub-query" / "small; Qdrant Facet API already a dep" in the exploration note) — but the *UI* built on top of them is the real cost, so sequencing them after the search/curation basics (which reuse existing RPCs) reduces risk.
- **NL query understanding conflicts with shipping it before the facet chip UI exists** — chips-from-NL only make sense if chips-from-manual-filtering already have a defined shape; building NL parsing first would create a second, divergent filter representation.
- **Rules-as-a-distinct-kind conflicts with any generic "make every record editable the same way" UI abstraction** — the row/detail components must branch on kind (rule vs. memory vs. discovery) for at least the visibility control, or the console will offer an action the server structurally rejects.
- **The derived-state word order (`archived` › `superseded` › `expired` › `scheduled`) is a single source of truth already implemented in `MemoryRow.svelte`** — every new surface (graph node styling, rules list, scheduled view) must consume that same derivation rather than re-deriving state locally, per the project's own `planning-artifacts`-style discipline against parallel implementations of one rule.

## MVP Definition

### Launch With (v1 of this milestone)

Minimum viable product for "recall-first, honest, keyboard-fast" — matches the milestone's own build order (developer recall → operator curation → newcomer browse):

- [ ] Id/short_id/prose resolution with honest reporting of which path was taken — fixes the motivating bug directly
- [ ] Server-driven command palette (`shouldFilter={false}`) — fixes the other motivating bug directly
- [ ] Dense result rows with `score` (always) and `relevance` (when present) rendered, hover-expand, `j`/`k` traversal, detail pane
- [ ] Facets exposed for category (OR), tags (AND), time window, derived state, scope — all already server-supported
- [ ] Honest empty/error states everywhere ("no memories match X in any scope you can read", field-named errors)
- [ ] Scope autocomplete with counts (reuses `ListScopes`)

### Add After Validation (v1.x within this milestone)

- [ ] `SupersedeMemory`/`ArchiveMemory`/`RestoreMemory` write RPCs + preview-before-commit UI, multi-target merge
- [ ] `ListRules`/`ListScheduled` read RPCs + distinct rules/scheduled views
- [ ] Tag cloud + `ListTags` with counts

### Future Consideration (beyond this milestone, or late in it if time allows)

- [ ] Related-memories graph (`RelatedMemories`), including edge-kind filtering and Jev `same_subject` labeling — genuinely valuable per research, but graphs are the item every source warns is easiest to over-invest in for looks over utility; defer polish (physics, layout) ruthlessly
- [ ] NL query understanding as removable chips — explicitly the last item in the "Wanted beyond search" list in the exploration note, and correctly sequenced last here since it depends on the facet-chip UI existing first
- [ ] Cross-surface tag-click linking (tag cloud click also highlights the graph) — a nice-to-have differentiator, not required for either to ship independently

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|----------------------|----------|
| Id/short_id/prose resolution + honest reporting | HIGH | LOW | P1 |
| Server-driven command palette | HIGH | LOW | P1 |
| score/relevance rendering | HIGH | LOW | P1 |
| Facet exposure (category/tags/time/state/scope) | HIGH | LOW–MEDIUM | P1 |
| Scope/tag autocomplete with counts | MEDIUM | LOW–MEDIUM | P1 |
| Supersede/merge with preview UI | HIGH | MEDIUM | P2 |
| Archive/restore surfaces | MEDIUM | LOW–MEDIUM | P2 |
| Rules/scheduled distinct views | MEDIUM | LOW–MEDIUM | P2 |
| Tag cloud (browse) | LOW–MEDIUM | LOW–MEDIUM | P2 |
| Related-memories graph | MEDIUM | MEDIUM–HIGH | P3 |
| NL query understanding chips | MEDIUM | MEDIUM | P3 |

**Priority key:** P1: table-stakes for a search-and-curation console at all; P2: differentiators that make engram's console competitive with (and, for supersede/rules, ahead of) the surveyed products; P3: valuable but correctly sequenced last per this research (graphs and NL chips are the two areas where every source warns "easy to over-build, easy to under-deliver").

## Competitor Feature Analysis

| Feature | Reference product(s) | Their approach | engram's approach |
|---------|----------------------|-----------------|--------------------|
| Command palette source of truth | Raycast, Linear, Notion | Client fuzzy-match over a bounded local item set (fast, no round-trip needed because the corpus is small/local) | Must be server-driven (`shouldFilter={false}`) because the corpus is the whole memory store, not a fixed menu — client filtering is actively wrong here |
| Result score display | Qdrant/Weaviate dashboards (developer-facing raw float), Notion (no number, just relative ordering language) | Two opposite extremes: full precision for developers, zero precision for end users | Split the difference: dim numeric badge for `score` (developer-adjacent audience #1), qualitative bucket option for curation audience #2 |
| Duplicate handling | Zotero, Reevo, Twenty CRM | Pairwise merge wizard with field-conflict resolution; irreversible; archives the loser | engram already has N-way merge server-side (ahead of all three); console should not regress to pairwise-only UI |
| Graph view | Obsidian, Roam, Logseq | Global graph (all ship it, all sources say it's decorative) + local graph (all sources say it's the useful one) | Ship *only* the local/focused equivalent (`RelatedMemories(id)`) — correctly matches what research says actually works, not what looks impressive in a demo |
| Tag browsing | Obsidian (tag pane + graph tag toggle), classic tag-cloud sites | Either a plain filterable list or a decorative cloud, rarely both integrated | Counts-annotated autocomplete (functional) *plus* an optional popularity cloud (browse) for the newcomer audience — covers both registered failure modes (Nielsen's "cloud alone is unusable" and "plain list alone is undiscoverable") |
| NL-to-filter | Linear (AI Filters), Gmail (search chips), Rover (semantic search mode) | All three keep NL as an alternate path into the *same* structured filter representation, confirmed/editable before running | Same pattern, plus the additional constraint (unique to engram) that the underlying NL provider must be swappable/off-by-default |

## Sources

- Raycast: [Search Bar manual](https://manual.raycast.com/search-bar), [design breakdown](https://blakecrosley.com/guides/design/raycast)
- Command palette design generally: [DesignSystems.one](https://www.designsystems.one/design-systems/patterns/search-and-command), [137Foundry](https://137foundry.com/articles/command-palette-interface-users-actually-reach-for), [SaaSFrame CMD+K analysis](https://www.saasframe.io/blog/the-rise-of-cmd-k-why-every-saas-needs-a-search-modal-in-2026), [Rover CMD-K docs](https://docs.rover.inc/core/cmd-k)
- Algolia DocSearch: [How It Works](https://algolia-docsearch.mintlify.app/concepts/how-it-works), [Autocomplete UX writeup](https://www.algolia.com/blog/ux/taking-documentation-search-to-new-heights-with-algolia-and-autocomplete), [Relevance overview](https://www.algolia.com/doc/guides/managing-results/relevance-overview), [DocSearch UI library v5](https://www.algolia.com/blog/ai/docsearch-ui-library-v5)
- GitHub code search: [syntax docs](https://docs.github.com/en/search-github/github-code-search/understanding-github-code-search-syntax), [searching code](https://docs.github.com/en/search-github/searching-on-github/searching-code), [search syntax comparators](https://docs.github.com/en/enterprise-cloud@latest/search-github/getting-started-with-searching-on-github/understanding-the-search-syntax)
- Linear: [Filters docs](https://linear.app/docs/filters), [AI Filters changelog](https://linear.app/changelog/2023-06-01-ai-filters), [New search / hybrid semantic search changelog](https://linear.app/changelog/2025-04-10-new-search), [Obvio Studio field analysis](https://obviostudio.substack.com/p/linear-searching)
- Notion: [Search help](https://www.notion.com/help/search), [Views/filters/sorts](https://www.notion.com/help/views-filters-and-sorts), [Links & backlinks](https://www.notion.com/help/create-links-and-backlinks)
- Obsidian graph view: [design talk (forum)](https://forum.obsidian.md/t/design-talk-about-the-graph-view/22594), [official docs](https://github.com/obsidianmd/obsidian-help/blob/5fb785ac/en/Plugins/Graph%20view.md), [Shadow.do practical guide](https://www.shadow.do/blog/obsidian-graph-view-ai-meeting-notes-2026), [Dan Holloran critique](https://danholloran.me/posts/making-obsidians-graph-view-actually-useful)
- Roam Research: [Graph Overview walkthrough](https://www.youtube.com/watch?v=F-xnDulxvaw), [local graph interpretation](https://app.studyraid.com/en/read/44365/2063264/interpreting-the-local-graph-view-on-a-page), [unlinked references](https://app.studyraid.com/en/read/93166/4230354/analyzing-the-unlinked-references-pane)
- Logseq: [graph explanation (community)](https://discuss.logseq.com/t/graphical-explanation-of-pages-blocks-and-references/15966), [agent-guide graph-view source](https://github.com/logseq/logseq/blob/3de7c751/docs/agent-guide/graph-view/001-graph-view.md), [block-references-on-graph request thread](https://discuss.logseq.com/t/the-option-to-show-block-references-on-the-graphview/3814)
- Tana Outliner: [search & finding](https://outliner.tana.inc/help/search-and-finding), [search nodes](https://outliner.tana.inc/learn/features/search-nodes), [related content](https://outliner.tana.inc/learn/features/related-content)
- Kumu: [focus control](https://docs.kumu.io/guides/controls/focus-control), [filter](https://docs.kumu.io/guides/filter), [focus](https://docs.kumu.io/guides/focus), [selectors](https://docs.kumu.io/guides/selectors)
- Mem: [Heads Up feature](https://get.mem.ai/features/heads-up), [research use case](https://get.mem.ai/use-cases/research), [note-taking guide](https://get.mem.ai/guides/note-taking-for-people-who-hate-organizing), [2.0 transition guide](https://get.mem.ai/blog/mem-2-dot-0-transition-guide)
- Readwise: [finding highlights](https://docs.readwise.io/readwise/docs/faqs/finding-highlights), [reviewing highlights](https://docs.readwise.io/readwise/docs/faqs/reviewing-highlights), [themed reviews](https://docs.readwise.io/readwise/guides/themed-reviews), [product homepage](https://readwise.io/)
- Vector DB dashboards: [Qdrant filtering](https://qdrant.tech/documentation/search/filtering/), [Qdrant search](https://qdrant.tech/documentation/search/search/), [Qdrant Web UI](https://qdrant.tech/documentation/web-ui/), [Weaviate search operators (hybrid)](https://docs.weaviate.io/weaviate/api/graphql/search-operators), [Weaviate query tool](https://docs.weaviate.io/cloud/tools/query-tool), [Weaviate hybrid search / explainScore](https://docs.weaviate.io/weaviate/search/hybrid), [Chroma community UIs — ChromaUI](https://github.com/Riko136/ChromaUI), [chromadb-ui](https://github.com/BlackyDrum/chromadb-ui/blob/main/README.md)
- Gmail: [search help / chips](https://support.google.com/mail/answer/6593), [refine searches / operators](https://support.google.com/mail/answer/7190), [9to5google chips launch coverage](https://9to5google.com/2020/02/19/gmail-search-chips/)
- Superhuman: [Search in Seconds](https://help.superhuman.com/hc/en-us/articles/46005814266253-Search-in-Seconds), [Your AI Assistant](https://help.superhuman.com/hc/en-us/articles/46005792429965-Your-AI-Assistant)
- Tag clouds & autocomplete: [ui-patterns.com Tag Cloud](https://ui-patterns.com/patterns/TagCloud), [Baymard autocomplete research](https://baymard.com/research-articles/autocomplete-design), [Telerik RadTagCloud docs](https://www.telerik.com/products/aspnet-ajax/documentation/controls/tagcloud/overview), [Hassan-Montero & Herrero-Solana (arXiv)](https://doi.org/10.48550/arxiv.2401.04947), [Nielsen Norman Group — Tag Cloud Examples](https://www.nngroup.com/articles/tag-cloud-examples/)
- Merge/duplicate UX: [Reevo merging records](https://help.reevo.ai/CRM-configurations/Merging-Records), [Twenty CRM merge PR](https://github.com/twentyhq/twenty/pull/13436), [Twenty CRM merge-button PR](https://github.com/twentyhq/twenty/pull/13537), [Zotero duplicate detection](https://www.zotero.org/support/duplicate_detection), [shadcnblocks field-merging pattern](https://www.shadcnblocks.com/block/field-merging1)
- engram-specific: `.planning/PROJECT.md` (What This Is, Current Milestone, Core Value, prior milestone deliveries), `.planning/notes/console-overhaul-exploration.md` (decisions, bugs, API gaps, skills plan), `/Volumes/Code/github.com/seanb4t/engram/CLAUDE.md` (memory contract)

---
*Feature research for: engram console overhaul (milestone 2026-09-25.01)*
*Researched: 2026-09-25*
