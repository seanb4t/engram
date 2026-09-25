# Requirements: engram — Milestone `2026-09-25.01` Console Overhaul

**Defined:** 2026-09-25
**Core Value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its
context, and wrong or stale memories can be corrected or superseded, so recall stays trustworthy as
the store grows.

**Milestone goal:** the operator console becomes a recall-first, keyboard-fast, honest search tool
that surfaces every server capability, then a real curation workbench, then a place a newcomer can
learn the store — on the existing stack (Svelte 5, SvelteKit 2, shadcn-svelte 1.7, bits-ui,
Tailwind 4, TanStack Query 6), with the missing Connect RPCs added rather than linked out to the CLI.

> **Research basis.** `.planning/research/SUMMARY.md` (2026-09-25, HIGH confidence) synthesizes
> STACK / FEATURES / ARCHITECTURE / PITFALLS; the milestone's decisions and the two motivating live
> bugs are in `.planning/notes/console-overhaul-exploration.md`. Build order is Sean's: developer
> recall first, operator curation second, newcomer browse third. Standing constraints: authz is
> enforced only in `internal/store` (DEC-cgb); every new mutating RPC goes through the CSRF
> allowlist and the `engramWrite` client; typed decisions stay provider-neutral, off by default,
> advisory (`rwtzp3m7y8`); zero new Go dependencies expected; two small UI libraries allowed.

## v1 Requirements

### Search entry

- [ ] **ENTRY-01**: A user can type a full UUID or a 10-char short_id into any console search box and land on that record (fetched by id, never sent as a semantic query), with the same behaviour on `/search`, the command palette, and the URL `?q=`.
- [ ] **ENTRY-02**: A user can type free text with no scope selected and get results across every scope they can read (`cross_spine` default), with scope available as a facet rather than a prerequisite.
- [ ] **ENTRY-03**: A user sees, on every result set, an honest statement of what was searched — which resolution path ran (id, short_id, text), how many hits, how many scopes, and `scopes_truncated`/`scopes_unknown` when reported — and an empty state that names the query and the scope coverage ("no memories match `github` in any scope you can read"), never a bare "no matches".
- [ ] **ENTRY-04**: The command palette is server-driven: bits-ui `Command` runs with `shouldFilter={false}`, results come from `SearchMemories`/`GetMemory`, navigation items stay available, and a typed query that matches memories never reports "no matches".
- [ ] **ENTRY-05**: A rejected search shows the server's field name and hint code (`field=<name> hint=<code>`) in the UI, never an opaque Connect `internal`/`unknown` error.
- [ ] **ENTRY-06**: Search state lives in the URL (`q`, `scope`, facets, `sel`) so a result set is shareable and survives reload; typing is debounced, a stale response never overwrites a newer query (per-query `AbortSignal`), and previous results stay visible while the next fetch is in flight (`placeholderData: keepPreviousData`).

### Results and facets

- [ ] **ROW-01**: A user sees one dense line per hit — summary, category accent, age, derived state words in canonical order, scope chip, tags — with about 30 rows visible on a laptop screen, rendered through a Svelte-5-native virtualized list that stays smooth at 1000 rows.
- [ ] **ROW-02**: Hovering or focusing a row expands it as an overlay anchored to the row (first lines of content, full tag set, ids) without changing row height or moving the keyboard selection.
- [ ] **ROW-03**: A user can traverse results with `j`/`k` and arrow keys, open with Enter, and act (edit, delete, share, supersede, archive) with keyboard shortcuts; DOM focus stays on the list container with `aria-activedescendant` pointing at the active row (WAI-ARIA APG Listbox), and hover never mutates the keyboard-active row.
- [ ] **ROW-04**: Every search hit renders its cosine `score`, and its Jev `relevance` when the reranker ran; a hit where the reranker did not run shows no fabricated relevance, and the two signals are visually distinct.
- [ ] **ROW-05**: A user can narrow results by category (OR), tags (AND), created-time window, derived state (`include_archived` / `include_superseded` / `include_scheduled`), and scope, as removable filter chips that map one-to-one onto `SearchMemoriesRequest` fields and round-trip through the URL.
- [ ] **ROW-06**: A user picking a scope gets a combobox with autocomplete over `ListScopes`, each entry showing its readable-record count.
- [ ] **ROW-07**: The selected hit opens in the right-hand detail pane with Summary / Content / Meta tabs, showing supersession links, schedule window, archive stamp, schema version, citations and usage counters, and every id is copyable in both forms.

### Store prerequisites

- [ ] **STORE-01**: `internal/store` gains authz-gated `Archive`/`Restore` paths that pass through the same owner-write gate as `Delete`/`Update`/`Supersede` (a caller can archive or restore only records they own; a shared record they can read is not one they can archive), while the CLI's subject-less operator-tier path keeps working unchanged.
- [ ] **STORE-02**: `internal/store` gains `RelatedMemories(subj, id)` returning typed edges — supersession chain in both directions, shared tags, shared citations, vector neighbours via a query-by-id sub-query — with the caller's read predicate composed into the Qdrant filter (never post-filtered in a handler), a bounded edge count, and a documented rule for a candidate reachable by more than one edge type.
- [ ] **STORE-03**: `internal/store` gains `ListTags(subj, scope)` facet counts over a new `tags` payload index under the caller's read filter, with the recall-gate test allowlist widened to recognise a filtered Facet call, and a bounded result size.

### Connect and MCP lanes

- [ ] **RPC-01**: `SupersedeMemory` exists on the Connect lane with the `store_memory` field set plus `supersedes` (one or more ids) and `idempotency_key`, delegating to the same core function the MCP tool calls, and rejecting an invalid target set with every offending target named.
- [ ] **RPC-02**: `ArchiveMemory` and `RestoreMemory` exist on the Connect lane and as MCP tools (`archive_memory` / `restore_memory`), each stamping or clearing `archived_at` on an owned record, never deleting, and reversible.
- [ ] **RPC-03**: `ListRules` and `ListScheduled` exist on the Connect lane with the same contracts as the MCP tools (`list_rules` index shape, oldest-first, up to 1000 per scope; `list_scheduled` with `state` = scheduled / expired / all), any widening beyond the MCP contract decided and recorded explicitly.
- [ ] **RPC-04**: `RelatedMemories` and `ListTags` exist on the Connect lane and as MCP tools (`related_memories` / `list_tags`), wrapping STORE-02 and STORE-03.
- [ ] **RPC-05**: Every new mutating Procedure is in `csrfWriteProcedures` and routed through the `engramWrite` client, proven by a positive test per RPC that a request without the double-submit token is rejected with `permission_denied`; every new RPC has a row in the MCP↔Connect parity test.
- [ ] **RPC-06**: All proto changes are additive (no field-number reuse, `buf breaking` green), `gen/go`, `gen/ts` and `ui/src/lib/gen` are regenerated and committed, the vendored SPA passes the `ui-drift` gate, and each RPC carries the blast-radius annotations and self-describe catalog entries the interface audit requires.

### Curation surfaces

- [ ] **CUR-01**: A user can supersede one or more selected records from the console with a preview-before-commit dialog that shows every target, the new record's fields, and the resulting history chain, and the detail pane of a superseded record links forward to its successor and back to its predecessors.
- [ ] **CUR-02**: A user can archive and restore owned records from the row menu, the detail pane, and a multi-select, with the derived state word updating in place and a one-click undo toast after archive.
- [ ] **CUR-03**: A user can open a Rules view listing every rule in the readable `rule:*` scopes as a one-line index with full text on demand, styled as always-shared, with no visibility toggle and no edit affordance (delete only).
- [ ] **CUR-04**: A user can open a Scheduled view of windowed records the recall gate is hiding (scheduled / expired / all), each showing its window and derived state, with archive available for expired ones.
- [ ] **CUR-05**: Every new write surface participates in the re-auth resume envelope (kind and destination added to the allowed unions) so a draft survives an OIDC re-login, proven by a resume round-trip test per surface.

### Graph and browse

- [ ] **GRAPH-01**: From any record a user can open a local related-memories graph (that record plus its neighbourhood, never a global view) rendered as inline SVG with d3-force layout, pan/zoom, drag, a fixed settle budget, a capped edge count per node, and visually distinct edge types with a legend and per-type toggles.
- [ ] **GRAPH-02**: Every graph node is reachable by keyboard (Tab / arrows, Enter to focus a node and re-centre, Escape to return), carries an accessible name, and an `aria-live` textual list of the neighbourhood is available as an equivalent for screen readers.
- [ ] **GRAPH-03**: The graph works in light and dark mode using the category colour tokens, and clicking a node selects it in the detail pane.
- [ ] **TAGS-01**: A user can open a tag cloud for the current scope (or all readable scopes) built from `ListTags` counts, sized by count quantile, in DOM order that matches reading order, where clicking a tag adds it as a filter chip.
- [ ] **TAGS-02**: The tag filter chip offers autocomplete over `ListTags` with counts, so tags are filterable without the cloud.

### Query understanding

- [ ] **NLQ-01**: An operator can enable query understanding through `ENGRAM_` config on top of `ENGRAM_DECISIONS_PROVIDER`; it is off by default, and with it off no decision call is made and search behaviour is byte-identical.
- [ ] **NLQ-02**: With it on, a prose query yields suggested filter chips (categories, time window, tags, scope) produced by a server-side `UnderstandQuery` RPC through `internal/decide` under a bounded no-retry timeout; a decision failure yields zero suggestions and never fails or delays the search beyond the budget.
- [ ] **NLQ-03**: Suggested chips are rendered as unapplied suggestions the user confirms or dismisses; results never change until a chip is accepted, and an accepted chip is indistinguishable from a manually added one.
- [ ] **NLQ-04**: Query text is never logged or exported by the understanding path unless an explicit opt-in audit flag (mirroring `ENGRAM_SEARCH_RERANK_AUDIT`) is set; content is never logged.

### Design system and quality gates

- [ ] **DSYS-01**: A project-local `engram-console-conventions` skill records the console's design facts — category colour tokens, derived-state word order, dim-iff-past, scope chip semantics, id/short_id/text resolution, the honest-feedback rule, keyboard model — and every UI phase's UI-SPEC cites it.
- [ ] **DSYS-02**: A project-local `engram-connect-client` skill records how the SPA talks to Connect — query-key conventions, `engram` vs `engramWrite`, CSRF and the resume envelope, the per-RPC contract — so no executor invents a second client.
- [ ] **DSYS-03**: The console passes a WCAG 2.2 keyboard and contrast audit (including dimmed past-state rows) and a Web Interface Guidelines review, using the vetted third-party skills once they pass `fable-security-review`, with findings fixed or recorded.
- [ ] **DSYS-04**: Every new surface has vitest-browser coverage with screenshots, and the chromedp console e2e in `internal/e2e` exercises the entry-point resolution, a supersede, and an archive/restore round-trip against a live server.

## Future Requirements

Deferred to a later milestone. Tracked but not in the current roadmap.

### Graph

- **GRAPH-04**: Edges labelled by the consolidate verdict (same fact / contradicts / merely similar) — seed `jev-edge-labelling-in-related-graph`, trigger after the graph has real use.
- **GRAPH-05**: Cross-surface linking — a tag-cloud click highlights matching nodes in the graph.

### Query understanding

- **NLQ-05**: A chat-LLM emulator backend for query understanding so self-hosted deployments need no hosted decision vendor.

### Curation

- **CUR-06**: Bulk supersede/archive from a saved search, with a preview of every affected record.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Stack migration (React, a different component library, SSR) | The stack is current; the work is design and product |
| Global "all records" graph view | Every surveyed product's retrospectives call it decorative; only the local graph is useful |
| `delete_all` on the console | Blast radius too large for a click; stays CLI-only |
| Physics or layout tuning exposed to end users | Decorative; a fixed settle budget replaces it |
| Auto-applied NL filters | Violates the advisory-only rule; chips are suggestions until confirmed |
| Pagination of a ranked result set | Ranked recall is a top-k list; virtualize, never paginate |
| Editing a rule in place | Rules are user-blessed ground truth; delete and re-bless instead |
| Fabricating `relevance` when the reranker did not run | Dishonest signal; show nothing instead |
| Browser-side calls to the decision provider | Would leak the API key; understanding runs server-side only |

## Traceability

Which phases cover which requirements. Filled during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| ENTRY-01 | — | Pending |
| ENTRY-02 | — | Pending |
| ENTRY-03 | — | Pending |
| ENTRY-04 | — | Pending |
| ENTRY-05 | — | Pending |
| ENTRY-06 | — | Pending |
| ROW-01 | — | Pending |
| ROW-02 | — | Pending |
| ROW-03 | — | Pending |
| ROW-04 | — | Pending |
| ROW-05 | — | Pending |
| ROW-06 | — | Pending |
| ROW-07 | — | Pending |
| STORE-01 | — | Pending |
| STORE-02 | — | Pending |
| STORE-03 | — | Pending |
| RPC-01 | — | Pending |
| RPC-02 | — | Pending |
| RPC-03 | — | Pending |
| RPC-04 | — | Pending |
| RPC-05 | — | Pending |
| RPC-06 | — | Pending |
| CUR-01 | — | Pending |
| CUR-02 | — | Pending |
| CUR-03 | — | Pending |
| CUR-04 | — | Pending |
| CUR-05 | — | Pending |
| GRAPH-01 | — | Pending |
| GRAPH-02 | — | Pending |
| GRAPH-03 | — | Pending |
| TAGS-01 | — | Pending |
| TAGS-02 | — | Pending |
| NLQ-01 | — | Pending |
| NLQ-02 | — | Pending |
| NLQ-03 | — | Pending |
| NLQ-04 | — | Pending |
| DSYS-01 | — | Pending |
| DSYS-02 | — | Pending |
| DSYS-03 | — | Pending |
| DSYS-04 | — | Pending |

**Coverage:**
- v1 requirements: 40 total
- Mapped to phases: 0
- Unmapped: 40 ⚠️

---
*Requirements defined: 2026-09-25*
*Last updated: 2026-09-25 after initial definition*
