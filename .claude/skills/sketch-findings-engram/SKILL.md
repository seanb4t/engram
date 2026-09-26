---
name: sketch-findings-engram
description: Validated design decisions, CSS patterns, and visual direction from engram console sketch experiments — recall results list with hover card and resizable detail pane, header search that resolves UUID / short_id / text honestly, supersede and archive dialogs, related-memories lanes with graph overview and tag bars, and the site-wide text-size preference. Auto-loaded during UI implementation on engram (ui/, Svelte 5 + shadcn-svelte console).
---

<context>
## Project: engram

engram's operator console as a dense developer tool in the Linear/Raycast mould: recall owns
the front door, rows are one line (~30 on screen) and expand on hover or focus, the keyboard
drives everything (`j`/`k`, `/` or `⌘K`, `Enter`, `Esc`), and every entry point tells the truth
about what it searched. Quiet neutral surfaces with a single violet accent (the console's
existing `--primary`), sans text with monospace for ids, short_ids, scores and scopes, and the
existing category colour tokens as the only other hue. Dark-first with a light theme. Audiences
in build order: developer recalling a fact → operator curating → newcomer learning the store.
Milestone 2026-09-25.01 Phase 01.1; decisions from
`.planning/notes/console-overhaul-exploration.md` and engram memory `st74vdk0gh`.

Reference points: Linear (issue-list density, hover affordances, command menu), Raycast
(server-driven command palette, honest empty states), and the existing console in `ui/`
(Svelte 5.57, SvelteKit adapter-static, shadcn-svelte 1.7 + bits-ui 2.x, Tailwind 4,
TanStack Query 6, paneforge, svelte-sonner, mode-watcher; tokens in `ui/src/app.css`).

Sketch sessions wrapped: 2026-09-26
</context>

<design_direction>
## Overall Direction

- **Palette:** GitHub-dark neutrals (`#0d1117` bg, `#161b22` surface, `#1c2230` raised) and
  GitHub-light counterparts; violet accent `#8B7BE8` / `#6E56CF` with a 12–16% soft tint for
  active and selected states; category colours (convention, gotcha, decision, preference,
  discovery, rule) are the only other hues; amber for `expired` and truncation warnings,
  orange for danger. New tokens the console lacks: surface-2, hover, selected, border-subtle,
  text-faint, primary-soft, warning, success, and `--cat-rule`.
- **Typography:** system sans for prose; system mono for every identifier, number, scope, tag,
  state word and error envelope. One **site-wide text size preference** (12–16px, default 15,
  persisted per viewer, applied on every route, set from an **Aa** Display popover in the app
  header, stepped with `⌘+` / `⌘-` / `⌘0`) scales every product dimension.
- **Spacing and shape:** 4px spacing scale, 28px rows at 13px (scaled), radii 4/6/10px,
  hairline borders, shadows only on floating layers.
- **Layout:** header with inline search → facet strip → honest results header → fixed-height
  row list with an overlay hover card → pinned, resizable right detail pane that a second click
  on the open row closes. Related view: edge-type lanes as the body, right rail with Graph |
  Tags. Curation happens in modal dialogs.
- **Interaction:** keyboard-first listbox (`aria-activedescendant`), 0.15s ease transitions,
  250ms hover delay (instant on keyboard focus), no flash-to-empty on re-query, every rejection
  shown as `field=<f> hint=<code>: <text>`, state chips in canonical order
  (archived › superseded › expired › scheduled) that dim only past states and collapse to
  `first +N` instead of overflowing.
</design_direction>

<findings_index>
## Design Areas

| Area | Reference | Key Decision |
|------|-----------|--------------|
| Foundations | references/foundations.md | Dense dev-tool direction, token map onto `app.css`, site-wide text size (12–16, default 15, `display.js` contract), state-word chips, keyboard conventions, timings |
| Recall surface | references/recall-surface.md | Fixed one-line rows + 250ms overlay hover card + resizable right pane (second click closes); header search classifies UUID / short_id / text, shows "interpreted as" chips, defaults to cross_spine, and reports coverage honestly |
| Curation | references/curation.md | Supersede in a modal with per-target validation chips, prefilled correcting record and live chain preview; archive/restore via a small confirm with Undo on the result |
| Related and tags | references/related-and-tags.md | Edge-type lanes as the body, rail Graph (linked overview) \| Tags (linear bars), one shared selection, evidence drawer; edge type by line style + glyph, never a blended score |

Each reference has Design Decisions, CSS Patterns, HTML Structures, What to Avoid (the losing
variants and why), and a stack mapping onto shadcn-svelte / bits-ui / paneforge / TanStack
Query / svelte-sonner.

## Theme

The winning theme files are `sources/themes/default.css` (dark, default) and
`sources/themes/light.css`. The text-size preference module is `sources/themes/display.js`.
In the real console the dark palette maps to `.dark` and light to `:root` in `ui/src/app.css`.

## Source Files

Original sketch HTML files (all variants, winner tab opens by default) are preserved in
`sources/NNN-name/index.html`; open one in a browser to see the interaction. They load
`../themes/*`, which resolves inside `sources/`.
</findings_index>

<metadata>
## Processed Sketches

- 001-results-surface
- 002-command-palette
- 003-curation-dialogs
- 004-related-and-tags
</metadata>
