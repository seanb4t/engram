# Project Research Summary

**Project:** engram — Console Overhaul (milestone 2026-09-25.01)
**Domain:** operator/developer console for a self-hosted memory MCP server (recall-first search UI, curation workbench, related-memories browse) — SvelteKit 2 / Svelte 5 SPA over ConnectRPC, backed by Go + Qdrant with store-layer authz and provider-neutral advisory typed decisions (Jev)
**Researched:** 2026-09-25
**Confidence:** HIGH

## Executive Summary

This milestone is a design-and-product job, not a stack migration. The console's stack (Svelte 5.57, SvelteKit 2.70, shadcn-svelte 1.7.0, bits-ui 2.18→2.19.3, Tailwind 4, TanStack Query 6) is already current; the work is exposing roughly eight server capabilities that are API-reachable but UI-invisible (facets, `score`/`relevance`, scope autocomplete, curation writes, related-memories, tag counts), and fixing two live entry-point bugs that share one root cause: the console silently stops asking the server and starts guessing. The command palette filters a static four-item list client-side instead of driving results from `SearchMemories` (bits-ui `Command.Root` defaults `shouldFilter` to `true`); free-text search never tries `GetMemory` for an id-shaped token, so an agent pasting a short_id gets a confusing "no results." Both are `Command`/routing bugs fixable without new server work, and both directly motivate this milestone's organizing principle: "the entry point must not lie" — every search reports honestly what it searched and which resolution path it took.

The recommended build order (developer recall → operator curation → newcomer browse, per Sean's own decision) is corroborated independently by all four research files: STACK ranks the results-row virtualization and search-box mechanics as the highest-certainty, lowest-risk work; FEATURES' prioritization matrix puts id/short_id resolution, the palette fix, score/relevance rendering, and facet exposure at P1 (all reuse existing RPCs, zero new server surface); ARCHITECTURE's suggested build order puts store-layer prerequisites and the entry-point fix before any new proto; and PITFALLS anchors five of its ten critical pitfalls in exactly this first phase. The graph view and NL-query-understanding chips are consistently sequenced last across all four files — not because they're unimportant, but because every source (Obsidian/Roam/Logseq graph-view retrospectives, Linear/Gmail NL-chip precedent) independently warns these are the two features easiest to over-build for looks and under-deliver for utility, and both benefit from the facet/filter-chip UI and `ListTags`/`ListScopes` data existing first.

The two biggest risks are architectural, not visual. First, DEC-cgb (store-layer-only authz enforcement) is a locked invariant that every new Connect RPC (`SupersedeMemory`, `ArchiveMemory`/`RestoreMemory`, `RelatedMemories`) must respect — a "quick guard" in a handler is the single most likely regression, and `ArchiveMemory`/`RestoreMemory` specifically need *new* authz-gated store methods since today's `store.Archive`/`Restore` are subject-less CLI-only paths. Second, the CSRF contract is a two-sided, silently-failing allowlist (`csrfWriteProcedures` in `connectcsrf.go` plus routing through the `engramWrite` client) — an omission produces no test failure, just a forgeable mutation. Both risks are well-understood and have direct code-level fixes (see ARCHITECTURE Gap 1/2 and PITFALLS 6/7), but they are exactly the kind of "looks done, isn't" failure the roadmap must gate explicitly rather than trust to manual click-through.

## Key Findings

### Recommended Stack

No stack migration — bump `bits-ui` to `2.19.3` (accessibility fixes relevant to the new `Combobox` usage) and add two small, purpose-fit libraries. Everything else (debounced search, tag cloud, scope autocomplete) is covered by what's already a dependency.

**Core technologies:**
- `@humanspeak/svelte-virtual-list@0.5.14` — virtualizes the dense results list (50–1000 rows); the only Svelte-5-native, actively-maintained virtualizer with a documented keyboard-accessible viewport. `@tanstack/svelte-virtual` is explicitly rejected — its Svelte 5 support issue (#866) has been open since 2024-10-28 and requires a private-API workaround.
- `d3-force` + `d3-drag` + `d3-zoom` + `d3-selection` (3.0.0 each, ~31 KB combined) — related-memories graph physics/gestures, rendered as plain Svelte-owned inline SVG. Chosen over Sigma.js and Cytoscape.js specifically because native SVG elements are directly keyboard/ARIA-addressable; canvas/WebGL renderers require building a synthetic accessibility layer from scratch (a confirmed, library-unsolved gap for both alternatives).
- `@tanstack/svelte-query@^6.1.34` (existing) — `placeholderData: keepPreviousData` avoids flash-to-empty between keystrokes; the codebase already uses v6's required thunk syntax everywhere.
- `bits-ui@2.19.3` (bump from ^2.18.1) — `Combobox` (unopinionated, server-driven by default) for scope autocomplete; `Command.Root shouldFilter={false}` for the palette.
- Plain `flex-wrap` of the existing shadcn-svelte `Badge` for the tag cloud — no dedicated tag-cloud library; those target a decorative, rotated-text aesthetic that actively hurts scannability/keyboard access for an operator control.

**Architectural note carried from STACK into the row design:** hover-expand must not be a virtualizer-managed dynamic row height (reintroduces exactly the remeasurement complexity the library exists to hide). Render fixed-height virtualized rows; render expanded content as an absolutely-positioned overlay anchored to the hovered row's bounding rect, reusing the detail pane's content.

### Expected Features

**Must have (table stakes, P1 — all reuse existing server capabilities, zero new RPCs):**
- Id/short_id/prose resolution with honest reporting of which path was taken — fixes the motivating bug directly
- Server-driven command palette (`shouldFilter={false}`) — fixes the other motivating bug directly
- `score` (always) and `relevance` (when present) rendered on dense rows, hover-expand, `j`/`k` traversal, detail pane
- Facets exposed for category (OR), tags (AND), time window, derived state, scope — all already server-supported, currently UI-invisible
- Honest empty/error states everywhere ("no memories match X in any scope you can read", field-named errors)
- Scope autocomplete with counts (reuses `ListScopes`)

**Should have (differentiators, P2 — need new Connect RPCs but reuse existing store/MCP logic):**
- `SupersedeMemory`/`ArchiveMemory`/`RestoreMemory` write RPCs with a preview-before-commit UI; multi-target merge (engram's server already supports N-way supersession — ahead of every competitor surveyed, which are pairwise-only)
- `ListRules`/`ListScheduled` read RPCs + distinct, always-shared-styled views (rules must never get a visibility toggle — the server rejects it)
- Tag cloud with counts (`ListTags`, Qdrant Facet API)

**Defer (P3, this milestone's last-priority capabilities):**
- Related-memories graph (`RelatedMemories`) — local/focused only (one record + neighborhood), never a global "hairball" view; every source (Obsidian, Roam, Logseq, Kumu) converges on this being the single most consistent graph-view finding
- NL query understanding as removable, user-confirmed filter chips — must sit on top of the facet-chip UI (P1), never replace it; never auto-apply

**Explicit anti-features to avoid:** client-side filtering of a server-authoritative list; pagination of a ranked result set (Algolia's own team rejects this for recall tools); silent/guessed id-vs-text resolution; fabricated `relevance` when the reranker didn't run; auto-applying NL-parsed filters; a global/decorative graph view; physics-tuning UI exposed to end users.

### Architecture Approach

The system already has a clean, doubled-lane shape: MCP tools and Connect handlers both call the same `deps.*` capability functions, and `internal/store` is the *only* place Cedar authz becomes a Qdrant filter (DEC-cgb, locked). Six new/extended Connect RPCs are needed (`SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`, `ListRules`, `ListScheduled`, `RelatedMemories`, `ListTags` — all additive on the wire). Two of these are genuinely new authz surface, not thin wrappers: `ArchiveMemory`/`RestoreMemory` need a new `getWritable`-gated store path (today's `store.Archive`/`Restore` are subject-less, CLI-only), and `ListTags` needs a new `tags` payload index plus widening a test-infrastructure allowlist (`recognizedFilterCarryingRequestMethods`) that currently rejects any filtered `Facet` call. `RelatedMemories` should be one RPC returning a flat, typed-edge list (supersession, shared-tags, shared-citations, vector-neighbor) with authz composed once into the underlying Qdrant filter — never four RPCs, never a handler-level post-filter.

**Major components:**
1. `internal/server/*deps` — the one capability-per-method core both MCP and Connect call; new capabilities land here first, before any proto change
2. `internal/store/*Store` — the only place authz composes into Qdrant filters (`ownerScopeFilter`, `getWritable`); every new per-record write RPC must gate through this, never a handler-level check
3. `internal/server/connectcsrf.go` — hand-maintained allowlist gating every mutating Procedure; an omission is a silent, non-failing CSRF hole
4. `internal/decide`/`internal/decide/jev` — provider-neutral, advisory-only typed decisions; the query-understanding capability is a new consumer here, server-side only (never called from the browser — it would leak the API key), reusing the existing two-client (short-timeout, no-retry) pattern
5. `ui/src/lib/mutations/*.ts` — one `useXMutation()` hook per write RPC with optimistic patch/rollback/invalidate, an established shape every new write surface must copy exactly

### Critical Pitfalls

1. **The command palette lies (client-side filtering)** — set `Command.Root shouldFilter={false}` on every palette/combobox driven by server results; this is the exact bug that motivated the milestone.
2. **Search-box race (stale response overwrites newer query)** — thread `AbortSignal` into every server-driven query's `queryFn`; the query key already includes the raw text, but nothing cancels abandoned in-flight requests today.
3. **Flash-to-empty during debounced search** — `placeholderData: keepPreviousData`, and gate the "no results" empty state strictly on `isFetching === false`, never on `data.length === 0` alone.
4. **Focus loss / mixed keyboard model on hover-expand rows** — keep real DOM focus on the list container (`aria-activedescendant`, WAI-ARIA APG Listbox pattern); hover must be purely visual and must never mutate the same "active id" state `j`/`k` writes, or mouse position and keyboard selection silently disagree about what's "current."
5. **Authz enforced in a Connect handler instead of the store (DEC-cgb violation)** and **a new mutating RPC shipped CSRF-unprotected** — both are locked invariants with concrete two-line checklists (route through `engramWrite`, add to `csrfWriteProcedures`, delegate all authz to the store method the MCP tool already calls) and both fail silently if skipped.

## Implications for Roadmap

Based on combined research, suggested phase structure (closely matching the milestone's own stated audience order and ARCHITECTURE's "suggested build order," cross-checked against FEATURES' MVP definition and PITFALLS' phase mapping):

### Phase 1: Store-layer prerequisites and API-gap groundwork
**Rationale:** ARCHITECTURE and PITFALLS both insist the authz-sensitive and test-infrastructure work (new `ArchiveMemory`/`RestoreMemory` authz gate, `tags` payload index, widening `recognizedFilterCarryingRequestMethods` for a filtered `Facet` call) land before any RPC or UI is built on top of it — building UI against an RPC whose authz shape isn't settled risks the DEC-cgb regression class.
**Delivers:** `Store.RelatedMemories`, `Store.ListTags(scope, subj)`, and an authz-gated Archive/Restore store path, each with unit/integration test coverage; no proto change yet.
**Addresses:** the two named "Gap" items in ARCHITECTURE (Gap 1: Archive/Restore authz; Gap 2: ListTags index + recall-gate test).
**Avoids:** Pitfall 6 (authz in handler instead of store).

### Phase 2: Recall-first search (developer audience — front door)
**Rationale:** This is Sean's own stated audience order and independently the P1 tier in FEATURES' prioritization matrix — every item here reuses an existing RPC (`SearchMemories`, `GetMemory`, `ListScopes`), so it is the lowest-risk, highest-visibility phase and should land first regardless of any new-RPC work.
**Delivers:** client-side id/short_id/text routing (try `GetMemory` first for id-shaped input, fall back to `SearchMemories` on `NotFound` — no new RPC), server-driven command palette, dense virtualized results row (`score`/`relevance` rendered, hover-expand via overlay not dynamic height, `j`/`k` traversal via `aria-activedescendant`), facet exposure (category/tags/time window/state/scope), honest empty/error/loading states, scope autocomplete with counts.
**Addresses:** FEATURES §1/§2 table stakes; the milestone's two motivating bugs.
**Avoids:** Pitfalls 1–5 (palette lying, search race, flash-to-empty, focus/hover conflation, virtualization breakage) — write the regression test for each before/alongside the corresponding UI.

### Phase 3: Curation RPCs (proto + server, one RPC at a time)
**Rationale:** ARCHITECTURE explicitly sequences "RPCs before the UI that consumes them"; `SupersedeMemory` first (zero new store work, de-risks the CSRF-allowlist step), then Archive/Restore (depends on Phase 1's authz work), then `ListRules`/`ListScheduled` (near-zero risk).
**Delivers:** `SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`, `ListRules`, `ListScheduled` Connect RPCs, each proto-gen'd, vendored into `ui/src/lib/gen/`, added to `csrfWriteProcedures` if mutating, with a parity-test row or documented MCP asymmetry decision.
**Uses:** the `connectapi_write_parity_test.go` pattern extended per new RPC; `getWritable` gate from Phase 1.
**Implements:** the thin-handler-delegates-to-store-method pattern (Component 1/2 above).
**Avoids:** Pitfall 7 (CSRF allowlist omission) — write the positive-assertion test first, before any UI touches these RPCs.

### Phase 4: Curation surfaces (operator audience)
**Rationale:** Second in Sean's stated audience order; depends on Phase 3's write RPCs existing.
**Delivers:** supersede-with-chain preview UI (multi-target, not pairwise), archive/restore actions, distinct rules/scheduled list views (index-first, full text on demand; no visibility toggle on rules).
**Addresses:** FEATURES §3 (curation workflows) table stakes and differentiators.
**Avoids:** Pitfall 10 (new curation surfaces falling outside the re-auth resume envelope) — extend `ALLOWED_DESTINATIONS`/`kind` unions and write a resume round-trip UAT for every new write surface, don't assume existing memory/discovery tests generalize.

### Phase 5: Related-memories graph and tag cloud (newcomer/browse audience)
**Rationale:** Third and last in Sean's stated audience order; needs Phase 1's store methods and Phase 3-adjacent `RelatedMemories`/`ListTags` RPCs (add these to Phase 3's RPC batch or treat as a small follow-on batch — ARCHITECTURE groups them with the other read RPCs).
**Delivers:** local/focused (never global) related-memories graph with capped edges-per-node, visually distinct edge types, a fixed settle budget, full keyboard/ARIA equivalence and an `aria-live` textual fallback; tag cloud with count-quantile sizing, paired with (not replacing) the counts-annotated autocomplete filter.
**Uses:** `d3-force`/`d3-drag`/`d3-zoom` + inline SVG (STACK); `RelatedMemories`/`ListTags` RPCs (Phase 3).
**Addresses:** FEATURES §4/§5.
**Avoids:** Pitfall 8 (decorative/hairball graph) — set edge-cap and keyboard-equivalence constraints in the UI-spec step before any layout library integration begins.

### Phase 6: Query understanding (NL chips)
**Rationale:** Correctly sequenced last by every research file — it is the least load-bearing capability, has the most design risk (no direct precedent for the Choice/Noul batching shape in this codebase), and depends on Phase 2's facet-chip UI existing first (chips-from-NL only make sense once chips-from-manual-filtering have a defined shape).
**Delivers:** a server-side-only `UnderstandQuery` RPC (never a browser-side decide call — leaks the API key) producing removable, unapplied-until-confirmed filter chips; bounded 2s no-retry timeout degrading to zero suggestions on any failure; query-text logging gated behind a new opt-in flag mirroring `ENGRAM_SEARCH_RERANK_AUDIT` (never content, never a default-on path).
**Addresses:** FEATURES §6.
**Avoids:** Pitfall 9 (advisory NL understanding quietly stopping being advisory) — test that results are unchanged until a chip is clicked, and that no query text appears in logs without the audit flag.

### Phase Ordering Rationale

- **Store-before-proto-before-UI** is not a preference but a consequence of DEC-cgb: authz shape must be settled in the store before any RPC exposes it, or the RPC locks in a handler-level shortcut that's expensive to unwind later.
- **Recall (Phase 2) before curation (Phases 3–4) before browse (Phases 5–6)** is Sean's explicit decision, independently corroborated by FEATURES' P1/P2/P3 prioritization matrix (everything Phase 2 needs already has a wire contract; everything Phase 5–6 needs is new RPC surface with the highest design risk in the whole milestone).
- **Query understanding last** because it is architecturally downstream of the facet-chip UI (Phase 2) it becomes a shortcut into — building it earlier would create a second, divergent filter representation.
- This ordering directly avoids the two most consequential pitfall classes (authz drift, CSRF gaps) by settling them once, early, rather than repeating a handler-level check per new RPC.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (Curation RPCs):** the Archive/Restore authz-gate design (new `authz.Action` vs. reusing `ActionWrite`; whether the CLI keeps its subject-less bypass) is a genuine open design decision, not a known pattern — flag for `/gsd-plan-phase --research-phase` or at minimum a design-review step before implementation.
- **Phase 5 (Graph/tag cloud):** the edge-scoring/capping heuristic (top-N by what signal — Jev `same_subject` score where available, else recency/score) has no established precedent in this codebase; needs a UI-spec decision pass before any layout library integration.
- **Phase 6 (Query understanding):** the Choice/Noul question-batching shape for query-to-filter parsing has no direct precedent in `internal/decide`'s existing consumers (`internal/relevance`, `internal/verdict`) — ARCHITECTURE explicitly flags this as MEDIUM confidence, extrapolated rather than confirmed.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Store prerequisites):** direct extensions of existing, well-understood patterns (`ensureIndexes`, `getWritable`, `NewQueryID` sub-queries) — HIGH confidence, file:line-grounded.
- **Phase 2 (Recall-first search):** every fix is either a documented bits-ui/TanStack Query idiom or a WAI-ARIA APG pattern with a live precedent already partially implemented in `MemoryRow.svelte` — HIGH confidence, well-trodden ground.
- **Phase 3 (Curation RPCs, non-Archive/Restore items):** `SupersedeMemory`/`ListRules`/`ListScheduled` are direct wrappers around existing `deps.*` functions with an established parity-test pattern to copy.
- **Phase 4 (Curation surfaces):** the mutation-hook shape (`useXMutation`) is fully established and documented with line-level precedent in `mutations/memory.ts`.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Every package version checked live against the npm registry and/or Context7-indexed source docs on 2026-09-25; bundle sizes are live Bundlephobia figures |
| Features | HIGH for table-stakes UI mechanics (multi-source corroborated across Raycast/Linear/Algolia/GitHub); MEDIUM for graph-view and NL-chip judgment calls (fewer independent sources, more editorializing, explicitly flagged inline in FEATURES.md) |
| Architecture | HIGH for the Go/proto/authz integration points (read directly from the actual code, with file:line citations); MEDIUM for the query-understanding decision shape (no direct precedent in this codebase, extrapolated from two related consumers) |
| Pitfalls | HIGH for engram-specific/repo-grounded findings (direct file:line evidence); MEDIUM for external library specifics not confirmed against first-party version-pinned docs (flagged inline, e.g. `placeholderData` carrying forward unchanged into TanStack Query v6) |

**Overall confidence:** HIGH

### Gaps to Address

- **Archive/Restore authz design (Phase 1/3):** whether to add a new `authz.Action` or reuse `ActionWrite`, and whether the CLI keeps a subject-less bypass — a genuine open decision, not resolvable from research alone; resolve during Phase 3 planning/design.
- **MCP parity policy for the four RPCs with no existing MCP tool** (`ArchiveMemory`/`RestoreMemory`/`RelatedMemories`/`ListTags`): ARCHITECTURE flags this as "decide explicitly" per RPC — either add matching MCP tools or document an intentional Connect/console-only asymmetry (mirroring the existing `SearchDiscoveries` precedent). Needs a decision during requirements/roadmap review, not left implicit.
- **TanStack Query v6's exact `placeholderData`/`keepPreviousData` behavior:** confirmed unchanged from v5 in the general migration guide, but not explicitly re-confirmed against the pinned `6.1.34` changelog — verify directly before relying on it in Phase 2.
- **Per-row `edge_type` de-duplication semantics for `RelatedMemories`** (a candidate can appear via more than one edge type — one row per type, or dedupe-and-list types): explicitly deferred to planning in ARCHITECTURE, not research.
- **`ListScheduled`'s `cross_spine` widening:** adding this to the new Connect RPC would be new behavior beyond the existing MCP tool's contract; decide whether to also widen the MCP tool or accept a second documented asymmetry.

## Sources

### Primary (HIGH confidence)
- Live npm registry queries (`npm view`), 2026-09-25 — all pinned package versions in STACK.md
- Context7 `/huntabyte/bits-ui`, `/tanstack/query` — quoted directly from source docs
- Direct codebase reads: `internal/server/connectapi.go`, `connectcsrf.go`, `decider.go`; `internal/store/store.go`, `spine.go`, `migrate_status.go`, `schemaversion_recallgate_test.go`; `internal/decide/*`; `ui/src/lib/client.ts`, `mutations/memory.ts`, `resume.ts`, `queries.ts`; `ui/src/routes/search/+page.svelte`; `ui/src/lib/components/MemoryRow.svelte`, `CommandPalette.svelte`
- `.planning/PROJECT.md`, `.planning/notes/console-overhaul-exploration.md`, `CLAUDE.md` (memory contract, locked ADRs)
- `github.com/qdrant/go-client@v1.19.2` — `FacetCounts.Filter` field, direct source read
- WAI-ARIA APG Listbox pattern (w3.org)

### Secondary (MEDIUM confidence)
- TanStack/virtual#866 (GitHub issue, confirmed open 2026-09-25) — community-confirmed, not first-party resolution
- cytoscape/cytoscape.js#3125 (community discussion on accessibility gap)
- Obsidian/Roam/Logseq/Kumu graph-view retrospectives (community blog posts and forum threads, convergent across independent sources)
- Baymard, Nielsen Norman Group, Algolia blog UX research (industry research, not primary-source docs)

### Tertiary (LOW confidence)
- None flagged as LOW in any of the four research files — all uncertain claims were explicitly labeled MEDIUM with the reason stated inline.

---
*Research completed: 2026-09-25*
*Ready for roadmap: yes*
