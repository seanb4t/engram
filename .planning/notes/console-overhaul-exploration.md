---
title: Console overhaul — recall-first UX, curation, browse (exploration)
date: 2026-09-25
context: >-
  Surfaced from /gsd-explore while opening the next milestone. Sean wants a
  serious UI/UX overhaul of the operator console: search usable by a human,
  every server capability surfaced, shadcn-svelte/Svelte 5 latest, performant,
  a delight to use. This note is the milestone context for
  /gsd-new-milestone --reset-phase-numbers.
tags: [ui, ux, console, search, curation, shadcn-svelte, svelte-5, connect, jev, skills, milestone-2026-09-25.01]
---

# Console overhaul — recall-first UX, curation, browse

## Decisions from the conversation (Sean, 2026-09-25)

1. **Three audiences, built in this order:** (b) the developer recalling a fact,
   then (a) the operator curating, then (c) a new teammate learning the store.
   Recall owns the front door.
2. **Results row is dense, expanding on hover.** One line per hit (summary,
   category, age, state words, scope chip, tags, score/relevance), ~30 rows on
   screen, `j`/`k` traversal, detail pane on the right. Hover (or focus)
   expands to show the first lines of content and the full tag set.
3. **Add the missing Connect RPCs** rather than link out to the CLI. Curation
   in the console is real, not read-only.
4. **Wanted beyond search:** a visual (non-text) view of related memories, a
   tag cloud with popularity, scope autocomplete, more free-form / natural
   language entry, and use of models / Jev wherever it helps — within the
   standing rule that typed decisions stay provider-neutral, off by default,
   advisory only (memory `rwtzp3m7y8`).

## The principle the bugs revealed: the entry point must not lie

Two live failures on 2026-09-25 at `engram.fzymgc.house/ui/search`:

- **Palette says "no matches" for `github`** although the store is full of
  github content. The command palette is a client-side menu filter over four
  static items; bits-ui's Command matches the typed text against item labels
  and the "Search memories for …" item snapshots its label while `q` is empty,
  so it filters itself out. The server is never asked. Fix:
  `Command.Root shouldFilter={false}` and drive the list from `SearchMemories`.
- **Searching by an id an agent mentioned errors.** `/search` only calls
  `SearchMemories(query, scope)`. There is no id/short_id path (`GetMemory` is
  reachable only via `?sel=` after a click), and with no scope chosen the
  server rejects under `scope-required-unless-cross-spine`.

Rule for the redesign: every text box accepts a UUID, a short_id, or free
text, resolves which it is, runs the right RPC, defaults to `cross_spine`, and
reports honestly what it searched ("12 hits across 3 scopes", "no memories
match `github` in any scope you can read"). Errors name the field and hint.

## Current state (facts, not guesses)

- Stack is current-generation: Svelte 5.57, SvelteKit 2.70, shadcn-svelte
  1.7.0 (registry latest, same), bits-ui 2.18.x (registry 2.19.3), Tailwind 4,
  TanStack Query 6, paneforge, svelte-sonner, mode-watcher. Not a stack
  migration; a design and product job.
- Surface: routes `/`, `/search`, `/observe`, `/discovery`; 16 domain
  components; 24 shadcn primitives; ~2k lines of Svelte; vitest-browser tests
  with screenshots.
- `SearchMemoriesRequest` already accepts `tags` (AND), `categories` (OR),
  `created_after`/`created_before`, `cross_spine`, `include_archived`,
  `include_superseded`, `include_scheduled`, `k`; results carry `score` and
  (with `ENGRAM_SEARCH_RANKER=jev`) `relevance`. The UI sends `query`, `scope`,
  `full` only. Roughly eight capabilities are API-reachable and UI-invisible.
- Derived state words (archived › superseded › expired › scheduled) and the
  dim-iff-past rule exist in `MemoryRow.svelte`; there is no faceting on them.

## API gaps (Connect lane has 12 RPCs; MCP/CLI have more)

| Gap | Lane | Cost / seam |
|---|---|---|
| `SupersedeMemory`, `ArchiveMemory`, `RestoreMemory` | new Connect write RPCs | proto + server; supersede reuses MCP `supersede_memory`; archive/restore reuse `spine-review archive/restore` store paths |
| `ListRules`, `ListScheduled` | new Connect read RPCs | small; wrap existing store reads |
| `RelatedMemories(id)` | new Connect read RPC | small; reuses the `qdrant.NewQueryID` neighbour sub-query in `internal/store/spine.go` (consolidate) |
| `ListTags(scope)` with counts | new Connect read RPC | small; Qdrant Facet API is in go-client v1.19.2 (already a dep) |
| Query understanding (prose → filter chips) | `internal/decide` consumer or chat client | medium; off by default, advisory, chips are removable and user-confirmed |
| `relevance`, `include_*`, `categories`, time window, `cross_spine`, scope autocomplete via `ListScopes` counts | already on proto | UI only |
| `delete_all` | intentionally absent | keep off the console |

Related-memory edge types the store already knows: supersession chain,
shared tags, shared citations, vector neighbours. Jev's `same_subject`
verdict (consolidate) can label an edge "same fact" vs "similar" — advisory.

## Skills plan

Installed and fit: `frontend-design` (visual direction), `shadcn-svelte`
(build tool), `brainstorm-prototypes` + `gsd-sketch` (variants before commit),
`gsd-ui-phase` / `gsd-ui-review` (contract + audit), `dataviz` (score and
count rendering), `webapp-testing` / `agent-browser` / vitest-browser (live
verification). Not applicable: `web-artifacts-builder`, `theme-factory`,
`agent-interface-design`.

Registry candidates, each needing `fable-security-review` before install
(tracked in a todo): `pbakaus/impeccable` (Apache-2.0, ships a Rust CLI and
browser extension; `shape`/`critique`/`audit`/`harden`/`onboard`/`polish`
plus 61 deterministic detector rules), `vercel-labs/agent-skills@web-design-guidelines`
(lightweight file:line review; no license file), `addyosmani/web-quality-skills@accessibility`
(WCAG 2.2 audit workflow, MIT). Rejected: `ui-ux-pro-max` (style catalog),
`leonxlnx/taste-skill` (sponsor-laden landing-page taste), `shadcn-ui/ui@shadcn` (React CLI).

Project-local skills to author during the first UI phase's discuss step:
`engram-console-conventions` (category tokens, state-word order, dim-iff-past,
scope chip semantics, id/short_id/text resolution, honest-feedback rule) and
`engram-connect-client` (query-key conventions, `engramWrite` interceptor
stack, CSRF + re-auth resume envelope, per-RPC contract).

## Design-skill security verdicts (Phase 4)

Reviewed 2026-09-27 via `Skill("fable-security-review")`, intended use "design
guidance and review skill loaded into coding-agent sessions on this repo".
Each candidate's outcome is normalized to `pass`/`fail` per the folded todo's
rule: anything other than an unqualified pass is `fail`.

- pbakaus/impeccable: fail — reviewed `9d715cc4f5564a990ca8345abfdd5df6dc9b41c8` on 2026-09-27; reviewer's own rubric tier is "USE WITH MITIGATIONS" (not an unqualified pass) — HIGH finding: the skill launcher downloads and executes an unsigned per-platform engine binary verified only by a same-origin sha256 sidecar, safe only after applying `IMPECCABLE_BIN`/`IMPECCABLE_NO_TELEMETRY` mitigations
- vercel-labs/agent-skills@web-design-guidelines: fail — reviewed `063bee94c3f4df8453406c830b0a7df0f2860278` on 2026-09-27; reviewer states "FAIL as shipped" — HIGH finding: `SKILL.md` fetches and obeys unpinned remote instructions from `vercel-labs/web-interface-guidelines@main/command.md` on every invocation, with no pin and no local copy
- addyosmani/web-quality-skills@accessibility: pass — installed at /Users/sean/.agents/skills/accessibility

Reviewed `afa8da942115f2961fdbfa80807ea0b232ff6c00` on 2026-09-27; reviewer
states "SAFE TO USE — changelog: PASS" — three plain-Markdown WCAG 2.2
reference files, no hidden instructions, no tool grants, no scripts; only LOW
findings (unpinned `npx`/`npm install -g` fallback commands, silent-update
channel via `skills update`).

## Research dispositions (2026-09-25, gsd-phase-researcher, sonnet tier)

Admitted, with sources:

- shadcn-svelte latest 1.7.0 (pinned); bits-ui latest 2.19.3. No dense-row /
  hover-expand or async-command block exists; compose from Command, Data
  Table, Combobox. — npmjs.com/package/shadcn-svelte, shadcn-svelte.com/docs/components
- bits-ui `Command.Root` `shouldFilter={false}` disables client filtering for
  server-driven results. — bits-ui.com/docs/components/command
- TanStack Query: `placeholderData: keepPreviousData` with the term in the
  query key avoids flash-to-empty between keystrokes (verified on v5 docs;
  v6 applicability carried forward, see research question). — github.com/TanStack/query/discussions/6460
- WAI-ARIA APG Listbox: keep DOM focus on the container and point
  `aria-activedescendant` at the active row. — w3.org/WAI/ARIA/apg/patterns/listbox

Corrected:

- `@tanstack/svelte-virtual` is published (3.13.39) but its "Svelte 5
  support" issue is open with a manual workaround; not turnkey on Svelte 5.
  — github.com/TanStack/virtual/issues/866

Unresolved:

- Whether a published Svelte-specific UX-review skill exists beyond
  shadcn-svelte CLI-context skills — unverifiable, no exhaustive registry search.
