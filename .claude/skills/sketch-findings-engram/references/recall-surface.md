# Recall surface: results list, hover card, detail pane, header search

The recall-first `/search` page (sketch 001, winner B′) and the one entry box in the app header
that feeds it (sketch 002, winner B). Read `foundations.md` first for tokens, text size, state
chips and timings.

## Design Decisions

### Results list (001 · B′)

- **Fixed-height one-line rows.** Rows never change height. Grid columns, left to right:
  category (dot + word) · summary (ellipsized, inline `code` allowed) · state chips · first two
  tags + `+N` · scope chip · age · score · optional `rel`. About 30 rows at 13px.
- **Overlay hover card, not inline expansion.** A floating card anchored under the hovered row
  shows: category badge, short_id, full scope; the summary; state chips; the first ~6 lines of
  content (line-clamped); the **full** tag set; a footer with `created_at · visibility · owner`
  and an `↵ open` hint. Width about 480px, `--shadow-lg`, `--color-surface-2`.
  - Opens after **250ms** on pointer hover; **instantly** when the active row moves by keyboard.
  - The pointer can move into the card without closing it (120ms close grace).
  - Hides on scroll, on opening the pane, on text-size change, and never shows for the row that
    is already open in the pane.
  - Places below the row, flips above when it would overflow the viewport, aligned to the
    summary column's left edge and clamped to the viewport.
- **Resizable right detail pane.** `Enter` or click opens the active record in a right pane.
  The pane is **pinned**: moving the active row with `j`/`k` does not change it.
  - **Toggle-close:** a second click (or `Enter`) on the row that is already open closes the
    pane; the list takes the width back over 0.15s. Clicking any other row switches the pane.
  - The open row shows a 3px violet left bar and a violet-tinted background
    (`color-mix(primary 10%, selected)`) and a tooltip "Open in the detail pane: click or press ↵
    again to close". The keyboard-active row shows a 2px left bar (faint when the list is not
    focused, violet when it is).
  - Resize by dragging the handle or by focusing it and using `←`/`→` (32px steps). Clamp to
    280px minimum and 72% of the body maximum. Default about 440px.
  - Below 760px container width the pane becomes a full overlay; below 620px the header wraps
    and the search goes full width.
- **Pane content:** sticky head (category badge, short_id + copy, visibility, close `✕`), then
  title, action buttons (Edit, Supersede…, Archive/Restore, Share/Make private, Delete in danger
  colour), State section, Content (markdown), Tags (clickable), Metadata `dl` (id, short_id,
  scope, created_at + age, score to 4 dp, relevance, owner, actor, visibility, summary_source,
  source, schema_version — all mono). If the open record is no longer in the results it stays
  open with a note: "Not in the current results (filtered out or not matched). Showing it by
  id, like get_memory."
- **Column drop-out by list width** (container queries, not viewport): tags go below ~860px
  list width; scope goes and the category collapses to a dot below ~560px; score, `rel` and
  state chips always stay.
- **Facet strip** (one line, horizontally scrollable, under the header): `all N` plus one chip
  per category with its count (zero-count chips at 45% opacity) · scope picker chip (`▾` menu)
  and a `cross_spine` checkbox chip, or a single `cross_spine · 7/8 scopes` chip when on ·
  `include_archived`, `include_superseded`, `include_scheduled` checkbox chips · `jev rerank`
  chip. Every chip maps to a real `SearchMemoriesRequest` field; nothing is UI-only.
- **Honest results header** (one line, `aria-live="polite"`, ellipsized):
  - ranked: **`12 hits`** `across` <u>`3 scopes`</u> `for` `query` `· ranked by cosine` (or
    `· reranked by jev`) `· 2 hidden by recall gate` `· scopes_truncated: 1 readable scope not searched`
  - with a category filter: `12 hits of 20 across …`
  - unranked listing: `Latest 40 memories across 3 scopes · unranked (list — no score)`
  - loading: `Searching 3 scopes for query…`
  - empty: `No memories match query in any scope you can read` (cross_spine) or
    `… in the 3 scopes searched · N hidden by recall gate`
  - error: `Search failed — nothing was searched. search_memory returned code=unavailable`
  - The "N scopes" part is a button that expands a `searched_scopes:` line listing every scope
    with its hit count; scopes cut by `scopes_truncated` show dashed in warning colour as
    "(not searched)". The hidden count's tooltip splits it by state.
- **Empty and error states say what happened.** Empty: the headline, the list of scopes that
  were searched, how many matches the recall gate hid, and one-click fixes (search every
  readable scope, include archived, clear the category filter). Error: "Nothing was searched,
  so this is not an empty result", the raw error with request id, Retry and Copy error.
- **Score and rel rendering:** score is mono to 2 dp plus a 28×3px bar filled in violet; unranked
  listings show `—`. `rel` (only when the jev ranker is on) is mono `rel 0.83` in its own
  column; the header says the order came from jev. Never combine the two into one number.

### Header search (002 · B)

- **The search box lives in the app header**, centred, up to about 560px wide, with a `/` hint.
  Focus opens an **anchored dropdown** under it (same width, `--shadow-lg`, max-height about
  540px). Below 760px the dropdown goes fixed to the viewport edges.
- **One classifier for every text box:**
  1. A single token matching a UUID → `GetMemory` (chip `uuid → GetMemory`, plus an info chip
     `any scope you can read`).
  2. A single 10-char Crockford base32 token (`[0-9a-hjkmnp-tv-z]{10}`, no i l o u) →
     short_id lookup (chip `short_id → lookup`).
  3. Anything else → `SearchMemories` with **`cross_spine` on by default** (chips
     `text "…"` and `cross-spine`).
  4. Operators become filter chips: `scope:`/`in:` (scope), `#tag`/`tag:` (tags, AND),
     `is:<category>` (categories, OR). Only operators → a listing, not a text search.
- **"Interpreted as" chip row** under the input shows how the input was read before Enter.
  Every chip has an `✕` that removes exactly its token from the input. Removing `cross-spine`
  leaves a dashed ghost chip `+ cross-spine` to restore it. An unfinished operator shows as a
  dashed pending chip (`scope:selfh…`) until completed. An unknown scope or category shows as
  a danger chip.
- **Status line with coverage:** headline plus a note plus a right-aligned source button that
  names the RPC (`SearchMemories · cross_spine · 3 scopes searched ▾`). The button expands a
  per-scope coverage list (`✓ scope  2 hits` / `· scope  0`) ending with
  "discovery:* not included (separate lane: search_discovery)".
- **Honest id outcomes:**
  - UUID found: `Resolved id 753aba22… → 1 memory`, with a note when the record is hidden from
    search (`this record is superseded → k3m9…; hidden from search, fetchable by id`).
  - UUID not found: `No memory with id … that you can read · not-found and not-yours look the
    same by design`.
  - short_id ambiguous: `short_id k3m9p2qr7a is ambiguous → 2 memories · pick one, or paste the
    full id to be exact` (warning tone, both listed).
  - short_id-shaped word with no match: `No short_id attachment. Searched it as text instead:
    …` — never silently switch interpretation.
  - text: `10 hits across 3 scopes · +1 superseded match hidden by the recall gate (fetch by id)`.
  - nothing: `No memories match zzzzzz in any scope you can read` plus a "Search discoveries for
    …" command row.
- **Rejections use the real envelope.** Status reads "Server rejected the request" and a mono
  box shows `field=scope hint=scope-required-unless-cross-spine: scope is required unless
  cross_spine is set`. The list becomes **fix rows** the user can select: "Re-enable
  cross-spine", "Pick one scope instead (types scope:)", "Remove the unknown is: filter",
  "Retry the request".
- **Dropdown sections, in order:** a top row `Search all memories for "…"  N hits  ↵ /search`,
  then Memories (max 5, then `N more on /search`), Scopes (from `ListScopes` counts), Tags
  (from `ListTags` counts) or Categories, Commands (navigation, filtered locally — they are not
  memories). Section headers name their source RPC in mono. While completing `scope:` the
  Scopes section moves to the top; while completing `#`/`is:`, Tags/Categories move up.
- **Where Enter goes:** on free text the default (top) row opens `/search?q=…` with the query
  and chips applied, and clears the header box. On a memory row it opens that record. On a
  scope/tag/category row it completes the token into a chip. `Tab` cycles sections;
  `↑`/`↓` move; `Esc` closes and blurs.
- **Active memory row** shows a second line with category, all `#tags`, date, full UUID and
  `shared`. Matches in the summary are highlighted with `--color-primary-soft`.

### Keyboard model (listbox)

The results list is a WAI-ARIA listbox: DOM focus stays on the list container
(`role="listbox" tabindex="0"`), rows are `role="option"` with stable ids, and
`aria-activedescendant` points at the active row. `j`/`k`/arrows, `Home`/`End` move; `Enter`
toggles the pane; `Esc` closes the hover card first, then the pane; `/` focuses search; `↓` in
the search box moves into the list; `Enter` in the search box runs immediately and moves focus
to the list. Pointer hover tracks by `mousemove`, not `mouseenter`, so keyboard-driven
scrolling never steals the hover.

### Stack mapping

| Sketch piece | Real stack |
|---|---|
| Hover card | shadcn-svelte `HoverCard` (bits-ui `LinkPreview`): set `openDelay={250}` (default 700) and `closeDelay={120}` (default 300). Because keyboard focus stays on the listbox, drive `open` as controlled state and anchor the content to the active row element (bits-ui floating content `customAnchor`; verify against bits-ui 2.x) so `j`/`k` can open it instantly |
| Detail pane | `paneforge` via shadcn-svelte `Resizable` (`PaneGroup direction="horizontal"`, a `collapsible` right `Pane` with `minSize`, `collapse()`/`expand()` through a bound ref for toggle-close, `keyboardResizeBy` for arrow-key resizing). Use a **static** `autoSaveId` per page |
| Header search dropdown | bits-ui `Command.Root shouldFilter={false}` inside a `Popover` anchored to the input; items driven from the server response; give every `Command.Item` a unique `value` (id-based) |
| Results list | custom listbox component (no shadcn primitive fits dense rows); TanStack Query with `placeholderData: keepPreviousData` and the full normalized query (`kind`, text, scopes, tags, categories, cross_spine, include_*) in the query key |
| Facet chips | `Toggle`/`Badge`-styled buttons; scope picker = `DropdownMenu` or `Popover` + `Command` fed by `ListScopes` |
| Toasts | svelte-sonner (copy confirmations, text-size changes) |
| Classifier | one shared TS module used by the header box, `/search`, and any other id-accepting input |

`@tanstack/svelte-virtual` is not turnkey on Svelte 5 (open issue); the sketch renders every
row, and pagination via `cursor` is preferred over virtualising huge result sets.

## CSS Patterns

```css
/* grid row: column template is one variable so container queries can swap it */
.list { --cols: calc(96*var(--u)) minmax(0,1fr) auto calc(172*var(--u)) calc(132*var(--u)) calc(34*var(--u)) calc(70*var(--u)); }
.list.jev { --cols-x: calc(56*var(--u)); }
.row .line { display: grid; grid-template-columns: var(--cols); align-items: center; column-gap: calc(10*var(--u));
  height: var(--row-h); padding: 0 calc(12*var(--u)) 0 calc(14*var(--u)); }
.list.jev .row .line { grid-template-columns: var(--cols) var(--cols-x); }
.row:hover { background: var(--color-hover); }
.row.active { background: var(--color-selected); }
.row.active::before { content: ''; position: absolute; inset: 0 auto 0 0; width: 2px; background: var(--color-text-faint); }
.list:focus .row.active::before { background: var(--color-primary); }
.row.opened { background: color-mix(in srgb, var(--color-primary) 10%, var(--color-selected));
  box-shadow: inset 3px 0 0 var(--color-primary); }

@container list (max-width: 860px) { .line .tags { display: none; } }
@container list (max-width: 560px) { .line .tags, .line .scope { display: none; } .cat { font-size: 0; } .score .bar { display: none; } }

/* score: mono number + tiny bar */
.score { display: flex; align-items: center; gap: calc(6*var(--u)); justify-content: flex-end; }
.score .num { font-family: var(--font-mono); font-size: var(--text-2xs); color: var(--color-text-muted); }
.score .bar { width: calc(28*var(--u)); height: calc(3*var(--u)); border-radius: 2px; background: var(--color-border-subtle); overflow: hidden; }
.score .bar b { display: block; height: 100%; background: var(--color-primary); }

/* hover card */
.hcard { position: fixed; z-index: 50; width: calc(480*var(--u)); max-width: calc(100vw - 24px);
  background: var(--color-surface-2); border: 1px solid var(--color-border); border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg); padding: calc(10*var(--u)) calc(12*var(--u)) calc(12*var(--u));
  opacity: 0; transform: translateY(-3px); transition: opacity .15s ease, transform .15s ease; pointer-events: none; }
.hcard.show { opacity: 1; transform: none; pointer-events: auto; }
.hcard .hc-prev { font-size: var(--text-xs); color: var(--color-text-muted); white-space: pre-line;
  display: -webkit-box; -webkit-line-clamp: 6; -webkit-box-orient: vertical; overflow: hidden; }

/* pane collapse animates width; content keeps a min width so it does not reflow while closing */
.detail { transition: width .15s ease; }
.detail > * { min-width: calc(280*var(--u)); }
.v-body:not(.open) .detail { width: 0 !important; overflow: hidden; visibility: hidden; transition: width .15s ease, visibility 0s .15s; }
.v-body.dragging .detail { transition: none; }
.resizer::after { content: ''; position: absolute; inset: 0 auto 0 2px; width: 1px; background: var(--color-border); }
.resizer:is(:hover, .drag, :focus-visible)::after { background: var(--color-primary); width: calc(3*var(--u)); left: 1px; }

/* header search + interpreted-as chips */
.hd-input:focus-within { border-color: var(--color-primary); box-shadow: 0 0 0 calc(3*var(--u)) var(--color-primary-soft); }
.chip { height: calc(20*var(--u)); border-radius: var(--radius-full); border: 1px solid var(--color-border);
  background: var(--color-surface-2); font: var(--text-xs)/1 var(--font-mono); }
.chip.kind { border-color: var(--color-primary); background: var(--color-primary-soft); }  /* uuid / short_id / text */
.chip.pending { border-style: dashed; background: none; color: var(--color-text-muted); }   /* unfinished operator */
.chip.ghost { border-style: dashed; background: none; color: var(--color-text-faint); }     /* "+ cross-spine" */
.chip.bad { border-color: var(--color-danger); color: var(--color-danger); }                /* unknown scope/category */
.envelope { font: var(--text-xs)/1.5 var(--font-mono); border: 1px solid var(--color-danger);
  color: var(--color-danger); background: var(--color-surface-2); border-radius: var(--radius-sm); white-space: pre-wrap; }
.list.stale .item.mem { opacity: .55; }   /* previous results while re-querying */
```

## HTML Structures

```html
<div class="v-head">
  <label class="search"><svg/><input class="q" aria-label="Search memories"><kbd>/</kbd></label>
  <div class="rhead" aria-live="polite">
    <strong>12 hits</strong> across <button class="link">3 scopes</button> for <code>recall gate</code>
    · ranked by cosine · <span title="1 archived, 1 superseded">2 hidden by recall gate</span>
  </div>
  <div class="legend"><kbd>j</kbd><kbd>k</kbd> move · <kbd>↵</kbd> open / close · <kbd>esc</kbd> close</div>
  <button class="aa">Aa</button><span class="avatar">sb</span>
</div>
<div class="facets">…category chips · scope ▾ · cross_spine · include_* · jev rerank…</div>
<div class="scopes-line" hidden>searched_scopes: <span class="sc">repo:…<b>4</b></span> …</div>
<div class="v-body open">
  <div class="list-wrap"><div class="loadbar"></div>
    <div class="list" role="listbox" tabindex="0" aria-label="Search results" aria-activedescendant="opt-3">
      <div class="row dim opened" role="option" id="opt-3" aria-selected="true">
        <div class="line">
          <span class="cat" style="--c:var(--cat-gotcha)"><i></i>gotcha</span>
          <span class="sum">gofmt drift is NOT caught by <code>golangci-lint</code></span>
          <span class="states"><span class="st superseded">superseded</span></span>
          <span class="tags"><span class="tag">ci</span><span class="tag">gofmt</span><span class="tag more">+2</span></span>
          <span class="scope"><em>repo·</em>engram</span>
          <span class="age">12d</span>
          <span class="score"><span class="num">0.81</span><span class="bar"><b style="width:81%"></b></span></span>
          <span class="rel"><em>rel</em>0.77</span>
        </div>
      </div>
    </div>
  </div>
  <div class="resizer" role="separator" aria-orientation="vertical" aria-label="Resize detail pane"
       tabindex="0" aria-valuemin="280" aria-valuemax="900" aria-valuenow="440"></div>
  <aside class="detail" tabindex="-1" aria-label="Memory detail">…</aside>
</div>
<div class="hcard" role="tooltip">…</div>

<!-- header search (002 B) -->
<div class="hd-search">
  <div class="hd-input">⌕<input placeholder="Search, paste an id, scope: #tag is:"><kbd>/</kbd></div>
  <div class="dropdown open">
    <div class="progress on"></div>
    <div class="chips"><span class="lbl">interpreted as</span>
      <span class="chip kind"><span class="t">text "github"</span><button aria-label="remove">✕</button></span>
      <span class="chip"><span class="t">cross-spine</span><button aria-label="remove">✕</button></span>
    </div>
    <div class="status"><span class="faint">previous results ·</span><span class="hl"><b>10 hits</b> across <b>3 scopes</b></span>
      <span class="note">· +1 superseded match hidden by the recall gate (fetch by id)</span>
      <button class="src">SearchMemories · cross_spine · 3 scopes searched ▾</button></div>
    <div class="list">
      <div class="item full active">⌕ Search all memories for "github" · 10 hits · <kbd>↵</kbd> /search</div>
      <div class="sec"><div class="sec-h"><span>Memories</span><span class="mono">SearchMemories · cross_spine</span></div>…</div>
      <div class="sec"><div class="sec-h"><span>Scopes</span><span class="mono">ListScopes</span></div>…</div>
    </div>
    <div class="pal-foot"><kbd>↵</kbd> on text opens /search · <kbd>⇥</kbd> section · <kbd>esc</kbd> close</div>
  </div>
</div>
```

## What to Avoid

- **Inline push-down expansion (001 A).** Growing the hovered row pushes every row below it
  down, so rows move under a still pointer and under your eye while scanning with `j`/`k`; it
  needed a 90ms hover delay and a second scroll correction and still felt jumpy at 30 rows.
- **Two-line rows with a permanent snippet (001 C).** Fits about half as many rows, and the
  50/50 split plus follow-focus pane means the detail changes on every keystroke. Recall wants a
  pinned pane you open on purpose.
- **A pane that can only be closed with `Esc` or `✕`** (plain 001 B). A second click on the open
  row closing it was the one change that made B the winner.
- **Hover card on the row already open in the pane.** Redundant; suppress it.
- **⌘K modal palette (002 A).** It covers the page you are working on, and a centred modal is a
  context switch for "developer recalling a fact"; its Enter-runs-active-row model also made
  "search all" a buried command. The bits-ui `Command` primitive is still used, just not in a
  modal.
- **Full-page takeover (002 C).** Heavy for a quick lookup, `Esc` must restore page and scroll,
  and its big surface handled stale-while-loading no better than the dropdown.
- **A client-side filtered palette.** The live bug: bits-ui `Command` filtered items by label,
  the "Search memories for …" item snapshotted its label while empty and filtered itself out,
  so `github` showed "no matches" without asking the server. Always `shouldFilter={false}`.
- **Silent scope failures.** With no scope chosen the server rejects under
  `scope-required-unless-cross-spine`; default to `cross_spine` and, if removed, show the
  envelope and fix rows, never a blank list.
- **Silently reinterpreting input.** A short_id-shaped word that misses must say it was
  searched as text instead.
- **Blending score and relevance, or hiding records without saying so.** The header counts what
  the recall gate hid; silent hiding reads as data loss.

## Origin

Synthesized from sketches: 001 (winner B′), 002 (winner B).
Source files available in: `sources/001-results-surface/index.html`,
`sources/002-command-palette/index.html`.
