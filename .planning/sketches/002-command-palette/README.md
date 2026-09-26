---
sketch: 002
name: command-palette
question: "How should one entry box resolve a UUID, a short_id, or free text, run the right lookup, and report honestly what it searched?"
winner: "B"
tags: [search, command-palette, id-resolution, honest-feedback, keyboard, scope-autocomplete]
---

# Sketch 002: Command Palette

## Design Question

How should one entry box resolve a UUID, a short_id, or free text, run the right lookup, and report honestly what it searched?

All three variants share one classifier and one fake backend. A full UUID goes to `GetMemory`. A 10-char Crockford base32 token goes to a short_id lookup. Anything else goes to `SearchMemories` with `cross_spine` on by default. `scope:`/`in:`, `#tag`/`tag:` and `is:<category>` become filters. A chip row labelled "interpreted as" shows how the input was read, and each chip can be removed. Every result set comes with a status line that states coverage, and every rejection shows the `field=… hint=…` envelope.

## How to View

```bash
open .planning/sketches/002-command-palette/index.html
```

Use the bottom-left panel for the test inputs, the server state cycler (live / loading / no-matches / error) and the jev ranker toggle. Use the bottom-right toolbar for the theme, the viewport width and annotation mode.

## Variants

- **A: ⌘K modal palette**: a centred Raycast-style modal over the console, with Memories, Scopes, Tags and Commands sections; the shadcn-svelte Command + Dialog path.
- **B: Top-bar inline search**: the box lives in the app header and results drop down under it; for free text, the default Enter action opens the full `/search` page with the query applied.
- **C: Full-page takeover**: `/` turns the page into a search surface with a large input, chips, a result list and a right-side preview; Esc restores the previous page and its scroll position.

## What to Look For

1. **Id resolution honesty.** Try `753aba22-61d0-493c-81a1-ee00c0b1852c` (it resolves to a *superseded* record, which search hides but fetch-by-id shows), `y0bzh06c11` (1 memory), `k3m9p2qr7a` (ambiguous: 2 memories are shown, with a "paste the full id" hint) and a word like `attachment` (it looks like a short_id, has no match, and the status line says it was searched as text instead). Which variant makes the "interpreted as" chip easiest to see before you press Enter?
2. **Coverage wording.** `github` shows "10 hits across 3 scopes · +1 superseded match hidden by the recall gate", and the source button expands the list of searched scopes. `zzzzzz` shows "No memories match `zzzzzz` in any scope you can read". Does the status line read as the truth, or as noise, at each variant's density?
3. **Filters and autocomplete.** `scope:selfh` moves the Scopes section to the top (ListScopes counts), and Enter completes it into a scope chip. `#gsd` lists 7 records across 2 scopes. `is:gotcha qdrant` combines a category with text. Remove the `cross-spine` chip on a text query to get the real `field=scope hint=scope-required-unless-cross-spine` rejection, with fix actions offered as selectable rows.
4. **Loading without flashing to empty.** Set the server to `loading`, then type. The previous results stay visible and dimmed, marked "previous results ·", with a progress bar. Compare how B's small dropdown and C's large surface handle this.
5. **Where Enter goes.** In A, Enter runs the active row. In B, Enter on text defaults to "open /search". In C, Enter opens the previewed memory. Try `Tab` to move between sections and `j`/`k` after clicking the list. Decide which model fits "developer recalling a fact" first.
