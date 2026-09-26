# Foundations: direction, tokens, text size, state chips, keys, timing

Cross-cutting rules every console surface follows. The other three references assume these.

## Design Decisions

### Direction

- **Dense developer tool in the Linear/Raycast mould.** Recall owns the front door. Rows are
  one line (about 30 on a laptop screen). The keyboard drives everything. Every entry point says
  exactly what it searched.
- **Quiet neutral surfaces, one accent.** Violet is the only accent (`--primary`: `#8B7BE8` dark,
  `#6E56CF` light). The category colours are the only other hues. Warning amber and danger orange
  are reserved for state and errors, never decoration.
- **Type split.** Sans for prose and summaries. Monospace for ids, short_ids, scores, `rel`,
  scopes, tags, ages, state words and `field=… hint=…` envelopes.
- **Dark-first with a light theme.** Both themes must pass every surface. The console already
  toggles `.dark` with mode-watcher; the sketch's `default.css` is the `.dark` palette and
  `light.css` is `:root`.
- **Audiences in build order:** developer recalling a fact, then operator curating, then newcomer
  learning the store.

### Tokens (sketch → `ui/src/app.css`)

The sketch themes were derived from `ui/src/app.css` and tightened for density. Existing tokens map
one to one; the sketch adds a few that the console does not have yet.

| Sketch token | Dark | Light | Console today |
|---|---|---|---|
| `--color-bg` | `#0d1117` | `#ffffff` | `--background` |
| `--color-surface` | `#161b22` | `#f6f8fa` | `--card` / `--popover` / `--muted` |
| `--color-surface-2` | `#1c2230` | `#eef1f4` | **new** (hover card, menus, chips, code) — close to `--code-bg` |
| `--color-hover` | `#1f2633` | `#f0f3f6` | **new** (`--accent` is close in dark only) |
| `--color-selected` | `#252d3d` | `#e7e3fb` | **new** (active row / open row base) |
| `--color-border` | `#30363d` | `#d0d7de` | `--border` |
| `--color-border-subtle` | `#21262d` | `#e6e9ed` | **new** (row and section dividers) |
| `--color-text` | `#e6edf3` | `#1f2328` | `--foreground` |
| `--color-text-muted` | `#8b949e` | `#59636e` | `--muted-foreground` |
| `--color-text-faint` | `#6e7681` | `#818b98` | **new** (ages, labels, kbd, dim meta) |
| `--color-primary` | `#8B7BE8` | `#6E56CF` | `--primary` / `--ring` |
| `--color-primary-soft` | violet @ 16% | violet @ 12% | **new** (active chips, open row tint, focus rings) |
| `--color-warning` | `#d29922` | `#9a6700` | **new** (expired, `scopes_truncated`) |
| `--color-danger` | `#ffa657` | `#bc4c00` | `--destructive` (= `--cat-gotcha`) |
| `--color-success` | `#3fb950` | `#1a7f37` | **new** |
| `--cat-rule` | `#f0b72f` | `#9a6700` | **missing** from `app.css`; add it before rules render |

The other category tokens (`convention`, `gotcha`, `decision`, `preference`, `discovery`) are
identical to `app.css`. Radii: `4 / 6 / 10px` (sm/md/lg). Shadows: `sm` for chips, `md` for
menus and toasts, `lg` for the hover card, dialogs and popovers. Spacing is a 4px scale.

### Site-wide text size (a display preference, not a per-page control)

- **One preference for the whole console:** 12–16px, **default 15**, persisted per viewer and
  applied on every route. It is not a zoom and not a per-list setting.
- **Where it lives:** an **Aa** icon button in the app header opens a small Display popover: a
  5-segment radiogroup (12/13/14/15/16 px), the current value in mono, a one-row live preview
  of a real result row, the shortcut hints, and the line "Applies to every console page".
- **Shortcuts:** `⌘+` / `⌘-` step it and `⌘0` resets to 15 (Ctrl on other platforms). They
  replace browser zoom inside the console. Arrow keys step the radiogroup while it has focus.
- **Feedback:** each change shows a short toast ("Text size 14px", with "(max)" / "(min)" at the
  ends) and the open popover re-anchors.
- **What scales:** every product dimension — row height, paddings, gaps, column widths, the
  hover card, menus, toasts, dialogs, graph labels. **What does not:** the Display popover's
  own control (fixed px so it does not jump under the pointer; only its preview row scales).
- **Density check:** at a 900px-tall viewport about 22 rows fit at 16px, 28 at 13px, 31 at 12px.

**`display.js` contract** (`sources/themes/display.js`), which the real implementation keeps:

| Aspect | Contract |
|---|---|
| Storage | `localStorage["engram.console.textSize"]`, integer string; clamp to 12–16, round; unreadable or missing → 15; a write that throws (private window) is session-only, never an error |
| Output | `--ui-font: <n>px` and `data-text-size="<n>"` on `<html>` |
| Events | dispatches `engram:textsize` `{ size }` on change; listens to `storage` so other tabs follow |
| API | `get()`, `set(n)`, `step(±1)`, `reset()`, constants `MIN/MAX/DEFAULT/KEY` |
| Keys | global `keydown`: meta/ctrl + `=`/`+`, `-`/`_`, `0`; ignored when Alt is held |

**Stack mapping:** a small Svelte 5 store module (`$state` in a `.svelte.ts` file) replaces
`window.EngramDisplay`. Apply the value before first paint to avoid a size flash: extend the
existing anti-flash `<script>` in `ui/src/app.html` (it already applies the mode-watcher theme)
to read the key and set `--ui-font` on `<html>`. The console is adapter-static, so there is no
server render that could do it. The root layout owns the keyboard
listener and the `storage` listener. The Aa popover is a shadcn-svelte `Popover` with a
bits-ui `RadioGroup` (or `ToggleGroup type="single"`) inside.

**Scaling in Tailwind 4:** the sketch scales with `--u: calc(var(--ui-font) / 13)` and writes
every size as `calc(N * var(--u))`, which is N px at 13px. Two ways to carry this over:

1. Set `html { font-size: var(--ui-font); }` so rem-based Tailwind utilities scale, and stop
   hard-coding `font-size: 13px` on `body` (today's `app.css`). Check that the default spacing
   still reads right at 15px, because rem is then 15px, not 16px.
2. Keep an explicit `--u` and define the few dense-row dimensions (`--row-h`, column widths,
   card width) as `calc(N * var(--u))` tokens in `@theme`.

Either is fine; mixing them is not. Do not leave any product dimension in fixed px.

### State-word chips

- **Canonical order:** `archived` › `superseded` › `expired` › `scheduled` (descending by
  finality). `expired` is evaluated first and suppresses `scheduled`; a record is never both.
- **Dim iff past:** a row or node with any past state (archived, superseded, expired) renders at
  about 50% opacity on its summary, category, tags and scope. A future `scheduled` record is
  **not** dimmed.
- **Chip colour:** neutral mono chip by default; `expired` in warning amber; `scheduled` in the
  violet accent; `superseded` muted.
- **Placement on cards and nodes:** along the **bottom** edge, left-aligned, behind a dashed
  hairline (`.states-foot`). In a one-line row they sit in their own column after the summary.
- **Never overflow:** chips first try to fit; if they would overflow, collapse to
  `first +N` (for example `archived +2`). The full list is always in the chip's tooltip
  (`title` / a Tooltip). The first chip may ellipsize; the `+N` chip never does.
- The detail pane lists each state with `since` / `until` and the timestamp plus a relative
  age, and `superseded` links to its successor.

### Keyboard conventions (all surfaces)

| Key | Meaning |
|---|---|
| `/` | focus the header search from anywhere that is not a text field |
| `j` / `k`, `↓` / `↑` | move the active item; `Home` / `End` jump |
| `Enter` | primary action of the active item (open, toggle the pane, run, re-centre) |
| `Esc` | close the topmost layer only (popover, then hover card, then pane or drawer), then blur |
| `Tab` | inside a result dropdown: next section |
| `⌘↵` | submit a dialog |
| `⌘+` / `⌘-` / `⌘0` | text size |
| `x`, `s`, `e`, `u`, `h` | curation list: select, supersede, archive, restore, show hidden |
| `[`, `g` | related view: back along the trail, toggle Graph / Tags rail |

Single-letter keys are ignored while a text field has focus or any modifier is held. Every
visible shortcut is shown as a `<kbd>` hint near the thing it drives.

### Interaction timing

- **0.15s ease** for every hover, colour, opacity and small transform transition. Drawers and
  sheets may use 0.2s. No springs, no bounces.
- **Hover card:** 250ms open delay on pointer hover, **instant** on keyboard focus, 120ms close
  grace so the pointer can travel into the card.
- **Search input debounce:** about 160ms on the /search page, about 70ms in the header dropdown.
- **No flash to empty on re-query.** Previous results stay on screen, dimmed to about 55%, with
  a 2px indeterminate progress bar and a "previous results ·" prefix in the status line. In
  TanStack Query this is `placeholderData: keepPreviousData` with the full query in the key.
- **Skeletons only on first load**, shaped like the real rows (same grid columns).

## CSS Patterns

```css
/* scale unit: N px at 13px, grows with the preference */
.scaled {
  --u: calc(var(--ui-font) / 13);
  --text-2xs: calc(11 * var(--u));
  --text-xs: calc(12 * var(--u));
  --text-sm: var(--ui-font);
  --text-lg: calc(16 * var(--u));
  --row-h: calc(28 * var(--u));
  font-size: var(--text-sm);
}

/* state chips: never overflow; JS swaps in "first +N" when scrollWidth > clientWidth */
.states { display: inline-flex; gap: calc(3 * var(--u)); min-width: 0; max-width: 100%; overflow: hidden; }
.states > .state-word { flex: none; }
.states.collapsed > .state-word:first-child { flex: 0 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.states-foot { display: flex; min-width: 0; margin-top: calc(4 * var(--u)); padding-top: calc(4 * var(--u));
  border-top: 1px dashed var(--color-border-subtle); }
.states-foot:empty { display: none; }
.st { font-family: var(--font-mono); font-size: var(--text-2xs); padding: 0 calc(5 * var(--u));
  border: 1px solid var(--color-border); border-radius: calc(3 * var(--u)); color: var(--color-text-faint); }
.st.expired { color: var(--color-warning); border-color: color-mix(in srgb, var(--color-warning) 45%, transparent); }
.st.scheduled { color: var(--color-primary); border-color: color-mix(in srgb, var(--color-primary) 45%, transparent); }
.row.dim :is(.sum, .cat, .tags, .scope) { opacity: .5; }

/* stale-while-loading */
.list-wrap.busy .list { opacity: .55; transition: opacity .15s ease; }
.loadbar::after { content: ''; position: absolute; width: 30%; height: 2px;
  background: var(--color-primary); animation: lb .9s ease-in-out infinite; }

/* kbd hint */
kbd { font-family: var(--font-mono); font-size: var(--text-2xs); color: var(--color-text-faint);
  border: 1px solid var(--color-border); border-bottom-width: 2px; border-radius: calc(3 * var(--u)); padding: 0 calc(4 * var(--u)); }
```

`first +N` fit, from sketch 004 (`fitStates`): render every chip, and if
`el.scrollWidth > el.clientWidth + 1` with more than one word, replace with the first word plus
a `+N` chip. Re-run on resize and on text-size change. In Svelte, run it from an action or
`$effect` with a `ResizeObserver` on the chip container.

## HTML Structures

```html
<!-- app header: brand · nav · search · Aa · avatar -->
<header class="app-hd">
  <div class="brand">…</div><nav>…</nav>
  <div class="hd-search">…</div>                     <!-- see recall-surface.md -->
  <button class="aa" aria-haspopup="dialog" aria-expanded="false" aria-controls="display-pop">Aa</button>
  <span class="avatar">sb</span>
</header>

<!-- Display popover -->
<div class="dpop" role="dialog" aria-label="Display">
  <h3>Display</h3>
  <div class="lbl"><span id="ts-lbl">Text size</span><span class="mono">15px</span></div>
  <div class="ts-seg" role="radiogroup" aria-labelledby="ts-lbl">
    <button role="radio" aria-checked="false">12<small>px</small></button> … <button role="radio" aria-checked="true">15<small>px</small></button> …
  </div>
  <div class="keys"><kbd>⌘</kbd><kbd>+</kbd> larger · <kbd>⌘</kbd><kbd>−</kbd> smaller · <kbd>⌘</kbd><kbd>0</kbd> reset to 15</div>
  <div class="pv scaled"><!-- one real result row --></div>
  <div class="keys">Applies to every console page, not just this list.</div>
</div>

<!-- state chips on a card -->
<div class="states-foot">
  <span class="states" data-words="archived superseded expired" title="archived · superseded · expired">
    <span class="state-word">archived</span><span class="state-word more">+2</span>
  </span>
</div>
```

## What to Avoid

- **A text-size control on one page only.** The first B′ draft put it in 001's header as a
  local control; it became a console-wide preference because every surface must match.
- **Using browser zoom as the text-size mechanism.** `⌘+`/`⌘-` are captured so the preference,
  not the zoom level, changes and persists.
- **Fixed px product dimensions.** Anything not derived from `--ui-font` breaks at 12 and 16px.
- **State chips that overflow or wrap into the next row.** Collapse to `first +N` with a tooltip.
- **Chips placed at the top or inline in a card's title line.** They crowd the summary; the
  bottom edge is the agreed place.
- **Dimming `scheduled`.** Only past states dim.
- **Colour as the only signal.** State words are text; edge types (see
  `related-and-tags.md`) use line style and a letter glyph.
- **Flash to empty or to a spinner between keystrokes.** Keep previous results, dimmed.
- **New accent colours.** Violet only; category colours only for category.

## Origin

Synthesized from sketches: 001, 002, 003, 004 (cross-cutting decisions in
`.planning/sketches/MANIFEST.md`).
Source files available in: `sources/themes/default.css`, `sources/themes/light.css`,
`sources/themes/display.js`, and the Display popover in every `sources/NNN-*/index.html`.
