# Sketch Manifest

## Design Direction

engram's operator console as a dense developer tool in the Linear/Raycast mould: recall owns the front door, rows are one line (~30 on screen) and expand on hover or focus, the keyboard drives everything (`j`/`k`, `/` or `⌘K`, `Enter`, `Esc`), and every entry point tells the truth about what it searched. Quiet neutral surfaces with a single violet accent (the console's existing `--primary`), sans text with monospace for ids, short_ids, scores and scopes, and the existing category colour tokens as the only other hue. Dark-first with a light theme. Audiences in build order: developer recalling a fact → operator curating → newcomer learning the store. Milestone 2026-09-25.01 Phase 01.1; decisions from `.planning/notes/console-overhaul-exploration.md` and engram memory `st74vdk0gh`.

## Reference Points

- Linear (issue list density, hover affordances, command menu)
- Raycast (server-driven command palette, honest empty states)
- Existing engram console (`ui/`): shadcn-svelte 1.7 + bits-ui 2.x + Tailwind 4, category tokens in `ui/src/app.css`

## Sketches

| # | Name | Design Question | Winner | Tags |
|---|------|----------------|--------|------|
| 001 | results-surface | How should the dense result row, its hover/focus expansion, and the detail pane fit together on the recall-first /search page? | B′ — fixed rows + 250ms overlay hover card + resizable right pane; a second click on the open row closes the pane | results, search, dense-row, hover-expand, detail-pane, keyboard |
| 002 | command-palette | How should one entry box resolve a UUID, a short_id, or free text, run the right lookup, and report honestly what it searched? | B — top-bar inline search with anchored dropdown; Enter on free text opens /search | search, command-palette, id-resolution, honest-feedback, keyboard, scope-autocomplete |
| 003 | curation-dialogs | How should supersede-with-chain and archive/restore feel — how much ceremony, how the chain is shown, and how reversal is offered? | A — modal dialog with per-target validation chips, prefilled correcting record, live chain preview; small confirm for archive/restore | curation, supersede, archive, restore, chain, dialogs |
| 004 | related-and-tags | How should a memory's related neighbourhood and the store's tag popularity be visualised so edge types stay distinct and the views stay useful, not decorative? | Synthesis — B's edge-type lanes + right rail Graph (C's linked overview) \| Tags; shared selection; C's evidence drawer; state chips along the bottom of cards/nodes | related-memories, graph, tag-cloud, edges, dataviz |
| 005 | tag-entry-points | How do tag popularity (bars) and tag autocomplete enter /search without crowding the facet strip? | C — docked Tags panel (bars, filter box, ● on active tags) sharing the right slot with the detail pane + compact "+ tag" picker; header `#`/`tag:` group; honest `more` copy (no totals) | tags, list-tags, facet-strip, autocomplete, header-search, phase-05 |

## Cross-cutting decisions

- **Text size is one site-wide display preference** — 12–16px, default 15, persisted per viewer and applied on every console route, set from an "Aa" Display popover in the app header; ⌘+ / ⌘- / ⌘0 step it. Sketch implementation: `themes/display.js` (`--ui-font` on `<html>`, localStorage `engram.console.textSize`, syncs across tabs).
- **State-word chips** (archived › superseded › expired › scheduled) sit along the bottom of their card or node, never overflow, and collapse to `first +N` with the full list in a tooltip.
